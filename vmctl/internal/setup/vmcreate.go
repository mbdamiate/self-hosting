package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"vmctl/internal/execrunner"
	"vmctl/internal/metadata"
)

// createdVMInfo carries state produced while creating a brand-new VM that
// the rest of Run needs afterward (e.g. for the connection-info summary).
type createdVMInfo struct {
	AdminSudoPolicy    string
	AdminPasswordShown string
	GuestFirewallPorts string
	GuestFWEnabled     bool
	LogForwardingSet   bool
	VirBr0IP           string
}

// createVM mirrors sections 5.1-9: everything that only happens the first
// time a VM is created (IP reservation, SSH key, disk image, cloud-init,
// virt-install). Skipped entirely when the VM already exists.
func createVM(ctx context.Context, r execrunner.Runner, out io.Writer, opts Options, workDir, virbr0IP string) (createdVMInfo, error) {
	vmName := opts.Name
	vmHostname := opts.Name
	info := createdVMInfo{VirBr0IP: virbr0IP}

	var reservedMAC string
	if opts.BridgeIface == "" {
		fmt.Fprintln(out, "==> Resolving a static IP/hostname reservation on the 'default' network...")
		netXML, _ := r.Run(ctx, "virsh", "net-dumpxml", "default")
		clearStaleReservation(ctx, r, string(netXML), vmHostname)
		// Re-read after clearing, since the stale entry could shadow a free check.
		netXML, _ = r.Run(ctx, "virsh", "net-dumpxml", "default")

		var resolvedIP string
		var err error
		if opts.StaticIP != "" {
			resolvedIP, err = resolveStaticIP(ctx, r, string(netXML), opts.StaticIP)
		} else {
			resolvedIP, err = autoPickIP(ctx, r, string(netXML))
		}
		if err != nil {
			return info, err
		}
		fmt.Fprintf(out, "    OK: %s is free.\n", resolvedIP)

		reservedMAC = generateMAC(string(netXML))
		fmt.Fprintf(out, "==> Reserving %s for '%s' (mac %s) on the 'default' network...\n", resolvedIP, vmHostname, reservedMAC)
		if err := registerReservation(ctx, r, resolvedIP, reservedMAC, vmHostname); err != nil {
			return info, fmt.Errorf("failed to register the DHCP host reservation on the 'default' network")
		}
		fmt.Fprintln(out, "    OK: reservation registered.")
	}

	sshKeyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
	if _, err := os.Stat(sshKeyPath + ".pub"); err != nil {
		fmt.Fprintf(out, "==> No SSH key found at %s.pub\n", sshKeyPath)
		fmt.Fprintln(out, "==> Generating a new key pair...")
		if _, err := r.Run(ctx, "ssh-keygen", "-t", "ed25519", "-f", sshKeyPath, "-N", "", "-C", fmt.Sprintf("%s@%s", vmUser, vmHostname)); err != nil {
			return info, err
		}
	}
	sshPubKeyBytes, err := os.ReadFile(sshKeyPath + ".pub")
	if err != nil {
		return info, fmt.Errorf("could not read %s.pub", sshKeyPath)
	}
	sshPubKey := trimNewline(string(sshPubKeyBytes))

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return info, err
	}

	imgFile := filepath.Join(workDir, "debian-12-generic-amd64.qcow2")
	vmDisk := filepath.Join(workDir, vmName+".qcow2")

	if fi, err := os.Stat(imgFile); err != nil || fi.Size() == 0 {
		fmt.Fprintln(out, "==> Downloading the official Debian 12 cloud image...")
		if _, err := r.Run(ctx, "wget", "-O", imgFile, cloudImgURL); err != nil {
			return info, err
		}
	} else {
		fmt.Fprintln(out, "==> Cloud image already downloaded, skipping.")
	}

	diskGB := opts.DiskGB
	if diskGB == 0 {
		diskGB = DefaultDiskGB
	}
	fmt.Fprintf(out, "==> Creating the VM disk (copy from the base image) and resizing to %dG...\n", diskGB)
	if err := copyFile(imgFile, vmDisk); err != nil {
		return info, err
	}
	if _, err := r.Run(ctx, "qemu-img", "resize", vmDisk, fmt.Sprintf("%dG", diskGB)); err != nil {
		return info, err
	}

	sudo, err := configureAdminSudo(ctx, r, opts.AdminPasswordRequested, opts.AdminPasswordValue)
	if err != nil {
		return info, err
	}
	info.AdminSudoPolicy = sudo.Policy
	info.AdminPasswordShown = sudo.PlaintextShown
	if sudo.PlaintextShown != "" {
		fmt.Fprintf(out, "==> Configuring password-required sudo for '%s'...\n", vmUser)
		passwordFile := filepath.Join(workDir, "admin-password")
		if err := os.WriteFile(passwordFile, []byte(sudo.PlaintextShown+"\n"), 0o600); err != nil {
			return info, err
		}
		fmt.Fprintln(out, "    OK: password configured (will be shown once when setup finishes).")
	}
	builder := newCloudInitBuilder()
	if !opts.NoAutoUpdates {
		fmt.Fprintln(out, "==> Enabling automatic security updates (unattended-upgrades)...")
		builder.addUnattendedUpgrades()
	}

	guestFWPolicy := "disabled"
	if !opts.NoGuestFirewall {
		guestFWPolicy = "enabled"
		ports := guestFirewallPorts(opts.AllowPorts, opts.ForwardRules)
		info.GuestFirewallPorts = ports
		info.GuestFWEnabled = true
		fmt.Fprintf(out, "==> Enabling guest firewall (ufw), allowed ports: %s...\n", ports)
		builder.addGuestFirewall(ports)
	}
	fmt.Fprintln(out, "==> Enabling fail2ban's sshd jail (always on)...")
	builder.addFail2ban()

	logForwardConfigured := false
	if opts.Monitor && opts.BridgeIface == "" && virbr0IP != "" {
		fmt.Fprintf(out, "==> Configuring guest log forwarding to %s:%s...\n", virbr0IP, monitorLogPort)
		builder.addLogForwarding(virbr0IP)
		logForwardConfigured = true
		info.LogForwardingSet = true
	}

	if err := metadata.Save(workDir, metadata.Record{
		AdminSudoPolicy:     sudo.Policy,
		LogForwarding:       logForwardConfigured,
		GuestFirewallPolicy: guestFWPolicy,
	}); err != nil {
		return info, err
	}

	if opts.Watchdog {
		fmt.Fprintln(out, "==> Configuring the guest to pet the virtual watchdog (systemd)...")
		builder.addWatchdogPetting()
	}

	fmt.Fprintln(out, "==> Generating cloud-init configuration...")
	userData, metaData := builder.render(vmName, vmHostname, vmUser, sudo.SudoEntry, sudo.PasswdBlock, sshPubKey)
	if err := os.WriteFile(filepath.Join(workDir, "user-data"), []byte(userData), 0o644); err != nil {
		return info, err
	}
	if err := os.WriteFile(filepath.Join(workDir, "meta-data"), []byte(metaData), 0o644); err != nil {
		return info, err
	}

	fmt.Fprintln(out, "==> Generating the cloud-init seed ISO...")
	seedISO := filepath.Join(workDir, "seed.iso")
	if _, err := r.Run(ctx, "cloud-localds", seedISO, filepath.Join(workDir, "user-data"), filepath.Join(workDir, "meta-data")); err != nil {
		return info, err
	}

	var networkArg string
	if opts.BridgeIface != "" {
		fmt.Fprintf(out, "==> Creating the VM (bridged networking via macvtap over %s)...\n", opts.BridgeIface)
		networkArg = fmt.Sprintf("type=direct,source=%s,source_mode=bridge,model=virtio", opts.BridgeIface)
	} else {
		fmt.Fprintln(out, "==> Creating the VM (NAT networking via virbr0)...")
		networkArg = fmt.Sprintf("network=default,model=virtio,mac=%s", reservedMAC)
	}

	ramMB := opts.RAMMB
	if ramMB == 0 {
		ramMB = DefaultRAMMB
	}
	vcpus := opts.VCPUs
	if vcpus == 0 {
		vcpus = DefaultVCPUs
	}

	virtInstallArgs := []string{
		"--name", vmName,
		"--memory", fmt.Sprintf("%d", ramMB),
		"--vcpus", fmt.Sprintf("%d", vcpus),
		"--disk", fmt.Sprintf("path=%s,format=qcow2", vmDisk),
		"--disk", fmt.Sprintf("path=%s,device=cdrom", seedISO),
		"--os-variant", "debian12",
		"--network", networkArg,
		"--graphics", "none",
		"--import",
		"--noautoconsole",
	}
	if opts.Watchdog {
		fmt.Fprintln(out, "==> Attaching a virtual watchdog device (i6300esb, action=reset)...")
		virtInstallArgs = append(virtInstallArgs, "--watchdog", "model=i6300esb,action=reset")
	}
	if !opts.NoCrashRestart {
		fmt.Fprintln(out, "==> Enabling automatic restart on QEMU crash (on_crash=restart)...")
		virtInstallArgs = append(virtInstallArgs, "--events", "on_crash=restart")
	}

	if _, err := r.Run(ctx, "virt-install", virtInstallArgs...); err != nil {
		return info, err
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, " VM '%s' created successfully!\n", vmName)
	fmt.Fprintln(out, "==================================================")

	if !opts.NoAutoUpdates {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "NOTE: security-origin updates will be applied automatically inside the VM")
		fmt.Fprintln(out, "      (unattended-upgrades). Kernel/library updates that require a reboot do")
		fmt.Fprintln(out, "      NOT reboot automatically — check and reboot manually when needed.")
	}
	if guestFWPolicy == "enabled" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "NOTE: the guest firewall (ufw) is active, default-deny inbound. Allowed TCP")
		fmt.Fprintf(out, "      ports: %s. Use --allow-port=PORT[,PORT...] to open more\n", info.GuestFirewallPorts)
		fmt.Fprintln(out, "      at creation time. fail2ban's sshd jail is also active.")
	}
	if opts.Monitor {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "NOTE: uptime monitoring is active for '%s' (checks every ~2 minutes).\n", vmName)
		fmt.Fprintln(out, "      Alerts go to the host journal (journalctl -t self-hosting-alert), a wall")
		fmt.Fprintln(out, "      broadcast, and the host's login banner — no remote/email notification.")
		if info.LogForwardingSet {
			fmt.Fprintf(out, "      Logs are forwarded to /var/log/self-hosting-vms/%s/ on the host.\n", vmHostname)
		} else if opts.BridgeIface != "" {
			fmt.Fprintln(out, "      Centralized logging is NOT available in bridged mode (macvtap isolation);")
			fmt.Fprintln(out, "      only uptime monitoring applies.")
		}
	}

	return info, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
