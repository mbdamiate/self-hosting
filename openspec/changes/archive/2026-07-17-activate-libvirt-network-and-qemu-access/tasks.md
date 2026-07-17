## 1. Host prerequisite preparation

- [x] 1.1 Add the ACL utility to the setup script's declared APT dependencies.
- [x] 1.2 Add a guarded `libvirt-qemu` account and execute-only ACL preparation step for the caller's home directory, with actionable failures.

## 2. NAT network readiness

- [x] 2.1 Add NAT-only checks that verify the libvirt `default` network is defined.
- [x] 2.2 Start an inactive `default` network and enable its autostart setting before `virt-install`, without modifying it in bridged mode.
- [x] 2.3 Report a clear failure before VM creation when the NAT network cannot be prepared.

## 3. Verification

- [x] 3.1 Run shell syntax validation for the updated setup script.
- [x] 3.2 Review the NAT, forwarding, and bridged-mode control paths against the new OpenSpec scenarios.
