## Context

This is the first change to add host-side, always-running processes rather than one-shot provisioning steps or guest-only cloud-init configuration. It also directly follows `harden-host-firewall` (host-wide, opt-in `ufw`) and interacts with it: a log receiver listening on the host is exactly the kind of thing a default-deny host firewall would block unless explicitly accounted for. It's scoped by the user's explicit choice to keep alerting local-only for now (no email/webhook), since internet connectivity and DNS/TLS are deliberately deferred to later work.

Two existing constraints shape what's possible here:
- `--bridge` mode's macvtap isolation means "host and VM can't see each other directly" (documented in `debian-vm-setup.sh`'s own header comments) — ruling out any host↔guest communication for that mode, including log forwarding.
- NAT-family VMs (plain NAT or `--forward`) always have the host reachable at the network's gateway IP (default `192.168.122.1`) from the guest's side, regardless of whether `--forward` is used — this is the one channel guest→host communication can rely on.

## Goals / Non-Goals

**Goals:**
- Detect and record when a VM stops responding, distinguishing "hypervisor process died" from "guest OS is hung but the QEMU process is still there."
- Give the operator one place on the host to read a VM's logs without SSHing into it.
- Surface state changes locally in a way a returning operator will actually see (not just a line buried in a log file no one tails).
- Compose cleanly with `harden-host-firewall`: if the host firewall is hardened, this doesn't get silently blocked, and if it isn't, this doesn't require it.

**Non-Goals:**
- Any remote/internet-dependent alert channel (email, webhook, push notification, ntfy.sh, etc.) — explicitly deferred per the scope decision behind this change.
- Monitoring or log forwarding for `--bridge`-mode VMs — architecturally blocked by macvtap isolation, not a gap this proposal can close.
- A full log-aggregation stack (ELK, Loki, etc.) — disproportionate to a repo whose entire footprint today is two bash scripts; `rsyslog` forwarding to per-VM files is enough to answer "what happened on VM X" without SSHing in.
- Configurable check intervals, retry thresholds, or alert-acknowledgment workflows — fixed sensible defaults for v1, revisited as an Open Question if they prove wrong in practice.
- Monitoring anything other than "is this VM's guest OS responsive" (no CPU/memory/disk metrics, no application-level health checks) — out of scope; this is uptime, not a metrics platform.

## Decisions

**1. Health check combines `virsh domstate` with a TCP reachability probe on the guest's SSH port, not either alone.**
`domstate` only tells you the QEMU process is running — a hung or kernel-panicked guest can still report `running` indefinitely. A bare SSH-port probe alone can't distinguish "guest is down" from "guest is fine but the firewall/network changed." Combining them: if `domstate` isn't `running`, that's unambiguously down; if it is `running` but the SSH port (obtained via `virsh domifaddr`) doesn't accept a TCP connection within a short timeout, treat that as down too (guest OS unresponsive). This mirrors the same "don't trust the hypervisor's view alone" reasoning already used for the guest firewall's SSH-always-allowed rule — the goal is detecting operator-visible outages, not just process existence.

**2. One templated systemd timer/service pair (`self-hosting-vm-uptime@.{service,timer}`), instantiated per VM, rather than one script looping over all VMs.**
Systemd template units are the standard idiom for "one unit, many instances," and map naturally onto this repo's existing per-VM lifecycle (`--name` on both scripts). Enabling monitoring for a VM is `systemctl enable --now self-hosting-vm-uptime@<name>.timer`; disabling it for exactly one VM (on `--vm-only` cleanup) is the corresponding `systemctl disable --now`, with no need to track a separate list of "which VMs are monitored" — `systemctl list-timers` already answers that.

**3. State transitions, not every tick, trigger an alert; last-known state lives in a tmpfs file (`/run/self-hosting-vm-uptime/<name>.state`), not a persistent one.**
Alerting on every check (every ~2 minutes while a VM is down) would bury the one signal that matters (it went down / it came back) in repetition. Using `/run` (cleared on host reboot) means the first check after a host reboot always establishes a fresh baseline rather than firing a spurious "recovered" alert for a VM that was simply never checked before the host itself restarted.

**4. Centralized logging uses `rsyslog` forwarding (guest → host), not `systemd-journal-remote`.**
Both are legitimate; `rsyslog`'s `*.* @@host:port` forwarding directive is simpler to reason about and doesn't require setting up `journal-remote`'s HTTP(S)-based transport and trust configuration, which would start to brush up against exactly the kind of TLS/cert complexity this change is trying to avoid (consistent with deferring DNS/TLS entirely). `rsyslog` ships in the Debian archive and is a one-package install on both ends.

**5. The host-side `rsyslog` receiver binds only to the `virbr0` interface (the NAT bridge's IP), never `0.0.0.0`.**
This is what makes "no explicit host firewall rule needed unless `ufw` happens to be active" true in the common case: a listener bound to `virbr0`'s address is simply unreachable from any other interface at the OS level, independent of `ufw`. It also means this receiver is never exposed to the LAN or beyond, by construction, regardless of whether `--harden-host-firewall` was ever used.

**6. When `ufw` is active (from `harden-host-firewall` or otherwise), add a scoped `ufw allow in on virbr0 to any port <PORT> proto tcp` rule for the receiver.**
Interface binding (Decision 5) prevents unwanted exposure, but a default-deny `ufw` policy still drops the packet before it reaches the bound socket unless a matching allow rule exists. Scoping the rule to `in on virbr0` keeps it as narrow as the interface binding already is — this never opens the port to the LAN/internet, only to guest↔host NAT traffic. This step checks for `ufw` dynamically (via `ufw status`) each time it runs rather than depending on `harden-host-firewall` having been applied; the two changes compose without requiring a specific order.

**7. Logs land under `/var/log/self-hosting-vms/<vm-hostname>/`, keyed by the hostname cloud-init already sets (`VM_HOSTNAME`), with `logrotate` applied.**
Reuses an identifier the operator already chose (`--name`/`--ip` hostname) instead of inventing a new one. `logrotate` (already a near-universal Debian/Ubuntu dependency) prevents the exact unbounded-growth problem this proposal would otherwise introduce.

**8. Local alerting: `logger -t self-hosting-alert` + `wall` + an `/etc/update-motd.d/` script, no ack-state file.**
`logger` puts alerts in the host's own journal/syslog under a filterable tag (`journalctl -t self-hosting-alert`), for free, without inventing a new log format — and it composes with `vm-centralized-logging`'s substrate rather than being a fourth, separate mechanism. `wall` gives real-time visibility to anyone logged into the host at the moment. The `update-motd.d` script re-surfaces the last few alerts at the next login, covering the case where nobody was watching when it happened. Explicitly not building an acknowledgment/dismissal workflow for v1 — an operator who wants history has `journalctl`; this is called out as an Open Question rather than solved now, to avoid growing this already-three-part change further.

**9. `--monitor` is opt-in, not default-on; scoped like `--harden-host-firewall`, not like `--no-auto-updates`.**
This installs persistent background processes (a timer per VM, an always-listening receiver) and accumulates disk usage (logs) — real, ongoing footprint on the host, unlike the zero-footprint `unattended-upgrades` default. Same reasoning as `--admin-password` and `--harden-host-firewall`: opt-in when there's a real, ongoing cost to defaults-on.

**10. `--vm-only`/per-VM cleanup disables the timer instance but preserves that VM's log directory; only `--purge-all`/full teardown offers to delete logs.**
The VM's disk has zero value once the VM is gone (it's regenerated from the base image on recreation), so `--vm-only` deletes it. Logs are different: their value is diagnostic and outlives the VM instance (e.g., "why did this VM crash last week" after already having recreated it). Treating logs like the disk would throw away the one thing this whole change exists to preserve.

## Risks / Trade-offs

- **[Risk] The uptime-check script itself could be a source of false positives (e.g., a transient network blip makes SSH momentarily unreachable).** → Mitigation: only alert on a state *transition*, not a single failed check; a follow-up Open Question covers whether to require N consecutive failures before flipping state, which would reduce false positives further at the cost of detection latency.
- **[Risk] `rsyslog` forwarding is fire-and-forget (no delivery guarantee); a guest that's down obviously can't forward its own "I'm going down" logs.** → Accepted: this is why uptime monitoring (Decision 1) exists as a separate, host-side signal rather than relying on the guest to self-report.
- **[Risk] Binding the receiver to `virbr0` (Decision 5) means centralized logging silently does nothing for `--bridge`-mode VMs, which could confuse an operator who enables `--monitor` on a bridged VM expecting logs to show up.** → Mitigation: documented as a known limitation in the proposal and README; the setup script should print a note when `--monitor` is combined with `--bridge`, explaining that only uptime monitoring (not logging) applies in that mode.
- **[Trade-off] No remote alerting means an operator away from the host's terminal/SSH session won't learn about an outage until they next check.** Accepted per the explicit scope decision behind this change; revisit once internet/DNS work happens.
- **[Trade-off] Fixed check interval and no consecutive-failure threshold trade some false-positive resistance for simplicity.** Left as an Open Question rather than a flag, to avoid growing the flag surface before there's evidence the default is wrong.

## Sequencing with other pending changes

`harden-host-firewall` was archived before this change, as required. Its `vm-cleanup-scope` update added a "Host firewall hardening is left untouched" scenario to the `--vm-only` requirement that this change's original delta draft didn't carry forward (OpenSpec's archive step caught the gap: `MODIFIED` blocks are matched and replaced by requirement header, not merged, so a delta written before a sibling change archives will silently drop that sibling's scenarios unless refreshed). This change's `vm-cleanup-scope` delta was refreshed against the post-`harden-host-firewall` base spec before archiving, to carry every existing scenario forward alongside this change's own additions.

## Migration Plan

N/A — additive, opt-in (`--monitor`), no effect on VMs or hosts where the flag is never passed. Rollback is `debian-vm-cleanup.sh`'s new steps (per-VM timer disable always available; host-wide artifact removal under `--purge-all`/interactive, following the same "refuse if other VMs still exist" guard already used for shared package purge).

## Open Questions

- Should the health check require N consecutive failures before declaring a VM "down," trading detection latency for fewer false positives? Left as a fixed single-check default for v1.
- Should there be a way to acknowledge/clear alerts rather than just accumulating in the journal? Left open; `journalctl -t self-hosting-alert` is the escape hatch for now.
- Once internet connectivity is addressed, should remote alerting be a new capability that hooks into the same `logger -t self-hosting-alert` tag (e.g., a small forwarder watching the journal), rather than a fourth alert code path? Flagged for whoever picks that up later.
