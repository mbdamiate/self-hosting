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

### Requirement: `--yes` is no longer a recognized flag
The cleanup script SHALL NOT recognize `--yes` as a valid argument.

#### Scenario: Invocation still passes --yes
- **WHEN** the cleanup script is invoked with `--yes`
- **THEN** it exits with an unknown-argument usage error directing the user to `--vm-only` or `--purge-all`

### Requirement: Host firewall hardening removal targets only what this repo added
When removing host firewall hardening (`--purge-all`, or interactively with confirmation), the cleanup script SHALL only remove the SSH rule it can identify by its comment tag and SHALL only revert `/etc/default/ufw`'s forward policy to `DROP` if it currently reads `ACCEPT`. It SHALL NOT disable, uninstall, or otherwise reset `ufw` as a whole.

#### Scenario: Tagged SSH rule exists
- **WHEN** cleanup removes host firewall hardening and a `ufw` rule tagged with this repo's identifying comment exists
- **THEN** that specific rule is deleted

#### Scenario: No tagged SSH rule exists
- **WHEN** cleanup removes host firewall hardening and no rule with the identifying comment is found
- **THEN** cleanup reports there is nothing to remove for this step and makes no `ufw` rule changes

#### Scenario: Forward policy is currently permissive
- **WHEN** cleanup removes host firewall hardening and `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY` is `ACCEPT`
- **THEN** it is reverted to `DROP` and `ufw` is reloaded

#### Scenario: Forward policy is already restrictive
- **WHEN** cleanup removes host firewall hardening and `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY` is already `DROP`
- **THEN** cleanup leaves it unchanged

#### Scenario: ufw itself is left installed and enabled
- **WHEN** cleanup removes host firewall hardening, regardless of mode
- **THEN** `ufw` remains installed and enabled on the host; only the tagged rule and the forward-policy value are affected

### Requirement: Cleanup never deletes backup files
Neither `--vm-only` nor `--purge-all`, nor the interactive walkthrough, SHALL delete any file under a VM's backup destination (`vm-disk-backup`'s `$HOME/vm-backups/<vm-name>/` by default, or a custom `--dest`).

#### Scenario: --vm-only removes a VM with existing backups
- **WHEN** `debian-vm-cleanup.sh --vm-only` removes a VM that has backups at its backup destination
- **THEN** those backup files are left untouched

#### Scenario: --purge-all removes a VM with existing backups
- **WHEN** `debian-vm-cleanup.sh --purge-all` removes a VM that has backups at its backup destination
- **THEN** those backup files are left untouched

### Requirement: Removing a VM implicitly removes its active snapshot
Because a VM's active snapshot (per `vm-disk-snapshot`) is stored as part of its attached disk storage, removing the VM's storage (`--vm-only` or `--purge-all`) SHALL implicitly remove any active snapshot along with it — no separate snapshot-removal step is needed.

#### Scenario: VM has an active snapshot when removed
- **WHEN** `debian-vm-cleanup.sh` removes a VM (`--vm-only` or `--purge-all`) that has an active snapshot
- **THEN** the snapshot's overlay file is removed as part of the VM's attached storage, with no separate confirmation step for it

