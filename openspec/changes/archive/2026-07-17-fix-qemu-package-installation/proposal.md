## Why

On current Ubuntu releases, `qemu-kvm` is a virtual package with no direct installation candidate. The local VPS setup therefore stops before it can create the virtual machine; it must select a concrete QEMU system emulator package.

## What Changes

- Install `qemu-system-x86`, the concrete x86 QEMU/KVM provider, in the Debian/Ubuntu setup script.
- Keep the cleanup package list aligned so it purges the package installed by setup.
- Declare the QEMU image utility dependency explicitly because the setup script invokes `qemu-img`.

## Capabilities

### New Capabilities

- `ubuntu-qemu-prerequisites`: Install concrete and complete QEMU prerequisites required to run the local Debian VPS setup on apt-based Ubuntu hosts.

### Modified Capabilities

<!-- None. No existing specifications are present. -->

## Impact

- Affected scripts: `scripts/debian-vps-setup.sh` and `scripts/debian-vps-cleanup.sh`.
- Affected host dependencies: QEMU system emulator and image utilities installed through APT.
- No VM interface, network behavior, or guest configuration changes.
