## Context

`debian-vps-setup.sh` and `debian-vps-cleanup.sh` both hardcode `VM_NAME="debian-vps"` as the only place that literal string appears — every other use already flows through the `$VM_NAME` variable (confirmed by grepping the script). `debian-vps-setup.sh` also hardcodes VM sizing (`VM_RAM_MB=2048`, `VM_VCPUS=2`, `VM_DISK_GB=20`) and gets its NAT IP from plain DHCP, with no way to predict or pin it. This design turns both scripts into tools that can provision and tear down many independently-named, independently-sized, independently-addressable VMs — the building blocks for a locally-simulated production topology (app hosts, databases, backup targets, reverse proxies).

## Goals / Non-Goals

**Goals:**

- Let each VM in a fleet be created with its own name, size, and a stable IP/hostname other fleet VMs can reach it by.
- Keep both scripts fully non-interactive when new flags are omitted, so fleet creation can be scripted/looped without blocking on prompts.
- Prevent a `--purge-all` run against one VM from silently breaking other VMs that still depend on the shared packages/network/groups.
- Keep VM-instance teardown (`--vm-only`) leaving no orphaned network state (a released VM's reserved IP/hostname must become available again).

**Non-Goals:**

- A fleet manifest/config file describing multiple VMs declaratively. The user confirmed a single parameterized script invoked once per VM is the model they want; a manifest format is a separate concern for later, if ever.
- Size "profiles" or `--role=` presets (e.g. a `db` role implying fixed RAM/disk values). `--ram`/`--vcpus`/`--disk` are independent, orthogonal flags; bundling them into named profiles is premature abstraction for three flags.
- An interactive prompt fallback when `--ram`/`--vcpus`/`--disk` are omitted. Rejected because it would block scripted/looped fleet creation, the primary use case driving this change.
- A `--force` override for the new `--purge-all` fleet-safety check. No concrete need identified; can be added later if one arises.
- Splitting the `default` network's dynamic DHCP range from a dedicated static-reservation range. The simpler approach (reserve within the existing range, checked against current leases) was chosen over this more collision-proof but more invasive alternative.
- Any change to `--bridge` mode's own networking behavior, beyond making `--ip` a usage error when combined with it.
- Concurrency/locking for two simultaneous `setup.sh` invocations picking the same free IP at the same time. Documented as a known, accepted risk (see Risks), not solved here.

## Decisions

### `--name` overrides today's single hardcoded identity

Both scripts gain `--name=<name>`, parsed the same way `--bridge=`/`--forward=` already are. When omitted, `VM_NAME` keeps defaulting to `"debian-vps"` exactly as today, so solo/no-flag use is unchanged. Since `$VM_NAME` already flows through every part of both scripts, this is a narrow, low-risk change — no other logic needs to change to support it.

### Static IP/hostname reservation via libvirt's own dnsmasq — reserve before creating

The `default` NAT network's dnsmasq instance supports DHCP host reservations (`virsh net-update default add ip-dhcp-host "<host mac='..' name='..' ip='..'/>" --live --config`), which simultaneously pins an IP to a MAC address and makes that hostname resolvable by other clients on the same network — no custom `/etc/hosts` management needed, since every fleet VM already uses this same dnsmasq as its DNS resolver via ordinary NAT DHCP.

Sequencing matters: the script generates/chooses a MAC address, registers the IP+hostname+MAC reservation on the network *before* calling `virt-install`, then passes that exact MAC to `virt-install` (`--network network=default,model=virtio,mac=...`). This way the VM's very first DHCP lease already matches the reservation — no reboot or lease-renewal dance needed to "pick up" a fixed address after the fact.

This mechanism is NAT-only (plain NAT or NAT+`--forward`), the same restriction `--forward` already has, since it depends on libvirt's own dnsmasq. Combining `--ip` with `--bridge` is a usage error: bridged VMs get their address from the user's physical router, which has no relationship to libvirt's reservation system.

Alternative considered: let the VM boot with a normal DHCP-assigned address, then add the reservation afterward and force a lease renewal. Rejected — reserving first and setting the MAC at creation time is simpler and avoids a window where the VM boots with the "wrong" (soon-to-change) address.

### `--ip` validation and auto-pick both use the same two checks

Whether the user supplies `--ip` or leaves it to auto-pick, the same two data sources decide "is this address free":
- `virsh net-dumpxml default` — existing static reservations (what our own fleet tooling has already claimed).
- `virsh net-dhcp-leases default` — addresses currently under an active dynamic lease (catches non-fleet or not-yet-reserved users of an address).

**Supplied `--ip`**: check both; if either shows the address taken, exit with a clear error naming the conflict, before any VM/disk work begins.

**Omitted `--ip`**: read the network's own configured DHCP range from `virsh net-dumpxml default` (`<ip><dhcp><range start=".." end=".."/></dhcp></ip>`) rather than hardcoding a subnet, and scan it for the first address absent from both the reservation list and the active-lease list.

Alternative considered: carve out a dedicated sub-range for fleet reservations, separate from the network's existing dynamic DHCP pool, eliminating any chance of collision. Rejected for this iteration — it requires a one-time reconfiguration of the shared `default` network's DHCP range, which the user preferred to avoid; the two-check approach (reservations + live leases) was accepted as "good enough" collision avoidance without touching existing network configuration.

### `--ram`/`--vcpus`/`--disk` — flags only, defaults when absent, no prompting

Mirrors `--bridge=`/`--forward=`'s existing convention exactly: parsed from `$arg`, override the corresponding hardcoded variable (`VM_RAM_MB`, `VM_VCPUS`, `VM_DISK_GB`) when present, silently keep today's value when absent. No new interactive code path is introduced anywhere in `debian-vps-setup.sh`, preserving its fully-scriptable, no-prompt design end to end.

### `--purge-all` fleet-safety pre-check

Before performing any removal step (same "fail fast before touching anything" pattern already used for the `--vm-only`/`--purge-all` mutual-exclusivity check), `--purge-all` runs `virsh list --all` and checks whether any VM other than the one named by `--name` exists. If so, it exits with a usage error listing the other VMs and directing the user to `--vm-only` each of them first. This is a pre-flight validation gate, not an interactive confirmation — it does not conflict with `--purge-all`'s existing requirement to never prompt.

Alternative considered: print a strong warning but still proceed. Rejected — proceeding would silently purge packages/network/groups still in use by other fleet VMs, and `--purge-all`'s whole contract is to never ask before acting, so a warning-then-continue would be actively harmful, not just noisy.

### `--vm-only` also releases the VM's network reservation

Since the IP/hostname reservation is specific to one VM's MAC, and `--vm-only`'s existing contract is "remove only this VM's instance, preserve everything shared," releasing that VM's reservation (`virsh net-update default delete ip-dhcp-host ...`) belongs to the same instance-level teardown — otherwise a deleted fleet VM's IP/hostname would stay claimed forever, unavailable for a replacement VM to reuse.

## Risks / Trade-offs

- [Two `setup.sh` invocations run at nearly the same moment and both auto-pick the same "free" IP before either reservation is registered] → Known, accepted race condition; not solved in this change. The user's actual usage pattern (manual/sequential fleet creation) makes this unlikely in practice; revisit if fleet creation ever becomes concurrent/automated.
- [Reserving an address that currently has no static reservation but is about to receive/has an active dynamic lease from something else] → Mitigated by checking `net-dhcp-leases` in addition to `net-dumpxml`, but a lease taken in between the check and the reservation is still possible (same class of race as above).
- [`--purge-all`'s new pre-check could surprise a user who forgot about a leftover VM from earlier testing] → Intentional: the error message lists the other VMs by name and tells the user exactly what to run (`--vm-only` per VM) to proceed, rather than silently doing destructive, cross-VM-impacting work.
- [Not carving out a dedicated static-IP range means fleet reservations live inside the same range ordinary one-off (non-fleet) VMs draw dynamic addresses from] → Accepted trade-off; revisit with a dedicated range if address exhaustion or collisions become an actual problem rather than a theoretical one.
