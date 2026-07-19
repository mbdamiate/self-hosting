## ADDED Requirements

### Requirement: Determine effective on_crash policy by inspecting the VM
Setup SHALL determine a VM's effective `on_crash` policy by inspecting its libvirt domain definition, not by trusting the `--no-crash-restart` flag passed on the current invocation.

#### Scenario: VM's domain definition sets on_crash to restart
- **WHEN** `virsh dumpxml` for the VM reports `<on_crash>restart</on_crash>`
- **THEN** setup treats the VM's effective crash-recovery policy as enabled

#### Scenario: VM's domain definition does not set on_crash to restart
- **WHEN** `virsh dumpxml` for the VM does not report `<on_crash>restart</on_crash>`
- **THEN** setup treats the VM's effective crash-recovery policy as disabled

### Requirement: Warn without failing on crash-recovery policy mismatch
When reusing an already-existing VM, setup SHALL warn rather than fail if the requested `--no-crash-restart` flag conflicts with the VM's effective crash-recovery policy, and SHALL continue using the effective policy.

#### Scenario: Restart requested (flag omitted) but VM was created without it
- **WHEN** `--no-crash-restart` is not passed and an already-existing VM's effective crash-recovery policy is disabled
- **THEN** setup prints a warning that crash-recovery policy is fixed at creation, states that the VM will stay stopped on a crash, points to `virsh undefine --remove-all-storage` as how to change it, and continues without altering the running VM

#### Scenario: Opt-out requested but VM already has restart enabled
- **WHEN** `--no-crash-restart` is passed and an already-existing VM's effective crash-recovery policy is enabled
- **THEN** setup prints a warning that crash-recovery policy is fixed at creation, states that the VM already restarts automatically on crash, points to `virsh undefine --remove-all-storage` as how to change it, and continues without altering the running VM
