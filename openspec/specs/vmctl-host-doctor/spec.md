## Purpose

Define `vmctl doctor`'s host-readiness reporting, fixing, and unfixing behavior, and its relationship to `vmctl setup`'s preflight checks.

## Requirements

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

### Requirement: `vmctl doctor --fix`/`--unfix` refuse cleanly on a non-apt host, pointing to the report
`vmctl doctor --fix` and `vmctl doctor --unfix` SHALL refuse to run on a host without `apt`, before making any change to the system, and the refusal message SHALL direct the user to run `vmctl doctor` (no flag) as the authoritative list of what is missing, rather than leaving the user with no next step.

#### Scenario: `--fix` on a non-apt host
- **WHEN** `vmctl doctor --fix` runs on a host without `apt`
- **THEN** it exits with an error before installing or configuring anything, and the error names `vmctl doctor` as where to see exactly what's missing

#### Scenario: `--unfix` on a non-apt host
- **WHEN** `vmctl doctor --unfix` runs on a host without `apt`
- **THEN** it exits with an error before removing or reverting anything, and the error names `vmctl doctor` as where to see exactly what's missing

### Requirement: `vmctl doctor --fix` installs and configures missing prerequisites
`vmctl doctor --fix` SHALL install any missing required package, add the caller to the `libvirt` and `kvm` groups, enable and start the `libvirtd` service, ensure the libvirt `default` network is defined, active, and set to autostart, and grant the `libvirt-qemu` execute-only ACL on the caller's home directory.

#### Scenario: Host is unprovisioned
- **WHEN** `vmctl doctor --fix` runs on a host missing some or all prerequisites
- **THEN** it installs and configures each missing item, matching what `vmctl setup` did unconditionally before this change

#### Scenario: Host is already provisioned
- **WHEN** `vmctl doctor --fix` runs on a host where every prerequisite is already present
- **THEN** each step is a no-op and `vmctl doctor --fix` exits zero without error

#### Scenario: Group membership requires a fresh session
- **WHEN** `vmctl doctor --fix` adds the caller to a required group not yet active in the current session
- **THEN** it stops without reexecuting itself and instructs the user to log out/in before rerunning `vmctl setup` or `vmctl doctor`

### Requirement: `vmctl doctor --unfix` reverts what `--fix` establishes
`vmctl doctor --unfix` SHALL purge the packages `--fix` installs, remove the caller from the `libvirt` and `kvm` groups, revoke the QEMU storage ACL grant, and remove the libvirt `default` network, refusing to proceed if any VM is currently defined on the host.

#### Scenario: No VM exists
- **WHEN** `vmctl doctor --unfix` runs and no VM is defined on the host
- **THEN** it purges the packages, removes group membership, revokes the ACL, and removes the `default` network

#### Scenario: A VM still exists
- **WHEN** `vmctl doctor --unfix` runs and at least one VM is still defined on the host
- **THEN** it exits with a usage error listing the existing VM(s) and directing the user to remove them first with `vmctl cleanup`, before performing any removal

### Requirement: `--fix` and `--unfix` are mutually exclusive
`vmctl doctor` SHALL reject an invocation that passes both `--fix` and `--unfix`.

#### Scenario: Both flags passed together
- **WHEN** `vmctl doctor` is invoked with both `--fix` and `--unfix`
- **THEN** it exits with a usage error before performing any check, fix, or unfix action

### Requirement: `vmctl setup` and `vmctl doctor` share one check implementation
The host-readiness checks `vmctl doctor` reports SHALL be the same checks `vmctl setup` runs as its preflight, implemented once and called from both places.

#### Scenario: A check is added or changed
- **WHEN** a host-readiness check's logic changes
- **THEN** both `vmctl doctor`'s report and `vmctl setup`'s preflight reflect the change, since neither maintains its own copy
</content>
