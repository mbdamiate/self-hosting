// Package hostready centralizes vmctl's host-level readiness checks and the
// actions that establish (Fix) or revert (Unfix) them: packages, libvirt/kvm
// group membership, the libvirtd service, the libvirt 'default' NAT
// network, and the QEMU storage ACL on $HOME. `vmctl create`'s preflight and
// `vmctl doctor`'s report both call Check, so the two can never drift apart.
package hostready

// CheckResult is the outcome of one host-readiness check. Detail is empty
// when OK is true.
type CheckResult struct {
	Name   string
	OK     bool
	Detail string
}
