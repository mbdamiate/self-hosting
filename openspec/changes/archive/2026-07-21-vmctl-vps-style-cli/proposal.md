## Why

`vmctl` exists to mimic a rented VPS locally, but its command surface doesn't read like one: `setup`/`cleanup`/`status`/`backup <sub-subcommand>` are implementation-era names, not the verbs a VPS control panel or API uses. Now that `vmctl-host-doctor` (a prior, sequenced change) has made `setup`/`cleanup` lean — pure per-VM lifecycle work, no implicit host bootstrap — the remaining subcommands can be reshaped into a VPS-style verb set without dragging host-provisioning baggage along with the rename.

## What Changes

- **BREAKING**: Hard rename (no back-compat aliases — personal repo, no external consumers):
  - `vmctl setup` → `vmctl create`
  - `vmctl cleanup` → `vmctl delete`
  - `vmctl status` → `vmctl info`
  - `vmctl backup snapshot` → `vmctl snapshot create`
  - `vmctl backup snapshot-restore` → `vmctl snapshot restore`
  - `vmctl backup snapshot-delete` → `vmctl snapshot delete`
  - `vmctl backup backup` → `vmctl backup create`
  - `vmctl backup backup-list` → `vmctl backup list`
  - `vmctl backup backup-restore` → `vmctl backup restore`
- **New subcommands** (VM power-state control, not previously exposed to users):
  - `vmctl start` — starts a stopped VM (`virsh start`); no-op with a clear message if already running.
  - `vmctl stop` — graceful shutdown by default (`virsh shutdown`, ACPI); `--force` for a hard power-off (`virsh destroy`); no-op with a clear message if already stopped.
  - `vmctl reboot` — graceful reboot by default (`virsh reboot`, ACPI); `--force` for a hard reset (`virsh reset`); surfaces `virsh`'s own error if the VM isn't running.
- `vmctl list` is unchanged.
- No bare top-level `vmctl restore` — dropped deliberately, since it would be ambiguous between `snapshot restore` (discards recent writes) and `backup restore` (replaces the whole disk with an older copy); callers must say which one they mean.
- `--harden-host-firewall`, `--monitor`, and all their flags/behavior are untouched by this change.

## Capabilities

### New Capabilities
(none — `start`/`stop`/`reboot` are additive subcommands of the existing `vmctl-cli` capability, not a new capability of their own)

### Modified Capabilities
- `vmctl-cli`: the "Single binary with subcommands" requirement's subcommand enumeration is replaced with the full renamed/expanded set: `create`, `start`, `stop`, `reboot`, `delete`, `list`, `info`, `snapshot` (`create`/`restore`/`delete`), `backup` (`create`/`list`/`restore`), and `doctor` (carried over from `vmctl-host-doctor`).

## Impact

- Code: `vmctl/cmd/vmctl/main.go` (subcommand switch, usage text), `setup.go`→`create.go`, `cleanup.go`→`delete.go`, `list.go` (`runStatus`→`runInfo`), `backup.go` splits into `snapshot.go` and a rewritten `backup.go` (CLI-layer parsing/routing only — `internal/backup`'s `Run`/`Options.Subcommand` semantics are untouched), new `start.go`/`stop.go`/`reboot.go` backed by a new `vmctl/internal/power` package (or similar) for `virsh start`/`shutdown`/`destroy`/`reboot`/`reset`.
- Docs: `README.md` and `TESTING.md` need every command example updated to the new verbs.
- Scope boundary: this change does not touch prose references to "the setup script"/"the cleanup script"/"the backup subcommand" scattered across other specs (`vm-setup-rerun-recovery`, `vm-fleet-provisioning`, `vm-disk-backup`, etc.) — those are pre-existing informal shorthand, not part of the `vmctl-cli` interface contract, and are left as-is.
- Sequencing: depends on `vmctl-host-doctor` being applied and archived first, since `delete`'s scope (VM-only, no host-package/group/ACL/network teardown) and this change's `vmctl-cli` delta both assume that change has already landed.
