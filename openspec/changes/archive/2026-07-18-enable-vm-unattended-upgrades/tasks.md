## 1. Argument parsing

- [x] 1.1 Add `--no-auto-updates` parsing to `debian-vm-setup.sh`, alongside the existing flag parsing block
- [x] 1.2 Add the flag to `print_help`, documenting that automatic security updates are on by default and what the flag disables

## 2. cloud-init user-data changes

- [x] 2.1 Add `unattended-upgrades` to the `packages:` list, conditional on `--no-auto-updates` not being set
- [x] 2.2 Add a `write_files:` entry for an auto-upgrades enable snippet (`APT::Periodic::Update-Package-Lists "1";`, `APT::Periodic::Unattended-Upgrade "1";`), conditional on the flag
- [x] 2.3 Add a `write_files:` entry overriding `Unattended-Upgrade::Allowed-Origins` to the security-only pattern, conditional on the flag
- [x] 2.4 Add a `write_files:` entry setting `Unattended-Upgrade::Automatic-Reboot "false";`, conditional on the flag

## 3. Setup output

- [x] 3.1 On fresh VM creation with automatic updates enabled, print the one-time note about automatic security updates and manual reboots
- [x] 3.2 Confirm the note is not printed when `--no-auto-updates` is passed, and not repeated when reusing an already-existing VM

## 4. Documentation

- [x] 4.1 Confirm no `README.md` changes are needed for `--no-auto-updates` beyond task 1.2's `--help` text and this change's `vm-unattended-upgrades` spec, per the existing `repository-readme` requirement to point to those instead of restating flag behavior

## 5. Verification

- [ ] 5.1 Manually verify: fresh VM without the flag has `unattended-upgrades` installed and `systemctl status apt-daily-upgrade.timer` / `apt-daily.timer` enabled
- [ ] 5.2 Manually verify: the security-only origin override is present and syntactically valid (`unattended-upgrades --dry-run --debug` inside the VM shows only the security origin considered)
- [ ] 5.3 Manually verify: `Unattended-Upgrade::Automatic-Reboot` is `false` inside the VM's config
- [ ] 5.4 Manually verify: fresh VM with `--no-auto-updates` does not install or configure `unattended-upgrades`
- [ ] 5.5 Manually verify: the one-time setup note appears/doesn't appear per Requirement scenarios

(Verification tasks require an actual KVM-capable host and a real VM boot — not runnable in this sandbox. Generated cloud-init YAML was validated for syntax with a standalone test harness. Run these manually on the target machine.)
