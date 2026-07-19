## Context

This repo now has, or is proposing, four distinct recovery/detection layers, each answering a different "what if" question, easy to conflate:

| Layer | Question it answers | Status |
|---|---|---|
| `virsh autostart` | Host rebooted — do VMs come back? | Already implemented |
| `on_crash` (this proposal) | QEMU process itself crashed while the host kept running — does the VM come back? | Gap this proposal closes |
| Watchdog device (`add-vm-watchdog`) | Guest OS hung but QEMU is still running — does the VM recover? | Proposed, not yet applied |
| `vm-uptime-monitoring` (`add-vm-observability`) | Something went down — does anyone find out? | Proposed, not yet applied; detection/alerting only, no action |

`on_crash` is a libvirt domain-XML setting (sibling to `on_reboot`/`on_poweroff`) governing what libvirt does when it detects the QEMU process has crashed (a genuine emulator fault or a guest condition that aborts QEMU itself — distinct from a guest OS merely hanging, which is the watchdog's territory). Left unset, libvirt's default behavior stops the domain and leaves it stopped.

## Goals / Non-Goals

**Goals:**
- Close the specific gap: an unattended QEMU-process crash should not require manual `virsh start` to recover.
- Default this on, since — unlike the watchdog — there's no plausible false-positive case that makes an opt-in default necessary.
- Keep the three recovery layers (autostart, `on_crash`, watchdog) clearly distinguished in documentation, so a future reader doesn't assume one covers what another does.

**Non-Goals:**
- Any throttling/circuit-breaker for repeated crash-restart cycles (a "crash loop"). Not built here; see Risks.
- Changing `on_reboot` or `on_poweroff` — both already have sensible defaults (`restart` and `destroy` respectively) that this proposal doesn't touch. A guest-initiated shutdown should stay off; a guest-initiated reboot should already restart the VM.
- Any new integration code with `vm-uptime-monitoring` or the watchdog — if those are also enabled, a crash-restart cycle simply shows up as a normal down/up transition pair to the monitoring layer, with nothing extra to wire.

## Decisions

**1. Default `on_crash=restart` on, unlike the opt-in pattern used for `--admin-password`, `--harden-host-firewall`, `--monitor`, and `--watchdog`.**
Those are all opt-in because each has a plausible cost to a default-on posture: a workflow change (password prompts), a reachability change (host firewall), a persistent resource footprint (monitoring), or a false-positive risk (watchdog resetting a busy-but-not-hung guest). A QEMU process crash has none of these — it's binary (it crashed or it didn't), and the "cost" of restarting is strictly better than leaving a crashed VM down indefinitely. This makes `on_crash=restart` closer in risk profile to `enable-vm-unattended-upgrades` (also default-on) than to its immediate sibling proposals.

**2. `--no-crash-restart` opt-out, not a bare unconditional setting.**
Even with no plausible ergonomic downside, an operator debugging a crash may want the crashed state preserved for inspection (e.g., to attach `gdb` to a coredump, or just confirm what state it died in) rather than have libvirt immediately wipe it out by restarting. Offering an opt-out costs one flag and covers that case, consistent with the `--no-auto-updates`/`--no-guest-firewall` precedent for default-on features.

**3. Extend `vm-setup-rerun-recovery` for `on_crash` mismatch detection, following the exact pattern `add-vm-watchdog` established.**
`on_crash` is domain-XML state, inspectable via `virsh dumpxml`, exactly like network mode and the watchdog device — not a case needing a host-side marker file (that mechanism is reserved for state cloud-init applies that leaves no externally-inspectable trace, like the admin sudo password). Reusing the same detect-and-warn shape keeps all three domain-XML-fixed settings (network mode, watchdog, `on_crash`) handled uniformly in one place rather than three slightly different mechanisms.

**4. No crash-loop throttling in this proposal.**
libvirt's `on_crash=restart` has no built-in backoff; a domain that crashes immediately on every start could restart repeatedly in a tight loop. Building a throttle (e.g., a wrapper script tracking restart counts/timestamps) is a meaningfully bigger scope than "set one domain XML value," and this failure mode is both rare (it requires the *hypervisor*, not the guest OS, to be crashing) and self-limiting in practice (each start attempt takes real wall-clock time). Treated as a Risk with a documented mitigation (visibility via `vm-uptime-monitoring`, if enabled) rather than solved here.

## Risks / Trade-offs

- **[Risk] A persistent, immediate crash cause (e.g., a bad hardware pass-through config, corrupted disk image) could cause a tight restart loop, consuming host resources.** → Mitigation: not automatically prevented in v1; if `vm-uptime-monitoring` is also enabled, the resulting rapid up/down alert pattern makes this visible to the operator quickly. Flagged as an Open Question for a possible future throttle.
- **[Risk] Restarting after a crash discards whatever in-memory/in-flight state existed at the moment of the crash**, same as any crash-recovery mechanism. → Accepted; the alternative (staying down indefinitely) is strictly worse for a server workload, and this is no different from what a real VPS host's own recovery behavior would do.
- **[Trade-off] `--no-crash-restart` is one more flag in an already-growing set of `debian-vm-setup.sh` options.** Accepted — it's a one-line, self-explanatory boolean, and the alternative (no opt-out at all) removes a legitimate debugging use case for a single flag's worth of cost.

## Migration Plan

N/A — applies to freshly created VMs only (`on_crash` is fixed at `virt-install` time, the same constraint already true of network mode and the watchdog device). Existing VMs created before this change keep whatever `on_crash` behavior they were created with (libvirt's default, i.e., left stopped) unless recreated.

## Open Questions

- If crash-loop behavior turns out to be a real problem in practice, should a future change add throttling (e.g., a small wrapper tracking restart attempts within a window, falling back to `destroy` after N rapid crashes)? Left open, not designed here.
