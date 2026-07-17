## Context

`scripts/debian-vps-setup.sh` creates NAT-mode VMs with `--network network=default`, but enabling `libvirtd` does not guarantee that its `default` network is running. The script also writes VM storage below `$HOME/vms`, where the `libvirt-qemu` account may lack execute (directory traversal) permission on the caller's home directory.

## Goals / Non-Goals

**Goals:**

- Make plain NAT and `--forward` setup self-sufficient when the defined default network is inactive.
- Persist the default network's automatic start setting.
- Grant only the `libvirt-qemu` account directory traversal access needed for VM storage below `$HOME`.
- Stop before `virt-install` with an actionable error when the defined default network cannot be used.

**Non-Goals:**

- Defining or replacing a missing `default` network configuration.
- Reconfiguring host firewall, DNS, bridges, or libvirt storage pools.
- Changing bridged/macvtap networking behavior.
- Removing ACLs during cleanup; they can be shared by future local VMs and their removal could disrupt them.

## Decisions

### Prepare the default network only for NAT modes

After enabling `libvirtd` and confirming the refreshed user session, the script will query `default` with privileged `virsh`. For plain NAT and `--forward`, it will start an inactive defined network and enable autostart before VM creation. Bridged mode does not consume this network and will not alter it.

If `default` is undefined or cannot be started, setup will exit with a command-oriented diagnostic. Automatically defining a network from a system XML file was rejected because that file is distribution-dependent and could overwrite an administrator's intended network configuration.

### Use a POSIX ACL for QEMU traversal

The script will install the `acl` package and apply `u:libvirt-qemu:--x` to `$HOME` using `setfacl`. Execute-only access permits QEMU to traverse the directory without granting directory listing or read access. This fixes the exact access warning while retaining the existing, user-owned `$HOME/vms/<name>` layout.

Moving disks to a libvirt-managed storage pool was rejected for this change because it changes where users find and manage the generated assets. Broadening `$HOME` permissions with `chmod o+x` was rejected because it grants traversal to every local account rather than the QEMU service account.

## Risks / Trade-offs

- [The host does not provide a `default` network] → Fail before VM creation and tell the user to define or select a usable libvirt network.
- [The service account name differs on an unsupported distribution] → The script remains scoped to the existing Debian/Ubuntu target and emits an error if the expected account is unavailable.
- [An ACL already exists on `$HOME`] → `setfacl -m` adds or updates only the named `libvirt-qemu` entry, preserving unrelated ACL entries.
- [The filesystem does not support POSIX ACLs] → The ACL command fails and setup stops before attempting VM creation, with a diagnostic that explains the requirement.
