## ADDED Requirements

### Requirement: VM name is configurable
The setup script SHALL accept a `--name=<name>` argument that overrides the default VM name/hostname used to create and manage the VM, and SHALL use `"debian-vps"` when the argument is omitted.

#### Scenario: Name provided
- **WHEN** the setup script is invoked with `--name=app-01`
- **THEN** it creates and manages a VM named and hosted as `app-01` instead of the default `debian-vps`

#### Scenario: Name omitted
- **WHEN** the setup script is invoked without `--name`
- **THEN** it creates and manages a VM named `debian-vps`, matching today's behavior

### Requirement: VM sizing is configurable
The setup script SHALL accept `--ram=<megabytes>`, `--vcpus=<count>`, and `--disk=<gigabytes>` arguments overriding the VM's memory, vCPU count, and disk size, and SHALL use today's defaults (2048 MB, 2 vCPUs, 20 GB) for any of the three left unset, without prompting for the missing values.

#### Scenario: Sizing flags provided
- **WHEN** the setup script is invoked with `--ram=4096 --vcpus=4 --disk=40`
- **THEN** the VM is created with 4096 MB RAM, 4 vCPUs, and a 40 GB disk

#### Scenario: Sizing flags omitted
- **WHEN** the setup script is invoked without `--ram`, `--vcpus`, or `--disk`
- **THEN** the VM is created with 2048 MB RAM, 2 vCPUs, and a 20 GB disk, without asking the user to supply values

### Requirement: Static IP and hostname reservation for NAT-family VMs
The setup script SHALL accept an optional `--ip=<address>` argument for plain-NAT or `--forward` invocations, reserving that address (and a resolvable hostname matching the VM's name) on the `default` libvirt network before creating the VM, so other VMs on the same network can reach it by name.

#### Scenario: --ip combined with --bridge
- **WHEN** the setup script is invoked with both `--ip` and `--bridge`
- **THEN** it exits with a usage error before creating anything, since bridged VMs do not use the `default` network's DHCP/DNS

#### Scenario: Reservation precedes VM creation
- **WHEN** the setup script creates a NAT-family VM with a resolved IP (supplied or auto-picked)
- **THEN** it registers the IP, hostname, and a chosen MAC address as a DHCP host reservation on the `default` network before invoking `virt-install`, and creates the VM's network interface with that same MAC address

#### Scenario: Reserved hostname is resolvable
- **WHEN** a fleet VM has a reserved hostname
- **THEN** other VMs on the same `default` network can resolve that hostname to its reserved IP via the network's own DNS, without manual `/etc/hosts` configuration

### Requirement: Explicit IP address is validated before use
When `--ip=<address>` is supplied, the setup script SHALL reject the address if it is already reserved for another VM or currently under an active DHCP lease, before performing any VM or disk work.

#### Scenario: Address already reserved
- **WHEN** the supplied `--ip` address already has a DHCP host reservation for a different VM
- **THEN** the setup script exits with an error identifying the VM the address is already reserved for, before creating anything

#### Scenario: Address under an active lease
- **WHEN** the supplied `--ip` address has no static reservation but currently has an active DHCP lease
- **THEN** the setup script exits with an error before creating anything

#### Scenario: Address is free
- **WHEN** the supplied `--ip` address has no reservation and no active lease
- **THEN** the setup script proceeds to reserve and use that address

### Requirement: Free IP address is auto-selected when omitted
When `--ip` is omitted for a NAT-family invocation, the setup script SHALL select the first address within the `default` network's own configured DHCP range that has no existing reservation and no active lease.

#### Scenario: Auto-pick finds a free address
- **WHEN** `--ip` is omitted and at least one address in the network's configured DHCP range has neither a reservation nor an active lease
- **THEN** the setup script reserves and uses the first such free address it finds

#### Scenario: Range introspected, not hardcoded
- **WHEN** the setup script auto-picks an address
- **THEN** it determines the address range to scan by reading the `default` network's own configuration rather than assuming a fixed subnet
