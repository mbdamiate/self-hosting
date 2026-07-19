# vm-crash-recovery Specification

## Purpose
TBD - created by archiving change add-vm-crash-recovery. Update Purpose after archive.
## Requirements
### Requirement: Crash restart is on by default
Setup SHALL create freshly created VMs with `on_crash=restart`, so libvirt automatically restarts a domain whose QEMU process crashes, unless `--no-crash-restart` is passed.

#### Scenario: Default invocation
- **WHEN** `debian-vm-setup.sh` creates a new VM without `--no-crash-restart`
- **THEN** the VM's domain definition sets `on_crash` to `restart`

#### Scenario: QEMU process crashes
- **WHEN** a VM created with `on_crash=restart` has its QEMU process crash while the host remains up
- **THEN** libvirt automatically restarts the domain instead of leaving it stopped

### Requirement: Opt-out via --no-crash-restart
Setup SHALL accept a `--no-crash-restart` flag that creates the VM without setting `on_crash=restart`, leaving libvirt's default (stopped-on-crash) behavior in effect.

#### Scenario: Flag passed
- **WHEN** `debian-vm-setup.sh` creates a new VM with `--no-crash-restart`
- **THEN** the VM's domain definition does not set `on_crash=restart`

#### Scenario: QEMU process crashes on an opted-out VM
- **WHEN** a VM created with `--no-crash-restart` has its QEMU process crash
- **THEN** the domain is left stopped, requiring a manual `virsh start` to recover, matching libvirt's default behavior

