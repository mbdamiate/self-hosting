## ADDED Requirements

### Requirement: Scope flags are mutually exclusive
The cleanup script SHALL reject an invocation that passes both `--vm-only` and `--purge-all`.

#### Scenario: Both scope flags passed together
- **WHEN** the cleanup script is invoked with both `--vm-only` and `--purge-all`
- **THEN** it exits with a usage error before performing any removal

### Requirement: `--vm-only` removes only the VM instance
The cleanup script SHALL, when invoked with `--vm-only`, remove only the named VM's definition and its attached storage, without prompting for confirmation, and SHALL NOT remove the working directory as a whole, installed packages, group membership, the default libvirt network, or the QEMU storage ACL.

#### Scenario: VM exists
- **WHEN** the cleanup script is invoked with `--vm-only` and a VM with the configured name exists
- **THEN** it stops the VM if running and removes its definition along with its attached storage (disk and cloud-init seed ISO), without asking for confirmation

#### Scenario: Base cloud image is preserved
- **WHEN** `--vm-only` removes the VM's attached storage
- **THEN** the downloaded base cloud image file in the working directory is left in place, since it was never attached to the VM as a disk

#### Scenario: No VM exists
- **WHEN** the cleanup script is invoked with `--vm-only` and no VM with the configured name exists
- **THEN** it reports that no VM was found and makes no other changes

### Requirement: `--purge-all` removes the entire environment
The cleanup script SHALL, when invoked with `--purge-all`, remove the VM, the entire working directory (including the base cloud image), the installed QEMU/libvirt packages, the caller's `libvirt`/`kvm` group membership, the default libvirt network, and the QEMU storage ACL grant on the caller's home directory, without prompting for confirmation for any step.

#### Scenario: Full teardown requested
- **WHEN** the cleanup script is invoked with `--purge-all`
- **THEN** it removes the VM and its storage, deletes the working directory in full, purges the declared packages, removes the caller from the `libvirt` and `kvm` groups, removes the default network, and revokes the `libvirt-qemu` ACL entry on the caller's home directory, without asking for confirmation at any step

#### Scenario: ACL revocation fails
- **WHEN** `--purge-all` attempts to revoke the QEMU storage ACL and the command fails
- **THEN** the script prints a warning and continues with the remaining steps instead of aborting

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
