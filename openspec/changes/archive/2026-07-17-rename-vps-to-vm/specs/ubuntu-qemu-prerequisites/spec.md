## MODIFIED Requirements

### Requirement: Install concrete QEMU prerequisites
The Debian VM setup script SHALL install `qemu-system-x86` and `qemu-utils` through APT and SHALL NOT request `qemu-kvm` as an install target.

#### Scenario: Ubuntu exposes qemu-kvm only as a virtual package
- **WHEN** the setup script runs on an Ubuntu host where `qemu-kvm` has no installation candidate
- **THEN** APT receives the concrete `qemu-system-x86` package and can continue package installation

#### Scenario: Setup resizes the virtual disk
- **WHEN** setup reaches the disk-resize step
- **THEN** `qemu-img` is available from the explicitly declared `qemu-utils` dependency
