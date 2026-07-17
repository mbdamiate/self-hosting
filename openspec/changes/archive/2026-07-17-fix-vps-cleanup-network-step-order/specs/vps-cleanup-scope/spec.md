## MODIFIED Requirements

### Requirement: `--purge-all` removes the entire environment
The cleanup script SHALL, when invoked with `--purge-all`, remove the VM, the entire working directory (including the base cloud image), the installed QEMU/libvirt packages, the caller's `libvirt`/`kvm` group membership, the default libvirt network, and the QEMU storage ACL grant on the caller's home directory, without prompting for confirmation for any step. The default-network removal SHALL execute before package purge, so that it is not skipped as a side effect of packages (and the `virsh` binary they provide) already having been removed earlier in the same run.

#### Scenario: Full teardown requested
- **WHEN** the cleanup script is invoked with `--purge-all`
- **THEN** it removes the VM and its storage, removes the default network, deletes the working directory in full, purges the declared packages, removes the caller from the `libvirt` and `kvm` groups, and revokes the `libvirt-qemu` ACL entry on the caller's home directory, without asking for confirmation at any step

#### Scenario: ACL revocation fails
- **WHEN** `--purge-all` attempts to revoke the QEMU storage ACL and the command fails
- **THEN** the script prints a warning and continues with the remaining steps instead of aborting

#### Scenario: Network removal is not skipped by package removal
- **WHEN** `--purge-all` removes both the default network and the installed packages in the same run
- **THEN** the default network is actually removed, rather than being silently skipped because `virsh` was already purged first

#### Scenario: No default network to remove
- **WHEN** the cleanup script reaches the network-removal step and `virsh` is unavailable or the `default` network is not defined
- **THEN** it reports that there is nothing to remove for this step instead of producing no output
