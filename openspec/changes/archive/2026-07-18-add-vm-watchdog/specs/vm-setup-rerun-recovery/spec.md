## ADDED Requirements

### Requirement: Determine effective watchdog configuration by inspecting the VM
Setup SHALL determine a VM's effective watchdog configuration by inspecting its libvirt domain definition, not by trusting the `--watchdog` flag passed on the current invocation.

#### Scenario: VM's domain definition includes a watchdog device
- **WHEN** `virsh dumpxml` for the VM reports a `<watchdog>` device
- **THEN** setup treats the VM's effective watchdog configuration as enabled

#### Scenario: VM's domain definition has no watchdog device
- **WHEN** `virsh dumpxml` for the VM reports no `<watchdog>` device
- **THEN** setup treats the VM's effective watchdog configuration as disabled

### Requirement: Warn without failing on watchdog mismatch
When reusing an already-existing VM, setup SHALL warn rather than fail if the requested `--watchdog` flag conflicts with the VM's effective watchdog configuration, and SHALL continue using the effective configuration.

#### Scenario: Watchdog requested but VM has none
- **WHEN** `--watchdog` is passed and an already-existing VM's effective watchdog configuration is disabled
- **THEN** setup prints a warning that watchdog configuration is fixed at creation, states that the VM has no watchdog device, points to `virsh undefine --remove-all-storage` as how to change it, and continues without a watchdog

#### Scenario: Watchdog not requested but VM has one
- **WHEN** `--watchdog` is not passed and an already-existing VM's effective watchdog configuration is enabled
- **THEN** setup prints a warning that watchdog configuration is fixed at creation, states that the VM already has a watchdog device, points to `virsh undefine --remove-all-storage` as how to change it, and continues with the existing watchdog device still in place
