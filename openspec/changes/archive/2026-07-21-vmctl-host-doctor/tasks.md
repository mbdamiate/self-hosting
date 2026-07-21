## 1. Shared host-readiness package

- [x] 1.1 Create `vmctl/internal/hostready` with `Check(ctx, r) []CheckResult`, `Fix(ctx, r, out) error`, `Unfix(ctx, r, out) error`
- [x] 1.2 Move `installPrerequisites`'s package-install, group-assignment, and `libvirtd`-enable bodies from `vmctl/internal/setup/prerequisites.go` into `Fix`
- [x] 1.3 Move `ensureNATNetworkReady`'s body into `Fix`
- [x] 1.4 Move `grantQEMUStorageACL`'s body into `Fix`
- [x] 1.5 Move the host-teardown steps from `vmctl/internal/cleanup/cleanup.go` (`--purge-all`/interactive: package purge, `apt autoremove`, `gpasswd -d` for both groups, `setfacl -x`, default-network removal, `libvirtd` stop/disable) into `Unfix`
- [x] 1.6 Add `Unfix`'s "no VM exists at all" guard (distinct from `--purge-all`'s existing "no other VM besides `--name`" guard)

## 2. Check implementation

- [x] 2.1 Add hardware-virtualization and apt-based-distro checks to `Check` (reuse existing `checkHardwareVirtualization`/`checkApt` logic)
- [x] 2.2 Add per-package presence checks: `exec.LookPath` for `virsh`, `virt-install`, `qemu-img`, `cloud-localds`, `ssh-keygen`, `setfacl`, `wget`; `dpkg -s` for `qemu-system-x86`, `bridge-utils`, and `genisoimage`
- [x] 2.3 Add the `libvirt`/`kvm` group-membership check, distinguishing "not granted" (`id -nG <username>` lacks the group) from "granted but session is stale" (current process's groups lack it)
- [x] 2.4 Add the `libvirtd` service-active check
- [x] 2.5 Add the `default` network check (defined, active, autostart)
- [x] 2.6 Add the QEMU storage ACL check (grant already present on `$HOME`)

## 3. `vmctl doctor` subcommand

- [x] 3.1 Add `vmctl/cmd/vmctl/doctor.go`: parse `--fix`/`--unfix` (mutually exclusive), default to plain report
- [x] 3.2 Plain report: run `Check`, print OK/MISSING per item, continue past failures, exit non-zero if any item is missing
- [x] 3.3 `--fix`: call `Fix`, matching today's `installPrerequisites`/`ensureNATNetworkReady`/`grantQEMUStorageACL` output/behavior
- [x] 3.4 `--unfix`: call `Unfix`
- [x] 3.5 Wire `doctor` into `vmctl/cmd/vmctl/main.go`'s subcommand switch and `usage()` text

## 4. `vmctl setup` becomes check-only for host prerequisites

- [x] 4.1 Replace `setup.go`'s calls to `installPrerequisites`/`ensureNATNetworkReady`/`grantQEMUStorageACL` with a call to `hostready.Check`
- [x] 4.2 Fail fast on the first non-OK result, with a message naming the specific gap and pointing to `vmctl doctor` / `vmctl doctor --fix`
- [x] 4.3 Preserve the existing `--bridge` skip of NAT-network verification (verify only in NAT/`--forward` mode, same as today's configure-only-in-NAT-mode behavior)

## 5. `vmctl cleanup --purge-all` drops host-level teardown

- [x] 5.1 Remove package purge, group removal, ACL revocation, and default-network removal from `--purge-all` and the interactive walkthrough
- [x] 5.2 Update `cleanup`'s help text and completion output to point to `vmctl doctor --unfix` for host-level teardown
- [x] 5.3 Confirm `--vm-only` is unaffected (it already excluded these steps)

## 6. Docs

- [x] 6.1 Update `README.md`'s "Prerequisites" section to describe the `vmctl doctor` / `vmctl doctor --fix` flow instead of "`vmctl setup` installs packages and manages group membership on your behalf"
- [x] 6.2 Update `TESTING.md` to insert a `vmctl doctor`/`--fix` step ahead of the existing setup-validation steps, and adjust the `--purge-all` test section to test `vmctl doctor --unfix` for host-level teardown instead

## 7. Verification

- [x] 7.1 Run `go build ./...` and existing unit tests in `vmctl/`
- [x] 7.2 On a real KVM-capable host: run `vmctl doctor` on an unprovisioned host and confirm it reports every missing item without changing anything
- [x] 7.3 Run `vmctl doctor --fix`, then `vmctl doctor` again, and confirm every item now reports OK
- [x] 7.4 Run `vmctl setup` on the now-provisioned host and confirm it performs no `apt`/`usermod`/`systemctl`/`setfacl` actions, only VM creation
- [x] 7.5 Run `vmctl setup` on a deliberately under-provisioned host (e.g. one missing package) and confirm it fails fast with an actionable message before any VM-creation work
- [x] 7.6 Remove the VM (`vmctl cleanup --vm-only`), then run `vmctl doctor --unfix` and confirm it fully reverts what `--fix` established
- [x] 7.7 Confirm `vmctl doctor --unfix` refuses to proceed while any VM still exists
