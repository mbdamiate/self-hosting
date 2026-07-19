## 1. Argument parsing

- [x] 1.1 Add `--admin-password` and `--admin-password=VALUE` parsing to `debian-vm-setup.sh`, alongside the existing flag parsing block
- [x] 1.2 Add the flag to `print_help`, documenting the default (NOPASSWD) vs. opt-in (password-required) behavior and that it only applies at VM creation

## 2. Password generation and hashing

- [x] 2.1 When `--admin-password` is passed without a value, generate a random password with `openssl rand -base64 18`
- [x] 2.2 Hash the plaintext (generated or explicit) with `openssl passwd -6 -salt "$(openssl rand -hex 8)" "$PASSWORD"`
- [x] 2.3 Add an explicit check for `openssl` availability with an actionable error, consistent with the script's existing prerequisite-check style

## 3. cloud-init user-data changes

- [x] 3.1 Make the admin user's `sudo:` line conditional: `ALL=(ALL) NOPASSWD:ALL` (default) vs. `ALL=(ALL) ALL` (`--admin-password`)
- [x] 3.2 When `--admin-password` is set, add `passwd: <hash>` and `lock_passwd: false` for the admin user in the generated `user-data`
- [x] 3.3 Confirm `ssh_pwauth: false` is left untouched in both cases

## 4. Password and policy persistence

- [x] 4.1 On fresh VM creation with `--admin-password`, write the plaintext to `${WORK_DIR}/admin-password` with `chmod 600`
- [x] 4.2 On fresh VM creation, write the applied policy (`nopasswd` or `password-required`) to `${WORK_DIR}/.admin-sudo-policy`

## 5. Rerun mismatch handling

- [x] 5.1 Before reusing an already-existing VM, read `${WORK_DIR}/.admin-sudo-policy` if present
- [x] 5.2 If the marker disagrees with the current invocation's `--admin-password` flag, print a warning (matching the existing `--bridge`/`--ip` mismatch phrasing) stating the VM's actual policy and pointing to `virsh undefine --remove-all-storage`, then continue
- [x] 5.3 If `--admin-password` is requested but no marker file exists for an already-existing VM, print a warning that the policy can't be determined and recreation is required, then continue

## 6. Output and documentation

- [x] 6.1 Print the generated/used password once in the setup output when `--admin-password` is used, alongside the path it was also written to
- [x] 6.2 Add the `virsh console` escape-hatch note to the final connection-info summary when the VM's effective sudo policy is password-required
- [x] 6.3 Confirm no `README.md` changes are needed for `--admin-password` beyond task 1.2's `--help` text and this change's `vm-admin-sudo-policy` spec, per the existing `repository-readme` requirement to point to those instead of restating flag behavior

## 7. Verification

- [ ] 7.1 Manually verify: fresh VM without the flag still has passwordless sudo and no password file is created
- [ ] 7.2 Manually verify: fresh VM with `--admin-password` (auto-generated) requires a password for `sudo` over SSH, SSH itself remains key-only, and the password file/marker are created with correct permissions
- [ ] 7.3 Manually verify: fresh VM with `--admin-password=VALUE` uses the supplied password
- [ ] 7.4 Manually verify: rerunning setup with a mismatched `--admin-password` state against an existing VM warns and continues without altering the running VM
- [ ] 7.5 Manually verify: `virsh console` can reset the admin password as documented

(Verification tasks require an actual KVM-capable host and a real VM boot — not runnable in this sandbox. Run these manually on the target machine.)
