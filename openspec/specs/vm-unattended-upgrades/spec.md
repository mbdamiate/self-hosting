# vm-unattended-upgrades Specification

## Purpose
TBD - created by archiving change enable-vm-unattended-upgrades. Update Purpose after archive.
## Requirements
### Requirement: Automatic security updates are on by default
When `--no-auto-updates` is not passed, setup SHALL generate cloud-init `user-data` that installs and enables `unattended-upgrades` on the VM.

#### Scenario: Setup run without the flag
- **WHEN** `debian-vm-setup.sh` is run without `--no-auto-updates`
- **THEN** the generated `user-data` installs the `unattended-upgrades` package
- **AND** it writes an `/etc/apt/apt.conf.d/20auto-upgrades` equivalent enabling `APT::Periodic::Update-Package-Lists` and `APT::Periodic::Unattended-Upgrade`

### Requirement: Automatic updates are restricted to the security origin
Setup SHALL configure `unattended-upgrades` to only apply updates from the distribution's security origin, not the general point-release stream.

#### Scenario: Automatic updates are enabled
- **WHEN** automatic updates are enabled for a VM (the default)
- **THEN** the generated cloud-init configuration overrides `Unattended-Upgrade::Allowed-Origins` to include only the security origin pattern (`${distro_id}:${distro_codename}-security`)

### Requirement: Automatic reboot stays disabled
Setup SHALL NOT enable automatic reboot of the VM after unattended-upgrades applies an update that requires one.

#### Scenario: Automatic updates are enabled
- **WHEN** automatic updates are enabled for a VM (the default)
- **THEN** the generated cloud-init configuration sets `Unattended-Upgrade::Automatic-Reboot` to `false`

### Requirement: Opt-out via --no-auto-updates
Setup SHALL accept a `--no-auto-updates` flag that skips installing and configuring `unattended-upgrades` entirely.

#### Scenario: Flag passed
- **WHEN** `debian-vm-setup.sh` is run with `--no-auto-updates`
- **THEN** the generated `user-data` does not install `unattended-upgrades` and does not write its configuration files

### Requirement: One-time note on fresh VM creation
When automatic updates are enabled for a freshly created VM, setup SHALL print a one-time note explaining that security updates apply automatically but reboots do not.

#### Scenario: Fresh VM created with automatic updates enabled
- **WHEN** setup creates a new VM without `--no-auto-updates`
- **THEN** the setup output includes a note stating that security-origin updates will be applied automatically and that a manual reboot may still be required after some updates

#### Scenario: Fresh VM created with automatic updates disabled
- **WHEN** setup creates a new VM with `--no-auto-updates`
- **THEN** the setup output does not include the automatic-updates note

#### Scenario: Reusing an already-existing VM
- **WHEN** setup reuses an already-existing VM (regardless of the current invocation's `--no-auto-updates` flag)
- **THEN** the setup output does not repeat the automatic-updates note, since cloud-init only applied its configuration at that VM's original creation

