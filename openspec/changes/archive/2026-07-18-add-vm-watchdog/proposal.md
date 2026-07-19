## Why

`vm-uptime-monitoring` (from `add-vm-observability`) already distinguishes "the QEMU process died" from "the guest OS is hung but QEMU is still running" — but it only detects and alerts, it never acts. A VM meant to run as a real server benefits from a second, lower-level, self-healing mechanism for exactly the "guest is hung" case: a virtual watchdog device that the guest OS itself must keep petting, wired to automatically reset the VM if it stops. This is the standard QEMU/libvirt idiom for guest-level watchdogging, and it's independent of whether host-side monitoring is even installed.

## What Changes

- Add a `--watchdog` flag to `debian-vm-setup.sh` that attaches a virtual watchdog device (`i6300esb`, the standard, widely-supported model) to the VM at creation, configured to reset the VM if the guest stops petting it.
- Configure the guest, via cloud-init, to pet the watchdog using systemd's built-in `RuntimeWatchdogSec=` — no extra daemon needed, since systemd (PID 1) already knows how to drive `/dev/watchdog` natively. If the kernel or PID 1 itself hangs badly enough to stop petting it, the device fires and QEMU resets the VM.
- This is distinct from, and complementary to, libvirt's `on_crash` policy (a separate, already-tracked TODO item): `on_crash` reacts to the QEMU *process* aborting; the watchdog reacts to the *guest OS* becoming unresponsive while QEMU keeps running fine. Neither substitutes for the other.
- Since the watchdog device is part of the VM's domain definition (fixed at creation, inspectable via `virsh dumpxml`, like network mode), extend the existing rerun-recovery behavior: rerunning setup against an already-existing VM with a different `--watchdog` state than what it was created with warns rather than fails, mirroring how network-mode mismatches are already handled.
- Application-level health checks (e.g., "is my web server actually serving requests") are explicitly out of scope — this repo provisions a base OS image and can't know what the operator will run on it. The classic Linux `watchdog` package's test-binary mechanism is documented as an extension point the operator can add themselves later, not implemented here.
- `--help` documents the flag, the reset behavior, and the distinction from `on_crash`. Per the existing `repository-readme` spec (README SHALL NOT restate the flag reference), `README.md` itself needs no new prose.

## Capabilities

### New Capabilities
- `vm-guest-watchdog`: the opt-in virtual watchdog device, its guest-side petting mechanism (`RuntimeWatchdogSec`), and its reset action.

### Modified Capabilities
- `vm-setup-rerun-recovery`: gains the ability to determine a VM's effective watchdog configuration by inspecting it (like it already does for network mode) and to warn, rather than fail, on a mismatch between the requested and actual configuration when reusing an already-existing VM.

## Impact

- `scripts/debian-vm-setup.sh`: argument parsing (`--watchdog`), `virt-install` invocation (attaching the watchdog device), cloud-init `user-data` changes (`RuntimeWatchdogSec` drop-in), watchdog-configuration introspection and mismatch warning on reuse, `--help` text.
- No impact on `README.md` — covered by the existing `--help`/`openspec/specs/` pointers per `repository-readme`.
- No impact on `scripts/debian-vm-cleanup.sh` — the watchdog device is part of the VM's own domain definition, removed automatically whenever the VM itself is removed (`--vm-only` or `--purge-all`), same as any other domain-level setting.
