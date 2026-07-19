## Why

`debian-vm-setup.sh` only ever touches the host's `iptables` to add `--forward`'s DNAT/FORWARD-accept rules for specific forwarded ports. There is no baseline policy on the host itself: whatever services the host machine runs (including its own `sshd`) are reachable on however the host is otherwise exposed, with nothing scripted here to narrow that. The guest already gets a default-deny firewall (`add-vm-guest-firewall`); the physical host it runs on — the machine the operator is turning into a server — has no equivalent layer.

## What Changes

- Add a `--harden-host-firewall` flag to `debian-vm-setup.sh` that installs and enables `ufw` on the **host**, with a default-deny inbound / default-allow outbound policy.
- Always allow inbound SSH (tcp/22) to the host before enabling, tagged with a `comment` so it's identifiable later, so the operator is never locked out of managing the physical machine.
- Ensure the host's `FORWARD` chain policy stays permissive (`ufw`'s `DEFAULT_FORWARD_POLICY` set to `ACCEPT`) so libvirt's NAT/bridge forwarding — and the existing `--forward` DNAT rules — keep working; `ufw`'s stock default here (`DROP`) would otherwise silently break every VM's outbound NAT connectivity and any `--forward`-exposed service. This is intentionally **not** the same thing as opening host ports: it only affects packets already being routed *through* the host to a VM, not services listening *on* the host itself.
- This is host-wide, opt-in, and idempotent: passing the flag on any fleet VM's setup run applies (or reaffirms) the same host policy; it is not tied to a specific VM's lifecycle.
- Extend `debian-vm-cleanup.sh`'s `--purge-all` and interactive modes to offer removing the SSH baseline rule (identified by its comment tag) and reverting the forward policy, mirroring how other host-wide state (packages, groups, the default network) is already handled. `--vm-only` continues to leave all host-wide state untouched, this included.
- `--help` documents the flag and what it does and does not affect; the cleanup behavior is captured in `vm-cleanup-scope`'s updated spec. Per the existing `repository-readme` spec (README SHALL NOT restate flag reference or behavioral guarantees already in `openspec/specs/`), `README.md` itself needs no new prose.

## Capabilities

### New Capabilities
- `host-firewall-hardening`: governs the opt-in host-level `ufw` firewall — default-deny inbound policy, the always-allowed host SSH rule, and preserving the `FORWARD` chain policy libvirt/`--forward` depend on.

### Modified Capabilities
- `vm-cleanup-scope`: `--purge-all` and the interactive walkthrough gain a step to remove the host firewall hardening this change can add (SSH baseline rule + forward policy), while `--vm-only` explicitly continues to leave it in place. **Sequencing:** archive this change's `vm-cleanup-scope` delta before `add-vm-observability`'s — see design.md.

## Impact

- `scripts/debian-vm-setup.sh`: argument parsing (`--harden-host-firewall`), a new host-level hardening step (installing/enabling `ufw`, adding the tagged SSH rule, setting the forward policy), `--help` text.
- `scripts/debian-vm-cleanup.sh`: a new removal step (interactive confirm + `--purge-all`) for the tagged SSH rule and forward-policy reversion.
- No impact on `README.md` — covered by the existing `--help`/`openspec/specs/` pointers per `repository-readme`.
- No change to `--forward`'s own DNAT/FORWARD-accept mechanism — this proposal only ensures `ufw`'s host-wide policy doesn't fight it.
