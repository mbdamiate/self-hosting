## Context

The local Debian VPS setup is designed for apt-based Debian and Ubuntu hosts. On the affected Ubuntu release, `qemu-kvm` is a virtual package offered by multiple concrete packages, so APT cannot select an installation candidate. The setup also uses `qemu-img` to resize the VM disk.

## Goals / Non-Goals

**Goals:**

- Select a concrete x86 QEMU system package that provides KVM/QEMU support on Ubuntu.
- Make the `qemu-img` dependency explicit.
- Keep cleanup synchronized with the packages setup declares as its own prerequisites.

**Non-Goals:**

- Support non-APT distributions.
- Change VM creation, networking, cloud-init, or guest packages.
- Select the hardware-enablements (HWE) QEMU variant automatically.

## Decisions

- Use `qemu-system-x86` instead of `qemu-kvm`. It is a concrete package and provides the required x86 QEMU/KVM emulator. The alternative `qemu-system-x86-hwe` is not selected because the standard package is sufficient and avoids opting into the HWE variant.
- Add `qemu-utils` explicitly. It owns `qemu-img`, so the script does not rely on `cloud-image-utils` continuing to install it transitively.
- Purge the same QEMU packages in cleanup that setup installs. This preserves the documented cleanup behavior and avoids leaving a declared prerequisite behind.

## Risks / Trade-offs

- [Package names can differ across Debian/Ubuntu releases] → The script remains explicitly limited to APT-based hosts; validate the declared packages with APT before running setup.
- [Cleanup can remove packages used by other VMs] → Retain the existing interactive warning and confirmation before purging packages.
