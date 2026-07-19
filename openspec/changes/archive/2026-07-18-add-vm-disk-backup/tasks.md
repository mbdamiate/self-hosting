## 1. Script scaffolding

- [x] 1.1 Create `scripts/debian-vm-backup.sh` with subcommand dispatch (`snapshot`, `snapshot-restore`, `snapshot-delete`, `backup`, `backup-list`, `backup-restore`) and shared `--name=`/`--help` handling, mirroring the argument-parsing style of the existing scripts
- [x] 1.2 Add `chmod +x` to the README quick-start alongside the existing two scripts

## 2. Snapshot subcommands

- [x] 2.1 Implement `snapshot`: check for an existing active snapshot (`virsh snapshot-list <name>`), fail with a clear error if one exists, otherwise `virsh snapshot-create-as <name> ... --disk-only --atomic`
- [x] 2.2 Implement `snapshot-restore`: confirm interactively, stop the VM if running (recording prior state), revert to the pre-snapshot disk state (`virsh snapshot-revert`), remove the snapshot, restart if it was running before. NOTE: external/disk-only snapshot revert has had version-dependent gaps in libvirt historically; verify against the actual installed libvirt version on first real use.
- [x] 2.3 Implement `snapshot-delete`: `virsh blockcommit --active --pivot` to merge the overlay back into a single active disk, preserving writes made since the snapshot

## 3. Backup subcommand

- [x] 3.1 Implement VM-state branch: if stopped, `qemu-img convert -O qcow2 -c` the disk directly to the destination
- [x] 3.2 If running: attempt `virsh domfsfreeze` with a bounded timeout, create a temporary external snapshot, `qemu-img convert -O qcow2 -c` the now-static base file to the destination, `virsh domfsthaw` (if frozen), then `virsh blockcommit --active --pivot` to merge the temporary snapshot back in
- [x] 3.3 Handle `domfsfreeze` timeout/failure: proceed without freezing, print a crash-consistent-only warning
- [x] 3.4 Handle `blockcommit` failure: report a clear error and the `virsh blockjob` inspection command, rather than silently leaving a lingering overlay
- [x] 3.5 Write backups to `$HOME/vm-backups/<vm-name>/` by default, overridable via `--dest`, using a filename convention that encodes the VM name and timestamp

## 4. Backup listing and retention

- [x] 4.1 Implement `backup-list`: enumerate files at the destination matching the VM's filename convention, showing filename and timestamp
- [x] 4.2 Implement `--keep=N` on `backup`: after a successful backup, delete that VM's own backups beyond the N most recent, scoped by the filename convention so other VMs' backups at a shared `--dest` are unaffected

## 5. Backup restore

- [x] 5.1 Implement `backup-restore --file=<path>`: confirm interactively, stop the VM if running (recording prior state), replace the disk with the chosen backup file, restart if it was running before

## 6. Cleanup.sh documentation update

- [x] 6.1 Update `debian-vm-cleanup.sh`'s summary/notes output to state that backups (if any) are never touched by this script
- [x] 6.2 Confirm (no code change expected) that `--remove-all-storage` already removes an active snapshot's overlay file along with the rest of the VM's storage

## 7. Documentation

- [x] 7.1 Add one minimal quick-start example command for `debian-vm-backup.sh` to `README.md`'s Quick Start section, per the `repository-readme` delta — establishing the script's existence only, not its subcommand reference
- [x] 7.2 Confirm the subcommand reference, the snapshot-vs-backup distinction, the default backup destination's same-disk caveat, and the guest-agent freeze behavior live in `--help` (task 1.1's dispatch/help handling) and this change's `vm-disk-snapshot`/`vm-disk-backup` specs, not restated in `README.md`, per the existing `repository-readme` requirement to point to those instead

## 8. Verification

- [ ] 8.1 Manually verify: `snapshot` on a running VM succeeds, the VM keeps running, and a second `snapshot` call fails with the expected error
- [ ] 8.2 Manually verify: `snapshot-restore` (confirmed) reverts writes made after the snapshot and restores the VM's prior running/stopped state
- [ ] 8.3 Manually verify: `snapshot-delete` preserves writes made after the snapshot (unlike `snapshot-restore`)
- [ ] 8.4 Manually verify: `backup` on a stopped VM and on a running VM both produce a valid, bootable-when-restored `qcow2` file at the destination
- [ ] 8.5 Manually verify: `backup` on a running VM leaves no lingering overlay afterward (`virsh domblklist`/`qemu-img info` shows a single active disk)
- [ ] 8.6 Manually verify: `backup-list` and `--keep=N` pruning behave as specified, including the shared-`--dest` scoping case
- [ ] 8.7 Manually verify: `backup-restore` (confirmed) replaces the disk with the chosen backup and restores the VM's prior running/stopped state
- [ ] 8.8 Manually verify: `debian-vm-cleanup.sh --purge-all` on a VM with existing backups leaves the backup directory untouched

(Verification tasks require an actual KVM-capable host and a real VM boot — not runnable in this sandbox. Script logic, argument dispatch, `--help` handling, the disk-info parsing, and the retention pipeline were all verified in isolation with mock inputs. `virsh snapshot-revert`'s exact behavior for external snapshots (task 2.2) especially needs real-world confirmation. Run these manually on the target machine.)
