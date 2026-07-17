# TODO

## Pending
- [ ] Review the VPS setup and cleanup scripts for inconsistencies
- [ ] Fix debian-vps-cleanup.sh step ordering so removing the default libvirt network (step 5) doesn't silently no-op because packages (step 3, which removes the virsh binary) are purged first
- [ ] Create an explanatory README.md
- [ ] Validate feasibility of renaming the provisioned "vps"
