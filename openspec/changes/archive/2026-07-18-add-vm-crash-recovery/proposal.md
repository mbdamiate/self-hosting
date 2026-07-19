## Why

`debian-vm-setup.sh` already configures `virsh autostart` so a VM comes back after the *host* reboots, and (with `add-vm-watchdog`) can reset itself if the *guest OS* hangs. Neither covers the case where the QEMU process itself crashes while the host keeps running — libvirt's default behavior for that (`on_crash` unset, effectively `destroy`) leaves the domain stopped until an operator notices and runs `virsh start` by hand. `on_crash=restart` is the one-line libvirt domain setting that closes this specific gap.

## What Changes

- Set `on_crash=restart` on freshly created VMs (via `virt-install`), so libvirt automatically restarts a domain whose QEMU process crashes, instead of leaving it stopped.
- Default this **on** (unlike the opt-in flags added in recent sibling proposals): a genuine QEMU crash is unambiguous — there's no "false positive" analogous to the watchdog's busy-but-not-hung guest, so there's no ergonomic cost to defaulting this on.
- Add a `--no-crash-restart` opt-out flag for the rare case an operator wants a crashed VM left stopped (e.g., to inspect state before it's touched again), mirroring the `--no-auto-updates`/`--no-guest-firewall` opt-out pattern already used for other default-on behavior.
- Extend the existing rerun-recovery behavior (already covering network mode, and extended for watchdog configuration in `add-vm-watchdog`) to also determine a VM's effective `on_crash` policy by inspecting it, and warn rather than fail on a mismatch when reusing an already-existing VM.
- `--help` documents the setting, the default-on rationale, and its relationship to `virsh autostart` (host reboot) and the watchdog device (guest hang), so the three recovery layers aren't conflated. Per the existing `repository-readme` spec (README SHALL NOT restate the flag reference), `README.md` itself needs no new prose.

## Capabilities

### New Capabilities
- `vm-crash-recovery`: the default-on `on_crash=restart` domain setting and its `--no-crash-restart` opt-out.

### Modified Capabilities
- `vm-setup-rerun-recovery`: gains the ability to determine a VM's effective `on_crash` policy by inspecting it and to warn, rather than fail, on a mismatch when reusing an already-existing VM — the same extension pattern `add-vm-watchdog` already applied for watchdog configuration.

## Impact

- `scripts/debian-vm-setup.sh`: `virt-install` invocation (`on_crash=restart` by default), argument parsing (`--no-crash-restart`), effective-`on_crash` introspection and mismatch warning on reuse, `--help` text.
- No impact on `README.md` — covered by the existing `--help`/`openspec/specs/` pointers per `repository-readme`.
- No impact on `scripts/debian-vm-cleanup.sh` — `on_crash` is part of the VM's own domain definition, removed automatically whenever the VM itself is removed.
