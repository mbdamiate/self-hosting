## ADDED Requirements

### Requirement: Apply port-forwarding rules idempotently
The setup script SHALL avoid creating duplicate `iptables` DNAT/FORWARD rules when the same `--forward` rule is applied more than once.

#### Scenario: Rule already present
- **WHEN** a requested host-port-to-VM-port forwarding rule already exists in `iptables`
- **THEN** setup detects the existing rule and does not add a duplicate

#### Scenario: Rule not yet present
- **WHEN** a requested forwarding rule does not exist in `iptables`
- **THEN** setup adds the DNAT and FORWARD-accept rules as before

### Requirement: Support --forward for an already-existing VM
Setup SHALL apply `--forward` rules against an already-existing VM, provided its effective network mode is NAT-family, as an officially supported flow rather than a create-only side effect.

#### Scenario: Reapplying forward to an existing NAT-family VM
- **WHEN** `--forward` is passed and an already-existing VM's effective network mode is NAT-family
- **THEN** setup detects the VM's current IP and applies (or confirms) the requested forwarding rules

#### Scenario: Forward requested for a bridged VM
- **WHEN** `--forward` is passed but the target VM's effective network mode is bridged
- **THEN** setup skips forwarding rule application and explains that port forwarding requires the NAT network
