## Context

`vmctl/internal/hostready/check.go` implements `Check()`, the single shared read-only check function consumed by `vmctl doctor` (report) and `vmctl setup`'s preflight (fail-fast on the first non-OK result). Today `Check()` appends a `boolResult("apt-based host", checkApt())` entry, and three of its package-presence checks (`dpkgChecks = []string{"qemu-system-x86", "bridge-utils", "genisoimage"}`) shell out to `dpkg -s <pkg>` because, per the prior change's design doc, "`vmctl` never invokes a binary from [these] directly" and "`checkApt` already hard-assumes an apt/dpkg-based host; this doesn't introduce a new distro assumption." That premise is exactly what this change removes: once `checkApt-as-a-report-line` is gone, the `dpkg -s` checks become the *only* apt-specific thing left in `Check()`, so they need to go too.

`hostready.Fix()` calls `checkApt()` directly as a guard (`fix.go:28`) before doing anything, and still installs via `apt`. `checkApt()` itself is not removed, only its error message and its use as a `Check()` result line change.

**Correction discovered during implementation:** `hostready.Unfix()` (`unfix.go`) had *no* `checkApt()` guard at all — `removePackages()` calls `apt purge`/`apt autoremove` and silently ignores the error, then proceeds to `removeGroups()` and `revokeACL()` regardless, which are not apt-specific and would actually mutate group membership and the ACL on a non-apt host that never ran `Fix()`. This contradicted the `vmctl-host-doctor` spec delta (which requires both `--fix` and `--unfix` to refuse cleanly before mutating). Fixed by adding the same `checkApt()` guard at the top of `Unfix()`, mirroring `Fix()`.

## Goals / Non-Goals

**Goals:**
- `vmctl doctor` (no flag) and `vmctl setup`'s preflight produce a complete, accurate report on any Linux distro, not just apt-based ones.
- `doctor --fix`/`--unfix`'s refusal on a non-apt host points the user at that now-trustworthy report instead of a dead end.
- Zero behavior change to what `Fix()`/`Unfix()` actually install/remove, and on which hosts they're willing to run.

**Non-Goals:**
- No `PackageManager` abstraction, no `dnf`/`pacman`/`zypper` install or purge support, no per-distro package-name mapping table. `--fix`/`--unfix` remain apt-only; only the diagnostic report becomes portable.
- Not re-litigating which packages `Fix()` installs (`ubuntu-qemu-prerequisites` spec, `qemu-system-x86` vs `qemu-kvm`) — unchanged.
- Not adding real multi-distro test coverage (no Fedora/Arch host in the `TESTING.md` real-host-validation roteiro) — this change has no install-path behavior to validate on those distros, only the report, which is exercised the same way `Check()` already is today (via `vmctl doctor` and `setup`'s preflight on the existing apt-based test host). Confirming the three swapped checks pass on a real non-apt host is out of scope for this change; the binary names are well-established across distros and don't need a live rig to justify.

## Decisions

**1. Drop the `apt-based host` line from `Check()`'s results entirely; keep `checkApt()` as a guard used only by `Fix()`/`Unfix()`.**
`Check()` never mutates state and, once the three `dpkg`-based checks are replaced (decision 2), has no remaining apt-specific logic — so there's nothing left for an `apt-based host` line to gate. Removing it (rather than keeping it as a non-blocking informational line) avoids reporting a `[MISSING] apt-based host` result on every non-apt host that is otherwise fully provisioned, which would be noise pointing at a non-actionable, non-requirement fact. `checkApt()` the function stays, called directly by `Fix()`/`Unfix()` exactly as today.

**2. Replace the three `dpkg -s` checks with `exec.LookPath` on their actual binaries.**
| Package (dpkg name) | Binary checked | Rationale |
|---|---|---|
| `qemu-system-x86` | `qemu-system-x86_64` | The concrete emulator binary the package installs; same name on Fedora/RHEL's `qemu-kvm`, Arch's `qemu-full`, etc. |
| `bridge-utils` | `brctl` | Same binary name across distros that still ship it. |
| `genisoimage` | `genisoimage` | Binary name matches package name and is consistent across distros. |

This reuses the existing `binaryCheck`/`binaryChecks` structure (`check.go:46-59`) rather than inventing a parallel mechanism — these three entries simply move from `dpkgChecks` into `binaryChecks`. `dpkgChecks` and the `dpkg -s` code path are deleted, not left dormant.

**3. `checkApt()`'s error message names `vmctl doctor` as the source of truth, instead of telling the user to "adapt it."**
New text (exact wording finalized during implementation, meaning preserved): *"`vmctl doctor --fix` only installs/configures prerequisites via apt (Ubuntu/Debian); 'apt' was not found on this host. Run 'vmctl doctor' (without a flag) to see exactly what's missing, then install the equivalents for your distro's package manager and rerun the individual steps manually — 'doctor --fix' itself only supports apt-based hosts."* This keeps `Fix()`/`Unfix()`'s refusal unconditional and immediate (unchanged control flow — `checkApt()` still returns an error before any mutation) while giving the user somewhere real to go.

**4. `vmctl-host-doctor` spec gets the delta; `ubuntu-qemu-prerequisites` does not.**
`ubuntu-qemu-prerequisites`'s requirements govern `Fix`/`Unfix`'s APT install/purge target packages and `setup`'s `qemu-img`-binary preflight check — none of that changes. The only requirement-level behavior change is to `vmctl doctor`'s report (owned by `vmctl-host-doctor`) no longer assuming an apt-based host, and to the apt-guard failure message.

## Risks / Trade-offs

- **[A host could have `qemu-system-x86_64`/`brctl`/`genisoimage` on `PATH` without the exact Debian package installed]** → Not a real risk: the check's purpose is verifying the *capability* (the binary vmctl/libvirt/cloud-localds transitively needs) is present, not verifying a specific distro's packaging metadata. This is strictly more accurate than today's `dpkg -s`, which only ever meant anything on Debian/Ubuntu anyway.
- **[No live non-apt host to verify the three swapped checks actually resolve correctly]** → Mitigation: these are well-known, stable binary names (see table in Decision 2); acceptable to ship without dedicated cross-distro test infra for a diagnostic-only change with no install/mutation path.
- **[Removing the `apt-based host` line changes `vmctl doctor`'s exact report output]** → Intended and covered by the spec delta; anyone scripting against the exact line count/text of `vmctl doctor`'s output (undocumented, not a supported interface) would need to adjust.

## Migration Plan

1. In `vmctl/internal/hostready/check.go`: move `qemu-system-x86` → `qemu-system-x86_64`, `bridge-utils` → `brctl`, `genisoimage` → `genisoimage` from `dpkgChecks` into `binaryChecks`; delete `dpkgChecks` and its loop in `Check()`.
2. Remove the `boolResult("apt-based host", checkApt())` line from `Check()`.
3. Update `checkApt()`'s error string per Decision 3. `Fix()`/`Unfix()` need no code changes — they already call `checkApt()` directly.
4. Update `openspec/specs/vmctl-host-doctor/spec.md` per the spec delta in this change.
5. Update `README.md`'s Prerequisites section to note `vmctl doctor` (report) works on any Linux host, while `--fix`/`--unfix` remain apt-based-host-only.
6. No rollback beyond normal git revert — pure code/spec/doc change, no data migration.

## Open Questions

None outstanding.
