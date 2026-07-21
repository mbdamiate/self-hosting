## Purpose

Ensure the `libvirt-qemu` account can traverse into a caller's home directory to access VM storage under `$HOME/vms`, using the minimum ACL grant necessary.

## Requirements

### Requirement: Permit QEMU access to user-owned VM storage
`vmctl doctor --fix` SHALL ensure that the `libvirt-qemu` account has execute-only ACL access on the caller's home directory.

#### Scenario: ACL support is available
- **WHEN** `vmctl doctor --fix` runs
- **THEN** it installs the required ACL utility if needed and grants `libvirt-qemu` execute-only traversal access to `$HOME`

#### Scenario: Storage access cannot be granted
- **WHEN** the expected QEMU account is unavailable or the ACL command fails during `vmctl doctor --fix`
- **THEN** it reports an actionable error describing the storage access prerequisite

### Requirement: Limit the storage permission grant
`vmctl doctor --fix` SHALL NOT grant read, write, or directory-listing access on the caller's home directory to `libvirt-qemu` or to all local users.

#### Scenario: QEMU traversal ACL is applied
- **WHEN** `vmctl doctor --fix` grants access to the caller's home directory
- **THEN** the ACL entry for `libvirt-qemu` contains execute permission only

### Requirement: Verify QEMU storage access before VM creation
Before creating VM storage below the caller's home directory, `vmctl setup` SHALL verify that the `libvirt-qemu` account already has execute-only ACL access on the caller's home directory, and SHALL NOT grant it.

#### Scenario: ACL already granted
- **WHEN** `vmctl setup` runs and the `libvirt-qemu` ACL entry is already present on `$HOME`
- **THEN** it continues without modifying the ACL

#### Scenario: ACL missing
- **WHEN** `vmctl setup` runs and the `libvirt-qemu` ACL entry is absent from `$HOME`
- **THEN** it exits before VM creation with an actionable error naming the missing grant and pointing to `vmctl doctor --fix`
</content>
