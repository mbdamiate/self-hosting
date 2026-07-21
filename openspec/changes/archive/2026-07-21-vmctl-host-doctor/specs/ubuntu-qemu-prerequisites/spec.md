## MODIFIED Requirements

### Requirement: Install concrete QEMU prerequisites
`vmctl doctor --fix` SHALL install `qemu-system-x86` and `qemu-utils` through APT and SHALL NOT request `qemu-kvm` as an install target.

#### Scenario: Ubuntu exposes qemu-kvm only as a virtual package
- **WHEN** `vmctl doctor --fix` runs on an Ubuntu host where `qemu-kvm` has no installation candidate
- **THEN** APT receives the concrete `qemu-system-x86` package and can continue package installation

#### Scenario: Setup resizes the virtual disk
- **WHEN** VM creation (`vmctl setup`) reaches the disk-resize step
- **THEN** `qemu-img` is available from the explicitly declared `qemu-utils` dependency, previously verified present by `vmctl setup`'s preflight

### Requirement: Clean up declared QEMU prerequisites
`vmctl doctor --unfix` SHALL include `qemu-system-x86` and `qemu-utils` in its purge list when it removes host prerequisites.

#### Scenario: User confirms dependency cleanup
- **WHEN** `vmctl doctor --unfix` runs and removes host-level prerequisites
- **THEN** it requests purge of the concrete QEMU system emulator and image utility packages installed by `vmctl doctor --fix`

## ADDED Requirements

### Requirement: Verify QEMU prerequisites before VM creation
`vmctl setup` SHALL verify that `qemu-system-x86` and `qemu-utils` (via the `qemu-img` binary) are present before performing any VM-creation work, and SHALL NOT install them.

#### Scenario: Prerequisite present
- **WHEN** `vmctl setup` runs and `qemu-img` is found on `PATH`
- **THEN** setup continues without attempting to install anything

#### Scenario: Prerequisite missing
- **WHEN** `vmctl setup` runs and `qemu-img` is not found on `PATH`
- **THEN** setup exits before any VM-creation work, naming the missing package and pointing to `vmctl doctor --fix`
