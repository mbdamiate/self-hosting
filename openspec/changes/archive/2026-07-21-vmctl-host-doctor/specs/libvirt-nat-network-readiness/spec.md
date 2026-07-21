## MODIFIED Requirements

### Requirement: Prepare the default NAT network
`vmctl doctor --fix` SHALL ensure that the libvirt network named `default` is defined, active, and set to autostart.

#### Scenario: Default network is inactive
- **WHEN** `vmctl doctor --fix` runs and the defined `default` network is inactive
- **THEN** it starts it and configures it to autostart on subsequent host boots

#### Scenario: Default network is already active
- **WHEN** `vmctl doctor --fix` runs and the `default` network is active
- **THEN** it configures it to autostart and continues without restarting the network

#### Scenario: Default network is unavailable
- **WHEN** `vmctl doctor --fix` runs and the `default` network is undefined or cannot be started
- **THEN** it stops and reports how to inspect or restore the libvirt network

## ADDED Requirements

### Requirement: Verify the default NAT network before VM creation
When `vmctl setup` uses plain NAT or `--forward`, it SHALL verify that the libvirt network named `default` is defined, active, and set to autostart before invoking `virt-install`, and SHALL NOT start, define, or configure it.

#### Scenario: Default network is ready
- **WHEN** `vmctl setup` runs in NAT mode and the `default` network is defined, active, and set to autostart
- **THEN** it continues to VM creation without modifying the network

#### Scenario: Default network is not ready
- **WHEN** `vmctl setup` runs in NAT mode and the `default` network is undefined, inactive, or not set to autostart
- **THEN** it exits before VM creation, reports the specific gap, and points to `vmctl doctor --fix`
