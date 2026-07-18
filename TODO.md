# TODO

## Pending
- [ ] Review the VM setup and cleanup scripts for inconsistencies
- [ ] Fix debian-vm-cleanup.sh step ordering so removing the default libvirt network (step 5) doesn't silently no-op because packages (step 3, which removes the virsh binary) are purged first
- [ ] Create an explanatory README.md

## Done
- [x] Validate feasibility of renaming the provisioned "vps" — done via the `rename-vps-to-vm` change: scripts renamed to `debian-vm-{setup,cleanup}.sh`, 4 OpenSpec capabilities renamed `vps-*` → `vm-*`, wording updated throughout (except 4 lines describing the external rented-VPS concept being mimicked)
