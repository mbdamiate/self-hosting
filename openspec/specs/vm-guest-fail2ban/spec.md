# vm-guest-fail2ban Specification

## Purpose
TBD - created by archiving change add-vm-guest-firewall. Update Purpose after archive.
## Requirements
### Requirement: fail2ban's sshd jail is always enabled
Setup SHALL generate cloud-init `user-data` that installs `fail2ban` and enables its `sshd` jail, regardless of `--no-guest-firewall`.

#### Scenario: Setup run with default flags
- **WHEN** `debian-vm-setup.sh` is run without any fail2ban-related flag
- **THEN** the generated `user-data` installs `fail2ban`
- **AND** it writes a `jail.local` configuration enabling the `sshd` jail

#### Scenario: Guest firewall disabled via --no-guest-firewall
- **WHEN** `debian-vm-setup.sh` is run with `--no-guest-firewall`
- **THEN** `fail2ban` is still installed and its `sshd` jail is still enabled, unaffected by that flag

### Requirement: fail2ban configuration is explicit, not left to package defaults
Setup SHALL write an explicit `jail.local` enabling the `sshd` jail rather than relying on the `fail2ban` package's own non-interactive install defaults.

#### Scenario: fail2ban is installed
- **WHEN** cloud-init installs `fail2ban` during first boot
- **THEN** a `jail.local` file setting `[sshd]` `enabled = true` is present, so the jail is active regardless of the package's shipped default

