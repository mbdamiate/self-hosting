## 1. Argument parsing

- [x] 1.1 Add `--no-crash-restart` parsing to `debian-vm-setup.sh`
- [x] 1.2 Add the flag to `print_help`, documenting the default-on `on_crash=restart` behavior and the opt-out

## 2. virt-install changes

- [x] 2.1 Used `--events on_crash=restart`, the documented upstream `virt-install` syntax. Not runnable in this sandbox to confirm against the exact installed `virtinst` version — re-check with `virt-install --events=?` on the target host on first use.
- [x] 2.2 Add the `on_crash=restart` argument to the `virt-install` invocation by default, omitted when `--no-crash-restart` is passed

## 3. Rerun-mismatch detection

- [x] 3.1 Add `on_crash` policy introspection (`virsh dumpxml <name>` inspection for `<on_crash>restart</on_crash>`) to the existing effective-configuration-detection logic in `debian-vm-setup.sh`
- [x] 3.2 Add the mismatch warning (restart expected but VM lacks it / opt-out requested but VM already has it), reusing the existing warning phrasing pattern
- [x] 3.3 Confirm setup continues with the VM's actual crash-recovery policy in both mismatch cases, rather than failing

## 4. Documentation

- [x] 4.1 Confirm no `README.md` changes are needed for `--no-crash-restart` beyond task 1.2's `--help` text and this change's `vm-crash-recovery` spec, per the existing `repository-readme` requirement to point to those instead of restating flag behavior

## 5. Verification

- [ ] 5.1 Manually verify: a freshly created VM (default flags) has `<on_crash>restart</on_crash>` in `virsh dumpxml`
- [ ] 5.2 Manually verify: killing the VM's QEMU process directly (e.g., `kill -9` on the host, simulating a crash rather than `virsh destroy`) results in libvirt restarting the domain
- [ ] 5.3 Manually verify: a VM created with `--no-crash-restart` stays stopped after its QEMU process is killed, requiring manual `virsh start`
- [ ] 5.4 Manually verify: rerunning setup with a mismatched `--no-crash-restart` state against an existing VM warns and continues without altering the running VM

(Verification tasks require an actual KVM-capable host and a real VM boot — not runnable in this sandbox. Run these manually on the target machine, including confirming the exact `--events` syntax per task 2.1.)
