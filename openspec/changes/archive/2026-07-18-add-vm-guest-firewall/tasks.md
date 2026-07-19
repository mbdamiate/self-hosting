## 1. Argument parsing

- [x] 1.1 Add `--allow-port=PORT[,PORT...]` parsing to `debian-vm-setup.sh`
- [x] 1.2 Add `--no-guest-firewall` parsing
- [x] 1.3 Add both flags to `print_help`, documenting the default-on firewall, SSH always allowed, `--allow-port`, the `--forward` VM-port auto-allow behavior, and that `--no-guest-firewall` makes `--allow-port` a no-op

## 2. Deriving the guest firewall allow list

- [x] 2.1 Build the guest firewall allow list from: port 22 (always), `--allow-port` values, and the VM-side ports parsed out of `--forward` (`VM_PORT` half of each `HOST_PORT:VM_PORT` pair)
- [x] 2.2 De-duplicate the resulting port list before generating `ufw allow` commands

## 3. cloud-init user-data changes — ufw

- [x] 3.1 Add `ufw` to the `packages:` list, conditional on `--no-guest-firewall` not being set
- [x] 3.2 Add `runcmd:` entries setting default-deny incoming / default-allow outgoing policy, conditional on the flag
- [x] 3.3 Add `runcmd:` entries for `ufw allow <port>/tcp` for each port in the derived allow list, conditional on the flag
- [x] 3.4 Add a `runcmd:` entry for `ufw --force enable`, ordered after all `allow` rules, conditional on the flag

## 4. cloud-init user-data changes — fail2ban

- [x] 4.1 Add `fail2ban` to the `packages:` list unconditionally (not gated by `--no-guest-firewall`)
- [x] 4.2 Add a `write_files:` entry for `/etc/fail2ban/jail.local` with `[sshd]` `enabled = true`
- [x] 4.3 Add a `runcmd:` entry to enable/restart the `fail2ban` service so the jail picks up `jail.local` on first boot

## 5. Setup output

- [x] 5.1 On fresh VM creation with the guest firewall enabled, print the one-time note listing allowed ports
- [x] 5.2 Confirm the note is not printed when `--no-guest-firewall` is passed, and not repeated when reusing an already-existing VM

## 6. Guest firewall state marker and --forward reapplication warning

- [x] 6.1 On fresh VM creation, write the guest firewall's enabled/disabled state to a marker file in the VM's working directory (e.g., `.guest-firewall-policy`)
- [x] 6.2 In the existing `--forward` handling path (which already supports reapplication against an already-existing VM per `vm-port-forward-reapplication`), read the marker file when it exists
- [x] 6.3 If the marker indicates the guest firewall is enabled, print a warning naming the VM-side port(s) from the current `--forward` rules and the `ufw allow <VM_PORT>/tcp` remediation command, after the host-side DNAT/FORWARD rules are applied
- [x] 6.4 If the marker indicates disabled, or is absent (pre-existing VM from before this capability), skip the warning

## 7. Documentation

- [x] 7.1 Confirm no `README.md` changes are needed for `--allow-port`, `--no-guest-firewall`, or the `--forward`-reapplication warning beyond task 1.3's `--help` text and this change's `vm-guest-firewall`/`vm-guest-fail2ban` specs, per the existing `repository-readme` requirement to point to those instead of restating flag behavior

## 8. Verification

- [ ] 8.1 Manually verify: fresh VM without flags allows SSH, denies an arbitrary unopened port (e.g., connect-refused/timeout test from the host to a port nothing is listening on), and `ufw status` shows the default-deny policy
- [ ] 8.2 Manually verify: `--allow-port=8080` opens 8080 (start a listener inside the VM, confirm reachability) without opening unrelated ports
- [ ] 8.3 Manually verify: `--forward=8080:80` also allows port 80 in `ufw status` without an explicit `--allow-port`
- [ ] 8.4 Manually verify: `--no-guest-firewall` results in `ufw` not installed, and a passed `--allow-port` has no effect
- [ ] 8.5 Manually verify: `fail2ban-client status sshd` shows the jail active on a fresh VM, including when `--no-guest-firewall` is passed
- [ ] 8.6 Manually verify: rule ordering never transiently blocks SSH during first boot (SSH remains reachable as soon as cloud-init finishes)
- [ ] 8.7 Manually verify: reapplying `--forward` with a new port against an already-existing VM created with the guest firewall enabled prints the remediation warning, and that running the suggested `ufw allow` command inside the guest makes the forwarded service reachable
- [ ] 8.8 Manually verify: reapplying `--forward` against an already-existing VM created with `--no-guest-firewall` does not print the warning

(Verification tasks require an actual KVM-capable host and a real VM boot — not runnable in this sandbox. Generated cloud-init YAML (merged with the admin-sudo and unattended-upgrades changes) was validated for syntax and content with a standalone test harness. Run these manually on the target machine.)
