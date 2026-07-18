# repository-readme Specification

## Purpose

Define what `README.md` must communicate to a first-time reader of this repo: the project's purpose and scope, the host prerequisites for running the setup script, copy-pasteable quick-start commands, and pointers to the detailed sources of truth instead of duplicating them.

## Requirements
### Requirement: Describe project purpose and scope
`README.md` SHALL state, near the top of the file, what the repo is for: locally provisioning a Debian VM via libvirt/KVM/QEMU that mimics a rented VPS, for self-hosting experiments before deploying to a real VPS.

#### Scenario: First-time reader opens the README
- **WHEN** a reader opens `README.md` without prior context on the repo
- **THEN** they can tell within the first few lines what the repo does and why it exists, without needing to open any script

### Requirement: Document host prerequisites
`README.md` SHALL list the host prerequisites needed before running the setup script: an apt-based Linux host with KVM support.

#### Scenario: Reader prepares a fresh host
- **WHEN** a reader wants to run the setup script on a host they haven't used with this repo before
- **THEN** the README tells them what the host needs to support before running the script

### Requirement: Provide quick-start commands
`README.md` SHALL include copy-pasteable commands to run `debian-vm-setup.sh` with default (NAT) networking and to run `debian-vm-cleanup.sh`, plus one fleet example combining `--name` and `--ip`.

#### Scenario: Reader wants to create a VM
- **WHEN** a reader wants to provision a VM for the first time
- **THEN** the README shows a command they can copy-paste to do so with default networking

#### Scenario: Reader wants to remove a VM
- **WHEN** a reader wants to undo what setup created
- **THEN** the README shows the cleanup command to run

#### Scenario: Reader wants a second, independently-addressable VM
- **WHEN** a reader wants to provision more than one VM at once
- **THEN** the README shows an example using `--name` and `--ip` to create a distinctly named, addressable VM

### Requirement: Point to detailed sources of truth instead of duplicating them
`README.md` SHALL NOT restate the full flag reference for either script or the detailed behavioral guarantees already captured in `openspec/specs/`; it SHALL instead point readers to each script's `--help` output for flags and to `openspec/specs/` for detailed behavior.

#### Scenario: Reader needs a flag not covered by the quick start
- **WHEN** a reader needs a flag or mode not shown in the README's quick-start examples (e.g. `--bridge`, `--forward`, `--purge-all`)
- **THEN** the README directs them to run the script with `--help` rather than listing every flag itself

#### Scenario: Reader wants to understand a specific behavioral guarantee
- **WHEN** a reader wants to know the precise rules behind a behavior (e.g. what `--purge-all` refuses to do, or how fleet IP reservation works)
- **THEN** the README directs them to the relevant capability under `openspec/specs/` rather than re-explaining the guarantee inline
