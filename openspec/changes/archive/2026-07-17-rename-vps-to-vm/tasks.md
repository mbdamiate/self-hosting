## 1. Rename scripts

- [x] 1.1 `git mv scripts/debian-vps-setup.sh scripts/debian-vm-setup.sh`
- [x] 1.2 `git mv scripts/debian-vps-cleanup.sh scripts/debian-vm-cleanup.sh`

## 2. Edit debian-vm-setup.sh

- [x] 2.1 Update the header filename comment (line 3) and every `./debian-vps-setup.sh` usage example (lines 9-15) to `debian-vm-setup.sh`
- [x] 2.2 Update the `--name` fleet-flag doc default (line 18) from `debian-vps` to `debian-vm`
- [x] 2.3 Update `VM_NAME="debian-vps"` (line 58) to `VM_NAME="debian-vm"`
- [x] 2.4 Update `VM_HOSTNAME="debian-vps"` (line 63) to `VM_HOSTNAME="debian-vm"`
- [x] 2.5 Update the `--name=NAME` help text default (line 79) from `debian-vps` to `debian-vm`
- [x] 2.6 Leave line 5 ("simulating a real VPS") and line 59 ("e.g. a basic VPS plan") unchanged — external concept, not this repo's naming

## 3. Edit debian-vm-cleanup.sh

- [x] 3.1 Update the header filename comment (line 3) and all `debian-vps-cleanup.sh`/`debian-vps-setup.sh` cross-references in comments/usage (lines 4, 11, 21-25, 45, and 150, found while sweeping the file) to the `debian-vm-*.sh` names
- [x] 3.2 Update `VM_NAME="debian-vps"` (line 29) to `VM_NAME="debian-vm"`
- [x] 3.3 Update the `--name=NAME` help text default (line 38) from `debian-vps` to `debian-vm`
- [x] 3.4 Update the "Rerun debian-vps-setup.sh" hint (line 272) to `debian-vm-setup.sh`
- [x] 3.5 Leave line 124 (`echo "Cleaning up the simulated VPS environment"`) unchanged — external concept, not this repo's naming
- [x] 3.6 Leave line 279 (references `setup-debian-vps.sh`) unchanged — external concept plus a pre-existing, separately tracked word-order typo; do not fix the word order here

## 4. Update wording-only specs directly

- [x] 4.1 Apply the `ubuntu-qemu-prerequisites` MODIFIED requirement from this change's `specs/` delta directly to `openspec/specs/ubuntu-qemu-prerequisites/spec.md` ("Debian VPS setup script" → "Debian VM setup script"), and update the capability's `## Purpose` line the same way
- [x] 4.2 Apply the `libvirt-group-session-handling` MODIFIED requirements from this change's `specs/` delta directly to `openspec/specs/libvirt-group-session-handling/spec.md` ("local VPS setup" → "local VM setup", 2 occurrences), and update the capability's `## Purpose` line the same way

## 5. Archive to apply the capability renames

- [x] 5.1 Run `openspec archive rename-vps-to-vm --yes`, applying the ADDED deltas (creating `vm-cleanup-scope`, `vm-fleet-provisioning`, `vm-port-forward-reapplication`, `vm-setup-rerun-recovery`) and the 2 MODIFIED wording deltas. Note: the change's `specs/` no longer carries REMOVED deltas for the 4 old `vps-*` capabilities — the tool rejects a delta that empties a capability to zero requirements ("Spec must have at least one requirement"), so removal of the old directories is a separate manual step (5.1b), not a delta operation.
- [x] 5.1b Delete the 4 old capability directories directly: `git rm -r openspec/specs/vps-cleanup-scope openspec/specs/vps-fleet-provisioning openspec/specs/vps-port-forward-reapplication openspec/specs/vps-setup-rerun-recovery`
- [x] 5.2 Replace the auto-generated `TBD` `## Purpose` on `openspec/specs/vm-cleanup-scope/spec.md` with the original `vps-cleanup-scope` Purpose text, reworded (`debian-vps-cleanup.sh` → `debian-vm-cleanup.sh`)
- [x] 5.3 Replace the auto-generated `TBD` `## Purpose` on `openspec/specs/vm-fleet-provisioning/spec.md` with the original `vps-fleet-provisioning` Purpose text, reworded
- [x] 5.4 Replace the auto-generated `TBD` `## Purpose` on `openspec/specs/vm-port-forward-reapplication/spec.md` with the original `vps-port-forward-reapplication` Purpose text, reworded
- [x] 5.5 Replace the auto-generated `TBD` `## Purpose` on `openspec/specs/vm-setup-rerun-recovery/spec.md` with the original `vps-setup-rerun-recovery` Purpose text, reworded (`VPS setup script` → `VM setup script`)

## 6. Update TODO.md

- [x] 6.1 Mark/remove the "Validate feasibility of renaming the provisioned 'vps'" item as resolved by this change
- [x] 6.2 Update the other two items referencing `debian-vps-cleanup.sh`/`debian-vps-setup.sh` to the new `debian-vm-*.sh` filenames

## 7. Verify

- [x] 7.1 `chmod +x scripts/debian-vm-setup.sh scripts/debian-vm-cleanup.sh` (mode bit doesn't survive `git mv` content edits automatically — confirm it's still set)
- [x] 7.2 `bash -n scripts/debian-vm-setup.sh && bash -n scripts/debian-vm-cleanup.sh` to confirm both still parse
- [x] 7.3 `./scripts/debian-vm-setup.sh --help` and `./scripts/debian-vm-cleanup.sh --help` show `debian-vm` as the default name, and the correct new filename in usage examples
- [x] 7.4 Run `grep -rni vps . --exclude-dir=.git --exclude-dir=archive` and confirm the only remaining hits are the 4 intentionally-kept lines in the two scripts (plus the explanatory `TODO.md` entry referencing the old capability/change name)
- [x] 7.5 `openspec validate <name>` passes individually for `vm-cleanup-scope`, `vm-fleet-provisioning`, `vm-port-forward-reapplication`, `vm-setup-rerun-recovery`, `ubuntu-qemu-prerequisites`, `libvirt-group-session-handling` (the CLI's `validate` command takes one spec name at a time, not a list)
