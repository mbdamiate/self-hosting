## Purpose

Define how the local VM setup handles libvirt and KVM group memberships without restarting itself or creating resources from an unprepared session.
## Requirements
### Requirement: Prevent doctor --fix self-reexecution for group activation
`vmctl doctor --fix` SHALL NOT reexecute itself through `sg` or another shell-based group-switching mechanism after adding the caller to host groups.

#### Scenario: Newly added group is absent from the current session
- **WHEN** `vmctl doctor --fix` adds the caller to a required host group and that group is not active in the current process
- **THEN** `vmctl doctor --fix` SHALL stop without rerunning package installation or restarting itself, reporting that a fresh login session is needed

### Requirement: Require an active libvirt and KVM session
`vmctl`'s host-readiness check SHALL verify that both `libvirt` and `kvm` are active in the current process before `vmctl setup` performs unprivileged libvirt or VM-creation operations, and SHALL report the same check as part of `vmctl doctor`'s report.

#### Scenario: Login session has not been refreshed
- **WHEN** `vmctl setup` runs and either `libvirt` or `kvm` is absent from the current process's group list
- **THEN** setup SHALL exit before creating VM files or a VM and instruct the user to log out/in and rerun

#### Scenario: Login session has both required groups
- **WHEN** both `libvirt` and `kvm` are active in the current process group list
- **THEN** `vmctl setup` SHALL continue to the SSH key, cloud image, and VM-creation steps

#### Scenario: doctor reports session staleness distinctly from absence
- **WHEN** `vmctl doctor` runs and the caller is a member of `libvirt`/`kvm` per the system's group database but the current session predates that membership
- **THEN** it reports that membership exists but the session needs a fresh login, distinct from reporting the group as entirely absent
</content>
