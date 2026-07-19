# vm-guest-firewall Specification

## Purpose
TBD - created by archiving change add-vm-guest-firewall. Update Purpose after archive.
## Requirements
### Requirement: Guest firewall is on by default
When `--no-guest-firewall` is not passed, setup SHALL generate cloud-init `user-data` that installs `ufw`, sets a default-deny inbound / default-allow outbound policy, and enables it.

#### Scenario: Setup run without the flag
- **WHEN** `debian-vm-setup.sh` is run without `--no-guest-firewall`
- **THEN** the generated `user-data` installs `ufw`
- **AND** it configures a default-deny incoming / default-allow outgoing policy
- **AND** it enables `ufw` non-interactively before first boot completes

### Requirement: SSH is always allowed through the guest firewall
Regardless of any other flag, the guest firewall SHALL allow inbound TCP traffic on port 22.

#### Scenario: Guest firewall is enabled
- **WHEN** the guest firewall is enabled for a VM (the default)
- **THEN** `ufw allow 22/tcp` (or equivalent) is applied before the firewall is enabled, so SSH access is never blocked

### Requirement: Additional guest-side ports via --allow-port
Setup SHALL accept an `--allow-port=PORT[,PORT...]` flag that opens the given TCP ports through the guest firewall, in addition to SSH.

#### Scenario: Flag passed with one or more ports
- **WHEN** `debian-vm-setup.sh` is run with `--allow-port=8080,8443`
- **THEN** the generated `user-data` allows inbound TCP on ports 8080 and 8443, in addition to 22

#### Scenario: Flag omitted
- **WHEN** `--allow-port` is not passed
- **THEN** only port 22 (plus any ports derived from `--forward`, per the next requirement) is allowed

### Requirement: VM-side --forward ports are automatically allowed
When `--forward=HOST_PORT:VM_PORT,...` is used together with the guest firewall being enabled, setup SHALL automatically allow the VM-side ports through the guest firewall without requiring a separate `--allow-port` entry.

#### Scenario: --forward used with the guest firewall enabled
- **WHEN** `debian-vm-setup.sh` is run with `--forward=2222:22,8080:80` and the guest firewall is enabled (the default)
- **THEN** the generated `user-data` allows inbound TCP on port 80 (the VM-side port from the forward rule), in addition to 22

### Requirement: Opt-out via --no-guest-firewall
Setup SHALL accept a `--no-guest-firewall` flag that skips installing and enabling `ufw` entirely. When set, `--allow-port` has no effect.

#### Scenario: Flag passed
- **WHEN** `debian-vm-setup.sh` is run with `--no-guest-firewall`
- **THEN** the generated `user-data` does not install or configure `ufw`
- **AND** any `--allow-port` value passed alongside it is ignored

### Requirement: One-time note listing allowed ports on fresh VM creation
When the guest firewall is enabled for a freshly created VM, setup SHALL print a one-time note listing which TCP ports are allowed through it.

#### Scenario: Fresh VM created with the guest firewall enabled
- **WHEN** setup creates a new VM without `--no-guest-firewall`
- **THEN** the setup output includes a note listing the allowed ports (22 plus any from `--allow-port` and derived `--forward` VM-side ports)

#### Scenario: Fresh VM created with the guest firewall disabled
- **WHEN** setup creates a new VM with `--no-guest-firewall`
- **THEN** the setup output does not include the allowed-ports note

#### Scenario: Reusing an already-existing VM
- **WHEN** setup reuses an already-existing VM
- **THEN** the setup output does not repeat the allowed-ports note, since cloud-init only applied its configuration at that VM's original creation

### Requirement: Guest firewall state is recorded for later --forward reapplication
Setup SHALL record, in the VM's working directory, whether the guest firewall was enabled at creation.

#### Scenario: Fresh VM created
- **WHEN** setup creates a new VM, with or without `--no-guest-firewall`
- **THEN** it records the guest firewall's enabled/disabled state in a marker file in the VM's working directory

### Requirement: Warn when --forward is reapplied to a guest-firewalled VM
When `--forward` is applied against an already-existing VM whose recorded guest firewall state is enabled, setup SHALL warn that the newly forwarded VM-side port is not automatically allowed through the guest's own firewall, and SHALL state the manual remediation command.

#### Scenario: --forward reapplied to an existing VM with the guest firewall enabled
- **WHEN** `--forward=HOST_PORT:VM_PORT,...` is applied against an already-existing VM whose recorded guest firewall state is enabled
- **THEN** setup prints a warning identifying the VM-side port(s) and stating that the operator must run `ufw allow <VM_PORT>/tcp` inside the guest (e.g., over SSH) for the forwarded traffic to reach a listening service
- **AND** the host-side DNAT/FORWARD-accept rules are still applied as normal

#### Scenario: --forward reapplied to an existing VM with the guest firewall disabled or no recorded state
- **WHEN** `--forward` is applied against an already-existing VM whose recorded guest firewall state is disabled, or which has no recorded state (created before this capability existed)
- **THEN** setup does not print the guest-firewall remediation warning

#### Scenario: --forward applied to a freshly created VM
- **WHEN** `--forward` is used while creating a new VM
- **THEN** no remediation warning is needed or printed, since the VM-side ports are already allowed through cloud-init per the automatic-allow requirement above

