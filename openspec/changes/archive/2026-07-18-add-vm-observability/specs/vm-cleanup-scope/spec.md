## MODIFIED Requirements

### Requirement: `--vm-only` removes only the VM instance
The cleanup script SHALL, when invoked with `--vm-only`, remove only the named VM's definition, its attached storage, and its network reservation, disable (but not remove) that VM's uptime-monitoring timer instance if one exists, without prompting for confirmation, and SHALL NOT remove the working directory as a whole, installed packages, group membership, the default libvirt network, the QEMU storage ACL, any host firewall hardening added by `--harden-host-firewall`, the host-wide monitoring/logging infrastructure, or that VM's accumulated logs.

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

#### Scenario: Host firewall hardening is left untouched
- **WHEN** the cleanup script is invoked with `--vm-only` and the host has firewall hardening applied (the tagged SSH rule and/or a permissive forward policy)
- **THEN** it makes no changes to the host's `ufw` configuration

#### Scenario: Monitoring timer instance is disabled, logs are preserved
- **WHEN** `--vm-only` removes a VM that has an uptime-monitoring timer instance enabled
- **THEN** it disables and stops that VM's `self-hosting-vm-uptime@<name>.timer` instance
- **AND** it leaves that VM's log directory under `/var/log/self-hosting-vms/` untouched

#### Scenario: No monitoring timer instance exists
- **WHEN** `--vm-only` removes a VM that has no uptime-monitoring timer instance enabled
- **THEN** it reports there is nothing to disable for this step

### Requirement: `--purge-all` removes the entire environment
The cleanup script SHALL, when invoked with `--purge-all`, remove the VM, the entire working directory (including the base cloud image), the installed QEMU/libvirt packages, the caller's `libvirt`/`kvm` group membership, the default libvirt network, the QEMU storage ACL grant on the caller's home directory, any host firewall hardening added by `--harden-host-firewall`, and the host-wide monitoring/logging infrastructure (template units, log receiver, its firewall rule if present, the motd alert script), offering to also delete accumulated logs, without prompting for confirmation for any step except the log-deletion offer. The default-network removal SHALL execute before package purge, so that it is not skipped as a side effect of packages (and the `virsh` binary they provide) already having been removed earlier in the same run. Before performing any removal, the cleanup script SHALL verify that no VM other than the one named by `--name` exists, and SHALL refuse to proceed if one does.

#### Scenario: Full teardown requested
- **WHEN** the cleanup script is invoked with `--purge-all` and no other VM exists on the host
- **THEN** it removes the VM and its storage, removes the default network, deletes the working directory in full, purges the declared packages, removes the caller from the `libvirt` and `kvm` groups, revokes the `libvirt-qemu` ACL entry on the caller's home directory, removes the host firewall hardening, and removes the host-wide monitoring/logging infrastructure, without asking for confirmation at any step except whether to also delete accumulated logs

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

#### Scenario: Accumulated logs are offered for deletion
- **WHEN** `--purge-all` reaches the monitoring/logging removal step and `/var/log/self-hosting-vms/` contains data
- **THEN** it prompts specifically whether to delete accumulated logs (the one confirmation `--purge-all` still asks for), removing the host-wide monitoring/logging infrastructure regardless of the answer

### Requirement: No-flags invocation remains interactive and reaches full-teardown parity
The cleanup script SHALL, when invoked without a scope flag, walk through each removal step individually, asking for confirmation before each one, including a step to revoke the QEMU storage ACL, a step to remove host firewall hardening, a step to disable that VM's monitoring timer instance, and a step to remove the host-wide monitoring/logging infrastructure (with a separate prompt for deleting accumulated logs), so that confirming every step reaches the same end state as `--purge-all`.

#### Scenario: User confirms every step
- **WHEN** the cleanup script is invoked without `--vm-only` or `--purge-all`, and the user confirms every prompt including ACL revocation, host firewall hardening removal, and monitoring/logging removal (including log deletion)
- **THEN** the resulting system state matches what `--purge-all` produces when also opting into log deletion

#### Scenario: User declines a step
- **WHEN** the cleanup script is invoked without a scope flag and the user declines a given step's prompt
- **THEN** that step is skipped and the script continues to the next step

#### Scenario: User declines only log deletion
- **WHEN** the cleanup script is invoked without a scope flag and the user confirms removing the host-wide monitoring/logging infrastructure but declines deleting accumulated logs
- **THEN** the infrastructure (timers, receiver, firewall rule, motd script) is removed while `/var/log/self-hosting-vms/` is left in place
