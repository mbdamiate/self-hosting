// Package cleanup implements `vmctl cleanup`, porting debian-vm-cleanup.sh's
// three modes (interactive, --vm-only, --purge-all) per the vm-cleanup-scope
// spec. All virsh/systemctl/apt/ufw/sudo calls go through execrunner.Runner.
package cleanup

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
	"vmctl/internal/netxml"
)

// Options mirrors the flags debian-vm-cleanup.sh accepts.
type Options struct {
	Name     string
	VMOnly   bool
	PurgeAll bool
}

// Tools records which optional external dependencies are present, resolved
// once by the caller (the composition root) so this package stays testable
// without touching the real PATH.
type Tools struct {
	Virsh bool
	UFW   bool
}

const ufwSSHTag = "self-hosting: host SSH baseline"

// Run performs the cleanup. r executes every external command; out/in drive
// confirmation prompts and status output.
func Run(ctx context.Context, r execrunner.Runner, out io.Writer, in io.Reader, opts Options, tools Tools) error {
	if opts.VMOnly && opts.PurgeAll {
		return fmt.Errorf("--vm-only and --purge-all are mutually exclusive")
	}

	target := cli.ResolveTarget(opts.Name)
	autoApprove := opts.VMOnly || opts.PurgeAll
	confirm := func(prompt string) bool { return cli.Confirm(out, in, prompt, autoApprove) }

	if opts.PurgeAll && tools.Virsh {
		if err := refusePurgeIfOtherVMsExist(ctx, r, out, target.Name); err != nil {
			return err
		}
	}

	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out, " Cleaning up the simulated VPS environment")
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out)

	if tools.Virsh {
		removeVM(ctx, r, out, target.Name, confirm)
	} else {
		fmt.Fprintln(out, "==> virsh not found, skipping VM removal.")
	}

	// The reservation lives on the network, not the VM instance, so it can
	// be released even when the VM itself was never found above.
	if opts.VMOnly && tools.Virsh {
		releaseNetworkReservation(ctx, r, out, target.Name)
	}
	fmt.Fprintln(out)

	if !opts.VMOnly {
		removeHostFirewallHardening(ctx, r, out, tools, confirm)
		removeMonitoringInfra(ctx, r, out, confirm)
		askDeleteLogs(ctx, r, out, in)
		removeDefaultNetwork(ctx, r, out, tools, confirm)
		removeWorkDir(ctx, out, target, confirm)
		removePackages(ctx, r, out, confirm)
		removeGroups(ctx, r, out, confirm)
		revokeACL(ctx, r, out, confirm)
	}

	printFinalNotes(out, opts.VMOnly)
	return nil
}

func refusePurgeIfOtherVMsExist(ctx context.Context, r execrunner.Runner, out io.Writer, vmName string) error {
	output, _ := r.Run(ctx, "virsh", "list", "--all", "--name")
	var others []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == vmName {
			continue
		}
		others = append(others, line)
	}
	if len(others) == 0 {
		return nil
	}
	fmt.Fprintln(out, "ERROR: --purge-all refuses to run while other VMs still exist:")
	for _, o := range others {
		fmt.Fprintf(out, "  - %s\n", o)
	}
	fmt.Fprintln(out, "       Purging shared packages/network/groups would break them.")
	fmt.Fprintf(out, "       Remove each one first with: vmctl cleanup --name=<name> --vm-only\n")
	return fmt.Errorf("other VMs still exist")
}

func removeVM(ctx context.Context, r execrunner.Runner, out io.Writer, vmName string, confirm func(string) bool) {
	if _, err := r.Run(ctx, "virsh", "dominfo", vmName); err != nil {
		fmt.Fprintf(out, "==> No VM named '%s' found, skipping.\n", vmName)
		return
	}

	if !confirm(fmt.Sprintf("Remove the VM '%s' (definition + disks)?", vmName)) {
		fmt.Fprintln(out, "==> Skipping VM removal.")
		return
	}

	fmt.Fprintln(out, "==> Stopping the VM (if running)...")
	_, _ = r.Run(ctx, "virsh", "destroy", vmName)

	fmt.Fprintln(out, "==> Removing the VM definition and all associated disks...")
	if _, err := r.Run(ctx, "virsh", "undefine", vmName, "--remove-all-storage", "--nvram"); err != nil {
		if _, err := r.Run(ctx, "virsh", "undefine", vmName, "--remove-all-storage"); err != nil {
			fmt.Fprintln(out, "WARNING: could not remove it automatically. Check manually with 'virsh list --all'.")
		}
	}
	fmt.Fprintln(out, "    VM removed.")

	timerUnit := fmt.Sprintf("self-hosting-vm-uptime@%s.timer", vmName)
	if _, err := r.Run(ctx, "systemctl", "list-unit-files", "self-hosting-vm-uptime@.timer"); err == nil {
		if _, err := r.Run(ctx, "systemctl", "is-enabled", timerUnit); err == nil {
			fmt.Fprintf(out, "==> Disabling the uptime monitoring timer for '%s' (logs are preserved)...\n", vmName)
			_, _ = r.Run(ctx, "sudo", "systemctl", "disable", "--now", timerUnit)
		}
	}
}

func releaseNetworkReservation(ctx context.Context, r execrunner.Runner, out io.Writer, vmName string) {
	if _, err := r.Run(ctx, "virsh", "net-info", "default"); err != nil {
		return
	}
	fmt.Fprintf(out, "==> Releasing '%s's network reservation (if any)...\n", vmName)
	dump, _ := r.Run(ctx, "virsh", "net-dumpxml", "default")
	reservation := netxml.FindHostEntryByName(string(dump), vmName)
	if reservation == "" {
		fmt.Fprintf(out, "    No network reservation found for '%s', nothing to release.\n", vmName)
		return
	}
	if _, err := r.Run(ctx, "virsh", "net-update", "default", "delete", "ip-dhcp-host", reservation, "--live", "--config"); err != nil {
		fmt.Fprintln(out, "WARNING: could not release the network reservation. Inspect with: virsh net-dumpxml default")
		return
	}
	fmt.Fprintln(out, "    Reservation released; the IP/hostname are available for reuse.")
}

func removeHostFirewallHardening(ctx context.Context, r execrunner.Runner, out io.Writer, tools Tools, confirm func(string) bool) {
	if !tools.UFW {
		fmt.Fprintln(out, "==> ufw not installed, skipping host firewall hardening removal.")
		fmt.Fprintln(out)
		return
	}
	if !confirm("Remove the host firewall hardening this script may have added (SSH baseline rule + forward policy)?") {
		fmt.Fprintln(out, "==> Skipping host firewall hardening removal.")
		fmt.Fprintln(out)
		return
	}

	status, _ := r.Run(ctx, "sudo", "ufw", "status", "numbered")
	if ruleNum := findNumberedRuleContaining(string(status), ufwSSHTag); ruleNum != "" {
		fmt.Fprintln(out, "==> Removing host SSH baseline rule...")
		_, _ = r.Run(ctx, "sudo", "ufw", "--force", "delete", ruleNum)
	} else {
		fmt.Fprintln(out, "==> No tagged host SSH baseline rule found, nothing to remove.")
	}

	if contents, err := os.ReadFile("/etc/default/ufw"); err == nil && strings.Contains(string(contents), `DEFAULT_FORWARD_POLICY="ACCEPT"`) {
		fmt.Fprintln(out, "==> Reverting DEFAULT_FORWARD_POLICY to DROP...")
		_, _ = r.Run(ctx, "sudo", "sed", "-i",
			`s/^DEFAULT_FORWARD_POLICY="ACCEPT"/DEFAULT_FORWARD_POLICY="DROP"/`, "/etc/default/ufw")
		_, _ = r.Run(ctx, "sudo", "ufw", "reload")
	} else {
		fmt.Fprintln(out, "==> DEFAULT_FORWARD_POLICY is already DROP, nothing to revert.")
	}
	fmt.Fprintln(out, "    Done. ufw itself was left installed and enabled — only the tagged rule")
	fmt.Fprintln(out, "    and forward policy were touched.")
	fmt.Fprintln(out)
}

// findNumberedRuleContaining mirrors
// `grep -F "$tag" | grep -oP '^\[\s*\K[0-9]+' | head -n1` against
// `ufw status numbered` output.
func findNumberedRuleContaining(status, tag string) string {
	for _, line := range strings.Split(status, "\n") {
		if !strings.Contains(line, tag) {
			continue
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		end := strings.Index(line, "]")
		if end == -1 {
			continue
		}
		return strings.TrimSpace(line[1:end])
	}
	return ""
}

func removeMonitoringInfra(ctx context.Context, r execrunner.Runner, out io.Writer, confirm func(string) bool) {
	paths := []string{
		"/etc/systemd/system/self-hosting-vm-uptime@.timer",
		"/etc/rsyslog.d/60-self-hosting-vm-receiver.conf",
		"/etc/update-motd.d/95-self-hosting-alerts",
	}
	anyExists := false
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			anyExists = true
			break
		}
	}
	if !anyExists {
		fmt.Fprintln(out, "==> No monitoring/logging infrastructure found, skipping.")
		fmt.Fprintln(out)
		return
	}
	if !confirm("Remove the host-wide monitoring/logging infrastructure (timer templates, log receiver, motd script)?") {
		fmt.Fprintln(out, "==> Skipping monitoring/logging infrastructure removal.")
		fmt.Fprintln(out)
		return
	}

	fmt.Fprintln(out, "==> Removing the uptime monitoring timer/service templates...")
	_, _ = r.Run(ctx, "sudo", "rm", "-f",
		"/etc/systemd/system/self-hosting-vm-uptime@.service",
		"/etc/systemd/system/self-hosting-vm-uptime@.timer")
	_, _ = r.Run(ctx, "sudo", "systemctl", "daemon-reload")
	_, _ = r.Run(ctx, "sudo", "rm", "-f", "/usr/local/bin/self-hosting-vm-uptime-check")

	if _, err := os.Stat("/etc/rsyslog.d/60-self-hosting-vm-receiver.conf"); err == nil {
		fmt.Fprintln(out, "==> Removing the log receiver...")
		_, _ = r.Run(ctx, "sudo", "rm", "-f", "/etc/rsyslog.d/60-self-hosting-vm-receiver.conf")
		_, _ = r.Run(ctx, "sudo", "systemctl", "restart", "rsyslog")
	}

	if status, err := r.Run(ctx, "sudo", "ufw", "status", "numbered"); err == nil {
		if ruleNum := findNumberedRuleContainingSubstr(string(status), "5140/tcp"); ruleNum != "" {
			fmt.Fprintln(out, "==> Removing the log receiver's firewall rule...")
			_, _ = r.Run(ctx, "sudo", "ufw", "--force", "delete", ruleNum)
		}
	}

	_, _ = r.Run(ctx, "sudo", "rm", "-f", "/etc/logrotate.d/self-hosting-vms")
	_, _ = r.Run(ctx, "sudo", "rm", "-f", "/etc/update-motd.d/95-self-hosting-alerts")
	fmt.Fprintln(out, "    Done.")
	fmt.Fprintln(out)
}

func findNumberedRuleContainingSubstr(status, substr string) string {
	for _, line := range strings.Split(status, "\n") {
		if !strings.Contains(line, substr) {
			continue
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		end := strings.Index(line, "]")
		if end == -1 {
			continue
		}
		return strings.TrimSpace(line[1:end])
	}
	return ""
}

// askDeleteLogs always prompts (even under --purge-all), matching the one
// confirmation the bash script never bypasses.
func askDeleteLogs(ctx context.Context, r execrunner.Runner, out io.Writer, in io.Reader) {
	if _, err := os.Stat("/var/log/self-hosting-vms"); err != nil {
		return
	}
	if cli.Confirm(out, in, "Delete accumulated VM logs under /var/log/self-hosting-vms?", false) {
		_, _ = r.Run(ctx, "sudo", "rm", "-rf", "/var/log/self-hosting-vms")
		fmt.Fprintln(out, "    Logs deleted.")
	} else {
		fmt.Fprintln(out, "==> Keeping /var/log/self-hosting-vms.")
	}
	fmt.Fprintln(out)
}

func removeDefaultNetwork(ctx context.Context, r execrunner.Runner, out io.Writer, tools Tools, confirm func(string) bool) {
	if !tools.Virsh {
		return
	}
	if _, err := r.Run(ctx, "virsh", "net-info", "default"); err != nil {
		fmt.Fprintln(out, "==> No 'default' libvirt network found, skipping.")
		fmt.Fprintln(out)
		return
	}
	if !confirm("Also remove libvirt's default virtual network (virbr0/'default')?") {
		fmt.Fprintln(out, "==> Keeping the default virtual network.")
		fmt.Fprintln(out)
		return
	}
	_, _ = r.Run(ctx, "virsh", "net-destroy", "default")
	_, _ = r.Run(ctx, "virsh", "net-undefine", "default")
	fmt.Fprintln(out, "    'default' network removed.")
	fmt.Fprintln(out)
}

func removeWorkDir(ctx context.Context, out io.Writer, target cli.Target, confirm func(string) bool) {
	if _, err := os.Stat(target.WorkDir); err != nil {
		fmt.Fprintf(out, "==> Working directory '%s' not found, skipping.\n", target.WorkDir)
		fmt.Fprintln(out)
		return
	}
	fmt.Fprintln(out, "The working directory holds the base image, the VM disk, and the cloud-init files:")
	fmt.Fprintf(out, "  %s\n", target.WorkDir)
	if !confirm("Delete this directory and all its contents?") {
		fmt.Fprintln(out, "==> Skipping working directory removal.")
		fmt.Fprintln(out)
		return
	}
	_ = os.RemoveAll(target.WorkDir)
	fmt.Fprintln(out, "    Directory removed.")
	fmt.Fprintln(out)
}

var cleanupPackages = []string{
	"qemu-system-x86", "qemu-utils", "libvirt-daemon-system", "libvirt-clients",
	"bridge-utils", "virtinst", "cloud-image-utils", "genisoimage",
}

func removePackages(ctx context.Context, r execrunner.Runner, out io.Writer, confirm func(string) bool) {
	fmt.Fprintln(out, "Packages that will be removed (purge):")
	fmt.Fprintf(out, "  %s\n", strings.Join(cleanupPackages, " "))
	fmt.Fprintln(out, "WARNING: if you use KVM/libvirt for other VMs besides this one, do NOT remove the packages.")
	if !confirm("Remove these packages from the system?") {
		fmt.Fprintln(out, "==> Skipping package removal.")
		fmt.Fprintln(out)
		return
	}
	fmt.Fprintln(out, "==> Stopping the libvirtd service...")
	_, _ = r.Run(ctx, "sudo", "systemctl", "stop", "libvirtd")
	_, _ = r.Run(ctx, "sudo", "systemctl", "disable", "libvirtd")

	fmt.Fprintln(out, "==> Removing packages...")
	purgeArgs := append([]string{"apt", "purge", "-y"}, cleanupPackages...)
	_, _ = r.Run(ctx, "sudo", purgeArgs...)
	_, _ = r.Run(ctx, "sudo", "apt", "autoremove", "-y")
	fmt.Fprintln(out, "    Packages removed.")
	fmt.Fprintln(out)
}

func removeGroups(ctx context.Context, r execrunner.Runner, out io.Writer, confirm func(string) bool) {
	user := currentUser()
	if !confirm(fmt.Sprintf("Remove your user (%s) from the 'libvirt' and 'kvm' groups?", user)) {
		fmt.Fprintln(out, "==> Skipping group removal.")
		fmt.Fprintln(out)
		return
	}
	_, _ = r.Run(ctx, "sudo", "gpasswd", "-d", user, "libvirt")
	_, _ = r.Run(ctx, "sudo", "gpasswd", "-d", user, "kvm")
	fmt.Fprintln(out, "    Done (full effect only after logout/login).")
	fmt.Fprintln(out)
}

func revokeACL(ctx context.Context, r execrunner.Runner, out io.Writer, confirm func(string) bool) {
	if !confirm(`Revoke the 'libvirt-qemu' traversal ACL on $HOME? (skip if other local VMs might still need it)`) {
		fmt.Fprintln(out, "==> Keeping the 'libvirt-qemu' ACL entry on $HOME.")
		fmt.Fprintln(out)
		return
	}
	home, _ := os.UserHomeDir()
	if _, err := r.Run(ctx, "sudo", "setfacl", "-x", "u:libvirt-qemu", home); err != nil {
		fmt.Fprintln(out, "WARNING: could not revoke the 'libvirt-qemu' ACL entry on $HOME (it may not exist, or the filesystem may not support ACLs).")
		fmt.Fprintln(out)
		return
	}
	fmt.Fprintln(out, "    ACL entry removed.")
	fmt.Fprintln(out)
}

func currentUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "unknown"
}

func printFinalNotes(out io.Writer, vmOnly bool) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out, " Cleanup finished.")
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out)
	if vmOnly {
		fmt.Fprintln(out, "Notes:")
		fmt.Fprintln(out, "  - Only the VM and its attached storage were removed (its monitoring timer,")
		fmt.Fprintln(out, "    if any, was disabled too).")
		fmt.Fprintln(out, "  - The downloaded base cloud image, installed packages, group membership,")
		fmt.Fprintln(out, `    the default network, the QEMU storage ACL on $HOME, any host firewall`)
		fmt.Fprintln(out, "    hardening, the VM's logs, and any backups were left in place.")
		fmt.Fprintln(out, "  - Rerun 'vmctl setup' to recreate the VM without re-downloading or")
		fmt.Fprintln(out, "    reinstalling anything.")
	} else {
		fmt.Fprintln(out, "Notes:")
		fmt.Fprintln(out, "  - If you removed the groups, log out/in for the change to fully apply.")
		fmt.Fprintln(out, "  - Your SSH key in ~/.ssh was NOT deleted (it may be used elsewhere).")
		fmt.Fprintln(out, "  - If you skipped package removal, KVM/libvirt remain installed on the system.")
		fmt.Fprintln(out, "  - Any backups ('vmctl backup') were NOT touched — this command never deletes them.")
		fmt.Fprintln(out, "  - If you used --forward in 'vmctl setup', the iptables port-forwarding")
		fmt.Fprintln(out, "    rules were NOT removed automatically (this command can't tell which are yours).")
		fmt.Fprintln(out, "    List them with 'sudo iptables -t nat -L PREROUTING -n --line-numbers' and")
		fmt.Fprintln(out, "    'sudo iptables -L FORWARD -n --line-numbers', then remove with:")
		fmt.Fprintln(out, "      sudo iptables -t nat -D PREROUTING <line-number>")
		fmt.Fprintln(out, "      sudo iptables -D FORWARD <line-number>")
	}
	fmt.Fprintln(out)
}
