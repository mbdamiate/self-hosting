## ADDED Requirements

### Requirement: Prevent setup self-reexecution for group activation
The local VPS setup script SHALL NOT reexecute itself through `sg` or another shell-based group-switching mechanism after adding the caller to host groups.

#### Scenario: Newly added group is absent from the current session
- **WHEN** setup adds the caller to a required host group and that group is not active in the current process
- **THEN** setup SHALL stop without rerunning package installation or restarting itself

### Requirement: Require an active libvirt and KVM session
The local VPS setup script SHALL verify that both `libvirt` and `kvm` are active in the current process before it performs unprivileged libvirt or VM-creation operations.

#### Scenario: Login session has not been refreshed
- **WHEN** either `libvirt` or `kvm` is absent from the current process group list after setup updates group membership
- **THEN** setup SHALL exit before creating VM files or a VM and instruct the user to log out/in and rerun the script

#### Scenario: Login session has both required groups
- **WHEN** both `libvirt` and `kvm` are active in the current process group list
- **THEN** setup SHALL continue to the SSH key, cloud image, and VM-creation steps without self-reexecution
