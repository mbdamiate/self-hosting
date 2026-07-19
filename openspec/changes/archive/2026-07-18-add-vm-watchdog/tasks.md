## 1. Argument parsing

- [x] 1.1 Add `--watchdog` parsing to `debian-vm-setup.sh`
- [x] 1.2 Add the flag to `print_help`, documenting the `i6300esb`/reset behavior and the distinction from `on_crash`

## 2. Watchdog device attachment

- [x] 2.1 Used `--watchdog model=i6300esb,action=reset`, the documented upstream `virt-install` syntax. Not runnable in this sandbox to confirm against the exact installed `virtinst` version — re-check with `virt-install --watchdog=?` on the target host on first use.
- [x] 2.2 Add the watchdog device argument to the `virt-install` invocation, conditional on `--watchdog`

## 3. Guest-side systemd watchdog configuration

- [x] 3.1 Add a `write_files` entry to cloud-init `user-data` for `/etc/systemd/system.conf.d/90-watchdog.conf` setting `RuntimeWatchdogSec`, conditional on `--watchdog`
- [x] 3.2 Confirm no watchdog-related cloud-init changes are made when the flag is omitted

## 4. Rerun-mismatch detection

- [x] 4.1 Add watchdog-configuration introspection (`virsh dumpxml <name> | grep -A1 '<watchdog'` or equivalent) to the existing effective-configuration-detection logic in `debian-vm-setup.sh`
- [x] 4.2 Add the mismatch warning (watchdog requested but VM has none / not requested but VM has one), reusing the existing warning phrasing pattern already used for network-mode mismatches
- [x] 4.3 Confirm setup continues with the VM's actual watchdog configuration in both mismatch cases, rather than failing

## 5. Documentation

- [x] 5.1 Confirm no `README.md` changes are needed for `--watchdog` beyond task 1.2's `--help` text and this change's `vm-guest-watchdog` spec, per the existing `repository-readme` requirement to point to those instead of restating flag behavior

## 6. Verification

- [ ] 6.1 Manually verify: a VM created with `--watchdog` shows a watchdog device in `virsh dumpxml`
- [ ] 6.2 Manually verify: inside the VM, `RuntimeWatchdogSec` is active (e.g., `systemctl show -p RuntimeWatchdogUSec`) and `/dev/watchdog` exists
- [ ] 6.3 Manually verify: forcing a guest hang (e.g., `echo c > /proc/sysrq-trigger` to simulate a kernel-level stop, or freezing PID 1) results in the VM resetting within the expected timeout window
- [ ] 6.4 Manually verify: rerunning setup with `--watchdog` against an existing VM created without it (and vice versa) warns and continues without altering the running VM
- [ ] 6.5 Manually verify: a VM created without `--watchdog` has no watchdog device and no systemd watchdog configuration

(Verification tasks require an actual KVM-capable host and a real VM boot — not runnable in this sandbox. Run these manually on the target machine, including confirming the exact `--watchdog` syntax per task 2.1.)
