## ADDED Requirements

### Requirement: List all defined VMs with live state
`vmctl list` SHALL query libvirt live and print, for every VM currently defined (running or stopped), its name, run state, RAM, vCPU count, disk size, effective network mode, and IP address (when available) — without reading or writing any persisted copy of this data.

#### Scenario: Multiple VMs defined
- **WHEN** `vmctl list` is run and two or more VMs are defined in libvirt
- **THEN** it prints one row per VM with name, run state, RAM, vCPUs, disk size, effective network mode, and IP

#### Scenario: No VMs defined
- **WHEN** `vmctl list` is run and no VM is defined in libvirt
- **THEN** it prints a message indicating no VMs exist, rather than an error

#### Scenario: A VM is stopped
- **WHEN** `vmctl list` is run and one of the defined VMs is not running
- **THEN** its row shows a stopped run state and omits the IP address instead of failing the whole command

### Requirement: Single-VM status detail
`vmctl status --name=NAME` SHALL query libvirt live and print the same fields as `vmctl list` for exactly the named VM.

#### Scenario: Named VM exists
- **WHEN** `vmctl status --name=app-01` is run and `app-01` is defined in libvirt
- **THEN** it prints `app-01`'s run state, RAM, vCPUs, disk size, effective network mode, and IP

#### Scenario: Named VM does not exist
- **WHEN** `vmctl status --name=app-01` is run and no VM named `app-01` is defined
- **THEN** it exits with an error naming `app-01` as not found, without printing partial data

### Requirement: No caching of libvirt-owned state
`vmctl list` and `vmctl status` SHALL NOT persist the data they display between invocations; every invocation SHALL re-query libvirt for current state.

#### Scenario: State changes between invocations
- **WHEN** a VM's run state or resources change between two `vmctl list` invocations (e.g. it crashes, or is resized by hand via `virsh`)
- **THEN** the next `vmctl list` invocation reflects the new state without requiring any cache invalidation step
