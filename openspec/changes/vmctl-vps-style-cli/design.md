## Context

`vmctl` (`vmctl/cmd/vmctl/main.go`) currently dispatches `setup`, `cleanup`, `backup` (sub-subcommands `snapshot`/`snapshot-restore`/`snapshot-delete`/`backup`/`backup-list`/`backup-restore`), `list`, and `status`. This change assumes `vmctl-host-doctor` has already landed: `setup`/`cleanup` no longer touch host-level packages/groups/services/network/ACL, and a `doctor` subcommand exists for that. With host bootstrap out of the way, `setup`/`cleanup`/`status`/`backup` are purely per-VM lifecycle operations — the natural next step is to name them like a VPS API does.

## Goals / Non-Goals

**Goals:**
- Rename existing subcommands to VPS-style verbs with no behavior change beyond the rename.
- Add `start`/`stop`/`reboot` as thin, idempotent wrappers over `virsh start`/`shutdown`/`destroy`/`reboot`/`reset`.
- Split `backup`'s six sub-subcommands into two symmetric noun+verb groups (`snapshot {create,restore,delete}`, `backup {create,list,restore}`) without touching the internal `internal/backup` package's tested logic.
- Keep the CLI-layer change (`cmd/vmctl/*.go`) separate from internal package changes wherever possible, to minimize the blast radius of a purely-naming change.

**Non-Goals:**
- No back-compat aliases for the old subcommand names.
- No change to `--harden-host-firewall`, `--monitor`, or any flag names/behavior on the renamed commands (`--name`, `--dest`, `--keep`, `--file`, `--vm-only`, `--purge-all` are all unchanged).
- No automatic graceful-then-force escalation for `stop`/`reboot` (no timeout/polling loop) — `--force` is an explicit user choice.
- No mass-edit of other specs' prose references to "the setup script"/"the cleanup script"/"the backup subcommand" (`vm-setup-rerun-recovery`, `vm-fleet-provisioning`, `vm-disk-backup`, `vm-disk-snapshot`, `vm-cleanup-scope`, etc.) — those are pre-existing informal shorthand for "the acting subcommand," not part of the `vmctl-cli` interface contract this change formally revises. Left as-is deliberately.
- No bare top-level `vmctl restore` alias.

## Decisions

**1. New `vmctl/internal/power` package for `start`/`stop`/`reboot`.**
These are VM power-state mutations — distinct from `internal/fleet` (read-only status querying: `List`/`Get`) and from `internal/backup` (disk-level snapshot/backup operations, which also happen to start/stop VMs but only as an internal means to a disk-consistency end, not as a user-facing power command). Alternative considered: add these to `internal/fleet`. Rejected — `fleet` never mutates state today, and mixing that in would blur an otherwise clean read/write split.

**2. `snapshot`/`backup` CLI split reuses `internal/backup` unchanged.**
`cmd/vmctl/snapshot.go` and the rewritten `cmd/vmctl/backup.go` each parse their own verb (`create`/`restore`/`delete` for snapshot; `create`/`list`/`restore` for backup) and translate it to the existing `backup.Options{Subcommand: "..."}` strings the internal package already expects (`"snapshot"`, `"snapshot-restore"`, `"snapshot-delete"`, `"backup"`, `"backup-list"`, `"backup-restore"`). The internal package's tested logic, flag semantics, and confirmation prompts are untouched — only the CLI-layer verb spelling and dispatch change. Alternative considered: rename the internal `Subcommand` strings too, for full-stack consistency. Rejected for this change — it multiplies the diff for a pure rename with no behavior change and can be revisited independently if it ever matters (e.g. if `internal/backup`'s tests or other callers depend on the string values, renaming them is a separate, lower-value refactor).

**3. `start`/`stop`/`reboot` idempotency and error handling.**
- `start`: if `virsh domstate` already reports running, print a message and exit 0 without calling `virsh start`.
- `stop`: if already shut off, print a message and exit 0. Default path: `virsh shutdown <name>`. `--force`: `virsh destroy <name>`.
- `reboot`: no pre-check for "not running" — let `virsh reboot`/`virsh reset` fail naturally and surface that failure verbatim, consistent with `vmctl-cli`'s existing "preserve actionable error messages naming the underlying virsh command" requirement. Default path: `virsh reboot <name>`. `--force`: `virsh reset <name>`.

**4. `vmctl-cli`'s subcommand-enumeration requirement is rewritten wholesale, composed on top of `vmctl-host-doctor`'s pending delta.**
Because `vmctl-host-doctor`'s change is not yet archived, the spec delta in this change is authored against the *end state* (current spec + `vmctl-host-doctor`'s delta applied), not against today's currently-archived text. This avoids two independent MODIFIED blocks for the same requirement conflicting when both changes are eventually synced/archived — `vmctl-host-doctor` must be archived before this change is archived.

## Risks / Trade-offs

- **[Hard rename breaks any scripts/muscle memory built on today's names]** → Mitigation: explicitly accepted (see proposal's Why) — personal repo, no external consumers; README/TESTING.md updated in the same change.
- **[Archiving order matters: this change's `vmctl-cli` delta assumes `vmctl-host-doctor` is archived first]** → Mitigation: called out explicitly in the proposal's Impact/Sequencing and here; do not archive this change before `vmctl-host-doctor`.
- **[`reboot`/`stop` on a VM with no ACPI support in the guest could hang waiting for a graceful shutdown that never completes]** → Mitigation: this mirrors `virsh shutdown`/`virsh reboot`'s own real-world behavior (they send the ACPI request and return immediately without waiting for completion) — `vmctl` does not add its own wait/poll loop, so it does not hang; the guest simply may not respond, which `--force` exists to handle.
- **[Splitting `backup.go` into two files while keeping `internal/backup`'s subcommand strings unrenamed creates a naming seam between CLI verb and internal string]** → Mitigation: documented explicitly in Decision 2; acceptable since it's an internal implementation detail with no user-facing effect.

## Migration Plan

1. Confirm `vmctl-host-doctor` has been applied and archived first.
2. Add `vmctl/internal/power` with `Start`/`Stop`/`Reboot`.
3. Rename `cmd/vmctl/setup.go`→`create.go`, `cleanup.go`→`delete.go`; rename their `run*` functions accordingly.
4. In `list.go`, rename `runStatus`→`runInfo` (keep `runList` as-is).
5. Split `backup.go` into `snapshot.go` (new top-level `vmctl snapshot <verb>`) and a rewritten `backup.go` (new top-level `vmctl backup <verb>`), both delegating into unchanged `internal/backup.Run`.
6. Add `start.go`/`stop.go`/`reboot.go`.
7. Update `main.go`'s subcommand switch and `usage()` text with the full final set: `create`, `start`, `stop`, `reboot`, `delete`, `list`, `info`, `snapshot`, `backup`, `doctor`.
8. Update `README.md` and `TESTING.md` command examples throughout.
9. No data migration; rollback is a normal git revert.

## Open Questions

None outstanding.
