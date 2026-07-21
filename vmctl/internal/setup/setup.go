package setup

import (
	"context"
	"fmt"
	"io"

	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
)

// Run performs `vmctl setup`: create a new VM, or safely reuse one that
// already exists, per vm-fleet-provisioning and vm-setup-rerun-recovery.
func Run(ctx context.Context, r execrunner.Runner, out io.Writer, opts Options) error {
	if opts.BridgeIface != "" && opts.ForwardRules != "" {
		return fmt.Errorf("--bridge and --forward are mutually exclusive (forwarding only makes sense on NAT)")
	}
	if opts.BridgeIface != "" && opts.StaticIP != "" {
		return fmt.Errorf(`--ip and --bridge are mutually exclusive (bridged VMs get their address from
       your router's DHCP, not libvirt's 'default' network, so there is nothing to reserve)`)
	}

	if opts.BridgeIface != "" {
		fmt.Fprintf(out, "==> Checking if interface '%s' exists and is wired...\n", opts.BridgeIface)
		if err := validateBridgeIface(ctx, r, opts.BridgeIface); err != nil {
			return err
		}
		fmt.Fprintf(out, "    OK: '%s' is a wired interface, using bridged mode (macvtap).\n", opts.BridgeIface)
	} else if opts.ForwardRules != "" {
		fmt.Fprintf(out, "==> Using NAT networking with port forwarding: %s\n", opts.ForwardRules)
	} else {
		fmt.Fprintln(out, "==> No --bridge or --forward given, using plain NAT (virbr0).")
	}

	fmt.Fprintln(out, "==> Checking for hardware virtualization support...")
	if err := checkHardwareVirtualization(); err != nil {
		return err
	}
	fmt.Fprintln(out, "    OK: hardware virtualization supported.")

	if err := cli.RequireNonRoot(); err != nil {
		return err
	}

	fmt.Fprintln(out, "==> Checking if the system is Debian/Ubuntu-based (apt)...")
	if err := checkApt(); err != nil {
		return err
	}
	fmt.Fprintln(out, "    OK.")

	if err := installPrerequisites(ctx, r, out); err != nil {
		return err
	}

	target := cli.ResolveTarget(opts.Name)

	if opts.BridgeIface == "" {
		fmt.Fprintln(out, "==> Checking the libvirt 'default' network (required for NAT networking)...")
		if err := ensureNATNetworkReady(ctx, r, out); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(out, "==> Bridged mode selected: leaving the 'default' network untouched.")
	}

	if err := grantQEMUStorageACL(ctx, r, out); err != nil {
		return err
	}

	if opts.HardenHostFirewall {
		if err := hardenHostFirewall(ctx, r, out); err != nil {
			return err
		}
	}

	var virbr0IP string
	if opts.Monitor {
		ip, err := installMonitoringInfra(ctx, r, out)
		if err != nil {
			return err
		}
		virbr0IP = ip
	} else {
		virbr0IP = detectVirbr0IP(ctx, r)
	}
	ensureLogReceiverFirewallRule(ctx, r, out)

	fmt.Fprintln(out, "==> Checking if a VM with this name already exists...")
	vmExists := false
	if _, err := r.Run(ctx, "virsh", "dominfo", opts.Name); err == nil {
		vmExists = true
	}

	var created createdVMInfo
	if !vmExists {
		fmt.Fprintf(out, "    OK: no existing VM named '%s', proceeding with setup.\n", opts.Name)
		info, err := createVM(ctx, r, out, opts, target.WorkDir, virbr0IP)
		if err != nil {
			return err
		}
		created = info
	} else {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "==================================================")
		fmt.Fprintf(out, " Reusing existing VM '%s'\n", opts.Name)
		fmt.Fprintln(out, "==================================================")
	}

	eff, err := determineEffectiveConfig(ctx, r, opts.Name, target.WorkDir)
	if err != nil {
		return fmt.Errorf("could not determine the network interface for VM '%s'. Inspect manually with: virsh domiflist %s", opts.Name, opts.Name)
	}

	if vmExists {
		if err := printRerunMismatchWarnings(ctx, r, out, opts.Name, opts, eff); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "==> Configuring '%s' to autostart on host boot...\n", opts.Name)
	if _, err := r.Run(ctx, "virsh", "autostart", opts.Name); err != nil {
		fmt.Fprintf(out, "WARNING: failed to enable autostart for VM '%s'. Retry manually with:\n", opts.Name)
		fmt.Fprintf(out, "         virsh autostart %s\n", opts.Name)
	}

	if opts.Monitor {
		fmt.Fprintf(out, "==> Enabling the uptime monitoring timer for '%s'...\n", opts.Name)
		timerUnit := fmt.Sprintf("self-hosting-vm-uptime@%s.timer", opts.Name)
		if _, err := r.Run(ctx, "sudo", "systemctl", "enable", "--now", timerUnit); err != nil {
			fmt.Fprintf(out, "WARNING: failed to enable the uptime monitoring timer for '%s'.\n", opts.Name)
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Waiting for the VM to report a DHCP IP...")
	vmIP := waitForVMIP(ctx, r, opts.Name, eff.NetworkMode)

	applyPortForwarding(ctx, r, out, opts.Name, opts.ForwardRules, eff.NetworkMode, vmIP, target.WorkDir, vmUser)

	printConnectionSummary(out, opts, vmExists, target.WorkDir, created, eff, vmIP)

	return nil
}
