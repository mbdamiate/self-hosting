## 1. `--name` on both scripts

- [x] 1.1 Add `--name=` parsing to `debian-vps-setup.sh`, overriding `VM_NAME`/`VM_HOSTNAME` (default `"debian-vps"` when omitted)
- [x] 1.2 Add `--name=` parsing to `debian-vps-cleanup.sh`, overriding `VM_NAME` (default `"debian-vps"` when omitted)
- [x] 1.3 Update both scripts' `-h`/`--help` output and header comments to document `--name`

## 2. VM sizing flags on setup.sh

- [x] 2.1 Add `--ram=`, `--vcpus=`, `--disk=` parsing to `debian-vps-setup.sh`, overriding `VM_RAM_MB`, `VM_VCPUS`, `VM_DISK_GB` (defaults unchanged when omitted)
- [x] 2.2 Update `-h`/`--help` output and header comments to document the three sizing flags

## 3. IP/hostname reservation on setup.sh

- [x] 3.1 Add `--ip=` parsing to `debian-vps-setup.sh`; reject with a usage error if combined with `--bridge`
- [x] 3.2 Implement the "is this address free" check: parse `virsh net-dumpxml default` for existing DHCP host reservations, and `virsh net-dhcp-leases default` for active leases
- [x] 3.3 When `--ip` is supplied: run the free-address check against it; exit with a clear error (naming the conflicting VM/lease) before any VM/disk work if it's taken
- [x] 3.4 When `--ip` is omitted: read the `default` network's configured DHCP range from `virsh net-dumpxml default`, scan it, and auto-pick the first address that passes the free-address check
- [x] 3.5 Generate/choose a MAC address for the new VM and register the IP+hostname+MAC reservation on the `default` network (`virsh net-update default add ip-dhcp-host ...`) before calling `virt-install`
- [x] 3.6 Pass the reserved MAC explicitly to `virt-install`'s `--network` argument so the VM's first DHCP lease matches the reservation
- [x] 3.7 Update `-h`/`--help` output and header comments to document `--ip` and its NAT-only scope

## 4. `--purge-all` fleet-safety check

- [x] 4.1 Before any removal step in `debian-vps-cleanup.sh`, run `virsh list --all` and check for VMs other than the one named by `--name`
- [x] 4.2 If any other VM exists, exit with a usage error listing them by name and directing the user to `--vm-only` each first, before performing any removal
- [x] 4.3 Update `--purge-all`'s help text/notes to mention this pre-check

## 5. `--vm-only` releases the network reservation

- [x] 5.1 In `debian-vps-cleanup.sh`'s `--vm-only` path, remove the target VM's DHCP host reservation from the `default` network (`virsh net-update default delete ip-dhcp-host ...`) alongside removing the VM itself
- [x] 5.2 Make this removal tolerant of the VM having no reservation (e.g. it was created without `--ip`/before this change) — skip silently rather than erroring

## 6. Verification

- [x] 6.1 Manually create two VMs with distinct `--name`/`--ip` values and confirm each gets its intended IP, and that one can resolve and reach the other by hostname (e.g. `getent hosts`/`ping` from inside one VM to the other's name)
- [x] 6.2 Manually run setup.sh a second time without `--ip` while the first VM's reservation exists, and confirm the auto-picked address differs from the already-reserved one
- [x] 6.3 Manually attempt `--ip` with an address already reserved by another VM, and with an address under an active (unreserved) lease, and confirm both are rejected before any VM/disk work
- [x] 6.4 Manually attempt `--ip` combined with `--bridge` and confirm it's rejected as a usage error
- [x] 6.5 Manually attempt `--purge-all` while two fleet VMs exist and confirm it refuses, listing both VMs, before removing anything
- [x] 6.6 Manually run `--vm-only` against one fleet VM and confirm its DHCP reservation is gone afterward (`virsh net-dumpxml default` no longer lists it) while the other VM and the shared environment are untouched
