## MODIFIED Requirements

### Requirement: Provide quick-start commands
`README.md` SHALL include copy-pasteable commands to run `debian-vm-setup.sh` with default (NAT) networking and to run `debian-vm-cleanup.sh`, plus one fleet example combining `--name` and `--ip`, plus one minimal example demonstrating that `debian-vm-backup.sh` exists and how to take a snapshot or backup of a VM's disk.

#### Scenario: Reader wants to create a VM
- **WHEN** a reader wants to provision a VM for the first time
- **THEN** the README shows a command they can copy-paste to do so with default networking

#### Scenario: Reader wants to remove a VM
- **WHEN** a reader wants to undo what setup created
- **THEN** the README shows the cleanup command to run

#### Scenario: Reader wants a second, independently-addressable VM
- **WHEN** a reader wants to provision more than one VM at once
- **THEN** the README shows an example using `--name` and `--ip` to create a distinctly named, addressable VM

#### Scenario: Reader wants to protect a VM's disk
- **WHEN** a reader wants to know whether this repo has any answer to "what if I break my VM, or lose its disk?"
- **THEN** the README shows one minimal example command invoking `debian-vm-backup.sh` (e.g., taking a snapshot or a backup), enough to establish that the script exists and where to look (`--help`) for the rest
