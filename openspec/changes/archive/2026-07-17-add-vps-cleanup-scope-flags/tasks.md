## 1. Argument parsing

- [x] 1.1 Remove the `--yes`/`-y` case from the argument-parsing loop in `scripts/debian-vps-cleanup.sh`
- [x] 1.2 Add `--vm-only` and `--purge-all` cases, setting distinct mode variables (replacing `ASSUME_YES`)
- [x] 1.3 After parsing, exit with a usage error if both `--vm-only` and `--purge-all` were passed
- [x] 1.4 Update the `-h`/`--help` output to document `--vm-only` and `--purge-all` and remove references to `--yes`

## 2. `--vm-only` behavior

- [x] 2.1 Gate steps 2–5 (working directory removal, package purge, group removal, network removal) so none of them run when `--vm-only` is set
- [x] 2.2 Ensure step 1 (VM stop + `virsh undefine --remove-all-storage --nvram`) runs without a confirmation prompt when `--vm-only` is set
- [x] 2.3 Update the "no VM found" message path to work correctly under `--vm-only` (report and exit cleanly, no other steps attempted)

## 3. `--purge-all` behavior

- [x] 3.1 Ensure steps 1–5 all run without confirmation prompts when `--purge-all` is set
- [x] 3.2 Add a new step 6: revoke the `libvirt-qemu` ACL entry on `$HOME` via `sudo setfacl -x u:libvirt-qemu "$HOME"`, run without a confirmation prompt when `--purge-all` is set
- [x] 3.3 Make the ACL revocation in 3.2 non-fatal: print a warning and continue if `setfacl -x` fails

## 4. No-flags interactive behavior

- [x] 4.1 Add a sixth `confirm()`-gated step asking whether to revoke the QEMU storage ACL, worded consistently with the existing five steps (mention that other local VMs may still rely on it)
- [x] 4.2 Verify steps 1–5 remain behaviorally unchanged when no scope flag is passed

## 5. Documentation and output text

- [x] 5.1 Update the script's header comment block to describe the three invocation modes (no flags, `--vm-only`, `--purge-all`) in place of the old `--yes` description
- [x] 5.2 Update the final notes/summary block printed at the end of the script to reflect the new ACL-revocation step where relevant

## 6. Verification

- [x] 6.1 Manually exercise `--vm-only` against a running VM created by `debian-vps-setup.sh` and confirm the base cloud image and packages remain afterward, and that `debian-vps-setup.sh` re-run recreates the VM without re-downloading
- [x] 6.2 Manually exercise `--purge-all` and confirm the VM, working directory, packages, groups, network, and ACL entry are all gone (`getfacl "$HOME"` no longer lists `libvirt-qemu`)
- [x] 6.3 Manually exercise the no-flags interactive path, confirming every prompt including the new ACL step, and confirm the end state matches 6.2
- [x] 6.4 Confirm passing both `--vm-only` and `--purge-all` together exits with a usage error before any removal
- [x] 6.5 Confirm passing `--yes` now exits with an unknown-argument error
