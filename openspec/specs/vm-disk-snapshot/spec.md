# vm-disk-snapshot Specification

## Purpose
TBD - created by archiving change add-vm-disk-backup. Update Purpose after archive.
## Requirements
### Requirement: A single external disk-only snapshot per VM
The `snapshot` subcommand SHALL create one external, disk-only libvirt snapshot for the named VM, and SHALL refuse to create a second one while one is already active.

#### Scenario: No snapshot exists
- **WHEN** `debian-vm-backup.sh snapshot --name=<vm>` is run and the VM has no active snapshot
- **THEN** an external, disk-only snapshot is created, and the VM continues running (if it was running) on a new overlay disk

#### Scenario: A snapshot already exists
- **WHEN** `debian-vm-backup.sh snapshot --name=<vm>` is run and the VM already has an active snapshot
- **THEN** the command fails with an error directing the operator to `snapshot-restore` or `snapshot-delete` first, and no new snapshot is created

### Requirement: Snapshot restore is destructive, confirmed, and stops/restarts the VM
The `snapshot-restore` subcommand SHALL revert the VM's disk to the state at the time the snapshot was taken, discarding all writes made since, after interactive confirmation, and SHALL stop the VM first if running and restart it afterward if it was running before.

#### Scenario: Restoring a running VM
- **WHEN** `debian-vm-backup.sh snapshot-restore --name=<vm>` is run, the VM is running, and the operator confirms
- **THEN** the VM is stopped, its disk is reverted to the pre-snapshot state, the snapshot is removed, and the VM is restarted

#### Scenario: Restoring a stopped VM
- **WHEN** `debian-vm-backup.sh snapshot-restore --name=<vm>` is run, the VM is stopped, and the operator confirms
- **THEN** its disk is reverted to the pre-snapshot state and the snapshot is removed, and the VM is left stopped

#### Scenario: Operator declines confirmation
- **WHEN** `debian-vm-backup.sh snapshot-restore --name=<vm>` is run and the operator declines the confirmation prompt
- **THEN** no changes are made to the VM's disk or running state

#### Scenario: No active snapshot
- **WHEN** `debian-vm-backup.sh snapshot-restore --name=<vm>` is run and the VM has no active snapshot
- **THEN** the command fails with an error, without prompting

### Requirement: Snapshot delete merges the overlay back without discarding writes
The `snapshot-delete` subcommand SHALL merge the active snapshot's overlay back into the VM's base disk (keeping all writes made since the snapshot was taken), rather than discarding them.

#### Scenario: Snapshot exists
- **WHEN** `debian-vm-backup.sh snapshot-delete --name=<vm>` is run and the VM has an active snapshot
- **THEN** the overlay is committed back into a single active disk, and the VM's current state (including writes made after the snapshot) is preserved

#### Scenario: No active snapshot
- **WHEN** `debian-vm-backup.sh snapshot-delete --name=<vm>` is run and the VM has no active snapshot
- **THEN** the command reports there is nothing to delete

