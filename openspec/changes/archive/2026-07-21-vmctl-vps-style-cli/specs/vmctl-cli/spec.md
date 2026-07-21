## MODIFIED Requirements

### Requirement: Single binary with subcommands
`vmctl` SHALL expose `create`, `start`, `stop`, `reboot`, `delete`, `list`, `info`, `snapshot` (with `create`, `restore`, and `delete` sub-subcommands), `backup` (with `create`, `list`, and `restore` sub-subcommands), and `doctor` (with plain, `--fix`, and `--unfix` modes) as subcommands of one compiled binary.

#### Scenario: Invoking a subcommand
- **WHEN** a user runs `vmctl create`, `vmctl start`, `vmctl stop`, `vmctl reboot`, `vmctl delete`, `vmctl list`, `vmctl info`, `vmctl snapshot <sub-subcommand>`, `vmctl backup <sub-subcommand>`, or `vmctl doctor`
- **THEN** `vmctl` performs the corresponding behavior

#### Scenario: Invoking with no subcommand or an unknown one
- **WHEN** a user runs `vmctl` with no subcommand, or an unrecognized one
- **THEN** `vmctl` prints usage listing the available subcommands and exits non-zero without performing any action

## ADDED Requirements

### Requirement: No aliases for previous subcommand names
`vmctl` SHALL NOT recognize any previous subcommand name (`setup`, `cleanup`, `status`, or `backup`'s previous sub-subcommand spellings `snapshot`, `snapshot-restore`, `snapshot-delete`, `backup`, `backup-list`, `backup-restore` used as top-level or first-level names) as valid input.

#### Scenario: Old top-level name used
- **WHEN** a user runs a previous subcommand name such as `vmctl setup`, `vmctl cleanup`, or `vmctl status`
- **THEN** `vmctl` treats it as an unknown subcommand, prints usage, and exits non-zero

#### Scenario: Old backup sub-subcommand spelling used
- **WHEN** a user runs `vmctl backup snapshot` or another pre-rename sub-subcommand spelling
- **THEN** `vmctl` treats it as an unknown sub-subcommand of `backup`, prints that subcommand's usage, and exits non-zero

### Requirement: No bare top-level restore
`vmctl` SHALL NOT expose a bare `vmctl restore` subcommand; restoring requires specifying `vmctl snapshot restore` or `vmctl backup restore` explicitly.

#### Scenario: Bare restore attempted
- **WHEN** a user runs `vmctl restore`
- **THEN** `vmctl` treats it as an unknown subcommand and prints usage, exiting non-zero

### Requirement: `snapshot` and `backup` verbs preserve prior sub-subcommand behavior
`vmctl snapshot create`/`restore`/`delete` and `vmctl backup create`/`list`/`restore` SHALL perform exactly the behavior their pre-rename names (`vmctl backup snapshot`/`snapshot-restore`/`snapshot-delete` and `vmctl backup backup`/`backup-list`/`backup-restore`) performed, using the same `--name`/`--dest`/`--keep`/`--file` flags.

#### Scenario: Snapshot verb equivalence
- **WHEN** a user runs `vmctl snapshot create`, `vmctl snapshot restore`, or `vmctl snapshot delete`
- **THEN** `vmctl` performs exactly what `vmctl backup snapshot`, `vmctl backup snapshot-restore`, or `vmctl backup snapshot-delete` performed, respectively

#### Scenario: Backup verb equivalence
- **WHEN** a user runs `vmctl backup create`, `vmctl backup list`, or `vmctl backup restore`
- **THEN** `vmctl` performs exactly what `vmctl backup backup`, `vmctl backup backup-list`, or `vmctl backup backup-restore` performed, respectively

### Requirement: `vmctl start` starts a stopped VM
`vmctl start` SHALL start the named VM if it is not already running, and SHALL report success without further action if it is already running.

#### Scenario: VM is stopped
- **WHEN** `vmctl start` runs and `virsh domstate` reports the VM is shut off
- **THEN** `vmctl` starts it via `virsh start`

#### Scenario: VM is already running
- **WHEN** `vmctl start` runs and `virsh domstate` reports the VM is running
- **THEN** `vmctl` reports it is already running and exits zero without calling `virsh start`

### Requirement: `vmctl stop` gracefully shuts down a running VM, with a --force hard option
`vmctl stop` SHALL request a graceful ACPI shutdown by default, SHALL accept `--force` to perform a hard power-off instead, and SHALL report success without further action if the VM is already stopped.

#### Scenario: Graceful stop
- **WHEN** `vmctl stop` runs without `--force` and the VM is running
- **THEN** it requests a graceful shutdown via `virsh shutdown`

#### Scenario: Forced stop
- **WHEN** `vmctl stop --force` runs and the VM is running
- **THEN** it performs a hard power-off via `virsh destroy`

#### Scenario: VM is already stopped
- **WHEN** `vmctl stop` runs and the VM is already shut off
- **THEN** it reports this and exits zero without further action

### Requirement: `vmctl reboot` gracefully reboots a running VM, with a --force hard option
`vmctl reboot` SHALL request a graceful ACPI reboot by default, SHALL accept `--force` to perform a hard reset instead, and SHALL surface the underlying `virsh` error if the VM is not running rather than performing its own pre-check.

#### Scenario: Graceful reboot
- **WHEN** `vmctl reboot` runs without `--force` and the VM is running
- **THEN** it requests a graceful reboot via `virsh reboot`

#### Scenario: Forced reboot
- **WHEN** `vmctl reboot --force` runs and the VM is running
- **THEN** it performs a hard reset via `virsh reset`

#### Scenario: VM is not running
- **WHEN** `vmctl reboot` runs and the VM is not running
- **THEN** `vmctl` surfaces `virsh`'s own error naming the underlying command, without a separate pre-check
