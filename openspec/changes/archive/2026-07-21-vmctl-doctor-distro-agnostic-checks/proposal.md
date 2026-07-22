## Why

`vmctl doctor`'s read-only report (`hostready.Check`) currently hard-assumes an apt/dpkg-based host even though it never installs anything: it reports an `apt-based host` check via `exec.LookPath("apt")`, and three of its package-presence checks shell out to `dpkg -s`. This makes the report — and `vmctl setup`'s preflight, which only calls `Check()` — misleadingly Debian/Ubuntu-only. A Fedora/Arch/openSUSE user gets no diagnostic value at all, and `doctor --fix`'s failure message on such a host ("Adapt it for your distro") is a dead end that points nowhere. Nobody has asked for `--fix`/`--unfix` to install on non-apt hosts, but there's no reason the diagnostic report itself should be apt-only, and the failure message should stop being a dead end.

## What Changes

- `hostready.Check()` no longer requires apt/dpkg to produce a meaningful report:
  - Drop the `apt-based host` line from the report's check results.
  - Replace the three `dpkg -s` package-presence checks (`qemu-system-x86`, `bridge-utils`, `genisoimage`) with `exec.LookPath` checks against their actual binaries (`qemu-system-x86_64`, `brctl`, `genisoimage` respectively) — the same pattern already used by the other seven presence checks, and portable across distros since these binary names don't change.
  - Net effect: `vmctl doctor` (no flag) and `vmctl setup`'s preflight (which only calls `Check()`) work as a genuine cross-distro requirements checklist on any Linux host.
- `hostready.Fix()` / `hostready.Unfix()` keep today's apt-only behavior unchanged (still install/purge via `apt`, still refuse immediately via `checkApt()` before any mutation on a non-apt host).
  - Only the refusal message changes: instead of the dead-end "this assumes a system with apt (Ubuntu/Debian). Adapt it for your distro", it now points the user at `vmctl doctor`'s report as the authoritative list of what's missing, since that report is now trustworthy on any distro.
- No new package-manager support, no per-distro package-name mapping table, no `dnf`/`pacman`/`zypper` install/purge logic — explicitly out of scope.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `vmctl-host-doctor`: `vmctl doctor`'s report requirement no longer assumes an apt-based host to produce a full, meaningful result; `--fix`/`--unfix`'s apt-guard failure message must name `vmctl doctor`'s report as the source of truth for what's missing, instead of telling the user to figure it out themselves.

## Impact

- `vmctl/internal/hostready/check.go`: remove the `apt-based host` check from `Check()`'s result list; change `dpkgChecks` handling to `binaryChecks`-style `LookPath` entries for the three affected packages; update `checkApt()`'s error message.
- `vmctl/internal/hostready/fix.go`: no behavior change, only consumes the updated `checkApt()` message.
- `vmctl/internal/hostready/unfix.go`: **discovered during implementation** — `Unfix()` had no `checkApt()` guard at all (only `Fix()` did), so on a non-apt host it would silently no-op the `apt purge` and then still remove group membership and the ACL. Added the same `checkApt()` guard `Fix()` uses, so `--unfix` now also refuses cleanly before mutating anything, per the spec delta.
- `README.md`: "Prerequisites" section can clarify that `vmctl doctor` (report) works on any Linux host, while `vmctl doctor --fix`/`--unfix` remain apt-based-host-only.
- `openspec/specs/vmctl-host-doctor/spec.md`: requirement text updated per the delta above.
- No changes to `openspec/specs/ubuntu-qemu-prerequisites/spec.md` — its requirements govern `Fix`/`Unfix`'s APT install/purge behavior and `setup`'s `qemu-img`-based preflight check, both of which are unchanged by this proposal.
