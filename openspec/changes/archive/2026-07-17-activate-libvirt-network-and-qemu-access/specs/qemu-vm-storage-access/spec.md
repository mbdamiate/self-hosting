## ADDED Requirements

### Requirement: Permit QEMU access to user-owned VM storage
Before creating VM storage below the caller's home directory, setup SHALL ensure that the `libvirt-qemu` account has execute-only ACL access on the caller's home directory.

#### Scenario: ACL support is available
- **WHEN** setup prepares to create VM disk and seed files below `$HOME/vms`
- **THEN** it installs the required ACL utility if needed and grants `libvirt-qemu` execute-only traversal access to `$HOME`

#### Scenario: Storage access cannot be granted
- **WHEN** the expected QEMU account is unavailable or the ACL command fails
- **THEN** setup exits before VM creation with an actionable error describing the storage access prerequisite

### Requirement: Limit the storage permission grant
The setup script SHALL NOT grant read, write, or directory-listing access on the caller's home directory to `libvirt-qemu` or to all local users.

#### Scenario: QEMU traversal ACL is applied
- **WHEN** setup grants access to the caller's home directory
- **THEN** the ACL entry for `libvirt-qemu` contains execute permission only
