## 1. `hostready.Check()` becomes distro-agnostic

- [x] 1.1 In `vmctl/internal/hostready/check.go`, remove `qemu-system-x86`, `bridge-utils`, and `genisoimage` from `dpkgChecks`, and add them to `binaryChecks` as `{"qemu-system-x86", "qemu-system-x86_64"}`, `{"bridge-utils", "brctl"}`, `{"genisoimage", "genisoimage"}` (name shown to the user stays the package name so existing report text/remediation wording is unaffected).
- [x] 1.2 Delete the now-empty `dpkgChecks` slice and its loop in `Check()`.
- [x] 1.3 Remove the `boolResult("apt-based host", checkApt())` line from `Check()`.

## 2. `checkApt()` message points to the report

- [x] 2.1 Rewrite `checkApt()`'s error string per design.md Decision 3, so it names `vmctl doctor` (no flag) as where to see what's missing, instead of "Adapt it for your distro."
- [x] 2.2 Confirm `Fix()` (`fix.go:28`) and `Unfix()` call `checkApt()` first, so the new message surfaces before any mutation on a non-apt host. **Discovered `Unfix()` had no such guard at all — added one** (see design.md addendum and proposal.md Impact).

## 3. Tests

- [x] 3.1 Update/add unit tests in `vmctl/internal/hostready` covering: the three swapped checks report OK via `LookPath` and MISSING when the binary is absent; `Check()`'s result set no longer contains an `apt-based host` entry; `checkApt()`'s new error text. Added `internal/hostready/check_test.go` (package had no tests before).
- [x] 3.2 Run the existing `hostready`, `setup`, and `cleanup` package test suites to confirm no regression from the `Check()` result-shape change (setup's preflight consumes `Check()`'s output). Full `go build ./... && go vet ./... && go test ./...` passes.

## 4. Docs and specs

- [x] 4.1 Update `README.md`'s Prerequisites section: `vmctl doctor` (report) works on any Linux host; `vmctl doctor --fix`/`--unfix` remain apt-based-host-only.
- [x] 4.2 Verify `openspec/specs/vmctl-host-doctor/spec.md`'s delta (already drafted in this change) matches the final implemented behavior before archiving. Confirmed both the MODIFIED requirement (distro-agnostic `Check()`) and the ADDED requirement (`--fix`/`--unfix` refuse cleanly, now true for both since the `Unfix()` gap was fixed).

## 5. Manual verification

- [x] 5.1 On the existing apt-based test host, run `vmctl doctor` before and after the change and confirm identical OK/MISSING results for every prerequisite except the removed `apt-based host` line. Both checks (`Check()`/report) and the apt-guard are read-only/pre-mutation, so this ran directly in the sandbox (same physical host, see memory note) without needing `sudo`. Real run: all 14 checks OK, "Host is ready.", no `apt-based host` line.
- [x] 5.2 Temporarily rename/hide `apt` on the test host (e.g. adjust `PATH`) and confirm `vmctl doctor --fix` and `--unfix` both refuse immediately with the new message, making no changes to the system. Ran with `PATH=/nonexistent`: both `--fix` and `--unfix` exited 1 with the new message before any `sudo` call.
