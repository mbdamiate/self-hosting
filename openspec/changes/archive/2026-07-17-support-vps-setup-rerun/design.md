## Context

`debian-vps-setup.sh` currently checks whether `$VM_NAME` already exists only in section 8, right before `virt-install` — after packages, group checks, NAT network prep, the QEMU storage ACL, SSH key generation, a full 20GB disk download/copy/resize, and cloud-init generation have all already run. On a match it prints one `ERROR` line and exits, so the user never sees the VM's IP or how to connect. The VM itself has no `virsh autostart` configured (only the `default` network does, from a prior change), so a host reboot leaves the VM `shut off` and re-running the script is a natural, expected recovery action — not just a user mistake.

## Goals / Non-Goals

**Goals:**

- Detect an existing `$VM_NAME` before any disk/cloud-init work, not right before `virt-install`.
- Never trust `--bridge`/`--forward` flags for a VM that already exists; determine its real network mode by inspecting the VM itself.
- Warn (not fail) when the requested flag conflicts with the VM's real mode, and continue using the real mode.
- Auto-start an existing-but-stopped VM before proceeding.
- Make `--forward` rule application idempotent so it can run safely against an already-existing VM.
- Configure `virsh autostart` on the VM so future host reboots need this recovery path less often.
- Always reach the same final connection/help block, regardless of which path was taken.

**Non-Goals:**

- Changing an existing VM's network mode on rerun. libvirt fixes network config at `virt-install` time; migrating it is out of scope. The script points the user at `virsh undefine --remove-all-storage` + rerun instead.
- Cleaning up stale `iptables` rules from a previous run (e.g. after the VM's DHCP lease changes). `debian-vps-cleanup.sh` already documents this limitation; this change does not extend cleanup's reach.
- Recovering from VM states other than "shut off" (e.g. `paused`, `crashed`). The script attempts `virsh start` and surfaces its native error; no special-cased recovery per state.
- Regenerating or re-injecting a new SSH key into an already-existing VM. cloud-init only runs on first boot, so a key generated after the fact would never reach `authorized_keys`.

## Decisions

### Move the existence check before SSH-key/disk/cloud-init work

The check moves to immediately after the QEMU storage ACL section (today's section 4), before SSH key handling. This does two things: it avoids the wasted download/copy/resize/cloud-init work on a rerun, and it avoids a subtler correctness bug — if the SSH key check ran unconditionally and the key file happened to be missing, it would silently generate a *new* keypair that was never baked into the already-existing VM's cloud-init, breaking the very connection instructions the script is about to print. The existing-VM branch skips SSH key handling, `$WORK_DIR` setup, and `virt-install` entirely.

### Determine effective network mode by introspection, not by trusting flags

After the existence branch (whether the VM was just created or already existed), the script runs `virsh domiflist "$VM_NAME"` and reads the interface `Type` column: `direct` means bridged (macvtap), `network` means NAT-family (plain NAT or `--forward`). This becomes the single source of truth (`EFFECTIVE_MODE`, plus `EFFECTIVE_BRIDGE_IFACE` from the `Source` column when bridged) for every downstream decision: which help text to print, and whether `--forward` can apply at all. The newly-created path also goes through this introspection rather than re-using `$BRIDGE_IFACE` directly, so there is exactly one implementation of "what mode is this VM in" instead of two (trust-flags for new, introspect for existing).

Rationale for introspecting even the just-created VM: one code path is simpler to keep correct than two, and it costs one extra `virsh` call.

If `virsh domiflist` returns no interface line (unexpected libvirt output), the script exits with a diagnostic rather than guessing — printing wrong connection instructions is worse than stopping.

### Warn, don't fail, on mode mismatch for an existing VM

Only in the existing-VM branch: compare the requested flag family (`--bridge` set vs. not) against `EFFECTIVE_MODE`. On mismatch, print a `WARNING` explaining that network mode is fixed at VM creation and cannot change via rerun, show the VM's actual mode/interface, and point to `virsh undefine --remove-all-storage` + rerun as the way to actually change it. The script then continues using `EFFECTIVE_MODE`. If `--forward` was requested but `EFFECTIVE_MODE` is bridged, forwarding is skipped with an explanation (port forwarding is a NAT-only overlay; there is nothing to forward to on a bridged VM).

Alternative considered: hard-fail on mismatch. Rejected — the user still has a usable VM and wants to reach it; refusing to show connection info over a flag mismatch is unhelpful and inconsistent with "always show the help block."

### Auto-start a stopped existing VM

In the existing-VM branch, check `virsh domstate "$VM_NAME"`. If it isn't `running`, run `virsh start "$VM_NAME"`; a failure here is fatal (actionable error, exit before the IP-detection tail), since nothing downstream works without a running VM. This mirrors the "repair and continue" pattern already used for the `default` NAT network.

### Configure VM autostart, but treat its failure as non-fatal

After the branch converges (covers both the just-created and the already-existing-and-now-running paths), run `virsh autostart "$VM_NAME"` unconditionally — idempotent, so it's safe to repeat every run, and it also retroactively fixes VMs created before this change existed. Unlike the `default` network's autostart (a hard requirement from a prior change, because without it the NAT network — and therefore the VM's connectivity — doesn't come up at all), a failure here only degrades a future-reboot convenience and must not block the current run's connection info. It's reported as a `WARNING`, and the script continues.

### Idempotent `--forward` application

Before each `iptables -t nat -A PREROUTING ...` / `iptables -I FORWARD ...`, run the equivalent `iptables -C` (check) call first; skip the `-A`/`-I` and print "already present" if the rule already exists. This is what makes `--forward` safe to officially support against an already-existing VM: without it, every rerun would stack a duplicate DNAT/ACCEPT pair for the same host/VM port combination.

## Risks / Trade-offs

- [User genuinely wants a different network mode] → Clear warning plus the exact recreate command, rather than silently reusing the wrong mode or attempting an unsupported live migration.
- [`virsh domiflist` output format differs across libvirt versions] → Parse only by the `Type` column value (`direct` / `network`), skipping header/separator lines; exit with a diagnostic if no interface line is found instead of guessing.
- [`virsh autostart` fails, e.g. permissions] → Warn and continue; this is a convenience feature, not a functional prerequisite for the current session.
- [VM's DHCP lease changed since a previous `--forward` run] → The idempotency check dedupes by exact host-port/VM-port/VM-IP match; a changed IP adds new rules alongside now-stale ones for the old IP. This is a pre-existing, documented limitation of `debian-vps-cleanup.sh` and is not extended or fixed by this change.
