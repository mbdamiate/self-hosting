## ADDED Requirements

### Requirement: Host firewall hardening is opt-in via --harden-host-firewall
Setup SHALL accept a `--harden-host-firewall` flag that installs and enables `ufw` on the host with a default-deny inbound / default-allow outbound policy. Without the flag, setup SHALL NOT install, enable, or otherwise modify `ufw` on the host.

#### Scenario: Flag passed
- **WHEN** `debian-vm-setup.sh` is run with `--harden-host-firewall`
- **THEN** `ufw` is installed on the host if not already present
- **AND** it is configured with a default-deny incoming / default-allow outgoing policy
- **AND** it is enabled non-interactively

#### Scenario: Flag omitted
- **WHEN** `debian-vm-setup.sh` is run without `--harden-host-firewall`
- **THEN** no host firewall changes are made

### Requirement: Host SSH is always allowed before enabling
Before enabling the host firewall, setup SHALL add a rule allowing inbound TCP traffic on port 22, tagged with an identifying comment.

#### Scenario: Host firewall hardening is applied
- **WHEN** `--harden-host-firewall` is passed
- **THEN** `ufw allow 22/tcp` with a comment identifying it as added by this repo is applied before `ufw` is enabled, so host SSH access is never blocked

### Requirement: Forward chain policy stays permissive
Setup SHALL ensure the host's forward-chain policy remains permissive so libvirt NAT/bridge forwarding and the existing `--forward` port-forwarding mechanism continue to work.

#### Scenario: Host firewall hardening is applied and the forward policy is currently restrictive
- **WHEN** `--harden-host-firewall` is passed and `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY` is `DROP`
- **THEN** setup changes it to `ACCEPT` and reloads `ufw` for the change to take effect

#### Scenario: Host firewall hardening is applied and the forward policy is already permissive
- **WHEN** `--harden-host-firewall` is passed and `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY` is already `ACCEPT`
- **THEN** setup leaves it unchanged

### Requirement: Idempotent and independent of VM lifecycle
Setup SHALL apply host firewall hardening the same way regardless of whether the invocation's target VM is being freshly created or already exists, and re-applying it SHALL NOT duplicate rules or fail.

#### Scenario: Flag passed on a fleet VM whose host already has hardening applied
- **WHEN** `--harden-host-firewall` is passed during setup of any VM, and the host SSH rule and forward policy are already in the desired state from a previous run
- **THEN** setup completes this step without error and without adding a duplicate rule
