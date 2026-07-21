## MODIFIED Requirements

### Requirement: `--purge-all` removes the entire environment
The cleanup script SHALL, when invoked with `--purge-all`, remove the VM, the entire working directory (including the base cloud image), any host firewall hardening added by `--harden-host-firewall`, and the host-wide monitoring/logging infrastructure (template units, log receiver, its firewall rule if present, the motd alert script), offering to also delete accumulated logs, without prompting for confirmation for any step except the log-deletion offer. It SHALL NOT remove installed QEMU/libvirt packages, the caller's `libvirt`/`kvm` group membership, the default libvirt network, or the QEMU storage ACL grant — those are host-level prerequisites owned by `vmctl doctor --unfix`. Before performing any removal, the cleanup script SHALL verify that no VM other than the one named by `--name` exists, and SHALL refuse to proceed if one does.

#### Scenario: Full teardown requested
- **WHEN** the cleanup script is invoked with `--purge-all` and no other VM exists on the host
- **THEN** it removes the VM and its storage, deletes the working directory in full, removes the host firewall hardening, and removes the host-wide monitoring/logging infrastructure, without asking for confirmation at any step except whether to also delete accumulated logs

#### Scenario: Other VMs still exist
- **WHEN** the cleanup script is invoked with `--purge-all` and at least one VM other than the one named by `--name` still exists
- **THEN** it exits with a usage error listing the other VMs by name, directing the user to remove them first with `--vm-only`, before performing any removal

#### Scenario: Accumulated logs are offered for deletion
- **WHEN** `--purge-all` reaches the monitoring/logging removal step and `/var/log/self-hosting-vms/` contains data
- **THEN** it prompts specifically whether to delete accumulated logs (the one confirmation `--purge-all` still asks for), removing the host-wide monitoring/logging infrastructure regardless of the answer

### Requirement: No-flags invocation remains interactive and reaches full-teardown parity
The cleanup script SHALL, when invoked without a scope flag, walk through each removal step individually, asking for confirmation before each one, including a step to remove host firewall hardening, a step to disable that VM's monitoring timer instance, and a step to remove the host-wide monitoring/logging infrastructure (with a separate prompt for deleting accumulated logs), so that confirming every step reaches the same end state as `--purge-all`.

#### Scenario: User confirms every step
- **WHEN** the cleanup script is invoked without `--vm-only` or `--purge-all`, and the user confirms every prompt including host firewall hardening removal and monitoring/logging removal (including log deletion)
- **THEN** the resulting system state matches what `--purge-all` produces when also opting into log deletion

#### Scenario: User declines a step
- **WHEN** the cleanup script is invoked without a scope flag and the user declines a given step's prompt
- **THEN** that step is skipped and the script continues to the next step

#### Scenario: User declines only log deletion
- **WHEN** the cleanup script is invoked without a scope flag and the user confirms removing the host-wide monitoring/logging infrastructure but declines deleting accumulated logs
- **THEN** the infrastructure (timers, receiver, firewall rule, motd script) is removed while `/var/log/self-hosting-vms/` is left in place
