# vmctl-cli Specification

## Purpose
TBD - created by archiving change migrate-vm-scripts-to-go-cli. Update Purpose after archive.
## Requirements
### Requirement: Single binary with subcommands
`vmctl` SHALL expose `setup`, `cleanup`, `backup` (with `snapshot`, `backup`, `restore`, and `list` sub-subcommands), and `doctor` (with plain, `--fix`, and `--unfix` modes) as subcommands of one compiled binary, replacing `debian-vm-setup.sh`, `debian-vm-cleanup.sh`, and `debian-vm-backup.sh`.

#### Scenario: Invoking a subcommand
- **WHEN** a user runs `vmctl setup`, `vmctl cleanup`, `vmctl backup <sub-subcommand>`, or `vmctl doctor`
- **THEN** `vmctl` performs the equivalent behavior the corresponding bash script performed, or, for `doctor` (which has no bash-script predecessor), the host-readiness check/fix/unfix behavior defined by the `vmctl-host-doctor` capability

#### Scenario: Invoking with no subcommand or an unknown one
- **WHEN** a user runs `vmctl` with no subcommand, or an unrecognized one
- **THEN** `vmctl` prints usage listing the available subcommands and exits non-zero without performing any action

### Requirement: Shared `--name` and working-directory convention
Every subcommand that targets a specific VM SHALL accept a `--name` flag (default `debian-vm`) and SHALL resolve that VM's working directory as `$HOME/vms/<name>`, using one shared implementation rather than a per-subcommand copy.

#### Scenario: Default name
- **WHEN** a subcommand is invoked without `--name`
- **THEN** it targets the VM named `debian-vm` and its working directory at `$HOME/vms/debian-vm`

#### Scenario: Explicit name
- **WHEN** a subcommand is invoked with `--name=app-01`
- **THEN** it targets the VM named `app-01` and its working directory at `$HOME/vms/app-01`

### Requirement: Shared preflight checks
Every subcommand SHALL refuse to run as root and SHALL verify `virsh` is present on `PATH` before performing any other action, using one shared implementation.

#### Scenario: Run as root
- **WHEN** any `vmctl` subcommand is invoked by the root user
- **THEN** `vmctl` exits with an error before performing any action, instructing the user to run it as a normal user

#### Scenario: `virsh` not installed
- **WHEN** any `vmctl` subcommand that requires `virsh` is invoked and `virsh` is not found on `PATH`
- **THEN** `vmctl` exits with an error before performing any action, naming `virsh` as the missing dependency

### Requirement: Shared confirmation semantics
Destructive or state-changing actions that are not already covered by an explicit non-interactive flag (e.g. `--vm-only`, `--purge-all`) SHALL prompt for interactive confirmation using one shared confirmation implementation, with an explicit parameter controlling whether the prompt is bypassed — not a function that reads global state implicitly.

#### Scenario: Interactive invocation
- **WHEN** a subcommand reaches a destructive step without a flag that marks the invocation as non-interactive
- **THEN** `vmctl` prompts the user for confirmation and proceeds only on an affirmative response

#### Scenario: Non-interactive invocation
- **WHEN** a subcommand is invoked with a flag that marks it as non-interactive (e.g. `--vm-only`, `--purge-all`)
- **THEN** `vmctl` performs the destructive step without prompting

### Requirement: Help text generated from flag declarations
Each subcommand's `--help` output SHALL be generated from the same flag declarations used for parsing, not maintained as separate hand-written text.

#### Scenario: Help requested
- **WHEN** a user runs any subcommand with `-h` or `--help`
- **THEN** `vmctl` prints a description of every flag that subcommand actually parses, sourced from the flag declarations themselves

### Requirement: Preserve actionable error messages naming the underlying `virsh` command
When an operation fails or an ambiguous state is detected, `vmctl` SHALL print the literal `virsh` (or equivalent) command the user can run to inspect the situation manually, matching the current scripts' behavior.

#### Scenario: An operation fails or state is ambiguous
- **WHEN** `vmctl` cannot complete a step (e.g. cannot start a VM, cannot determine its network interface) or reaches a state it cannot fully resolve automatically
- **THEN** the printed error names the specific `virsh` (or equivalent) command the user can run by hand to inspect further
