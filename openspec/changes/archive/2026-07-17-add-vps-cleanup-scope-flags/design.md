## Context

`debian-vps-cleanup.sh` currently exposes a single flag, `--yes`, which skips every `confirm()` prompt and always executes all five existing steps: remove the VM (`virsh undefine --remove-all-storage`), delete the entire working directory (`rm -rf "$WORK_DIR"`, which also holds the downloaded base cloud image), purge QEMU/libvirt packages, remove the caller from the `libvirt`/`kvm` groups, and optionally tear down the default libvirt network. There is no way to express "just get rid of this VM" without also losing the base image and the installed toolchain, which is exactly the state a `debian-vps-setup.sh` development loop wants to keep between iterations.

Separately, cross-referencing the existing `openspec/specs/` capabilities against both scripts showed that `debian-vps-cleanup.sh` has almost no formal spec coverage — its removal behavior, and two known gaps in it (the QEMU storage ACL grant on `$HOME` from `qemu-vm-storage-access`, and stale `--forward` `iptables` rules), exist only as prose inside setup-focused design docs, never as first-class cleanup requirements.

## Goals / Non-Goals

**Goals:**

- Let a fast VM-only teardown/recreate loop skip re-downloading the base cloud image and reinstalling QEMU/libvirt packages.
- Let a full teardown also revoke the QEMU storage ACL, closing the gap left by the prior "don't remove ACLs during cleanup" decision once there is no longer any installed libvirt environment left to share it with.
- Make cleanup's removal scope explicit and mutually exclusive, following the same validation pattern `debian-vps-setup.sh` already uses for `--bridge`/`--forward`.
- Keep the interactive (no-flags) path capable of reaching the exact same end state as `--purge-all`, so the two don't silently diverge.

**Non-Goals:**

- Changing `scripts/debian-vps-setup.sh` in any way.
- Changing which packages are purged (still whatever `ubuntu-qemu-prerequisites` and the script's own `PACKAGES` variable declare).
- Fixing the separately-known stale `--forward` `iptables` rules gap; it remains a documented limitation, unchanged by this design.
- Auto-detecting or warning about *other* local VMs that might depend on the shared packages/groups/network before `--purge-all` removes them — the existing inline warning text is preserved as-is; `--purge-all` simply makes the same destructive path `--yes` already offered reachable non-interactively under a clearer name.
- Providing a "vm-only, but ask first" or "purge-all, but ask first" hybrid — see the alternatives-considered note below.

## Decisions

### Two mutually exclusive scope flags, no flag for "confirm anyway"

`--vm-only` and `--purge-all` are both inherently non-interactive; there is no variant that combines an explicit scope choice with per-step confirmation. Alternative considered: keep `--yes` as an independent axis orthogonal to scope (e.g. `--vm-only --yes` vs. `--vm-only` alone, asking before the VM removal). Rejected — choosing an explicit scope flag already states the user's intent unambiguously; asking to confirm it again adds a prompt that protects nothing the flag choice didn't already decide. The one case where "ask per step" has real value — genuinely not knowing yet which of the five steps you want — is already served by running with no flags at all, unchanged.

Validation follows the existing pattern in `debian-vps-setup.sh`: if both `--vm-only` and `--purge-all` are passed, exit with a usage error before doing anything, the same way `--bridge`/`--forward` are rejected together today.

### `--vm-only` maps to exactly "run step 1, skip the rest" — no new file-deletion logic

`virsh undefine "$VM_NAME" --remove-all-storage --nvram` only removes storage volumes actually attached to the VM as libvirt-managed disks: the resized VM disk copy and the cloud-init `seed.iso`. The downloaded base cloud image (`debian-12-generic-amd64.qcow2`) sitting alongside them in `$WORK_DIR` was never attached to the VM as a disk — it was only the source `cp` was run against — so `--remove-all-storage` does not touch it. This means `--vm-only` requires no new selective-deletion logic: it is simply today's step 1, run non-interactively, with steps 2–6 skipped entirely (not merely auto-confirmed). The `user-data`/`meta-data` plaintext files left behind in `$WORK_DIR` are harmless; `debian-vps-setup.sh` overwrites them unconditionally on its next run regardless of whether a VM already exists.

Alternative considered: have `--vm-only` explicitly delete just the VM disk and seed ISO by filename, leaving the base image. Rejected as redundant — `virsh undefine --remove-all-storage` already produces exactly that result, so adding explicit file deletion would duplicate behavior libvirt already guarantees, with more code to keep in sync if the disk/seed filenames ever change.

### `--purge-all` adds ACL revocation as a genuinely new step

`--purge-all` runs today's steps 1–5 (VM, full working directory via `rm -rf`, packages, groups, network) plus a new step 6: `sudo setfacl -x u:libvirt-qemu "$HOME"` to revoke the execute-only traversal grant `debian-vps-setup.sh` applies before VM creation. The original decision not to remove this ACL (recorded as a non-goal in the `activate-libvirt-network-and-qemu-access` change) was scoped to partial cleanup, where libvirt/QEMU remain installed and a future local VM might reuse the grant. That rationale does not hold once `--purge-all` has already removed the packages and groups that make the ACL meaningful, so this design revokes it as part of that flag's contract specifically — it does not change what partial/interactive cleanup does to the ACL by default.

Alternative considered: always revoke the ACL, even under `--vm-only` or a plain VM-removal step. Rejected — it would contradict the still-valid "shared by future local VMs" rationale whenever the environment is not being fully torn down, and `--vm-only`'s whole purpose is to preserve everything except the VM instance.

### The no-flags interactive path gains a 6th confirmable step

To keep the interactive walkthrough and `--purge-all` from silently diverging in end state, a sixth `confirm()`-gated step is added at the end of the existing sequence, asking whether to revoke the QEMU storage ACL, worded consistently with the existing steps (explaining what it reverts and why it's safe to skip if other local VMs might still need it). Steps 1–5 are otherwise unchanged.

### `--yes` is removed with no compatibility alias

Confirmed with the user this is acceptable for a personal script. Keeping `--yes` as a deprecated synonym for `--purge-all` was considered and rejected: it would keep two spellings alive for a flag whose entire purpose this change is replacing with a more explicit, better-named pair, on a script with a single user who has already agreed to update any existing invocations.

## Risks / Trade-offs

- [Existing invocations using `--yes` break] → Intentional, agreed **BREAKING** change; `-h`/`--help` output and the script's own header comment are updated to point at `--vm-only`/`--purge-all`.
- [`--purge-all` still removes packages/groups/network shared by other local VMs, same as `--yes` did] → Unchanged risk, already mitigated by the existing inline warning text; not made worse or better by this change.
- [A future filesystem or libvirt version changes what `--remove-all-storage` considers "attached storage"] → `--vm-only`'s base-image preservation depends on this libvirt behavior rather than an explicit file list; if it ever changes, the failure mode is "the base image also gets deleted," which only costs a re-download, not a correctness or security issue.
- [`setfacl -x` on `$HOME` fails, e.g. ACL was already removed manually or the filesystem doesn't support ACLs] → Non-fatal, matching the existing pattern for other best-effort cleanup steps (e.g. network teardown already tolerates failure); print a warning and continue rather than aborting the rest of `--purge-all`.
