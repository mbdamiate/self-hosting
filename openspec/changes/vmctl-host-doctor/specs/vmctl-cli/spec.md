## MODIFIED Requirements

### Requirement: Single binary with subcommands
`vmctl` SHALL expose `setup`, `cleanup`, `backup` (with `snapshot`, `backup`, `restore`, and `list` sub-subcommands), and `doctor` (with plain, `--fix`, and `--unfix` modes) as subcommands of one compiled binary, replacing `debian-vm-setup.sh`, `debian-vm-cleanup.sh`, and `debian-vm-backup.sh`.

#### Scenario: Invoking a subcommand
- **WHEN** a user runs `vmctl setup`, `vmctl cleanup`, `vmctl backup <sub-subcommand>`, or `vmctl doctor`
- **THEN** `vmctl` performs the equivalent behavior the corresponding bash script performed, or, for `doctor` (which has no bash-script predecessor), the host-readiness check/fix/unfix behavior defined by the `vmctl-host-doctor` capability

#### Scenario: Invoking with no subcommand or an unknown one
- **WHEN** a user runs `vmctl` with no subcommand, or an unrecognized one
- **THEN** `vmctl` prints usage listing the available subcommands and exits non-zero without performing any action
