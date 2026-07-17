## Why

`debian-vps-cleanup.sh` purges QEMU/libvirt packages (step 3, which removes the `virsh` binary via `libvirt-clients`) before it attempts to remove the default libvirt network (step 5, which requires `virsh`). Whenever a run actually purges packages, step 5's `command -v virsh` guard fails and the entire network-removal step — including its confirmation prompt — is skipped with no output at all. This means `--purge-all` and the interactive no-flags path (when packages are confirmed) never actually remove the default network, silently falling short of what they already claim to do, and the user gets no indication anything was skipped. This was confirmed live while manually verifying the `add-vps-cleanup-scope-flags` change; the bug predates that change (present since the script's first commit) and is being fixed as a separate follow-up.

## What Changes

- Reorder `debian-vps-cleanup.sh` so default-network removal runs before package purge, so `virsh` is still present when that step needs it.
- Add a visible message for the case where `virsh`/libvirt is unavailable for the network step (e.g., a second cleanup run after libvirt is already gone), matching how steps 1 and 2 already report their own "nothing to do" cases instead of producing no output.
- No changes to what any individual step removes, to the `--vm-only`/`--purge-all`/no-flags gating logic, or to `debian-vps-setup.sh`.

## Capabilities

### New Capabilities

<!-- None. -->

### Modified Capabilities
- `vps-cleanup-scope`: The `--purge-all` requirement's guarantee that the default network is removed must actually hold when packages are also purged in the same run, and the script must report rather than silently skip when the network step has nothing to act on.

## Impact

- Affected script: `scripts/debian-vps-cleanup.sh` (step ordering only; no changes to individual step logic, flag parsing, or gating).
- No changes to `scripts/debian-vps-setup.sh`.
- Depends on the `add-vps-cleanup-scope-flags` change's `vps-cleanup-scope` capability (that change is implemented; this proposal modifies its spec, whether read from the archive or from that change's own delta if not yet archived).
