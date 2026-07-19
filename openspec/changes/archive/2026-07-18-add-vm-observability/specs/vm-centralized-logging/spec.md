## ADDED Requirements

### Requirement: Log forwarding is installed alongside monitoring, for NAT-family VMs
When `--monitor` is passed for a VM using plain NAT or `--forward` networking, setup SHALL configure the guest to forward its logs to a host-side receiver, and SHALL install that host-side receiver (once, host-wide) if not already present.

#### Scenario: --monitor used with plain NAT
- **WHEN** `debian-vm-setup.sh` is run with `--monitor` and no `--bridge`
- **THEN** the generated cloud-init `user-data` configures `rsyslog` to forward the guest's logs to the host's NAT gateway address
- **AND** the host-side `rsyslog` receiver is installed if not already present

#### Scenario: --monitor used with --forward
- **WHEN** `debian-vm-setup.sh` is run with `--monitor` and `--forward`
- **THEN** log forwarding is configured the same as the plain-NAT case, since `--forward` is still NAT-family networking

### Requirement: Centralized logging does not apply to --bridge mode
Setup SHALL NOT configure guest-to-host log forwarding when `--bridge` is used, and SHALL print a note explaining why when `--monitor` and `--bridge` are combined.

#### Scenario: --monitor used with --bridge
- **WHEN** `debian-vm-setup.sh` is run with both `--monitor` and `--bridge`
- **THEN** uptime monitoring is still enabled for the VM
- **AND** log forwarding is not configured
- **AND** the setup output includes a note that centralized logging is unavailable in bridged mode due to macvtap isolation

### Requirement: Host-side receiver is scoped to the NAT bridge interface
The host-side log receiver SHALL bind only to the libvirt NAT bridge interface's address, never to all interfaces.

#### Scenario: Receiver is installed
- **WHEN** the host-side `rsyslog` receiver is installed
- **THEN** it is configured to listen only on the `virbr0` interface's address, not `0.0.0.0` or any other interface

### Requirement: Receiver port is reachable through an active host firewall
When the host firewall (`ufw`) is active at the time the receiver is installed or a VM's log forwarding is configured, setup SHALL add a rule allowing inbound traffic to the receiver's port on the `virbr0` interface only.

#### Scenario: ufw is active
- **WHEN** log forwarding is configured and `ufw` is currently active on the host (regardless of how it was enabled)
- **THEN** setup adds a rule allowing inbound TCP traffic to the receiver's port, scoped to arriving on `virbr0`

#### Scenario: ufw is not active
- **WHEN** log forwarding is configured and `ufw` is not active on the host
- **THEN** no firewall rule is added

### Requirement: Logs are stored per VM with rotation
The host-side receiver SHALL store each VM's forwarded logs under a directory keyed by that VM's hostname, and SHALL be subject to log rotation.

#### Scenario: Guest forwards a log line
- **WHEN** a VM configured for log forwarding emits a log line
- **THEN** it is written to `/var/log/self-hosting-vms/<vm-hostname>/` on the host

#### Scenario: Logs accumulate over time
- **WHEN** a VM's forwarded logs grow over time
- **THEN** `logrotate` is configured to rotate them, bounding disk usage
