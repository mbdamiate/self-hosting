## Context

Every VM's disk is a `qcow2` file under `$HOME/vms/<name>/`. `qemu-guest-agent` is already installed by cloud-init in every VM (added for basic guest introspection), which this proposal reuses for filesystem-consistent live backups. libvirt's external (disk-only) snapshot mechanism is the standard, currently-recommended way to get a point-in-time, crash-consistent (or, with guest-agent freeze, fully consistent) copy of a running VM's disk without stopping it: `virsh snapshot-create-as --disk-only` redirects new writes to a fresh overlay file and leaves the previous disk file static, after which it can be safely copied elsewhere and then merged back in (`virsh blockcommit --pivot`).

"Snapshot" and "backup" are related but answer different questions, and conflating them is a common real-world mistake: a snapshot lives on the *same* disk as the VM (protects against "I broke something," not against "the disk died"), while a backup is a copy stored *elsewhere* (protects against disk/host loss too). This proposal treats the snapshot mechanism as the shared primitive underneath both, but keeps the two operations and their guarantees distinct in the tooling and the documentation.

## Goals / Non-Goals

**Goals:**
- A fast, local rollback point (`snapshot`) for "before I try this," usable without interrupting the running VM.
- An actual off-disk copy (`backup`) of the VM's data, consistent whenever possible, space-efficient, with simple retention.
- Never let this repo's own tooling be the thing that deletes an operator's backup.

**Non-Goals:**
- Remote/cloud backup transfer (rclone, S3, rsync-to-a-remote-host, etc.). `--dest` accepts any local path, including a manually-mounted external drive or network share, but this script doesn't implement the transfer itself — consistent with this session's broader decision to defer anything internet-dependent.
- Multiple concurrent named snapshots, snapshot trees, or a snapshot history/versioning system. One active snapshot per VM at a time; `snapshot` fails with a clear error if one already exists, directing the operator to `snapshot-restore` or `snapshot-delete` first.
- Scheduled/automatic backups (e.g., a cron job or systemd timer running `backup` periodically). This proposal provides the primitive an operator (or a future change) could schedule; it doesn't schedule it itself.
- Guaranteed application-level consistency beyond what `qemu-guest-agent` filesystem freeze provides (e.g., no database-specific quiescing). Best-effort, documented as such.

## Decisions

**1. External (disk-only) snapshots, not internal qcow2 snapshots.**
Internal snapshots (`virsh snapshot-create-as` without `--disk-only`) are considered legacy by current libvirt guidance and have known rough edges with live disks. External snapshots are the modern, recommended approach and are what this proposal uses for both `snapshot` and as the mechanism underlying `backup`.

**2. A new script (`debian-vm-backup.sh`) with subcommands, not more flags on the existing scripts.**
`setup`/`cleanup` are single-action scripts (`--flag`-driven, one operation per invocation). Backup/snapshot is a set of related, ongoing actions (create, list, restore, delete) better expressed as subcommands (`backup.sh snapshot`, `backup.sh backup-restore`, ...) than by inventing a new flag-combination grammar on top of two already flag-heavy scripts.

**3. `backup` branches on VM state: direct copy when stopped, external-snapshot-based live copy when running.**
Copying a `qcow2` file while QEMU is actively writing to it produces an inconsistent, likely-corrupt copy. When the VM is stopped, its disk is already static — a direct `qemu-img convert` is correct and simpler than going through the snapshot machinery for no reason. When running, `backup` creates a temporary external snapshot (freezing the base file), copies that now-static file out, then `virsh blockcommit --active --pivot` to merge the temporary overlay back into a single active disk — the VM keeps running throughout, and no lingering overlay chain is left behind afterward.

**4. Best-effort `qemu-guest-agent` filesystem freeze/thaw (`virsh domfsfreeze`/`domfsthaw`) around live backups.**
Already installed by this repo's own cloud-init, so using it costs nothing extra. `domfsfreeze` briefly quiesces the guest's filesystem writes so the snapshot point is consistent from the guest's perspective, not just crash-consistent. If the agent doesn't respond (older VM, agent not yet started, or the operator removed it), `backup` proceeds anyway without freezing, printing a warning that the result is only crash-consistent — degraded, not blocked.

**5. Backups are written as compressed `qcow2` (`qemu-img convert -O qcow2 -c`), not a raw `cp`.**
`qemu-img` is already a dependency of this repo (`qemu-utils`). Compression meaningfully shrinks backup storage for a mostly-sparse/text-heavy server disk, at a CPU cost paid once per backup — a reasonable trade for something invoked on demand (or on a schedule the operator controls), not on a hot path.

**6. `--keep=N` retention is opt-in, no pruning by default.**
Defaulting to keep-everything means `backup` never surprises an operator by silently deleting something they didn't ask to have removed. `--keep=N` is there for operators who do want bounded storage, applied only to that VM's own backups in the destination directory (identified by filename convention, e.g., `<vm-name>-<timestamp>.qcow2`), never touching another VM's backups that might share the same `--dest`.

**7. `snapshot-restore` and `backup-restore` always prompt interactively; there is no non-interactive override flag.**
Mirrors the precedent already set in `vm-cleanup-scope`, which explicitly removed a blanket `--yes` bypass for destructive operations. A disk-replacing restore is at least as destructive as anything `debian-vm-cleanup.sh` does; the same reasoning applies with the same conclusion.

**8. Restore operations stop the VM first (if running) and restart it afterward only if it was running before.**
Replacing a disk out from under a running guest is unsafe. Recording and restoring the prior running/stopped state mirrors the pattern `debian-vm-setup.sh` already uses when reusing an existing VM (start it if it wasn't running, leave it alone if it already was).

**9. Backups live outside the VM's own `WORK_DIR` (`$HOME/vm-backups/<name>/` by default, not `$HOME/vms/<name>/`), snapshots live inside it.**
This isn't just organizational — it's what makes the "cleanup never deletes backups" guarantee structural rather than a rule that has to be remembered. `debian-vm-cleanup.sh`'s existing removal logic only ever touches `$HOME/vms/<name>/`; putting backups in a directory that code never references means there's no removal path to accidentally trigger, by construction. Snapshots, by contrast, are legitimately part of the VM's own attached storage (they're overlay files in the domain's active disk chain) and are correctly removed along with everything else when the VM itself is removed.

## Risks / Trade-offs

- **[Risk] The default backup destination (`$HOME/vm-backups/`) is typically on the same physical disk as the VM's own storage, so it does not protect against a disk failure unless the operator points `--dest` elsewhere.** → Mitigation: documented prominently in the proposal, design, and README — this is a real, easy-to-miss limitation, not a hidden implementation detail.
- **[Risk] `virsh blockcommit` failing partway through a live backup could leave a VM running on a lingering overlay.** → Mitigation: the backup script checks the commit's result and reports a clear error/next-step (`virsh blockjob <name> <disk> --info`) rather than silently leaving the VM in a degraded chain state; covered as a verification task.
- **[Risk] Filesystem freeze via `qemu-guest-agent` can itself hang if the guest is unhealthy, stalling the backup.** → Mitigation: `domfsfreeze` is called with a bounded timeout; on timeout, `domfsthaw` is attempted and the backup proceeds without freezing (Decision 4's degraded path), rather than hanging indefinitely.
- **[Trade-off] One snapshot at a time (Non-Goals) is simpler but less flexible than a full snapshot tree.** Accepted — this repo's audience needs "let me undo my last change," not a version-control-grade history; `snapshot-delete` plus a fresh `snapshot` covers the common case.
- **[Trade-off] Compression (Decision 5) adds CPU time to every backup.** Accepted as a one-time, on-demand cost in exchange for meaningfully smaller backups.

**10. `repository-readme` gets a minimal, pointer-only mention of the new script — not a subcommand reference.**
`repository-readme`'s existing "point to sources of truth instead of duplicating them" requirement governs *flags on scripts the reader already knows exist*; it says nothing about how a reader discovers a script exists in the first place. Every other sibling proposal in this session (admin-sudo, unattended-upgrades, guest-firewall, harden-host-firewall, observability, watchdog, crash-recovery) only adds flags to the two scripts the README's quick-start already covers, so none of them needed a `repository-readme` change — `--help` and `openspec/specs/` are enough once a reader is already looking at `debian-vm-setup.sh`/`debian-vm-cleanup.sh`. `debian-vm-backup.sh` is different: nothing in the current README structure would ever lead a reader to it. The fix is the smallest one consistent with `repository-readme`'s existing spirit — one quick-start-style example command establishing that the script exists, exactly as terse as the existing setup/cleanup examples, with subcommand details left to `--help` like everything else.

## Migration Plan

N/A — new, standalone script; no changes to existing VMs or existing script behavior beyond the documented, non-destructive `vm-cleanup-scope` clarification (which changes documentation/guarantees, not removal logic), and the minimal `repository-readme` addition.

## Open Questions

- Should a future change add a systemd timer to run `backup --keep=N` on a schedule, once there's a real need for unattended backups (analogous to how `vm-uptime-monitoring` scheduled the health check)? Left open; the primitive this proposal builds is what such a timer would call.
- Should `backup-restore` support restoring onto a *new*, differently-named VM (rather than only overwriting the source VM's own disk), e.g., to test a backup without touching the original? Left open for a follow-up if the need arises.
