## Context

`debian-vm-setup.sh` already has one guest-side network concept to build alongside: `--forward=HOST_PORT:VM_PORT,...`, which adds host-level `iptables` DNAT rules so traffic from the LAN reaches the VM. That mechanism controls what's reachable *before* it hits the VM's network stack. This change adds a second, independent layer *inside* the VM's own OS — a firewall and a brute-force-response daemon — installed via the same cloud-init `user-data` mechanism already used for the admin user (`restrict-vm-admin-sudo`) and automatic security updates (`enable-vm-unattended-upgrades`).

The risk this closes: today, once any traffic reaches the VM's network interface (whether via `--bridge`, `--forward`, or later direct internet exposure), the only thing gating access is `sshd` itself. There's no in-guest policy limiting which ports respond, and no mechanism reacting to repeated failed logins.

## Goals / Non-Goals

**Goals:**
- Default-deny inbound traffic inside the guest except SSH, without ever locking the operator out.
- Let operators declare additional guest-side ports for their own services, and avoid the trap where a `--forward` rule "works" at the host but is silently dropped by the guest's own firewall.
- Always protect SSH against brute-force attempts via `fail2ban`, since it has no legitimate-traffic downside.
- Keep local/disposable-testing workflows working by default, with an explicit opt-out for the one piece that has real ergonomic cost (the firewall's default-deny policy).

**Non-Goals:**
- Any host-level firewall hardening — that's the separate "harden the host-level firewall" TODO item, not duplicated here.
- Rate-limiting, geo-blocking, or SSH port changes — out of scope; `sshd` stays on port 22, unconfigurable by this script today.
- Configuring `fail2ban`'s ban duration/retry thresholds — its stock defaults are reasonable, and this proposal doesn't want to grow the flag surface further; left as an Open Question below.
- UDP or non-TCP allow rules — `--allow-port` mirrors the existing `--forward` flag's TCP-only assumption for consistency.

## Decisions

**1. `ufw` over raw `nftables`.**
Debian 12 uses `nftables` as the default packet-filtering backend regardless, and `ufw` is a thin, well-documented wrapper over it. The operators of a VM created by this script — testing self-hosting setups, or later running one as a small production box — are far more likely to be comfortable with `ufw allow 8080/tcp` / `ufw status` than raw `nft` rule syntax if they ever need to adjust things by hand after boot. Raw `nftables` would be more "correct" for a from-scratch security tool, but this script optimizes for an operator who can read and extend what it generates.

**2. SSH (tcp/22) is unconditionally allowed, even with `--allow-port` unset and regardless of network mode.**
The VM is provisioned with SSH-key-only access and no other login path (`ssh_pwauth: false`, no console password by default outside of `restrict-vm-admin-sudo`). A default-deny firewall that also blocked SSH would strand every VM immediately. This rule is not gated by any flag.

**3. `--allow-port=PORT[,PORT...]` is a flat, comma-separated TCP port list, mirroring `--forward`'s `HOST_PORT:VM_PORT` parsing style.**
Consistency with the existing flag's parsing (`IFS=',' read -ra ...`) keeps the script's conventions uniform rather than introducing a new syntax for a similar concept.

**4. VM-side ports from `--forward` are automatically added to the `ufw` allow list — no separate flag needed to "also" open them.**
If the operator already asked the host to forward `8080:80`, they've expressed clear intent for the guest's port 80 to be reachable; a default-deny guest firewall silently eating that traffic would look like a bug, not a feature. Parse `FORWARD_RULES`, extract the `VM_PORT` half of each pair, and fold those into the same `ufw allow` list built from `--allow-port`. This is derived, not independently configurable — there's no scenario where you'd forward a port to the VM but not want the guest to accept it.

**5. `fail2ban`'s `sshd` jail is enabled unconditionally, not gated by `--no-guest-firewall`.**
Unlike a default-deny firewall, `fail2ban` never blocks a legitimate, correctly-authenticating client — it only bans IPs after repeated failed attempts. There's no equivalent of the "I forgot to open my app's port" footgun, so there's no reason to make it optional; bundling it under the same opt-out as `ufw` would remove protection from operators who only meant to skip the firewall's default-deny behavior.

**6. Enable `fail2ban` via an explicit `jail.local` stanza (`[sshd]\nenabled = true`), not by trusting the package's shipped default.**
Same rationale already applied to `unattended-upgrades` in the sibling change: don't depend on whatever a non-interactive `apt-get install` happens to leave enabled by default. Let fail2ban's own auto-detection handle `backend`/`logpath` (Debian's default fail2ban config already resolves these correctly against systemd/journald), rather than hardcoding values that could drift from the package's own defaults.

**7. `ufw --force enable` at the end of `runcmd`, after all `allow` rules are added.**
`ufw enable` normally prompts for confirmation (it warns about breaking existing SSH connections); `--force` is required for unattended cloud-init execution. Rules must be added before enabling so the default-deny policy never has a window where SSH is blocked.

**8. `--no-guest-firewall` disables `ufw` installation entirely (not just `ufw disable` after installing it) and makes `--allow-port` a no-op.**
Skipping the package avoids unattended-upgrades later patching a piece of software that was deliberately opted out of, and keeps the "what's actually running in this VM" story simple: if you didn't ask for the firewall, it was never installed.

**9. A host-side marker file (`${WORK_DIR}/.guest-firewall-policy`, `enabled`/`disabled`) records the guest firewall's state at creation, specifically so `--forward` reapplication can warn.**
`vm-port-forward-reapplication` (an existing, already-shipped capability) officially supports adding a *new* `--forward` rule to an already-existing VM later. Decision 4 only auto-allows `--forward` VM-side ports *at cloud-init time* — a port forwarded for the first time on a later rerun never gets an equivalent `ufw allow` inside the guest, because cloud-init can't reapply. Without something to flag this, the host-side DNAT rule would appear to succeed while the service stays unreachable, with no indication why. This is a narrower purpose than general firewall-state mismatch tracking (Migration Plan below): the marker isn't used to detect or warn about a `--no-guest-firewall`/`--allow-port` mismatch on its own (that remains directly inspectable on the guest, per Migration Plan), only to let the host-side `--forward` handling know whether the extra warning is relevant.

## Risks / Trade-offs

- **[Risk] Default-deny guest firewall breaks a service the operator exposes via `--bridge` without also passing `--allow-port`.** → Mitigation: the one-time setup note (Decision matches prior changes' pattern) explicitly lists the allowed ports at creation time, and the README documents `--allow-port` alongside `--bridge`. Not fully eliminated — this is the accepted cost of a secure-by-default guest firewall, same trade-off already accepted for `--admin-password` and `--no-auto-updates`.
- **[Risk] `--forward` reapplied against an already-existing VM (an officially supported flow) opens the host-side path for a new port, but the guest's own firewall — fixed at creation — never learns about it, silently dropping the traffic.** → Mitigation: Decision 9's marker file lets the `--forward` handling path warn explicitly when this applies, with the exact remediation command (`ssh` in, `sudo ufw allow <VM_PORT>/tcp`), rather than leaving the operator to debug a port that "should" work.
- **[Risk] `ufw --force enable` misordered relative to rule creation could transiently lock out SSH during first boot.** → Mitigation: `runcmd` step ordering is explicit and sequential in cloud-init (each command runs to completion before the next); rules are added before `enable`, and this ordering is called out as a specific verification task.
- **[Risk] fail2ban misconfiguration (e.g., wrong log path) silently means the jail never triggers.** → Mitigation: rely on the package's own auto-detected defaults instead of hand-specifying `logpath`/`backend`, and add a verification task (`fail2ban-client status sshd`) to confirm the jail is actually active after boot.
- **[Trade-off] `ufw`'s abstraction is less flexible than raw `nftables` for advanced rules (rate-limiting, complex chains).** Accepted per Decision 1 — the target operator profile is well served by `ufw`'s simplicity, and advanced users can always extend the generated rules by hand or bypass `--no-guest-firewall`.

## Migration Plan

N/A — additive, applies to freshly created VMs only (cloud-init is first-boot-only, same constraint already documented in `vm-setup-rerun-recovery` and relied on by both prior sibling changes). No general rerun-mismatch tracking is added: like `enable-vm-unattended-upgrades`, the guest firewall's state is trivially inspectable on the VM itself (`ufw status`, `systemctl status fail2ban`) if an operator needs to check it after the fact. The one host-side marker file this change does introduce (Decision 9) exists solely to power the `--forward`-reapplication warning, not to track or resolve `--allow-port`/`--no-guest-firewall` mismatches generally.

## Open Questions

- Should `fail2ban`'s `bantime`/`findtime`/`maxretry` become configurable flags once this VM is actually exposed to the internet (the deferred networking TODO item)? Left open; stock defaults are a reasonable starting point.
- Should `--allow-port` support a protocol suffix (e.g., `53/udp`) once a real workload needs non-TCP traffic? Not needed by anything in scope today; deferred.
