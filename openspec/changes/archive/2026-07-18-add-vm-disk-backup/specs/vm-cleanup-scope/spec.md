## ADDED Requirements

### Requirement: Cleanup never deletes backup files
Neither `--vm-only` nor `--purge-all`, nor the interactive walkthrough, SHALL delete any file under a VM's backup destination (`vm-disk-backup`'s `$HOME/vm-backups/<vm-name>/` by default, or a custom `--dest`).

#### Scenario: --vm-only removes a VM with existing backups
- **WHEN** `debian-vm-cleanup.sh --vm-only` removes a VM that has backups at its backup destination
- **THEN** those backup files are left untouched

#### Scenario: --purge-all removes a VM with existing backups
- **WHEN** `debian-vm-cleanup.sh --purge-all` removes a VM that has backups at its backup destination
- **THEN** those backup files are left untouched

### Requirement: Removing a VM implicitly removes its active snapshot
Because a VM's active snapshot (per `vm-disk-snapshot`) is stored as part of its attached disk storage, removing the VM's storage (`--vm-only` or `--purge-all`) SHALL implicitly remove any active snapshot along with it — no separate snapshot-removal step is needed.

#### Scenario: VM has an active snapshot when removed
- **WHEN** `debian-vm-cleanup.sh` removes a VM (`--vm-only` or `--purge-all`) that has an active snapshot
- **THEN** the snapshot's overlay file is removed as part of the VM's attached storage, with no separate confirmation step for it
