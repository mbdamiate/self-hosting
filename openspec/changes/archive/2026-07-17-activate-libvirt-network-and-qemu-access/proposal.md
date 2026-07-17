## Why

The setup script assumes that libvirt's default NAT network is active and that the QEMU service account can traverse the caller's home directory. On a fresh or partially configured Ubuntu host, those assumptions cause VM creation to fail or leave a created VM unable to start.

## What Changes

- Make NAT-mode setup ensure the libvirt `default` network is active before VM creation and configure it to start automatically on future host boots.
- Make setup grant the `libvirt-qemu` service account the minimum directory traversal permission needed to access VM disk and cloud-init seed files stored below the caller's home directory.
- Fail with actionable errors if either prerequisite cannot be established.

## Capabilities

### New Capabilities

- `libvirt-nat-network-readiness`: Ensure the default libvirt NAT network is available for NAT-mode VM creation.
- `qemu-vm-storage-access`: Ensure the QEMU service account can access VM storage created in the caller's home directory.

### Modified Capabilities

- None.

## Impact

- Affected code: `scripts/debian-vps-setup.sh`.
- Host systems: libvirt network configuration and access-control lists on the caller's home directory.
- Dependencies: uses the existing `virsh` client and adds the ACL utility package if it is not already present.
