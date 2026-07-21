## 1. Prerequisite

- [x] 1.1 Confirm `vmctl-host-doctor` has been applied and archived (this change's `delete` scope and `vmctl-cli` delta both assume that end state)

## 2. Power-state package

- [x] 2.1 Create `vmctl/internal/power` with `Start`, `Stop` (graceful + force), `Reboot` (graceful + force)
- [x] 2.2 `Start`: check `virsh domstate`, no-op with a clear message if already running, otherwise `virsh start`
- [x] 2.3 `Stop`: check `virsh domstate`, no-op with a clear message if already stopped; default `virsh shutdown`, `--force` → `virsh destroy`
- [x] 2.4 `Reboot`: default `virsh reboot`, `--force` → `virsh reset`; no pre-check, let `virsh`'s own error surface if not running

## 3. Rename existing subcommands (CLI layer only)

- [x] 3.1 Rename `cmd/vmctl/setup.go` → `create.go`, `runSetup` → `runCreate`
- [x] 3.2 Rename `cmd/vmctl/cleanup.go` → `delete.go`, `runCleanup` → `runDelete`
- [x] 3.3 In `cmd/vmctl/list.go`, rename `runStatus` → `runInfo` (keep `runList` unchanged)
- [x] 3.4 Split `cmd/vmctl/backup.go` into `snapshot.go` (`vmctl snapshot create|restore|delete`) and a rewritten `backup.go` (`vmctl backup create|list|restore`), both translating to the existing `backup.Options{Subcommand: ...}` strings (`"snapshot"`, `"snapshot-restore"`, `"snapshot-delete"`, `"backup"`, `"backup-list"`, `"backup-restore"`) — `internal/backup` itself is untouched

## 4. New subcommands

- [x] 4.1 Add `cmd/vmctl/start.go`
- [x] 4.2 Add `cmd/vmctl/stop.go` with `--force`
- [x] 4.3 Add `cmd/vmctl/reboot.go` with `--force`

## 5. Wire up dispatch

- [x] 5.1 Update `main.go`'s subcommand switch to the final set: `create`, `start`, `stop`, `reboot`, `delete`, `list`, `info`, `snapshot`, `backup`, `doctor`
- [x] 5.2 Remove the old case labels (`setup`, `cleanup`, `status`) so they fall through to the unknown-subcommand path
- [x] 5.3 Update `usage()` text to list the final subcommand set

## 6. Docs

- [x] 6.1 Update every command example in `README.md` to the new verbs
- [x] 6.2 Update every command example in `TESTING.md` to the new verbs

## 7. Verification

- [x] 7.1 Run `go build ./...` and existing unit tests in `vmctl/`
- [x] 7.2 Confirm `vmctl setup`, `vmctl cleanup`, `vmctl status`, and `vmctl backup snapshot` (old spelling) each print an unknown-subcommand usage error
- [x] 7.3 Confirm `vmctl restore` (bare) prints an unknown-subcommand usage error
- [x] 7.4 On a real KVM-capable, already-provisioned host: run `vmctl create`, then `vmctl stop`, `vmctl start`, `vmctl reboot`, confirming each against `vmctl info`'s reported state
- [x] 7.5 Run `vmctl stop --force` and `vmctl reboot --force` and confirm the hard path is taken (`virsh destroy`/`virsh reset`)
- [x] 7.6 Run `vmctl snapshot create` / `restore` / `delete` and `vmctl backup create` / `list` / `restore` and confirm each matches the pre-rename behavior
- [x] 7.7 Run `vmctl delete --vm-only` and confirm scope matches what `vmctl cleanup --vm-only` did post-`vmctl-host-doctor`
