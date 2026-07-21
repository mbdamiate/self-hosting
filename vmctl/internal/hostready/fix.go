package hostready

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"vmctl/internal/execrunner"
)

var basePackages = []string{
	"qemu-system-x86", "qemu-utils", "libvirt-daemon-system", "libvirt-clients",
	"bridge-utils", "virtinst", "cloud-image-utils", "genisoimage",
	"wget", "openssh-client", "acl",
}

// Fix installs and configures every host-level prerequisite vmctl depends
// on: packages, libvirt/kvm group membership, the libvirtd service, the
// libvirt 'default' NAT network, and the QEMU storage ACL on $HOME. It is
// the relocated body of what `vmctl create` used to do unconditionally on
// every invocation.
func Fix(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	if err := checkHardwareVirtualization(); err != nil {
		return err
	}
	if err := checkApt(); err != nil {
		return err
	}

	if err := installPackagesAndGroups(ctx, r, out); err != nil {
		return err
	}
	if err := ensureNATNetworkReady(ctx, r, out); err != nil {
		return err
	}
	if err := grantQEMUStorageACL(ctx, r, out); err != nil {
		return err
	}
	return nil
}

func installPackagesAndGroups(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	fmt.Fprintln(out, "==> Installing KVM, QEMU, libvirt, and cloud-init tools...")
	if _, err := r.Run(ctx, "sudo", "apt", "update"); err != nil {
		return err
	}
	installArgs := append([]string{"apt", "install", "-y"}, basePackages...)
	if _, err := r.Run(ctx, "sudo", installArgs...); err != nil {
		return err
	}

	user := currentUsername()
	fmt.Fprintln(out, "==> Adding your user to the libvirt and kvm groups...")
	if _, err := r.Run(ctx, "sudo", "usermod", "-aG", "libvirt", user); err != nil {
		return err
	}
	if _, err := r.Run(ctx, "sudo", "usermod", "-aG", "kvm", user); err != nil {
		return err
	}

	fmt.Fprintln(out, "==> Enabling and starting the libvirtd service...")
	if _, err := r.Run(ctx, "sudo", "systemctl", "enable", "--now", "libvirtd"); err != nil {
		return err
	}

	groupsOut, _ := r.Run(ctx, "id", "-nG")
	currentGroups := strings.Fields(string(groupsOut))
	var missing []string
	for _, required := range []string{"libvirt", "kvm"} {
		found := false
		for _, g := range currentGroups {
			if g == required {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(`the current session is missing required groups: %s
       Your user was added to these groups, but the change only takes effect
       after a new login session. Log out and back in, then run 'vmctl doctor --fix' again.
       Verify with: id -nG`, strings.Join(missing, ", "))
	}
	return nil
}

func ensureNATNetworkReady(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	if _, err := r.Run(ctx, "virsh", "net-info", "default"); err != nil {
		fmt.Fprintln(out, "==> The 'default' network is not defined, defining it from the packaged template...")
		if _, defErr := r.Run(ctx, "virsh", "net-define", "/usr/share/libvirt/networks/default.xml"); defErr != nil {
			return fmt.Errorf(`failed to define the libvirt network 'default' from /usr/share/libvirt/networks/default.xml.
       Inspect available networks with: virsh net-list --all`)
		}
	}

	info, _ := r.Run(ctx, "virsh", "net-info", "default")
	active := false
	for _, line := range strings.Split(string(info), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Active:") {
			active = strings.Contains(line, "yes")
		}
	}
	if active {
		fmt.Fprintln(out, "    OK: 'default' network is already active.")
	} else {
		fmt.Fprintln(out, "==> Starting the inactive 'default' network...")
		if _, err := r.Run(ctx, "virsh", "net-start", "default"); err != nil {
			return fmt.Errorf("failed to start the libvirt 'default' network. Inspect with: virsh net-info default")
		}
	}

	fmt.Fprintln(out, "==> Enabling autostart for the 'default' network...")
	if _, err := r.Run(ctx, "virsh", "net-autostart", "default"); err != nil {
		return fmt.Errorf("failed to enable autostart for the libvirt 'default' network")
	}
	return nil
}

func grantQEMUStorageACL(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	fmt.Fprintln(out, "==> Granting the 'libvirt-qemu' service account traversal access to $HOME...")
	if _, err := r.Run(ctx, "id", "-u", "libvirt-qemu"); err != nil {
		return fmt.Errorf(`the 'libvirt-qemu' service account was not found.
       This targets Debian/Ubuntu libvirt packages, which create this
       account when libvirt-daemon-system is installed. Verify with: id libvirt-qemu`)
	}
	home, _ := os.UserHomeDir()
	if _, err := r.Run(ctx, "sudo", "setfacl", "-m", "u:libvirt-qemu:--x", home); err != nil {
		return fmt.Errorf("failed to grant 'libvirt-qemu' execute-only access to $HOME via setfacl. Ensure the filesystem hosting $HOME supports POSIX ACLs")
	}
	fmt.Fprintln(out, "    OK: 'libvirt-qemu' can now traverse $HOME (execute-only, no read/listing).")
	return nil
}
