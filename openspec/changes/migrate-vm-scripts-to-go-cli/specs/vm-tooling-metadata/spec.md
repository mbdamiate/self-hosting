## ADDED Requirements

### Requirement: One consolidated metadata record per VM
`vmctl` SHALL record the guest-only facts libvirt cannot report itself — admin sudo policy, log-forwarding configuration, and guest firewall policy — as one consolidated metadata record per VM, replacing the current separate `.admin-sudo-policy`, `.log-forwarding-configured`, and `.guest-firewall-policy` files.

#### Scenario: Setup configures a guest-only fact
- **WHEN** `vmctl setup` configures admin sudo policy, log-forwarding, or guest firewall policy for a VM
- **THEN** the resulting fact is written into that VM's single consolidated metadata record

#### Scenario: Rerun reads effective configuration
- **WHEN** `vmctl setup` re-runs against an already-existing VM and needs its effective admin sudo policy, log-forwarding, or guest firewall policy
- **THEN** it reads the value from the VM's consolidated metadata record instead of separate files

### Requirement: Metadata record lifecycle matches full VM removal
The consolidated metadata record for a VM SHALL be removed when that VM is fully removed (i.e. `vmctl cleanup --purge-all`, or the equivalent full removal of the VM and its data), and SHALL survive a scoped removal that preserves the VM's working directory (i.e. `vmctl cleanup --vm-only`).

#### Scenario: Full removal
- **WHEN** `vmctl cleanup --purge-all` removes a VM and its data
- **THEN** that VM's consolidated metadata record no longer exists afterward

#### Scenario: Scoped removal
- **WHEN** `vmctl cleanup --vm-only` removes only the VM object and its attached storage
- **THEN** that VM's consolidated metadata record still exists afterward, available to a subsequent `vmctl setup` rerun

### Requirement: Missing metadata treated as unconfigured
`vmctl` SHALL treat a VM with no consolidated metadata record (e.g. one created before this record existed, or one where none of the tracked facts were ever configured) the same as one where every tracked fact is explicitly unconfigured, rather than erroring.

#### Scenario: VM has no metadata record
- **WHEN** `vmctl setup` re-runs against an existing VM that has no consolidated metadata record
- **THEN** it treats admin sudo policy, log-forwarding, and guest firewall policy as unconfigured and continues, rather than exiting with an error
