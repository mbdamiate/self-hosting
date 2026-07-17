## 1. Align QEMU package declarations

- [x] 1.1 Update the setup APT package list to use `qemu-system-x86` and explicitly include `qemu-utils`, with no `qemu-kvm` install target.
- [x] 1.2 Update the cleanup purge list to include `qemu-system-x86` and `qemu-utils`.

## 2. Verify the setup prerequisites

- [x] 2.1 Check the setup and cleanup scripts for shell syntax errors.
- [x] 2.2 Verify from APT metadata that the selected packages are installable and that `qemu-utils` provides `qemu-img`.
