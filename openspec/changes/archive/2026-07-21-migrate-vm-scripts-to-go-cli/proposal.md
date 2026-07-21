## Why

The three VM lifecycle scripts (`debian-vm-setup.sh`, `debian-vm-cleanup.sh`, `debian-vm-backup.sh`) have grown to ~2124 lines of bash with no automated tests (`TESTING.md` is a fully manual roteiro requiring a real KVM host). Real duplication has already surfaced across them: the `WORK_DIR` convention is computed differently in `debian-vm-backup.sh` than in the other two, `confirm()` (cleanup) and `confirm_destructive()` (backup) implement the same intent with silently different bypass behavior, and the "don't run as root" check is copy-pasted verbatim in all three files. The author is also more fluent in Go than bash. Unifying the three scripts into a single Go CLI (`vmctl`) consolidates this shared logic behind one implementation and makes it unit-testable without a KVM host.

## What Changes

- Replace `debian-vm-setup.sh`, `debian-vm-cleanup.sh`, and `debian-vm-backup.sh` with a single compiled Go binary, `vmctl`, exposing subcommands: `vmctl setup`, `vmctl cleanup`, `vmctl backup snapshot|backup|restore|list`. **BREAKING**: invocation changes from `./scripts/debian-vm-*.sh ...` to `vmctl <subcommand> ...`; the bash scripts are removed once `vmctl` covers their behavior.
- Consolidate logic duplicated across the current scripts into shared internal packages: flag/subcommand parsing (with `--help` generated from the same flag declarations, not hand-written text), the `--name` → `$HOME/vms/$NAME` working-directory convention, the non-root and `virsh`-presence preflight checks, and a single confirmation helper with an explicit `autoApprove` parameter (replacing the two divergent `confirm()`/`confirm_destructive()` implementations).
- Scope of this phase (Level 1, decided during exploration): `vmctl` still shells out to the same external tools the bash scripts call today (`virsh`, `virt-install`, `qemu-img`, `cloud-localds`/`genisoimage`, `iptables`). It does not adopt libvirt Go bindings (`go-libvirt`/`libvirtxml`) in this phase — that remains a possible future phase, out of scope here. `vmctl` is a single executable that runs and exits, like the scripts it replaces; it is not a daemon or network service. `libvirtd` remains the only long-running process in the picture, already installed and managed by systemd, unchanged by this work.
- New: `vmctl list` / `vmctl status [--name=NAME]` — an aggregated, fleet-wide view (name, running state, RAM, vCPUs, disk size, effective network mode, IP) built by querying libvirt live for every defined VM. No new capability exists today for viewing more than one VM at a time (`backup-list` is per-VM only). This view queries libvirt live on every invocation and persists nothing, so it cannot drift from the state `vm-setup-rerun-recovery` already treats as authoritative.
- New: consolidate the guest-only facts that `debian-vm-setup.sh` currently records as separate ad hoc dotfiles (`${WORK_DIR}/.admin-sudo-policy`, `.log-forwarding-configured`, `.guest-firewall-policy`) into a single structured per-VM metadata record. `design.md` will decide and justify between (a) one typed JSON/YAML file under the VM's `WORK_DIR`, or (b) libvirt's native domain `<metadata>` XML element (via `virsh metadata` / `DomainSetMetadata`), which ties the metadata's lifecycle to the domain's own definition.
- Explicitly out of scope: libvirt API bindings replacing `virsh` exec calls (considered as "Level 2" during exploration), and any daemon/HTTP/gRPC service in front of `vmctl` (considered as "Level 3"). Both were evaluated and deferred, not rejected outright.

## Capabilities

### New Capabilities
- `vmctl-cli`: The unified binary's subcommand structure, shared flags (`--name` and its `$HOME/vms/$NAME` working-directory convention), preflight checks (non-root, `virsh` present), and confirmation semantics (interactive prompt vs. non-interactive auto-approve) that replace the three independent scripts' CLI surfaces.
- `vm-fleet-status`: The `vmctl list` / `vmctl status` commands that report live, non-cached state (name, run state, resources, network mode, IP) across all defined VMs.
- `vm-tooling-metadata`: The consolidated record of guest-only facts (admin sudo policy, log-forwarding configuration, guest firewall policy) that libvirt itself cannot report, replacing the current scattered dotfiles with one structured record whose storage mechanism is chosen in `design.md`.

### Modified Capabilities
(none — existing specs under `openspec/specs/` phrase their requirements in terms of `virsh` subcommands and generic "setup"/"cleanup"/"backup script" behavior, not the bash filenames. Since this phase keeps every existing `virsh`/`virt-install`/`qemu-img`/`cloud-localds` invocation and its observable behavior unchanged, no existing requirement changes — only the implementation language and the CLI entry point do.)

## Impact

- Affected code: `scripts/debian-vm-setup.sh`, `scripts/debian-vm-cleanup.sh`, `scripts/debian-vm-backup.sh` are removed and replaced by a new Go module (module layout decided in `design.md`).
- Affected docs: `README.md` quick-start commands and `TESTING.md` roteiro need updating to the `vmctl` invocation form.
- New build/runtime dependency: a Go toolchain to build `vmctl` (the compiled binary itself has no new runtime dependency beyond what the scripts already require: `virsh`, `virt-install`, `qemu-img`, `cloud-localds`/`genisoimage`, `iptables`).
- No change to host prerequisites, package installation behavior, or any libvirt/network/firewall guarantee documented in existing specs.
