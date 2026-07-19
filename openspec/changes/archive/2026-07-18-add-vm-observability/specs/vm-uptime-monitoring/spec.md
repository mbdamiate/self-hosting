## ADDED Requirements

### Requirement: Monitoring is opt-in via --monitor
Setup SHALL accept a `--monitor` flag that installs host-wide monitoring infrastructure (if not already present) and enables uptime monitoring for the VM being created. Without the flag, setup SHALL NOT install or enable any monitoring for that VM.

#### Scenario: Flag passed
- **WHEN** `debian-vm-setup.sh` is run with `--monitor`
- **THEN** the host-wide `self-hosting-vm-uptime@.service`/`.timer` template units are installed if not already present
- **AND** the timer instance for this VM's name is enabled and started

#### Scenario: Flag omitted
- **WHEN** `debian-vm-setup.sh` is run without `--monitor`
- **THEN** no monitoring timer instance is enabled for this VM, and host-wide monitoring infrastructure is not installed if it wasn't already

### Requirement: Health check combines domstate and SSH reachability
The uptime check for a VM SHALL treat the VM as down if `virsh domstate` is not `running`, or if `virsh domstate` is `running` but a TCP connection to the guest's SSH port cannot be established within a short timeout.

#### Scenario: VM's domain is not running
- **WHEN** a scheduled health check runs and `virsh domstate <name>` reports a state other than `running`
- **THEN** the VM is treated as down

#### Scenario: VM's domain is running but SSH is unreachable
- **WHEN** a scheduled health check runs, `virsh domstate <name>` reports `running`, and a TCP connection to the guest's SSH port fails within the check's timeout
- **THEN** the VM is treated as down

#### Scenario: VM's domain is running and SSH is reachable
- **WHEN** a scheduled health check runs, `virsh domstate <name>` reports `running`, and a TCP connection to the guest's SSH port succeeds
- **THEN** the VM is treated as up

### Requirement: Alerts fire only on state transitions
The uptime check SHALL only trigger a local alert when a VM's status changes from up to down or from down to up, not on every check while the status is unchanged.

#### Scenario: Status is unchanged from the previous check
- **WHEN** a scheduled health check's result matches the VM's last recorded status
- **THEN** no alert is triggered

#### Scenario: Status changes from up to down
- **WHEN** a scheduled health check finds the VM down after it was previously recorded as up (or with no prior recorded status established this boot)
- **THEN** a local alert is triggered reporting the VM as down

#### Scenario: Status changes from down to up
- **WHEN** a scheduled health check finds the VM up after it was previously recorded as down
- **THEN** a local alert is triggered reporting the VM as recovered

### Requirement: Last-known status does not survive a host reboot
The uptime check SHALL store each VM's last-known status in a location that is cleared on host reboot, so the first check after a host restart establishes a fresh baseline instead of firing a spurious transition alert.

#### Scenario: Host has just rebooted
- **WHEN** the first scheduled health check for a VM runs after a host reboot
- **THEN** it records the observed status as the new baseline without triggering a transition alert, regardless of what the status was before the reboot
