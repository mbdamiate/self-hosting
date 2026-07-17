## Why

The local VPS setup script can restart itself forever after adding the caller to `libvirt` and `kvm`. On affected Ubuntu hosts, its nested `sg` sessions do not satisfy the script's group check, so each restart returns to package installation instead of progressing to VM creation.

## What Changes

- Replace the self-reexecution through nested `sg` commands with deterministic session-group handling.
- When new group memberships are not active in the current login session, stop before VM creation and clearly instruct the user to log out/in and rerun setup.
- Check that both `libvirt` and `kvm` memberships are active before running unprivileged libvirt commands.

## Capabilities

### New Capabilities

- `libvirt-group-session-handling`: Safely handle newly granted libvirt/KVM group memberships during local VPS setup without repeated script execution.

### Modified Capabilities

<!-- None. The existing QEMU prerequisite specification does not cover session-group activation. -->

## Impact

- Affected script: `scripts/debian-vps-setup.sh`.
- The first setup run may require a logout/login before a second, idempotent run creates the VM.
- Removes the nested `sg` process behavior that caused repeated APT operations.
