## Why

Nothing in this repo protects the VM's disk today. If a change made inside the VM breaks something, there's no quick way back short of recreating the VM from scratch (losing everything). If the disk itself is lost (host disk failure, an operator mistake), there's no copy anywhere else. A VM meant to run as a real server needs both: a fast, local rollback point for "before I try this," and an actual copy of the data stored somewhere other than the live disk.

## What Changes

- Add a new script, `scripts/debian-vm-backup.sh`, since this is an ongoing, subcommand-shaped lifecycle concern distinct from provisioning (`debian-vm-setup.sh`) or teardown (`debian-vm-cleanup.sh`), rather than more flags bolted onto either.
- `snapshot` / `snapshot-restore` / `snapshot-delete` subcommands: a single, external (disk-only) libvirt snapshot per VM — a fast local rollback point, not a substitute for a real backup (it lives on the same disk as the VM). Deliberately one at a time, not a stack or tree of named snapshots.
- `backup` / `backup-list` / `backup-restore` subcommands: a compressed, point-in-time copy of the VM's disk written to a separate destination directory (default `$HOME/vm-backups/<name>/`, overridable). Works whether the VM is running (via the same external-snapshot mechanism, so the guest keeps running throughout) or stopped (a direct copy). Uses `qemu-guest-agent` (already installed by cloud-init) to freeze the guest filesystem around the snapshot for a consistent backup when the VM is running, falling back to a crash-consistent backup with a warning if the agent doesn't respond.
- `backup --keep=N`: optional retention — deletes the oldest backups for that VM beyond the N most recent, after a successful backup. No pruning by default.
- `snapshot-restore` and `backup-restore` are destructive and always prompt for confirmation — no non-interactive bypass flag, consistent with `vm-cleanup-scope`'s existing removal of `--yes`. Both stop the VM first if it's running, and restart it afterward if it was running before.
- Extend `debian-vm-cleanup.sh`'s scope: explicitly, neither `--vm-only` nor `--purge-all` ever deletes backup files — that would defeat the point of a backup. Removing a VM's active snapshot (if any) still happens implicitly, since `--remove-all-storage` already removes the snapshot's overlay file along with everything else attached to the domain.
- `--help` documents the script's subcommands and flags; the snapshot-vs-backup distinction, the default destination's same-disk caveat, and the no-remote-upload scope are captured in the new capability specs. Unlike a new flag on an already-discoverable script, a wholly new script has no existing pointer leading a reader to it — so this change extends the existing `repository-readme` spec's quick-start requirement with one minimal, pointer-style mention of `debian-vm-backup.sh`'s existence (not its subcommand reference, which stays in `--help`).

## Capabilities

### New Capabilities
- `vm-disk-snapshot`: the single, external, disk-only rollback point per VM (`snapshot`/`snapshot-restore`/`snapshot-delete`).
- `vm-disk-backup`: point-in-time, compressed copies of the VM's disk to a separate destination, live or stopped, with optional retention (`backup`/`backup-list`/`backup-restore`).

### Modified Capabilities
- `vm-cleanup-scope`: gains an explicit guarantee that neither `--vm-only` nor `--purge-all` deletes backup files, while clarifying that a VM's active snapshot is implicitly removed along with its storage.
- `repository-readme`: the quick-start requirement gains a minimal mention of `debian-vm-backup.sh`'s existence (one example command), since a brand-new script — unlike a new flag on `debian-vm-setup.sh`/`debian-vm-cleanup.sh` — isn't discoverable through any pointer the README already provides.

## Impact

- New file: `scripts/debian-vm-backup.sh`.
- `scripts/debian-vm-cleanup.sh`: no functional removal logic added for backups (the point is that it explicitly does *not* touch them), but documentation/output updated to state this guarantee.
- `README.md`: one minimal quick-start mention of the new script's existence, per the `repository-readme` delta — not its subcommand reference, snapshot-vs-backup distinction, or destination caveats, which stay in `--help`/`openspec/specs/`.
