## 1. Module scaffolding and shared plumbing (`vmctl-cli`)

- [x] 1.1 Create the Go module and package skeleton: `cmd/vmctl/main.go`, `internal/cli/`, `internal/exec/`
- [x] 1.2 Implement `internal/exec.Runner` interface wrapping `os/exec`, plus a fake implementation for tests
- [x] 1.3 Implement shared `--name` flag parsing and `$HOME/vms/<name>` working-directory resolution in `internal/cli`
- [x] 1.4 Implement shared preflight checks (refuse to run as root; require `virsh` on `PATH`) in `internal/cli`
- [x] 1.5 Implement `Confirm(prompt string, autoApprove bool) bool` in `internal/cli`, replacing `confirm()`/`confirm_destructive()`
- [x] 1.6 Implement subcommand dispatch in `cmd/vmctl/main.go` with per-subcommand `flag.FlagSet` and generated `--help`
- [x] 1.7 Unit-test flag parsing, workdir resolution, preflight checks, and `Confirm` using the fake `Runner` (no KVM host required)

## 2. Port `cleanup` (smallest script first)

- [x] 2.1 Implement `vmctl cleanup` covering interactive, `--vm-only`, and `--purge-all` modes per `vm-cleanup-scope`
- [x] 2.2 Port the IP-reservation-release logic (`net-dumpxml` / `net-update` calls) behind `internal/exec`
- [x] 2.3 Unit-test cleanup's branching (mode selection, other-VMs-exist refusal for `--purge-all`) with the fake `Runner`
- [x] 2.4 Manually validate `vmctl cleanup` against the relevant `TESTING.md` steps on a real KVM host
- [x] 2.5 Delete `scripts/debian-vm-cleanup.sh` once parity is confirmed

## 3. Port `backup`

- [x] 3.1 Implement `vmctl backup snapshot|snapshot-restore|snapshot-delete` per `vm-disk-snapshot`
- [x] 3.2 Implement `vmctl backup backup|backup-restore|list` per `vm-disk-backup`
- [x] 3.3 Port `get_disk_info`-equivalent (`domblklist --details` parsing) behind `internal/exec`
- [x] 3.4 Unit-test subcommand dispatch and destructive-step confirmation with the fake `Runner`
- [x] 3.5 Manually validate `vmctl backup` (all sub-subcommands) against the relevant `TESTING.md` steps on a real KVM host
- [x] 3.6 Delete `scripts/debian-vm-backup.sh` once parity is confirmed

## 4. Port `setup` (largest, most introspection logic, last)

- [x] 4.1 Implement VM creation path: SSH key handling, disk image download/copy/resize, cloud-init generation, `virt-install`
- [x] 4.2 Implement fleet provisioning: naming, sizing, static IP/hostname reservation per `vm-fleet-provisioning`
- [x] 4.3 Implement rerun-recovery introspection (effective network mode, watchdog, on_crash policy, connection-info summary) per `vm-setup-rerun-recovery`
- [x] 4.4 Port remaining setup features (`--admin-password`, `--allow-port`, `--no-guest-firewall`, `--harden-host-firewall`, `--monitor`, `--watchdog`, `--no-crash-restart`, etc.) preserving existing spec behavior
- [x] 4.5 Verify every existing error message that names a `virsh` command for manual inspection is preserved per `vmctl-cli`'s transparency requirement
- [x] 4.6 Unit-test flag combinations and introspection branching with the fake `Runner`
- [x] 4.7 Manually validate `vmctl setup` (fresh create and rerun paths) against the full `TESTING.md` roteiro on a real KVM host
- [x] 4.8 Delete `scripts/debian-vm-setup.sh` once parity is confirmed

## 5. Fleet status (`vm-fleet-status`)

- [x] 5.1 Implement `vmctl list`: enumerate all defined VMs and print name, run state, RAM, vCPUs, disk size, effective network mode, IP
- [x] 5.2 Implement `vmctl status --name=NAME` reusing the same per-VM introspection as `vmctl list`
- [x] 5.3 Handle stopped/unreachable VMs gracefully (omit IP, don't fail the whole listing)
- [x] 5.4 Unit-test aggregation and per-VM error isolation with the fake `Runner`
- [x] 5.5 Manually validate `vmctl list`/`status` against a multi-VM fleet on a real KVM host

## 6. Consolidated metadata (`vm-tooling-metadata`)

- [~] 6.1 Spike `virsh metadata --set`/`--get` round-trip fidelity (escaping, size) for the three tracked facts against the libvirt version in use — **could not be run**: this environment has no working libvirtd connection (`Permission denied` on the socket), so there's no real host to spike against. Decision made without it (see 6.2).
- [x] 6.2 Implement `internal/metadata` using a JSON file under `WORK_DIR` (design.md option (a)) — chosen over libvirt domain `<metadata>` (option (b)) specifically because 6.1 couldn't be validated; revisit option (b) once a real libvirtd is available to spike against
- [x] 6.3 Wire `vmctl setup` to write admin sudo policy, log-forwarding, and guest firewall policy into the consolidated record instead of the three separate dotfiles
- [x] 6.4 Wire rerun-recovery introspection (task 4.3) to read from the consolidated record, treating a missing record as fully unconfigured
- [x] 6.5 Wire `vmctl cleanup --purge-all` to remove the record, and confirm `--vm-only` preserves it
- [x] 6.6 Unit-test missing-record and full/scoped-removal behavior with the fake `Runner`
- [x] 6.7 Manually validate the full setup → cleanup --vm-only → setup rerun → cleanup --purge-all cycle preserves/removes metadata as specified

## 7. Documentation and cutover

- [x] 7.1 Update `README.md` quick-start and flag references to the `vmctl` invocation form
- [x] 7.2 Update `TESTING.md` roteiro to invoke `vmctl` instead of the bash scripts
- [x] 7.3 Confirm all three `scripts/debian-vm-*.sh` files have been deleted (tasks 2.5, 3.6, 4.8)
- [ ] 7.4 Final end-to-end pass of `TESTING.md` against `vmctl` on a clean host
