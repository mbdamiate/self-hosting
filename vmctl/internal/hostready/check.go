package hostready

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"

	"vmctl/internal/execrunner"
)

// NATNetworkCheckName lets a caller with mode-specific knowledge (like
// `vmctl create` in bridged mode, which never touches the 'default' network)
// pick this one result out of Check's slice and ignore it.
const NATNetworkCheckName = "libvirt 'default' network"

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

func currentUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

// binaryCheck is one package verified by looking for a binary vmctl invokes
// directly.
type binaryCheck struct {
	name   string
	binary string
}

var binaryChecks = []binaryCheck{
	{"libvirt-clients (virsh)", "virsh"},
	{"virtinst (virt-install)", "virt-install"},
	{"qemu-utils (qemu-img)", "qemu-img"},
	{"cloud-image-utils (cloud-localds)", "cloud-localds"},
	{"wget", "wget"},
	{"openssh-client (ssh-keygen)", "ssh-keygen"},
	{"acl (setfacl)", "setfacl"},
}

// dpkgChecks are packages vmctl installs but never invokes a binary from
// directly (libvirtd/virt-install/cloud-localds use them transitively) — so
// presence is verified via dpkg instead of exec.LookPath.
var dpkgChecks = []string{"qemu-system-x86", "bridge-utils", "genisoimage"}

// Check runs every host-level readiness check and returns one CheckResult
// per requirement, in a stable order. It never modifies the system.
func Check(ctx context.Context, r execrunner.Runner) []CheckResult {
	var results []CheckResult

	results = append(results, boolResult("hardware virtualization (VT-x/AMD-V)", checkHardwareVirtualization()))
	results = append(results, boolResult("apt-based host", checkApt()))

	for _, bc := range binaryChecks {
		_, err := exec.LookPath(bc.binary)
		results = append(results, CheckResult{
			Name: bc.name,
			OK:   err == nil,
			Detail: notEmptyIf(err != nil, fmt.Sprintf(
				"'%s' not found on PATH. Run 'vmctl doctor --fix' to install it.", bc.binary)),
		})
	}

	for _, pkg := range dpkgChecks {
		_, err := r.Run(ctx, "dpkg", "-s", pkg)
		results = append(results, CheckResult{
			Name: pkg,
			OK:   err == nil,
			Detail: notEmptyIf(err != nil, fmt.Sprintf(
				"package '%s' is not installed. Run 'vmctl doctor --fix' to install it.", pkg)),
		})
	}

	results = append(results, groupMembershipCheck(ctx, r))
	results = append(results, libvirtdActiveCheck(ctx, r))
	results = append(results, natNetworkCheck(ctx, r))
	results = append(results, aclCheck(ctx, r))

	return results
}

func boolResult(name string, err error) CheckResult {
	if err == nil {
		return CheckResult{Name: name, OK: true}
	}
	return CheckResult{Name: name, OK: false, Detail: err.Error()}
}

func notEmptyIf(cond bool, detail string) string {
	if cond {
		return detail
	}
	return ""
}

func groupMembershipCheck(ctx context.Context, r execrunner.Runner) CheckResult {
	const name = "libvirt/kvm group membership"
	username := currentUsername()

	grantedOut, _ := r.Run(ctx, "id", "-nG", username)
	if !hasGroup(string(grantedOut), "libvirt") || !hasGroup(string(grantedOut), "kvm") {
		return CheckResult{Name: name, OK: false, Detail: fmt.Sprintf(
			"user '%s' is not a member of the 'libvirt'/'kvm' groups. Run 'vmctl doctor --fix' to add it.", username)}
	}

	sessionOut, _ := r.Run(ctx, "id", "-nG")
	if !hasGroup(string(sessionOut), "libvirt") || !hasGroup(string(sessionOut), "kvm") {
		return CheckResult{Name: name, OK: false, Detail: fmt.Sprintf(
			"user '%s' is a member of 'libvirt'/'kvm', but this session predates that membership. Log out and back in, then rerun.", username)}
	}

	return CheckResult{Name: name, OK: true}
}

func hasGroup(idOutput, group string) bool {
	for _, f := range strings.Fields(idOutput) {
		if f == group {
			return true
		}
	}
	return false
}

func libvirtdActiveCheck(ctx context.Context, r execrunner.Runner) CheckResult {
	const name = "libvirtd service"
	out, err := r.Run(ctx, "systemctl", "is-active", "libvirtd")
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		return CheckResult{Name: name, OK: true}
	}
	return CheckResult{Name: name, OK: false, Detail: "libvirtd is not active. Run 'vmctl doctor --fix' to enable and start it."}
}

func natNetworkCheck(ctx context.Context, r execrunner.Runner) CheckResult {
	name := NATNetworkCheckName
	info, err := r.Run(ctx, "virsh", "net-info", "default")
	if err != nil {
		return CheckResult{Name: name, OK: false, Detail: "the 'default' network is not defined. Run 'vmctl doctor --fix', or restore it manually: virsh net-define /usr/share/libvirt/networks/default.xml"}
	}
	active, autostart := false, false
	for _, line := range strings.Split(string(info), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Active:") {
			active = strings.Contains(line, "yes")
		}
		if strings.HasPrefix(line, "Autostart:") {
			autostart = strings.Contains(line, "yes")
		}
	}
	if !active || !autostart {
		return CheckResult{Name: name, OK: false, Detail: "the 'default' network is defined but not active/autostart-enabled. Run 'vmctl doctor --fix'."}
	}
	return CheckResult{Name: name, OK: true}
}

func aclCheck(ctx context.Context, r execrunner.Runner) CheckResult {
	const name = "libvirt-qemu storage ACL on $HOME"
	home, _ := os.UserHomeDir()
	out, err := r.Run(ctx, "getfacl", "-p", home)
	if err == nil && strings.Contains(string(out), "user:libvirt-qemu:--x") {
		return CheckResult{Name: name, OK: true}
	}
	return CheckResult{Name: name, OK: false, Detail: "the 'libvirt-qemu' execute-only ACL grant on $HOME is missing. Run 'vmctl doctor --fix'."}
}
