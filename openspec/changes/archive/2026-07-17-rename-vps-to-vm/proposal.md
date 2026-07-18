## Why

The setup/cleanup scripts and four OpenSpec capabilities use "vps" to name a locally provisioned KVM/QEMU/libvirt VM, but a VPS is a rented cloud offering — this repo only mimics one for local testing. The mismatch was already tracked as an open TODO ("Validate feasibility of renaming the provisioned 'vps'") and has spread from the script filenames into VM defaults, help text, and capability names, so code and specs use different vocabulary depending on which file you're reading.

## What Changes

- Rename `scripts/debian-vps-setup.sh` → `scripts/debian-vm-setup.sh` and `scripts/debian-vps-cleanup.sh` → `scripts/debian-vm-cleanup.sh`.
- Replace the `VM_NAME`/`VM_HOSTNAME` default and every `--name` default reference in both scripts' help text and comments from `"debian-vps"` to `"debian-vm"`.
- Keep "VPS" wording as-is on 4 lines that describe the external rented-VPS concept being mimicked, not this repo's own naming (a literal swap would read as nonsense, e.g. "simulating a real VM"):
  - `debian-vps-setup.sh` line 5 ("simulating a real VPS")
  - `debian-vps-setup.sh` line 59 ("e.g. a basic VPS plan")
  - `debian-vps-cleanup.sh` line 124 ("Cleaning up the simulated VPS environment")
  - `debian-vps-cleanup.sh` line 279 (references `setup-debian-vps.sh`; also carries a pre-existing word-order typo vs. the real filename, left as-is — out of scope for this change)
- Rename 4 OpenSpec capabilities to match the new vocabulary, including every `"debian-vps"` default quoted inside their requirements/scenarios:
  - `vps-cleanup-scope` → `vm-cleanup-scope`
  - `vps-fleet-provisioning` → `vm-fleet-provisioning`
  - `vps-port-forward-reapplication` → `vm-port-forward-reapplication`
  - `vps-setup-rerun-recovery` → `vm-setup-rerun-recovery`
- Update wording-only "VPS" mentions to "VM" in two capabilities whose directory names are already VM-neutral, for full consistency: `ubuntu-qemu-prerequisites`, `libvirt-group-session-handling`. No behavior changes, text only.
- Update `TODO.md`: resolve the "Validate feasibility of renaming the provisioned 'vps'" item, and refresh the two other items that reference the old script filenames.
- `openspec/changes/archive/**` is explicitly out of scope — it's a historical record and stays untouched.

No **BREAKING** changes: this is a single-operator local tool, not a published interface — nothing external depends on the old script filenames or capability names.

## Capabilities

### New Capabilities
- `vm-cleanup-scope`: renamed from `vps-cleanup-scope`; same removal-scope behavior for `debian-vm-cleanup.sh`, with `"debian-vm"` as the default target name.
- `vm-fleet-provisioning`: renamed from `vps-fleet-provisioning`; same fleet naming/sizing/IP-reservation behavior for `debian-vm-setup.sh`, with `"debian-vm"` as the default name/hostname.
- `vm-port-forward-reapplication`: renamed from `vps-port-forward-reapplication`; same idempotent `--forward` behavior, unchanged other than the capability name.
- `vm-setup-rerun-recovery`: renamed from `vps-setup-rerun-recovery`; same rerun-detection/recovery behavior, unchanged other than the capability name.

### Modified Capabilities
- `vps-cleanup-scope`: all requirements removed (renamed to `vm-cleanup-scope`).
- `vps-fleet-provisioning`: all requirements removed (renamed to `vm-fleet-provisioning`).
- `vps-port-forward-reapplication`: all requirements removed (renamed to `vm-port-forward-reapplication`).
- `vps-setup-rerun-recovery`: all requirements removed (renamed to `vm-setup-rerun-recovery`).
- `ubuntu-qemu-prerequisites`: requirement text updated ("Debian VPS setup" → "Debian VM setup") — wording only, no behavior change.
- `libvirt-group-session-handling`: requirement text updated ("local VPS setup" → "local VM setup", 3 occurrences) — wording only, no behavior change.

## Impact

- Code: `scripts/debian-vps-setup.sh`, `scripts/debian-vps-cleanup.sh` (renamed + edited).
- Specs: the 6 capability spec files listed above, under `openspec/specs/`.
- Docs: `TODO.md`.
- Excluded: `openspec/changes/archive/**` (historical record).
- No dependency, API, or infrastructure impact — this is a local-only rename.
