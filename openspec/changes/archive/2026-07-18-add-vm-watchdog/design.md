## Context

QEMU/libvirt supports attaching a virtual watchdog device to a domain. The guest OS must periodically "pet" it (write to `/dev/watchdog`) or the device fires and triggers a configured action in the hypervisor (reset, poweroff, pause, dump, inject-nmi, or none). This is a guest-OS-level liveness mechanism, sitting below application health and below the QEMU-process-level concerns `on_crash` already covers (tracked as a separate TODO item — this proposal doesn't touch `on_crash`). It's also independent of `vm-uptime-monitoring` (`add-vm-observability`): that mechanism is host-side, detects the same class of failure (guest hung, QEMU running), but only alerts — it never restarts anything. This proposal adds the corrective-action counterpart, and the two can run together (defense in depth) or independently (`--watchdog` requires nothing from `--monitor` or vice versa).

## Goals / Non-Goals

**Goals:**
- Automatically recover a VM whose guest OS has hung, without depending on host-side monitoring being installed or an operator being available to notice and act.
- Use standard, well-supported QEMU/libvirt and systemd mechanisms rather than a bespoke daemon.
- Make the watchdog's presence/absence on an existing VM inspectable and its rerun-mismatch behavior consistent with how network mode is already handled.

**Non-Goals:**
- Application-level health checks (e.g., "is the web server on port 80 actually responding"). This repo provisions a base Debian image; it has no way to know what the operator will run on it later. The classic `watchdog` package's `test-binary`/`repair-binary` mechanism is the standard way to add this later, and is documented as an extension point, not built here.
- Replacing or duplicating `on_crash` — that's a separate, already-tracked TODO item covering QEMU-process-level crashes, a different failure class than a hung-but-still-running guest.
- Configurable watchdog timeout or action — fixed, documented defaults for v1 (see Decisions), revisited as an Open Question if needed.
- Any interaction with `vm-uptime-monitoring`'s alerting — the watchdog acts silently (from the host's perspective) unless the operator is also running host-side monitoring, which would observe the VM going down and back up as a normal state transition, with no special-casing needed.

## Decisions

**1. Watchdog device model: `i6300esb`.**
The standard, most broadly guest-compatible choice documented across QEMU/libvirt tooling; Debian's kernel includes the corresponding `i6300esb_wdt` driver and loads it automatically once the emulated PCI device is present, needing no explicit guest-side driver configuration.

**2. Action: `reset` (the VM reboots), not `poweroff`, `pause`, or `none`.**
Matches the "self-healing" intent of a watchdog — the whole point is unattended recovery. `poweroff`/`pause` would leave the VM down until an operator intervenes, which is what `vm-uptime-monitoring` already covers (detect-and-alert, no action) — offering no additional value over that existing capability. `none` would defeat the purpose entirely. Not exposed as a flag value in v1 (see Non-Goals); if a real need for a different action surfaces, it's a small follow-up.

**3. Guest-side petting via systemd's `RuntimeWatchdogSec=`, not the userspace `watchdog` package.**
systemd (PID 1 on Debian) already natively drives `/dev/watchdog` when this option is set — no additional package, daemon, or configuration file format to maintain. It directly captures the failure mode this proposal targets (kernel or PID 1 hung), without pulling in the classic `watchdog` package's broader feature set (load-average checks, custom test binaries, temperature sensors) that this proposal explicitly doesn't need per the Non-Goals. Set via a drop-in, `/etc/systemd/system.conf.d/90-watchdog.conf`, rather than editing the shipped `/etc/systemd/system.conf` directly — consistent with this repo's established pattern of writing explicit override files instead of modifying package-shipped defaults (same reasoning already applied to `unattended-upgrades` and `fail2ban`'s `jail.local`).

**4. Watchdog configuration is inspected via `virsh dumpxml`, not tracked with a marker file.**
Unlike the admin-sudo-policy case (which needed a host-side marker file because cloud-init leaves no externally-inspectable trace), the watchdog device is part of the VM's own domain XML and is directly queryable at any time with `virsh dumpxml <name> | grep -A1 '<watchdog'`. This is the same situation as network mode, which `vm-setup-rerun-recovery` already handles by inspecting the live VM rather than trusting the invocation's flags — so this proposal extends that existing capability with an equivalent requirement, instead of duplicating a self-contained mismatch mechanism the way `restrict-vm-admin-sudo` had to.

**5. `--watchdog` is opt-in, a simple boolean (no `[=ACTION]` value).**
An automatic reset is a real availability trade-off — a false-positive watchdog trigger (e.g., a guest legitimately busy long enough to miss pettings, though `RuntimeWatchdogSec`'s default granularity makes this unlikely for normal workloads) reboots a VM that might have recovered on its own. That risk profile matches why `--admin-password`, `--harden-host-firewall`, and `--monitor` are all opt-in rather than default-on; this follows the same precedent. Keeping it boolean (rather than accepting a timeout or action value) matches Decision 2's reasoning — there's one sensible action, so there's nothing to select.

## Risks / Trade-offs

- **[Risk] A guest that's legitimately busy (not hung) for longer than the watchdog timeout gets reset unnecessarily.** → Mitigation: `RuntimeWatchdogSec`'s default-scale timeout (tens of seconds) is calibrated by systemd/upstream for exactly this trade-off (long enough to avoid false positives under normal load, short enough to catch real hangs promptly); not something this proposal needs to re-tune. Flagged as an Open Question if real-world use proves otherwise.
- **[Risk] A `reset` in the middle of disk I/O could corrupt application state that wasn't fsynced.** → Accepted trade-off inherent to any hard-reset-based recovery mechanism (identical to what happens on a real VPS provider's own hardware watchdog); out of scope to solve at this layer — applications meant to run unattended on a server should already tolerate an unexpected hard reset.
- **[Risk] The watchdog device firing and the host-side `vm-uptime-monitoring` health check both reacting to the same hang could produce a confusing double-signal (an alert for a VM that's already recovering on its own).** → Accepted; the alert is still accurate (the VM *was* down), and the two mechanisms operating independently is the intended defense-in-depth design, not a bug to reconcile.
- **[Trade-off] Boolean-only `--watchdog` means an operator who wants `poweroff` instead of `reset` (e.g., to investigate the hung state before it's wiped out) has no flag for that.** Accepted per Decision 5; `virsh dumpxml`/manual `virt-install --watchdog` editing remains available outside the script for that case.

## Migration Plan

N/A — additive, opt-in, applies only to freshly created VMs (the watchdog device is fixed at `virt-install` time, same constraint already true of network mode). No effect on existing VMs unless recreated with `--watchdog`.

## Open Questions

- Should the watchdog timeout become configurable if the default `RuntimeWatchdogSec` value proves too aggressive or too lax for real workloads observed on VMs created by this script? Left at systemd's/this proposal's chosen default for v1.
- Should a future change wire the classic `watchdog` package's test-binary mechanism as an optional, operator-supplied extension (e.g., `--watchdog-test=/path/to/check`) once there's a concrete need? Documented as a possibility, not designed here.
