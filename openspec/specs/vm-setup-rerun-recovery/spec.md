# vm-setup-rerun-recovery Specification

## Purpose

Allow the VM setup script to be re-run safely against an already-existing VM: detect the existing VM instead of failing, determine its effective network mode from libvirt rather than trusting the current invocation's flags, and reach a working connection-info summary regardless of whether the VM was freshly created or reused.

## Requirements
### Requirement: Detect an existing VM before disk and cloud-init work
Before performing SSH key handling, disk image download/copy/resize, or cloud-init generation, setup SHALL check whether a VM named `$VM_NAME` already exists.

#### Scenario: VM already exists
- **WHEN** a VM named `$VM_NAME` is already defined in libvirt
- **THEN** setup skips SSH key handling, disk image download/copy/resize, cloud-init generation, and `virt-install`

#### Scenario: VM does not exist
- **WHEN** no VM named `$VM_NAME` is defined in libvirt
- **THEN** setup proceeds through SSH key handling, disk preparation, cloud-init generation, and `virt-install` as before

### Requirement: Determine effective network mode by inspecting the VM
Setup SHALL determine a VM's effective network mode by inspecting its libvirt interface definition, not by trusting the `--bridge`/`--forward` flags passed on the current invocation.

#### Scenario: VM's interface type is direct
- **WHEN** `virsh domiflist` reports the VM's interface type as `direct`
- **THEN** setup treats the effective network mode as bridged, using the interface reported by libvirt

#### Scenario: VM's interface type is network
- **WHEN** `virsh domiflist` reports the VM's interface type as `network`
- **THEN** setup treats the effective network mode as NAT-family (plain NAT or forwarding)

#### Scenario: Interface cannot be determined
- **WHEN** `virsh domiflist` reports no interface for the VM
- **THEN** setup exits with a diagnostic before proceeding further

### Requirement: Warn without failing on network mode mismatch
When reusing an already-existing VM, setup SHALL warn rather than fail if the requested `--bridge` flag conflicts with the VM's effective network mode, and SHALL continue using the effective mode.

#### Scenario: Bridge requested but VM is NAT-family
- **WHEN** `--bridge` is passed and an already-existing VM's effective network mode is NAT-family
- **THEN** setup prints a warning that network mode is fixed at creation, states the VM's actual mode, points to `virsh undefine --remove-all-storage` as how to change it, and continues using the NAT-family mode

#### Scenario: Bridge not requested but VM is bridged
- **WHEN** `--bridge` is not passed and an already-existing VM's effective network mode is bridged
- **THEN** setup prints a warning that network mode is fixed at creation, states the VM's actual mode and interface, points to `virsh undefine --remove-all-storage` as how to change it, and continues using the bridged mode

### Requirement: Auto-start an existing VM that is not running
When reusing an already-existing VM, setup SHALL start it if it is not already running.

#### Scenario: Existing VM is not running
- **WHEN** an already-existing VM's `virsh domstate` is not `running`
- **THEN** setup starts it with `virsh start` before continuing

#### Scenario: Starting the existing VM fails
- **WHEN** `virsh start` fails for an already-existing VM
- **THEN** setup exits with an actionable error before reaching the connection-info summary

### Requirement: Configure VM autostart without blocking on failure
Setup SHALL configure `virsh autostart` for the VM, whether freshly created or already existing, treating failure as non-fatal.

#### Scenario: VM is ready
- **WHEN** the VM is ready, whether freshly created or reused from an already-existing definition
- **THEN** setup runs `virsh autostart` for the VM

#### Scenario: Autostart configuration fails
- **WHEN** `virsh autostart` fails for the VM
- **THEN** setup prints a warning and continues to the connection-info summary instead of exiting

### Requirement: Always show the final connection-info summary
Setup SHALL always reach and display the connection-info summary (VM IP lookup command, SSH command, useful `virsh` commands), regardless of whether the VM was freshly created or already existed.

#### Scenario: Completing via a freshly created VM
- **WHEN** setup completes by creating a new VM
- **THEN** it displays the connection-info summary using the VM's effective network mode

#### Scenario: Completing via an already-existing VM
- **WHEN** setup completes by reusing an already-existing VM
- **THEN** it displays the same connection-info summary using the VM's effective network mode, instead of exiting with an error

