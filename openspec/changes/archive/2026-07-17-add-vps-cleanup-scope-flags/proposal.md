## Why

`debian-vps-cleanup.sh` today has exactly one non-interactive mode, `--yes`, which always tears down everything: the VM, its disk, the downloaded base cloud image, the QEMU/libvirt packages, group membership, and the default network. That makes it unusable for the fast "destroy the VM, tweak `debian-vps-setup.sh`, recreate" loop this repo's own change history shows happening repeatedly — every cycle re-downloads the base image and reinstalls QEMU from scratch. Separately, cleanup's actual removal contract has never been formalized as an OpenSpec capability: only one requirement about it exists today (purging the QEMU packages setup installs, specified as part of `ubuntu-qemu-prerequisites`), and two known gaps in its coverage — it never revokes the QEMU storage ACL granted on `$HOME`, and it can leave stale `--forward` `iptables` rules behind — live only as prose in unrelated setup-focused design docs, not as first-class, discoverable requirements.

## What Changes

- Add `--vm-only`: a new non-interactive flag that removes only the VM definition and its attached storage (disk + cloud-init seed ISO), leaving the downloaded base cloud image, installed packages, group membership, and the default network untouched — enabling a fast rerun of `debian-vps-setup.sh` without re-downloading or reinstalling anything.
- Add `--purge-all`: a new non-interactive flag that removes everything `--yes` removes today (VM, full working directory including the base image, packages, group membership, default network) plus a new step: revoking the QEMU storage ACL grant on `$HOME`.
- **BREAKING**: Remove `--yes` entirely, with no compatibility alias. `--vm-only` and `--purge-all` are the only non-interactive modes going forward.
- Running with no flags keeps today's interactive, step-by-step walkthrough, but gains a 6th confirmable step to revoke the QEMU storage ACL — so answering "yes" to every interactive prompt now leaves the system in the same state `--purge-all` reaches non-interactively.
- `--vm-only` and `--purge-all` are mutually exclusive; passing both is a usage error, matching the existing `--bridge`/`--forward` validation pattern already used in `debian-vps-setup.sh`.

## Capabilities

### New Capabilities
- `vps-cleanup-scope`: Defines the cleanup script's removal scope selection — the three supported invocation modes (no flags, `--vm-only`, `--purge-all`), what each one removes or preserves, and that scope flags are mutually exclusive and inherently non-interactive.

### Modified Capabilities

<!-- None. qemu-vm-storage-access's existing requirements only govern granting the ACL during setup; it makes no claim about cleanup, so revoking the ACL under --purge-all is new scope owned entirely by vps-cleanup-scope, not a change to that spec's requirements. -->

## Impact

- Affected script: `scripts/debian-vps-cleanup.sh` (argument parsing, step gating, new ACL-revocation step, help text, header comment).
- No changes to `scripts/debian-vps-setup.sh`.
- Breaking change to `debian-vps-cleanup.sh`'s CLI: any existing invocation using `--yes` must be updated to `--purge-all`.
