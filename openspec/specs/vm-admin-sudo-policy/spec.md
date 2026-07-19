# vm-admin-sudo-policy Specification

## Purpose
TBD - created by archiving change restrict-vm-admin-sudo. Update Purpose after archive.
## Requirements
### Requirement: Default sudo policy stays password-less
When `--admin-password` is not passed, setup SHALL generate cloud-init `user-data` with the admin user's sudo entry as `ALL=(ALL) NOPASSWD:ALL`, unchanged from current behavior.

#### Scenario: Setup run without the flag
- **WHEN** `debian-vm-setup.sh` is run without `--admin-password`
- **THEN** the generated `user-data` grants the admin user `sudo: ALL=(ALL) NOPASSWD:ALL`
- **AND** no password is generated, hashed, printed, or written to disk

### Requirement: Opt-in password-required sudo via --admin-password
Setup SHALL accept an `--admin-password[=PASSWORD]` flag that switches the admin user's sudo entry to password-required and sets a usable login password for local authentication.

#### Scenario: Flag passed without a value
- **WHEN** `--admin-password` is passed with no `=VALUE`
- **THEN** setup generates a random password, hashes it with `openssl passwd -6`, and sets `sudo: ALL=(ALL) ALL` and `lock_passwd: false` for the admin user in `user-data`

#### Scenario: Flag passed with an explicit value
- **WHEN** `--admin-password=VALUE` is passed
- **THEN** setup hashes `VALUE` with `openssl passwd -6` and uses it as the admin user's password, applying the same `sudo`/`lock_passwd` settings as the auto-generated case

### Requirement: SSH login stays key-only regardless of sudo policy
Setting a sudo password for the admin user SHALL NOT enable SSH password authentication.

#### Scenario: --admin-password is used
- **WHEN** setup runs with `--admin-password` (with or without an explicit value)
- **THEN** the generated `user-data` still sets `ssh_pwauth: false`

### Requirement: Generated password is surfaced once and guarded on disk
When a password is set for the admin user, setup SHALL print it once in the run's output and persist it to a permission-restricted file in the VM's working directory.

#### Scenario: VM freshly created with --admin-password
- **WHEN** setup creates a new VM with `--admin-password`
- **THEN** the plaintext password is printed once in the setup output
- **AND** it is written to `admin-password` inside the VM's working directory (`$HOME/vms/<name>/`) with permissions `600`

#### Scenario: VM freshly created without --admin-password
- **WHEN** setup creates a new VM without `--admin-password`
- **THEN** no password file is created in the VM's working directory

### Requirement: Applied sudo policy is recorded for rerun detection
Setup SHALL record which sudo policy was applied to a freshly created VM in a marker file, so later reruns can detect a mismatch without trusting the invocation's flags alone.

#### Scenario: VM freshly created
- **WHEN** setup creates a new VM, with or without `--admin-password`
- **THEN** setup writes the applied policy (`nopasswd` or `password-required`) to `.admin-sudo-policy` inside the VM's working directory

### Requirement: Warn without failing on sudo-policy mismatch on rerun
Because cloud-init `user-data` only applies at first boot, setup SHALL warn rather than fail when a rerun's requested sudo policy cannot be applied to an already-existing VM, and SHALL continue using the VM's actual (unchanged) policy.

#### Scenario: Rerun requests a different policy than the recorded one
- **WHEN** setup is rerun against an already-existing VM whose `.admin-sudo-policy` marker disagrees with the current invocation's `--admin-password` flag
- **THEN** setup prints a warning stating that sudo policy is fixed at creation, states the VM's actual policy, points to `virsh undefine --remove-all-storage` as how to change it, and continues without modifying the running VM

#### Scenario: Rerun against a VM with no recorded policy
- **WHEN** setup is rerun with `--admin-password` against an already-existing VM that has no `.admin-sudo-policy` marker (created before this feature existed)
- **THEN** setup prints a warning that the VM's actual sudo policy cannot be determined and that recreation is required to apply `--admin-password`, and continues without modifying the running VM

### Requirement: Escape hatch is documented when password-required sudo is active
When a VM is using password-required sudo, setup's final connection-info summary SHALL mention `virsh console` as a way to regain access if the password is lost.

#### Scenario: Setup completes with password-required sudo in effect
- **WHEN** setup's connection-info summary is displayed for a VM whose effective sudo policy is password-required
- **THEN** the summary includes a note that `virsh console <name>` gives host-root guest access that can reset the admin user's password (`passwd admin`) if it is lost

