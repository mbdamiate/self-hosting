package hostready

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"vmctl/internal/execrunner"
)

var cleanupPackages = []string{
	"qemu-system-x86", "qemu-utils", "libvirt-daemon-system", "libvirt-clients",
	"bridge-utils", "virtinst", "cloud-image-utils", "genisoimage",
}

// Unfix reverts everything Fix establishes: it removes the libvirt
// 'default' network, purges the packages, removes libvirt/kvm group
// membership, and revokes the QEMU storage ACL on $HOME. It is the
// relocated body of what `vmctl delete --purge-all` used to do, and
// refuses to run while any VM is still defined on the host (a stricter
// guard than `--purge-all`'s "no VM other than mine", since Unfix has no
// VM of its own).
func Unfix(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	if err := checkApt(); err != nil {
		return err
	}
	if err := refuseIfAnyVMExists(ctx, r); err != nil {
		return err
	}

	removeDefaultNetwork(ctx, r, out)
	removePackages(ctx, r, out)
	removeGroups(ctx, r, out)
	revokeACL(ctx, r, out)
	return nil
}

func refuseIfAnyVMExists(ctx context.Context, r execrunner.Runner) error {
	output, _ := r.Run(ctx, "virsh", "list", "--all", "--name")
	var vms []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			vms = append(vms, line)
		}
	}
	if len(vms) == 0 {
		return nil
	}
	return fmt.Errorf(`refusing to remove host prerequisites while VMs still exist: %s
       Remove each one first with: vmctl delete --name=<name> --vm-only`, strings.Join(vms, ", "))
}

func removeDefaultNetwork(ctx context.Context, r execrunner.Runner, out io.Writer) {
	if _, err := r.Run(ctx, "virsh", "net-info", "default"); err != nil {
		fmt.Fprintln(out, "==> No 'default' libvirt network found, skipping.")
		return
	}
	_, _ = r.Run(ctx, "virsh", "net-destroy", "default")
	_, _ = r.Run(ctx, "virsh", "net-undefine", "default")
	fmt.Fprintln(out, "==> 'default' network removed.")
}

func removePackages(ctx context.Context, r execrunner.Runner, out io.Writer) {
	fmt.Fprintln(out, "==> Stopping the libvirtd service...")
	_, _ = r.Run(ctx, "sudo", "systemctl", "stop", "libvirtd")
	_, _ = r.Run(ctx, "sudo", "systemctl", "disable", "libvirtd")

	fmt.Fprintln(out, "==> Removing packages...")
	purgeArgs := append([]string{"apt", "purge", "-y"}, cleanupPackages...)
	_, _ = r.Run(ctx, "sudo", purgeArgs...)
	_, _ = r.Run(ctx, "sudo", "apt", "autoremove", "-y")
	fmt.Fprintln(out, "    Packages removed.")
}

func removeGroups(ctx context.Context, r execrunner.Runner, out io.Writer) {
	user := currentUsername()
	_, _ = r.Run(ctx, "sudo", "gpasswd", "-d", user, "libvirt")
	_, _ = r.Run(ctx, "sudo", "gpasswd", "-d", user, "kvm")
	fmt.Fprintln(out, "==> Group membership removed (full effect only after logout/login).")
}

func revokeACL(ctx context.Context, r execrunner.Runner, out io.Writer) {
	home, _ := os.UserHomeDir()
	if _, err := r.Run(ctx, "sudo", "setfacl", "-x", "u:libvirt-qemu", home); err != nil {
		fmt.Fprintln(out, "WARNING: could not revoke the 'libvirt-qemu' ACL entry on $HOME (it may not exist, or the filesystem may not support ACLs).")
		return
	}
	fmt.Fprintln(out, "==> ACL entry removed.")
}
