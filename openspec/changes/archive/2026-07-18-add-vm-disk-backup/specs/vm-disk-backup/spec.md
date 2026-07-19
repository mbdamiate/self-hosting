## ADDED Requirements

### Requirement: Backup works whether the VM is running or stopped
The `backup` subcommand SHALL produce a point-in-time copy of the VM's disk regardless of whether the VM is currently running, using a direct copy when stopped and a live, non-disruptive method when running.

#### Scenario: VM is stopped
- **WHEN** `debian-vm-backup.sh backup --name=<vm>` is run and the VM is stopped
- **THEN** its disk is copied directly to the backup destination

#### Scenario: VM is running
- **WHEN** `debian-vm-backup.sh backup --name=<vm>` is run and the VM is running
- **THEN** a temporary external snapshot is used to copy a consistent point-in-time image of the disk to the backup destination without stopping the VM
- **AND** the temporary snapshot is committed back into the VM's active disk afterward, leaving no lingering overlay

### Requirement: Live backups attempt filesystem freeze via the guest agent
When backing up a running VM, `backup` SHALL attempt to freeze the guest's filesystems via `qemu-guest-agent` before taking the snapshot, and SHALL proceed without freezing (with a warning) if the agent does not respond within a bounded timeout.

#### Scenario: Guest agent responds
- **WHEN** `backup` runs against a running VM whose guest agent responds to a freeze request
- **THEN** the guest's filesystems are frozen before the snapshot is taken and thawed immediately after

#### Scenario: Guest agent does not respond
- **WHEN** `backup` runs against a running VM whose guest agent does not respond within the timeout
- **THEN** the backup proceeds without freezing, and the operator is warned that the result is only crash-consistent

### Requirement: Backups are stored compressed at a separate destination
`backup` SHALL write the copy as a compressed `qcow2` file to a destination directory outside the VM's own working directory, defaulting to `$HOME/vm-backups/<vm-name>/` and overridable via `--dest`.

#### Scenario: Default destination
- **WHEN** `debian-vm-backup.sh backup --name=<vm>` is run without `--dest`
- **THEN** the compressed backup is written under `$HOME/vm-backups/<vm-name>/`

#### Scenario: Custom destination
- **WHEN** `debian-vm-backup.sh backup --name=<vm> --dest=<path>` is run
- **THEN** the compressed backup is written under `<path>` instead

### Requirement: Backups can be listed
The `backup-list` subcommand SHALL list the backups available for a given VM at its backup destination.

#### Scenario: Backups exist
- **WHEN** `debian-vm-backup.sh backup-list --name=<vm>` is run and one or more backups exist for that VM
- **THEN** they are listed, identifying at least their filename and creation time

#### Scenario: No backups exist
- **WHEN** `debian-vm-backup.sh backup-list --name=<vm>` is run and no backups exist for that VM
- **THEN** it reports that none were found

### Requirement: Optional retention pruning
`backup` SHALL accept a `--keep=N` flag that, after a successful backup, deletes that VM's own backups beyond the N most recent at its destination. Without the flag, no pruning occurs.

#### Scenario: --keep is passed and more than N backups exist
- **WHEN** `debian-vm-backup.sh backup --name=<vm> --keep=N` completes successfully and more than N backups for that VM exist at the destination
- **THEN** the oldest backups beyond the N most recent are deleted

#### Scenario: --keep is not passed
- **WHEN** `debian-vm-backup.sh backup --name=<vm>` is run without `--keep`
- **THEN** no existing backups are deleted, regardless of how many accumulate

#### Scenario: Pruning only affects the targeted VM's own backups
- **WHEN** `--keep=N` pruning runs at a `--dest` shared with other VMs' backups
- **THEN** only backups identified as belonging to the targeted VM are considered for deletion

### Requirement: Backup restore is destructive, confirmed, and stops/restarts the VM
The `backup-restore` subcommand SHALL replace the VM's current disk with the contents of a chosen backup, after interactive confirmation, and SHALL stop the VM first if running and restart it afterward if it was running before.

#### Scenario: Restoring a running VM
- **WHEN** `debian-vm-backup.sh backup-restore --name=<vm> --file=<backup>` is run, the VM is running, and the operator confirms
- **THEN** the VM is stopped, its disk is replaced with the contents of `<backup>`, and the VM is restarted

#### Scenario: Restoring a stopped VM
- **WHEN** `debian-vm-backup.sh backup-restore --name=<vm> --file=<backup>` is run, the VM is stopped, and the operator confirms
- **THEN** its disk is replaced with the contents of `<backup>`, and the VM is left stopped

#### Scenario: Operator declines confirmation
- **WHEN** `debian-vm-backup.sh backup-restore --name=<vm> --file=<backup>` is run and the operator declines the confirmation prompt
- **THEN** no changes are made to the VM's disk or running state
