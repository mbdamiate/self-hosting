package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"vmctl/internal/execrunner"
)

func checkHardwareVirtualization() error {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return fmt.Errorf("could not read /proc/cpuinfo")
	}
	if !strings.Contains(string(data), "vmx") && !strings.Contains(string(data), "svm") {
		return fmt.Errorf("your CPU doesn't report VT-x/AMD-V support, or it's disabled in the BIOS. Enable virtualization in the BIOS/UEFI and run this again")
	}
	return nil
}

func checkApt() error {
	if _, err := exec.LookPath("apt"); err != nil {
		return fmt.Errorf("this assumes a system with apt (Ubuntu/Debian). Adapt it for your distro")
	}
	return nil
}

var basePackages = []string{
	"qemu-system-x86", "qemu-utils", "libvirt-daemon-system", "libvirt-clients",
	"bridge-utils", "virtinst", "cloud-image-utils", "genisoimage",
	"wget", "openssh-client", "acl",
}

func currentUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

// installPrerequisites mirrors sections 2 and its group-membership check:
// installs packages, adds the user to libvirt/kvm, starts libvirtd, and
// verifies the CURRENT session actually has those groups (a fresh usermod
// only takes effect in a new login session).
func installPrerequisites(ctx context.Context, r execrunner.Runner, out io.Writer) error {
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
       after a new login session. Log out and back in, then run this again.
       Verify with: id -nG`, strings.Join(missing, ", "))
	}
	return nil
}

// ensureNATNetworkReady mirrors section 3: skipped entirely in bridged mode.
func ensureNATNetworkReady(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	if _, err := r.Run(ctx, "virsh", "net-info", "default"); err != nil {
		return fmt.Errorf(`the libvirt network 'default' is not defined.
       Inspect available networks with: virsh net-list --all
       Define/restore it, e.g.: virsh net-define /usr/share/libvirt/networks/default.xml`)
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

// grantQEMUStorageACL mirrors section 4.
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
