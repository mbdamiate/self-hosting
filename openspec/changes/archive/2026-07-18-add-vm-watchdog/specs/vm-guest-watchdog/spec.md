## ADDED Requirements

### Requirement: Watchdog device is opt-in via --watchdog
Setup SHALL accept a `--watchdog` flag that attaches a virtual watchdog device (model `i6300esb`) to the VM at creation, configured to reset the VM when triggered. Without the flag, setup SHALL NOT attach a watchdog device.

#### Scenario: Flag passed
- **WHEN** `debian-vm-setup.sh` is run with `--watchdog` and creates a new VM
- **THEN** the VM is created with an `i6300esb` watchdog device configured with a reset action

#### Scenario: Flag omitted
- **WHEN** `debian-vm-setup.sh` is run without `--watchdog` and creates a new VM
- **THEN** the VM is created with no watchdog device

### Requirement: Guest is configured to pet the watchdog via systemd
When the watchdog device is attached, setup SHALL configure the guest, via cloud-init, to enable systemd's `RuntimeWatchdogSec` so PID 1 pets the device automatically.

#### Scenario: Watchdog is enabled
- **WHEN** `--watchdog` is passed for a freshly created VM
- **THEN** the generated cloud-init `user-data` writes a systemd configuration drop-in enabling `RuntimeWatchdogSec`

#### Scenario: Watchdog is not enabled
- **WHEN** `--watchdog` is not passed
- **THEN** no systemd watchdog configuration is written

### Requirement: Watchdog action is fixed at reset
The watchdog device's configured action SHALL be `reset` (the VM is reset) and SHALL NOT be exposed as a configurable value.

#### Scenario: Watchdog fires
- **WHEN** the guest stops petting the watchdog for longer than its configured timeout
- **THEN** the hypervisor resets the VM
