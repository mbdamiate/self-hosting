## Why

`vmctl setup` currently bootstraps the host on every single invocation: it runs `apt update && apt install -y <11 packages>`, adds the caller to the `libvirt`/`kvm` groups, enables `libvirtd`, starts/autostarts the libvirt `default` network, and grants an ACL on `$HOME` to `libvirt-qemu` — all unconditionally, all requiring broad `sudo`, even when the host is already fully provisioned. This conflates two different concerns: one-time host bootstrap (packages, services, group membership) and per-invocation VM lifecycle work (creating/reusing a VM). We want `vmctl` to assume the host arrives ready (provisioned out-of-band — manually, via Ansible, or a golden image — guided by the dependency list already documented in `DEPENDENCIES.md`) and to only verify that readiness, failing fast with an actionable message instead of silently installing/configuring. This shrinks the `sudo` surface of routine VM operations to just what VM lifecycle work actually needs.

## What Changes

- Add a new `vmctl doctor` subcommand:
  - `vmctl doctor` (no flag): runs every host-readiness check and prints a full OK/MISSING report; does not stop at the first failure.
  - `vmctl doctor --fix`: performs the host bootstrap actions `vmctl setup` performs today (install packages, add groups, enable `libvirtd`, start/autostart the `default` network, grant the ACL) — this reuses, not duplicates, the existing `installPrerequisites`/`ensureNATNetworkReady`/`grantQEMUStorageACL` logic.
  - `vmctl doctor --unfix`: performs the host teardown actions `vmctl cleanup --purge-all` performs today (purge packages, remove group membership, revoke the ACL, remove the `default` network).
- **BREAKING**: `vmctl setup` no longer installs packages, adds group membership, enables `libvirtd`, starts/autostarts the `default` network, or grants the ACL. It instead runs the same readiness checks `doctor` uses and fails fast on the first missing requirement, pointing to `vmctl doctor` / `vmctl doctor --fix`.
- **BREAKING**: `vmctl cleanup --purge-all` (and the equivalent interactive walkthrough) no longer purges packages, removes group membership, revokes the ACL, or removes the `default` network. It continues to remove VM-scoped resources plus the opt-in host features it already owns (`--harden-host-firewall` hardening, the `--monitor` infrastructure) — those flags remain unaffected by this change. Host bootstrap teardown moves to `vmctl doctor --unfix`.
- Package-presence checks use whichever mechanism actually verifies the package: `exec.LookPath` for packages whose binary `vmctl` invokes directly (`virsh`, `virt-install`, `qemu-img`, `cloud-localds`, `ssh-keygen`, `setfacl`, `wget`), and `dpkg -s` for packages with no directly-invoked binary (`qemu-system-x86`, `bridge-utils`, `genisoimage`) since libvirtd/`virt-install`/`cloud-localds` use them transitively. `libvirt-daemon-system`'s presence is implied by the `libvirtd` service-active check instead of a separate package check.
- Out of scope: `--harden-host-firewall` and `--monitor` keep auto-installing/configuring when requested — they are deliberate, opt-in actions taken at the user's request in that invocation, not implicit host bootstrap.
- `README.md` and `TESTING.md` are updated to describe the new host-readiness flow (`vmctl doctor` before first use) instead of "`vmctl setup` installs packages and manages group membership on your behalf."

## Capabilities

### New Capabilities
- `vmctl-host-doctor`: the `vmctl doctor` subcommand (plain report, `--fix`, `--unfix`) and the shared host-readiness check functions it and `vmctl setup`'s preflight both call.

### Modified Capabilities
- `ubuntu-qemu-prerequisites`: package installation requirement becomes a package-presence check requirement; the cleanup purge requirement moves to `doctor --unfix`.
- `libvirt-group-session-handling`: `usermod`-based group assignment is removed from `setup`; setup's existing "verify both groups are active in the current session" requirement is retained as-is (it becomes the entirety of setup's group-related behavior), and group assignment itself becomes part of `doctor --fix`.
- `qemu-vm-storage-access`: the "install ACL utility if needed and grant access" requirement becomes "verify the ACL grant already exists"; granting moves to `doctor --fix`.
- `libvirt-nat-network-readiness`: the "start and configure autostart" requirement becomes "verify the network is defined, active, and set to autostart"; starting/autostart-configuring moves to `doctor --fix`.
- `vm-cleanup-scope`: `--purge-all`'s requirement to remove installed packages, group membership, the ACL, and the `default` network is removed (that removal moves to `doctor --unfix`); all other `--purge-all` and `--vm-only` behavior (VM removal, monitoring/firewall teardown, log handling) is unchanged.
- `vmctl-cli`: the "Single binary with subcommands" requirement's enumerated subcommand list gains `doctor`. This is the only touch this change makes to `vmctl-cli`; the broader verb rename (`create`/`start`/`stop`/etc.) is deferred to the follow-up change.

## Impact

- Code: `vmctl/internal/setup/prerequisites.go` (checks replace actions, functions become shared), `vmctl/internal/cli/preflight.go` (generalize the `RequireVirsh`-style check-only pattern), `vmctl/internal/setup/setup.go` (call checks instead of `installPrerequisites`/`ensureNATNetworkReady`/`grantQEMUStorageACL`), `vmctl/internal/cleanup/cleanup.go` (remove host-teardown steps from `--purge-all`/interactive flow), new `vmctl/cmd/vmctl/doctor.go` and a new `vmctl/internal/doctor` (or similar) package hosting the shared checks plus `--fix`/`--unfix`.
- Docs: `README.md`, `TESTING.md`, and the five modified specs above.
- Sudo footprint: a bare `vmctl setup` (no `--harden-host-firewall`/`--monitor`) should require little to no `sudo` once the host is provisioned, since it no longer installs or configures anything at the host level.
- Sequencing: this change is the prerequisite for a follow-up change (not part of this proposal) that renames/expands `vmctl`'s per-VM subcommands into a VPS-style verb set (`create`/`start`/`stop`/`reboot`/`delete`/`list`/`info`/`snapshot`/`backup`); that change assumes `setup`/`cleanup` are already lean before renaming them.
