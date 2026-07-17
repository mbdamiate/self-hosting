## Why

Both `debian-vps-setup.sh` and `debian-vps-cleanup.sh` hardcode a single VM identity (`VM_NAME="debian-vps"`), so only one VM can exist at a time. The user's goal is to provision dozens of local VMs simulating a real production topology (app hosts, databases, backup targets, reverse proxies) to validate their own deploy tooling before paying for a real VPS. That requires naming, addressing, and sizing each VM independently, plus making sure tearing down one VM (or the whole shared environment) can't silently break the others.

## What Changes

- Add `--name=<name>` to both scripts, overriding today's hardcoded `"debian-vps"` default (used unchanged when the flag is omitted).
- Add `--ip=<ip>` to `debian-vps-setup.sh` for NAT-family modes (plain NAT or `--forward`): validates the address isn't already reserved or actively leased on the `default` network before creating anything; if omitted, auto-picks the next free address within the network's existing DHCP range. Reserves a MAC↔IP↔hostname binding on the `default` network before `virt-install` runs, so fleet VMs can reach each other by name (e.g. `app-01` resolving `db-01`) via libvirt's built-in dnsmasq — no manual IP lookup or `/etc/hosts` editing needed. **Usage error** if combined with `--bridge` (that mode's IP comes from the user's router, not libvirt's dnsmasq).
- Add `--ram=`, `--vcpus=`, `--disk=` to `debian-vps-setup.sh`, overriding today's hardcoded VM sizing (2048 MB / 2 vCPUs / 20 GB), defaulting silently to those values when omitted — no interactive prompt, keeping the script scriptable for repeated/batch invocation.
- **BREAKING** (behavior change, not flag removal): `--purge-all` on `debian-vps-cleanup.sh` now refuses to run — before touching anything — if any VM other than the one named by `--name` still exists on the host, since purging shared packages/groups/network would break those other VMs.
- `--vm-only` on `debian-vps-cleanup.sh` now also releases the named VM's network reservation (its DHCP host entry), so the IP/hostname becomes available again for reuse.

## Capabilities

### New Capabilities
- `vps-fleet-provisioning`: Naming, static IP/hostname reservation, and resource sizing for creating multiple independently-addressable VMs with `debian-vps-setup.sh`.

### Modified Capabilities
- `vps-cleanup-scope`: `--name` support on the cleanup script; `--purge-all` gains a pre-flight safety check that refuses to run while other VMs still exist; `--vm-only` additionally releases the target VM's network reservation.

## Impact

- Affected scripts: `scripts/debian-vps-setup.sh`, `scripts/debian-vps-cleanup.sh`.
- No fleet manifest/config file is introduced — each VM is still provisioned by one invocation of `debian-vps-setup.sh` per VM, not a declarative multi-VM description.
- No change to `--bridge` mode's networking behavior itself, only a new usage-error interaction with `--ip`.
