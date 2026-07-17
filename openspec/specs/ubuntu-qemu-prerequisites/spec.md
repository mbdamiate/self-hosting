## Purpose

Define the QEMU package prerequisites for running and cleaning up the local Debian VPS setup on apt-based Ubuntu hosts.

## Requirements

### Requirement: Install concrete QEMU prerequisites
The Debian VPS setup script SHALL install `qemu-system-x86` and `qemu-utils` through APT and SHALL NOT request `qemu-kvm` as an install target.

#### Scenario: Ubuntu exposes qemu-kvm only as a virtual package
- **WHEN** the setup script runs on an Ubuntu host where `qemu-kvm` has no installation candidate
- **THEN** APT receives the concrete `qemu-system-x86` package and can continue package installation

#### Scenario: Setup resizes the virtual disk
- **WHEN** setup reaches the disk-resize step
- **THEN** `qemu-img` is available from the explicitly declared `qemu-utils` dependency

### Requirement: Clean up declared QEMU prerequisites
The cleanup script SHALL include `qemu-system-x86` and `qemu-utils` in its purge list when the user confirms package removal.

#### Scenario: User confirms dependency cleanup
- **WHEN** the cleanup script is run and the user confirms package removal
- **THEN** it requests purge of the concrete QEMU system emulator and image utility packages installed by setup
