## ADDED Requirements

### Requirement: Prepare the default NAT network
When setup uses plain NAT or `--forward`, it SHALL verify that the libvirt network named `default` is defined before it invokes `virt-install`.

#### Scenario: Default network is inactive
- **WHEN** the defined `default` network is inactive during NAT-mode setup
- **THEN** setup starts it and configures it to autostart on subsequent host boots before creating the VM

#### Scenario: Default network is already active
- **WHEN** the `default` network is active during NAT-mode setup
- **THEN** setup configures it to autostart and continues without restarting the network

#### Scenario: Default network is unavailable
- **WHEN** the `default` network is undefined or cannot be started
- **THEN** setup exits before VM creation and reports how to inspect or restore the libvirt network

### Requirement: Preserve bridged-mode network isolation
The setup script SHALL NOT start or configure the `default` libvirt network when `--bridge` is selected.

#### Scenario: Bridged setup
- **WHEN** setup runs with a valid `--bridge` interface
- **THEN** it proceeds with the direct/macvtap network configuration without modifying the `default` network
