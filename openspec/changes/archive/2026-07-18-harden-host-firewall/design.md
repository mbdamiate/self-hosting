## Context

The host machine running `debian-vm-setup.sh` today has no firewall policy applied by this repo. Two independent kinds of traffic pass through or terminate at the host once VMs exist:

1. **Traffic destined for the host itself** (its own `sshd`, or anything else listening on the host) — governed by the `INPUT` chain.
2. **Traffic routed through the host to a VM** — libvirt's NAT (`virbr0` MASQUERADE) and the existing `--forward` DNAT rules both depend on the `FORWARD` chain letting packets through. This traffic never touches `INPUT`.

`ufw` is Debian/Ubuntu's standard front-end for exactly the first case, and ships with a `DEFAULT_FORWARD_POLICY` of `DROP` in `/etc/default/ufw` — appropriate for a machine that isn't a router, wrong for this one, since this repo's entire purpose is routing traffic to VMs.

## Goals / Non-Goals

**Goals:**
- Give the host a default-deny inbound policy for services listening on the host itself, opt-in via `--harden-host-firewall`.
- Never lock the operator out of the host: SSH to the host is always allowed as part of enabling this.
- Guarantee libvirt NAT/bridge forwarding and the existing `--forward` mechanism are unaffected — this change must not be the thing that breaks VM networking.
- Make the added state identifiable and removable by `debian-vm-cleanup.sh`, without assuming this repo owns every `ufw` rule that might exist on the host.

**Non-Goals:**
- Managing arbitrary host-service ports (e.g., if the operator runs something else on the host besides this repo's VMs). `--harden-host-firewall` only ever adds the SSH baseline; anything else the operator wants open on the host is on them to add with `ufw allow`, same as they'd do without this script.
- Replacing or restructuring `--forward`'s existing raw `iptables` DNAT/FORWARD-accept commands. Those keep working exactly as they do today; this change only ensures `ufw`'s host-wide policy doesn't conflict with them.
- Detecting or preserving a fully arbitrary pre-existing `ufw` configuration perfectly. See Risks below for the accepted imprecision.

## Decisions

**1. `ufw` on the host too, for the same reason chosen for the guest (`add-vm-guest-firewall`).**
Consistency of tooling and operator experience: the same `ufw allow`/`ufw status` vocabulary applies whether inspecting the guest or the host.

**2. Host SSH is always allowed, tagged with a comment, before `ufw` is enabled.**
`ufw allow 22/tcp comment 'self-hosting: host SSH baseline'`. The comment tag is the mechanism that lets `debian-vm-cleanup.sh` later identify and remove exactly this rule (via `ufw status numbered` + grep on the comment) without guessing which of potentially many `ufw` rules belong to this repo. Ordering matters: the rule is added *before* `ufw --force enable`, same reasoning as the guest firewall's Decision 7 — never enable a default-deny policy with a gap where the management channel (SSH) isn't yet allowed.

**3. `DEFAULT_FORWARD_POLICY` is set to `ACCEPT` in `/etc/default/ufw`, not left at `ufw`'s stock `DROP`.**
This is the load-bearing decision of this whole change. `ufw`'s `INPUT`/`OUTPUT` default-deny/default-allow only governs traffic to/from the host itself; `FORWARD` governs traffic passing through it. Leaving `DROP` here would silently break every VM's internet access (libvirt's `virbr0` NAT depends on `FORWARD` accepting the traffic it wants to MASQUERADE) and every `--forward`-exposed service (its manually inserted `FORWARD ACCEPT` rule would now compete with `ufw`'s own default-forward-policy machinery instead of a host that simply had no forwarding policy at all). Setting this to `ACCEPT` restores today's status quo for forwarding while still gaining the `INPUT` hardening this change is actually about. This is checked and set idempotently — if already `ACCEPT` (e.g., set by another tool, or a prior run of this flag), it's left alone; only changed if found as `DROP`.

**4. Idempotent application, not gated on `VM_EXISTS`.**
This runs in the host-prerequisite section of `debian-vm-setup.sh` (alongside package installation and NAT-network readiness), independent of which VM is being created or whether it already exists — same rationale as those existing steps. Re-running with `--harden-host-firewall` on any fleet VM reaffirms the same host state rather than erroring or duplicating rules (`ufw allow` is naturally idempotent; re-adding an existing rule is a no-op).

**5. `--harden-host-firewall` is opt-in, not default-on.**
Unlike `enable-vm-unattended-upgrades` or the guest firewall's `fail2ban` half, this has a real ergonomic cost with a blast radius beyond the VM: it changes what's reachable on the *host* the operator is sitting at (or remotely administering), which may already run other services or already have its own firewall policy the operator manages independently of this repo. Defaulting this on risks silently changing host reachability for reasons unrelated to VM provisioning. Opt-in matches the precedent set by `--admin-password` for exactly this kind of trade-off.

**6. Cleanup identifies "our" state narrowly: the tagged SSH rule and the forward-policy file value, not "ufw" as a whole.**
`debian-vm-cleanup.sh` does not offer to disable or uninstall `ufw` itself — only to remove the specifically tagged rule and to check whether `/etc/default/ufw` currently reads `ACCEPT` and, if so, offer to set it back to `DROP`. This mirrors the existing package-purge step's own caution ("if you use KVM/libvirt for other VMs besides this one, do NOT remove the packages") rather than assuming this repo is the sole owner of the host's `ufw` configuration.

## Risks / Trade-offs

- **[Risk] Enabling `ufw` on a host that already has other, unrelated firewall rules (raw `iptables`, `nftables`, or a differently-configured `ufw`) could interact unpredictably.** → Mitigation: this change only adds one `ufw allow` rule and enables `ufw` if not already active; it does not reset or flush existing rules. If `ufw` is already active with a different policy, `ufw --force enable` is a no-op re-affirmation, not a reset, so pre-existing rules are preserved. Documented as an explicit caveat in the README: operators with existing host firewall configuration should review before opting in.
- **[Risk] `ufw enable`/`reload` may re-flush the built-in `FORWARD` chain, potentially disturbing `--forward`'s manually-inserted `iptables -I FORWARD ... -j ACCEPT` rules if `--harden-host-firewall` is applied *after* `--forward` was already set up.** → Mitigation: no new code needed — `debian-vm-setup.sh`'s existing `--forward` handling already checks for its rules with `iptables -t nat -C ...` / `-C FORWARD ...` and re-adds them if missing, every time it runs. Document that re-running setup with the same `--forward` value after first applying `--harden-host-firewall` (or in the same run) re-asserts those rules; call this out in the README and as a verification task.
- **[Risk] Reverting `DEFAULT_FORWARD_POLICY` to `DROP` on cleanup could be wrong if it was already `ACCEPT` for a reason unrelated to this script.** → Accepted imprecision, consistent with Decision 6; cleanup only offers this as a confirm-gated step (interactive) or under `--purge-all` (already documented as the "remove everything" mode), never silently.
- **[Trade-off] Opt-in means an operator who doesn't know about the flag gets no host hardening at all.** Accepted, per Decision 5 and the same trade-off already accepted for `--admin-password`; the README should recommend it for any host meant to run as a real server.

## Sequencing with other pending changes

`add-vm-observability` also modifies `vm-cleanup-scope`'s `--vm-only`, `--purge-all`, and no-flags-interactive requirements, and its delta text already incorporates this change's "any host firewall hardening" addition (written as the union of both). **This change's `vm-cleanup-scope` delta must be archived before `add-vm-observability`'s** — or, if `add-vm-observability` lands first, its `MODIFIED Requirements` block for these three requirements must be re-diffed against whatever this change actually archived, rather than applied verbatim. OpenSpec deltas are matched and replaced by requirement header, not merged automatically, so two pending `MODIFIED` blocks touching the same requirement don't reconcile themselves.

## Migration Plan

N/A — additive, opt-in, idempotent. No effect on hosts where the flag is never passed. Rollback is `debian-vm-cleanup.sh`'s new removal step (interactive, or `--purge-all`), or manually: `sudo ufw delete allow 22/tcp` (matching the tagged rule) and restoring `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY` to `DROP` if desired.

## Open Questions

- Should the host SSH allow rule be restricted to the LAN subnet rather than any source, once this VM/host is expected to be reachable from the wider internet (the deferred networking TODO item)? Left open — out of scope while internet exposure itself is deferred.
- Should `--harden-host-firewall` also manage the host's `nftables`/`iptables`-persistent setup needed to make `--forward` rules survive a reboot (already flagged as a gap in `--forward`'s own `NOTE`)? Related but distinct concern; left for a possible follow-up rather than folded into this change.
