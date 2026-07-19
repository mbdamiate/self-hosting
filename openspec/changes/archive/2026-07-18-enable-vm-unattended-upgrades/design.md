## Context

`debian-vm-setup.sh` provisions Debian 12 (bookworm) cloud images via cloud-init. `user-data` currently sets `package_upgrade: false` (a one-time, first-boot full-upgrade toggle) and installs only `qemu-guest-agent`. Nothing keeps the VM patched afterward — the operator would have to log in and run `apt` manually, indefinitely, for the life of the VM. Debian's `unattended-upgrades` package is the standard mechanism for closing that gap, and it's already present in the bookworm archive (no third-party repo needed).

## Goals / Non-Goals

**Goals:**
- Make ongoing Debian security patching the default for freshly created VMs, without requiring any operator action after boot.
- Keep the behavior predictable: only security-origin updates, no automatic reboot, no surprise package churn from the general stable-updates stream.
- Give operators who need a pinned/reproducible environment (e.g., testing against a known package snapshot) a way to turn it off.

**Non-Goals:**
- Automatic reboot after kernel/library updates that require one. Rebooting a running server unattended is a separate, higher-stakes decision than applying patches; left for a future change if wanted.
- Update notifications, logging, or alerting on applied/pending updates — that overlaps with the separate observability TODO item and isn't duplicated here.
- Changing `package_upgrade: false` (the first-boot full-upgrade behavior). That's orthogonal: this change is about the ongoing patching cadence after boot, not what happens during provisioning.
- Supporting distributions other than the Debian 12 cloud image this script already targets.

## Decisions

**1. Install `unattended-upgrades` via cloud-init `packages`, configure it via `write_files`, don't rely on debconf defaults.**
`packages:` in cloud-init runs `apt-get install` non-interactively; Debian's `unattended-upgrades` postinst normally offers a debconf prompt to enable itself, which doesn't fire in a non-interactive apt run. Explicitly writing `/etc/apt/apt.conf.d/20auto-upgrades` guarantees the feature is actually on, rather than depending on whatever the package's non-interactive default happens to be.

**2. Restrict `Unattended-Upgrade::Allowed-Origins` to the security origin only, overriding the package's shipped default.**
The `unattended-upgrades` package's default `/etc/apt/apt.conf.d/50unattended-upgrades` template typically also allows the general `${distro_codename}` (non-security) origin, which pulls in ordinary point-release package updates, not just security fixes. For a server whose main risk is unpatched CVEs, and to avoid the same "surprise upgrade" concern that motivated `package_upgrade: false` in the first place, this change writes its own `51unattended-upgrades-security-only` snippet restricting `Allowed-Origins` to `"${distro_id}:${distro_codename}-security"` and `"${distro_id}ESM:${distro_codename}-security"` (harmless no-op on Debian, matches Ubuntu naming if this script is ever adapted). Using `${distro_id}`/`${distro_codename}` variables (resolved by `unattended-upgrades` itself at runtime, not by cloud-init) keeps this independent of the specific Debian release pinned in `CLOUD_IMG_URL`.

**3. `Unattended-Upgrade::Automatic-Reboot "false"` explicitly.**
Matches the Non-Goals above. An unattended reboot of a server the operator doesn't know is about to restart is a worse failure mode than a delayed kernel update. Left as a manual step; the one-time setup note (see Decision 5) tells the operator this.

**4. Default on, with an opt-out flag (`--no-auto-updates`) rather than opt-in.**
Unlike the sudo-password change (`restrict-vm-admin-sudo`), this doesn't alter how the operator interacts with the VM day to day — no new password, no new prompt, nothing to remember at SSH time. It only affects a background `apt` timer. Defaulting it on is "safe by default" with no workflow cost, so the asymmetry that justified opt-in for sudo doesn't apply here. The opt-out exists for the narrower case of wanting a frozen package set for reproducible testing.

**5. One-time setup note, not a persistent per-run summary line.**
Printed only when a VM is freshly created (mirroring the existing pattern for the `--forward` iptables-persistence note), stating that security updates apply automatically but reboots don't. Not repeated on every reuse of an already-existing VM, since the behavior was already fixed at that VM's creation time and repeating it on every rerun would just be noise (same rationale already applied to network-mode and, in the prior change, sudo-policy rerun handling).

**6. No rerun-mismatch tracking (unlike the sudo-policy change).**
Whether `unattended-upgrades` is installed/configured is trivially inspectable by the operator directly on the VM (`systemctl status apt-daily-upgrade.timer`, or checking for `/etc/apt/apt.conf.d/20auto-upgrades`) if they ever need to check — there's no host-side secret or hard-to-observe state like there was for the sudo password, so a dedicated marker file would be redundant complexity. If `--no-auto-updates` is passed on a rerun against an existing VM, setup simply has no effect on that VM (cloud-init doesn't reapply), with no special warning needed beyond what's already documented in `vm-setup-rerun-recovery` about cloud-init being first-boot-only.

## Risks / Trade-offs

- **[Risk] `apt-get install unattended-upgrades` during cloud-init first boot adds time/network dependency to VM startup.** → Mitigation: single small package, no larger than `qemu-guest-agent` already installed the same way; acceptable given the security benefit.
- **[Risk] Restricting `Allowed-Origins` to security-only means non-security Debian updates never apply automatically, so the system can still drift from the latest point release.** → Accepted trade-off, consistent with Non-Goals; operators wanting full updates can run `apt full-upgrade` manually or pass `--no-auto-updates` and manage patching themselves.
- **[Risk] No automatic reboot means a security fix requiring a kernel restart sits half-applied until the operator reboots.** → Mitigation: the one-time setup note tells the operator this explicitly; `/var/run/reboot-required` (written by the standard `update-notifier-common`-style hook, or checkable via `needrestart`) remains the operator's signal — left as a manual check, consistent with Non-Goals on notifications.
- **[Trade-off] Security-only `Allowed-Origins` snippet is Debian/Ubuntu-naming-shaped, not something cloud-init validates.** A typo in the origin pattern would silently result in zero updates ever applying rather than a visible error. → Mitigation: use the well-documented, widely copy-pasted canonical pattern from Debian/Ubuntu's own `unattended-upgrades` documentation rather than inventing one; call this out in the verification task so it's checked manually once (see tasks.md).

## Migration Plan

N/A — additive, defaults to on for newly created VMs, no effect on already-existing VMs (cloud-init is first-boot-only). No rollback mechanism needed beyond `--no-auto-updates` on the next fresh VM, or manually disabling the timer/removing the config on an already-provisioned VM if desired.

## Open Questions

- Should a future observability change surface whether unattended-upgrades actually ran successfully (e.g., surfacing `/var/log/unattended-upgrades/` status)? Left for the separate observability TODO item rather than duplicated here.
- Should automatic reboot become a further opt-in flag (e.g., `--auto-reboot=03:00`) later, once resilience items (crash-restart, watchdog) are also in place and a mid-night restart is less likely to strand the VM? Left open.
