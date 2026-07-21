## Context

`vmctl setup` (`vmctl/internal/setup/setup.go`) currently calls, unconditionally and in this order: `checkHardwareVirtualization` (check-only), `checkApt` (check-only), `installPrerequisites` (apt install 11 packages, `usermod -aG libvirt,kvm`, `systemctl enable --now libvirtd`), `ensureNATNetworkReady` (start/autostart the libvirt `default` network, skipped only in `--bridge` mode), and `grantQEMUStorageACL` (`setfacl` on `$HOME`) — all in `vmctl/internal/setup/prerequisites.go`. `vmctl cleanup --purge-all` (`vmctl/internal/cleanup/cleanup.go`) performs the mirror-image teardown: `apt purge`/`autoremove`, `gpasswd -d` for both groups, `setfacl -x`, removing the `default` network, stopping/disabling `libvirtd`.

A check-only pattern already exists in this codebase: `vmctl/internal/cli/preflight.go`'s `RequireVirsh()` does `exec.LookPath("virsh")` and fails with `"'virsh' was not found. Run 'vmctl setup' first"`. This change generalizes that exact pattern to the rest of the host-bootstrap surface.

`DEPENDENCIES.md` (repo root) already enumerates every package/service this affects and is the reference operators use to provision a host out-of-band.

## Goals / Non-Goals

**Goals:**
- One set of host-readiness check functions, shared by `vmctl doctor` (report) and `vmctl setup`'s preflight (fail-fast), so the two never drift.
- `vmctl doctor --fix` / `vmctl doctor --unfix` preserve today's install/configure and purge/revert behavior byte-for-byte in effect — this is a relocation, not a rewrite of that logic.
- `vmctl setup` and `vmctl cleanup --purge-all` stop touching host-level package/service/group/ACL/network state.
- Package-presence checks are accurate for packages `vmctl` never invokes directly.

**Non-Goals:**
- Not touching `--harden-host-firewall` or `--monitor` — they keep installing/configuring on request.
- Not touching guest-side (cloud-init) package installation — irrelevant here, each VM is freshly imaged.
- Not renaming any subcommand or changing `vmctl setup`/`vmctl cleanup`/`vmctl backup`'s per-VM behavior — that's the follow-up change.
- Not automating the out-of-band host provisioning itself (no Ansible playbook, no golden image) — `vmctl doctor --fix` remains the one supported "do it for me" path; anything more elaborate is the operator's choice, guided by `DEPENDENCIES.md`.

## Decisions

**1. New `vmctl/internal/hostready` package holds the shared checks, `Fix`, and `Unfix`.**
Neither `internal/setup` nor `internal/cleanup` is the natural home: the checks are consumed by both (`setup`'s preflight and `doctor`'s report), and `Unfix` is logically paired with `Fix`, not with VM-scoped cleanup. A neutral package avoids `setup`↔`cleanup` cross-imports and avoids duplicating check logic in two places.
- `Check() []CheckResult` — one result per requirement (hardware virt, apt-based distro, each of the 11 packages, `libvirt`/`kvm` group active in the current session, `libvirtd` active, `default` network defined+active+autostart, ACL granted on `$HOME`). Never mutates state.
- `Fix(ctx, r, out) error` — today's `installPrerequisites`/`ensureNATNetworkReady`/`grantQEMUStorageACL` bodies, moved as-is (they're already idempotent, so no behavior change).
- `Unfix(ctx, r, out) error` — today's `cleanup.go` host-teardown steps, moved as-is.
`setup.go`'s preflight calls `Check()` and returns the first non-OK result's detail as an error, naming `vmctl doctor` / `vmctl doctor --fix`. `cmd/vmctl/doctor.go` calls `Check()` (plain), `Fix()` (`--fix`), or `Unfix()` (`--unfix`) — the three are mutually exclusive flags.

**2. Package-presence check strategy: `exec.LookPath` where possible, `dpkg -s` otherwise.**
`virsh` (`libvirt-clients`), `virt-install` (`virtinst`), `qemu-img` (`qemu-utils`), `cloud-localds` (`cloud-image-utils`), `wget` (`wget`), `ssh-keygen` (`openssh-client`), `setfacl` (`acl`) are binaries `vmctl` already invokes directly — checking their presence via `exec.LookPath` is both accurate (it's the exact thing that will fail later if absent) and cheap (no subprocess). `qemu-system-x86`, `bridge-utils`, and `genisoimage` provide no binary `vmctl` calls directly (libvirtd/`virt-install`/`cloud-localds` use them transitively) — for these three, `Check()` shells out to `dpkg -s <pkg>`. This is acceptable because `checkApt` already hard-assumes an apt/dpkg-based host; it does not introduce a new distro assumption. `libvirt-daemon-system` needs no separate presence check at all — its install is already implied by the (stronger) `libvirtd` service-active check. `openssl` is not part of `basePackages` at all (it's assumed already present on any base Debian/Ubuntu install, exactly like `checkApt`'s own baseline assumption) and is not checked here.

**3. Group-membership check reports session-staleness distinctly from absence.**
`id -nG` (no argument) reflects the *current process's* groups — stale until a fresh login even after `usermod` succeeds. `id -nG <username>` asks the system directly and reflects real membership immediately. `Check()` uses `id -nG <username>` to determine whether the grant exists at all, and separately inspects the current process's groups to determine whether the *active session* has it. This lets `doctor`'s report and `setup`'s fail-fast distinguish two different actionable messages: "not granted — run `vmctl doctor --fix`" vs. "granted, but this session predates it — log out and back in." This mirrors what `libvirt-group-session-handling` already requires today; the check simply moves out of `setup`'s post-`usermod` path into a standalone, reusable check.

**4. `doctor --unfix` requires no VM to exist at all, not "no other VM besides the named one."**
Today's `vm-cleanup-scope` guards `--purge-all` against removing shared host state while other named VMs still exist ("other than the one named by `--name`"), because `--purge-all` is itself VM-scoped (it also removes the named VM). `doctor --unfix` has no VM name argument — it only tears down host state — so its guard is simpler and stricter: refuse if *any* VM is currently defined, full stop.

**5. `vmctl-cli`'s subcommand enumeration gains `doctor`; nothing else in that spec changes here.**
The follow-up change owns the full verb rename; this change only adds the one new subcommand it introduces.

## Risks / Trade-offs

- **[Onboarding friction]** A first-time user on an unprovisioned host now hits a hard failure on `vmctl setup` instead of one command doing everything. → Mitigation: the failure message names the exact missing requirement and points to `vmctl doctor --fix`, which still does it all in one shot — it's just no longer silent/implicit. README documents the new two-step flow.
- **[`dpkg -s` ties two checks to Debian/Ubuntu specifically]** → Mitigation: `checkApt` already hard-assumes apt/dpkg; this doesn't add a new constraint, just makes two existing package checks concrete instead of implicit.
- **[Group-membership dual-check adds a subtle distinction users could find confusing]** → Mitigation: keep the two messages sharply different in wording ("not granted" vs. "granted, needs a fresh login") so the next action is unambiguous either way.
- **[Silent behavior drift between `Check()` and `Fix()`]** → Mitigation: `Fix()` is a straight move of existing, already-tested logic; no new install/config behavior is introduced in this change.
- **[`--purge-all` losing host-teardown could surprise existing users relying on today's one-shot full purge]** → Mitigation: this is the explicit, intended trade-off of this change (see proposal's Why); `doctor --unfix` is the direct replacement and is called out in the `cleanup` help text and cleanup's completion output.

## Migration Plan

1. Add `vmctl/internal/hostready` with `Check`/`Fix`/`Unfix`, moving (not rewriting) the bodies of `installPrerequisites`, `ensureNATNetworkReady`, `grantQEMUStorageACL` into `Fix`, and the host-teardown portion of `cleanup.go` into `Unfix`.
2. Add the package-presence (`LookPath`/`dpkg -s`) and group/service/network/ACL checks to `Check`.
3. Update `setup.go` to call `hostready.Check()` and fail fast on the first non-OK result.
4. Update `cleanup.go` to drop the host-teardown steps from `--purge-all` and the interactive walkthrough, keeping VM-scoped and opt-in-feature (firewall/monitoring) teardown.
5. Add `cmd/vmctl/doctor.go` (report / `--fix` / `--unfix`), wire it into `main.go`'s subcommand switch and usage text.
6. Update `README.md` and `TESTING.md` to reflect the new flow.
7. No rollback beyond normal git revert is needed — this is a pure code/spec change with no data migration.

## Open Questions

None outstanding — all prior ambiguities were resolved in conversation before this proposal was written (scope B confirmed, `--harden-host-firewall`/`--monitor` explicitly out of scope, `doctor --fix`/`--unfix` naming confirmed, sequencing before the CLI-rename change confirmed).
