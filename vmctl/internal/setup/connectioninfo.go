package setup

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"vmctl/internal/execrunner"
	"vmctl/internal/metadata"
	"vmctl/internal/virshparse"
)

// waitForVMIP mirrors section 12's polling loop, with the NAT DHCP-lease
// fallback if domifaddr hasn't reported an address yet.
func waitForVMIP(ctx context.Context, r execrunner.Runner, vmName, effectiveMode string) string {
	var ip string
	for i := 0; i < 30; i++ {
		out, err := r.Run(ctx, "virsh", "domifaddr", vmName)
		if err == nil {
			ip = virshparse.DomifaddrIPv4(string(out))
		}
		if ip != "" {
			return ip
		}
		time.Sleep(2 * time.Second)
	}
	if ip == "" && effectiveMode == "nat" {
		leases, _ := r.Run(ctx, "virsh", "net-dhcp-leases", "default")
		ip = virshparse.DHCPLeaseIP(string(leases), vmName)
	}
	return ip
}

// applyPortForwarding mirrors section 13: NAT + --forward only.
func applyPortForwarding(ctx context.Context, r execrunner.Runner, out io.Writer, vmName, forwardRules, effectiveMode, vmIP, workDir, vmUserName string) {
	if forwardRules == "" {
		return
	}
	if effectiveMode == "bridged" {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "WARNING: --forward was requested, but VM '%s' uses bridged networking.\n", vmName)
		fmt.Fprintln(out, "         Port forwarding only applies to the NAT network; skipping.")
		return
	}
	if vmIP == "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "WARNING: could not detect the VM's IP automatically, so port forwarding")
		fmt.Fprintln(out, "         rules were NOT applied. Once the VM is up, get its IP with:")
		fmt.Fprintf(out, "           virsh domifaddr %s\n", vmName)
		fmt.Fprintln(out, "         and add rules manually, e.g. for host port 2222 -> VM port 22:")
		fmt.Fprintln(out, "           sudo iptables -t nat -A PREROUTING -p tcp --dport 2222 -j DNAT --to-destination <VM_IP>:22")
		fmt.Fprintln(out, "           sudo iptables -I FORWARD -p tcp -d <VM_IP> --dport 22 -j ACCEPT")
		return
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "==> Applying port forwarding rules (host -> %s)...\n", vmIP)
	rules := strings.Split(forwardRules, ",")
	for _, rule := range rules {
		parts := strings.SplitN(rule, ":", 2)
		if len(parts) != 2 {
			continue
		}
		hostPort, vmPort := parts[0], parts[1]
		dest := fmt.Sprintf("%s:%s", vmIP, vmPort)
		if _, err := r.Run(ctx, "sudo", "iptables", "-t", "nat", "-C", "PREROUTING", "-p", "tcp", "--dport", hostPort, "-j", "DNAT", "--to-destination", dest); err == nil {
			fmt.Fprintf(out, "    %s -> %s (DNAT rule already present, skipping)\n", hostPort, dest)
		} else {
			fmt.Fprintf(out, "    %s -> %s\n", hostPort, dest)
			_, _ = r.Run(ctx, "sudo", "iptables", "-t", "nat", "-A", "PREROUTING", "-p", "tcp", "--dport", hostPort, "-j", "DNAT", "--to-destination", dest)
		}
		if _, err := r.Run(ctx, "sudo", "iptables", "-C", "FORWARD", "-p", "tcp", "-d", vmIP, "--dport", vmPort, "-j", "ACCEPT"); err != nil {
			_, _ = r.Run(ctx, "sudo", "iptables", "-I", "FORWARD", "-p", "tcp", "-d", vmIP, "--dport", vmPort, "-j", "ACCEPT")
		}
	}

	meta, _ := metadata.Load(workDir)
	if meta.GuestFirewallPolicy == "enabled" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "WARNING: this VM's guest firewall (ufw) was enabled at creation, but its allow")
		fmt.Fprintln(out, "         list can't be updated by rerunning this (cloud-init only applies")
		fmt.Fprintln(out, "         at first boot). The newly forwarded port(s) may still be blocked inside")
		fmt.Fprintln(out, "         the guest. Allow them manually, e.g. over SSH:")
		for _, rule := range rules {
			parts := strings.SplitN(rule, ":", 2)
			if len(parts) != 2 {
				continue
			}
			fmt.Fprintf(out, "           ssh %s@%s sudo ufw allow %s/tcp\n", vmUserName, vmIP, parts[1])
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "NOTE: these iptables rules are NOT persistent across host reboots.")
	fmt.Fprintln(out, "      To make them permanent, install 'iptables-persistent' (apt) and save with")
	fmt.Fprintln(out, "      'sudo netfilter-persistent save', or re-run this with the same")
	fmt.Fprintln(out, "      --forward flag after a reboot.")
	fmt.Fprintln(out, "      Also note: if the VM's DHCP lease changes, these rules will point to the")
	fmt.Fprintf(out, "      wrong IP — check with 'virsh domifaddr %s' if forwarding stops working.\n", vmName)
}

// printConnectionSummary mirrors sections 13.1 onward: the admin password
// reveal (only for a freshly created VM), and the final connect/useful
// commands block, always reached whether the VM was created or reused.
func printConnectionSummary(out io.Writer, opts Options, vmExists bool, workDir string, created createdVMInfo, eff effectiveConfig, vmIP string) {
	finalAdminSudoPolicy := created.AdminSudoPolicy
	if vmExists {
		finalAdminSudoPolicy = eff.AdminSudo
	}

	if !vmExists && created.AdminPasswordShown != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "==================================================")
		fmt.Fprintln(out, " Admin sudo password (shown once, also saved to")
		fmt.Fprintf(out, " %s, chmod 600)\n", filepath.Join(workDir, "admin-password"))
		fmt.Fprintln(out, "==================================================")
		fmt.Fprintf(out, " %s\n", created.AdminPasswordShown)
		fmt.Fprintln(out, "==================================================")
	}

	if finalAdminSudoPolicy == "password-required" {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "NOTE: sudo on this VM requires the password above. If it's lost, 'virsh console\n")
		fmt.Fprintf(out, "      %s' gives host-root guest access independent of SSH/sudo, which can\n", opts.Name)
		fmt.Fprintf(out, "      reset it: run 'passwd %s' from the console (Ctrl+] to exit).\n", vmUser)
	}

	effectiveMode := eff.NetworkMode
	fmt.Fprintln(out)
	switch {
	case effectiveMode == "bridged":
		fmt.Fprintf(out, "Bridged mode (via '%s'): to find the VM's IP (via qemu-guest-agent), run:\n", eff.BridgeIface)
		fmt.Fprintf(out, "  virsh domifaddr %s --source agent\n", opts.Name)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Then connect via SSH:")
		fmt.Fprintf(out, "  ssh %s@<VM_IP>\n", vmUser)
	case opts.ForwardRules != "":
		fmt.Fprintln(out, "From other devices on your LAN, connect via the host's IP and the forwarded port, e.g.:")
		fmt.Fprintf(out, "  ssh -p <HOST_PORT> %s@<HOST_IP>\n", vmUser)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "From this host itself, you can still connect directly to the NAT IP:")
		ipOrPlaceholder := vmIP
		if ipOrPlaceholder == "" {
			ipOrPlaceholder = "<VM_IP>"
		}
		fmt.Fprintf(out, "  ssh %s@%s\n", vmUser, ipOrPlaceholder)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "To find (or re-check) the VM's IP, run:")
		fmt.Fprintf(out, "  virsh domifaddr %s\n", opts.Name)
	default:
		fmt.Fprintln(out, "To find the VM's IP, run:")
		fmt.Fprintf(out, "  virsh domifaddr %s\n", opts.Name)
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Then connect via SSH (from this host):")
		fmt.Fprintf(out, "  ssh %s@<VM_IP>\n", vmUser)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Useful commands:")
	fmt.Fprintln(out, "  virsh list --all                # list VMs")
	fmt.Fprintf(out, "  virsh console %s        # view console (Ctrl+] to exit)\n", opts.Name)
	fmt.Fprintf(out, "  virsh shutdown %s       # power off\n", opts.Name)
	fmt.Fprintf(out, "  virsh start %s          # power on\n", opts.Name)
	fmt.Fprintf(out, "  virsh undefine %s --remove-all-storage   # delete everything\n", opts.Name)
	fmt.Fprintln(out)
}
