# vm-cleanup-scope Specification

## Purpose

Define the removal scope of `debian-vm-cleanup.sh`: which invocation modes it supports, what each mode removes or preserves, and the guarantees that keep the interactive and non-interactive paths consistent with each other.

## Requirements
### Requirement: Scope flags are mutually exclusive
The cleanup script SHALL reject an invocation that passes both `--vm-only` and `--purge-all`.

#### Scenario: Both scope flags passed together
- **WHEN** the cleanup script is invoked with both `--vm-only` and `--purge-all`
- **THEN** it exits with a usage error before performing any removal

### Requirement: VM name is configurable
The cleanup script SHALL accept a `--name=<name>` argument identifying which VM's instance to target for `--vm-only`, and which VM to remove as part of `--purge-all`, using `"debian-vm"` when the argument is omitted.

#### Scenario: Name provided
- **WHEN** the cleanup script is invoked with `--name=app-01`
- **THEN** it targets the VM named `app-01` instead of the default `debian-vm`

#### Scenario: Name omitted
- **WHEN** the cleanup script is invoked without `--name`
- **THEN** it targets the VM named `debian-vm`, matching today's behavior

### Requirement: `--vm-only` removes only the VM instance
The cleanup script SHALL, when invoked with `--vm-only`, remove only the named VM's definition, its attached storage, and its network reservation, without prompting for confirmation, and SHALL NOT remove the working directory as a whole, installed packages, group membership, the default libvirt network, or the QEMU storage ACL.

#### Scenario: VM exists
- **WHEN** the cleanup script is invoked with `--vm-only` and a VM with the configured name exists
- **THEN** it stops the VM if running and removes its definition along with its attached storage (disk and cloud-init seed ISO), without asking for confirmation

#### Scenario: Base cloud image is preserved
- **WHEN** `--vm-only` removes the VM's attached storage
- **THEN** the downloaded base cloud image file in the working directory is left in place, since it was never attached to the VM as a disk

#### Scenario: No VM exists
- **WHEN** the cleanup script is invoked with `--vm-only` and no VM with the configured name exists
- **THEN** it reports that no VM was found and makes no other changes

#### Scenario: Network reservation is released
- **WHEN** `--vm-only` removes a VM that has a DHCP host reservation (IP and hostname) on the `default` network
- **THEN** it also removes that reservation, so the IP and hostname become available for reuse by another VM

### Requirement: `--purge-all` removes the entire environment
The cleanup script SHALL, when invoked with `--purge-all`, remove the VM, the entire working directory (including the base cloud image), the installed QEMU/libvirt packages, the caller's `libvirt`/`kvm` group membership, the default libvirt network, and the QEMU storage ACL grant on the caller's home directory, without prompting for confirmation for any step. The default-network removal SHALL execute before package purge, so that it is not skipped as a side effect of packages (and the `virsh` binary they provide) already having been removed earlier in the same run. Before performing any removal, the cleanup script SHALL verify that no VM other than the one named by `--name` exists, and SHALL refuse to proceed if one does.

#### Scenario: Full teardown requested
- **WHEN** the cleanup script is invoked with `--purge-all` and no other VM exists on the host
- **THEN** it removes the VM and its storage, removes the default network, deletes the working directory in full, purges the declared packages, removes the caller from the `libvirt` and `kvm` groups, and revokes the `libvirt-qemu` ACL entry on the caller's home directory, without asking for confirmation at any step

#### Scenario: ACL revocation fails
- **WHEN** `--purge-all` attempts to revoke the QEMU storage ACL and the command fails
- **THEN** the script prints a warning and continues with the remaining steps instead of aborting

#### Scenario: Network removal is not skipped by package removal
- **WHEN** `--purge-all` removes both the default network and the installed packages in the same run
- **THEN** the default network is actually removed, rather than being silently skipped because `virsh` was already purged first

#### Scenario: No default network to remove
- **WHEN** the cleanup script reaches the network-removal step and `virsh` is unavailable or the `default` network is not defined
- **THEN** it reports that there is nothing to remove for this step instead of producing no output

#### Scenario: Other VMs still exist
- **WHEN** the cleanup script is invoked with `--purge-all` and at least one VM other than the one named by `--name` still exists
- **THEN** it exits with a usage error listing the other VMs by name, directing the user to remove them first with `--vm-only`, before performing any removal

### Requirement: No-flags invocation remains interactive and reaches full-teardown parity
The cleanup script SHALL, when invoked without a scope flag, walk through each removal step individually, asking for confirmation before each one, including a step to revoke the QEMU storage ACL, so that confirming every step reaches the same end state as `--purge-all`.

#### Scenario: User confirms every step
- **WHEN** the cleanup script is invoked without `--vm-only` or `--purge-all`, and the user confirms every prompt including ACL revocation
- **THEN** the resulting system state matches what `--purge-all` produces non-interactively

#### Scenario: User declines a step
- **WHEN** the cleanup script is invoked without a scope flag and the user declines a given step's prompt
- **THEN** that step is skipped and the script continues to the next step

### Requirement: `--yes` is no longer a recognized flag
The cleanup script SHALL NOT recognize `--yes` as a valid argument.

#### Scenario: Invocation still passes --yes
- **WHEN** the cleanup script is invoked with `--yes`
- **THEN** it exits with an unknown-argument usage error directing the user to `--vm-only` or `--purge-all`

