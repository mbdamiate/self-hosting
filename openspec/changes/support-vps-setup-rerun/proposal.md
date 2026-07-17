## Why

Running `debian-vps-setup.sh` a second time against an already-created VM wastes time re-downloading/copying/resizing the disk image and regenerating cloud-init, then fails with a single-line error before `virt-install` — never showing the connection info the user needs. This is a plausible everyday scenario, not just user error: the VM has no `virsh autostart` configured (only the `default` network does), so after a host reboot the VM is `shut off` and re-running the script is the natural recovery action.

## What Changes

- Move the "VM already exists" check earlier, before disk image download/copy/resize and cloud-init generation, so a rerun short-circuits before doing unnecessary work.
- When the VM already exists, inspect its actual network mode via `virsh domiflist` instead of trusting the `--bridge`/`--forward` flags passed on the rerun. Warn (not fail) when the requested flag conflicts with the VM's real network mode, and continue using the real mode for the rest of the flow.
- Auto-start the VM with `virsh start` if it already exists but is not running, mirroring the existing "repair and continue" pattern used for the default NAT network.
- Make `--forward` port-forwarding application idempotent (check with `iptables -C` before each `-A`) so it can be safely reapplied against an already-existing VM, turning it into an officially supported flow rather than a create-only side effect.
- Configure `virsh autostart` on the VM itself right after a successful `virt-install`, reducing (but not eliminating) how often the rerun path is needed after a host reboot.
- Always show the final connection/help block (VM IP lookup, SSH command, useful `virsh` commands) at the end of the script, for both the newly-created and the already-existing VM paths, using the real detected network mode to pick the right instructions.

## Capabilities

### New Capabilities

- `vps-setup-rerun-recovery`: Detect an already-existing VM early, reconcile its real network mode against the requested flags, auto-start it if stopped, and always reach the same helpful connection-info ending as a fresh run.
- `vps-port-forward-reapplication`: Apply `--forward` port-forwarding rules idempotently, whether against a freshly created VM or one that already existed, without duplicating iptables rules on repeated runs.

### Modified Capabilities

- None.

## Impact

- Affected code: `scripts/debian-vps-setup.sh`.
- Host systems: libvirt VM autostart configuration and host `iptables` NAT/FORWARD rules.
- Dependencies: uses the existing `virsh` and `iptables` clients already relied on by the script; no new packages.
