## MODIFIED Requirements

### Requirement: `vmctl doctor` reports full host readiness without mutating state
`vmctl doctor`, invoked with no flag, SHALL check every host-level prerequisite (required packages, `libvirt`/`kvm` group membership, the `libvirtd` service, the libvirt `default` network, and the QEMU storage ACL) and print an OK/MISSING result for each, without stopping at the first failure and without making any change to the system. This report SHALL NOT require an apt/dpkg-based host to produce a complete, accurate result — every check SHALL verify the underlying binary, group, service, network, or ACL state directly, so the report is meaningful on any Linux distro.

#### Scenario: All prerequisites present
- **WHEN** `vmctl doctor` runs on a fully-provisioned host
- **THEN** it reports every checked item as OK and exits zero

#### Scenario: Some prerequisites missing
- **WHEN** `vmctl doctor` runs and one or more prerequisites are missing or misconfigured
- **THEN** it reports each missing/misconfigured item individually, continues checking the remaining items instead of stopping at the first failure, and exits non-zero

#### Scenario: Non-apt host with prerequisites present
- **WHEN** `vmctl doctor` runs on a host without `apt` (e.g. Fedora, Arch, openSUSE) where the required binaries, group membership, service, network, and ACL are all already present
- **THEN** it reports every checked item as OK and exits zero, without any check failing solely because `apt`/`dpkg` is absent

## ADDED Requirements

### Requirement: `vmctl doctor --fix`/`--unfix` refuse cleanly on a non-apt host, pointing to the report
`vmctl doctor --fix` and `vmctl doctor --unfix` SHALL refuse to run on a host without `apt`, before making any change to the system, and the refusal message SHALL direct the user to run `vmctl doctor` (no flag) as the authoritative list of what is missing, rather than leaving the user with no next step.

#### Scenario: `--fix` on a non-apt host
- **WHEN** `vmctl doctor --fix` runs on a host without `apt`
- **THEN** it exits with an error before installing or configuring anything, and the error names `vmctl doctor` as where to see exactly what's missing

#### Scenario: `--unfix` on a non-apt host
- **WHEN** `vmctl doctor --unfix` runs on a host without `apt`
- **THEN** it exits with an error before removing or reverting anything, and the error names `vmctl doctor` as where to see exactly what's missing
