## 1. Argument parsing (setup)

- [x] 1.1 Add `--monitor` parsing to `debian-vm-setup.sh`
- [x] 1.2 Add the flag to `print_help`, documenting uptime monitoring, centralized logging (NAT-family only), and local-only alerting
- [x] 1.3 Print a note when `--monitor` and `--bridge` are combined, explaining logging is unavailable in bridged mode

## 2. Uptime monitoring: host-wide template units

- [x] 2.1 Write the `self-hosting-vm-uptime@.service` unit: runs the health-check script for instance name `%i`
- [x] 2.2 Write the health-check script: `virsh domstate %i`, fallback TCP probe to the guest's SSH port via `virsh domifaddr %i`, compare against `/run/self-hosting-vm-uptime/%i.state`, update state, and call the alert path (`logger` + `wall`) on a transition
- [x] 2.3 Write the `self-hosting-vm-uptime@.timer` unit with a fixed check interval
- [x] 2.4 Install both units host-wide (idempotently) when `--monitor` is passed, and `systemctl daemon-reload`

## 3. Uptime monitoring: per-VM enablement

- [x] 3.1 Enable and start `self-hosting-vm-uptime@<name>.timer` for the VM being created when `--monitor` is passed

## 4. Centralized logging: host-side receiver

- [x] 4.1 Install `rsyslog` on the host if not already present, when `--monitor` is passed and the VM is NAT-family (no `--bridge`)
- [x] 4.2 Configure the receiver to bind only to `virbr0`'s address, listening on a dedicated TCP port
- [x] 4.3 Configure per-hostname log file templating under `/var/log/self-hosting-vms/<vm-hostname>/`
- [x] 4.4 Add a `logrotate` config for `/var/log/self-hosting-vms/`
- [x] 4.5 Check `ufw status`; if active, add `ufw allow in on virbr0 to any port <PORT> proto tcp` idempotently

## 5. Centralized logging: guest-side forwarding

- [x] 5.1 When `--monitor` is passed and the VM is NAT-family, add `rsyslog` to cloud-init `packages` and a `write_files` entry configuring `*.* @@<nat-gateway-ip>:<port>` forwarding
- [x] 5.2 Confirm no logging-related cloud-init changes are made when `--bridge` is used

## 6. Local alerting

- [x] 6.1 Implement the alert path called from the health-check script: `logger -t self-hosting-alert "<message>"` followed by `wall`
- [x] 6.2 Write the `/etc/update-motd.d/` script summarizing recent `self-hosting-alert` journal entries, installed host-wide when `--monitor` is passed

## 7. Cleanup: per-VM (--vm-only and per-VM interactive)

- [x] 7.1 Add a step disabling/stopping `self-hosting-vm-uptime@<name>.timer` for the targeted VM, if present, non-interactively under `--vm-only`
- [x] 7.2 Confirm this step does not touch `/var/log/self-hosting-vms/<vm-hostname>/`

## 8. Cleanup: host-wide (--purge-all and interactive)

- [x] 8.1 Add a step removing the `self-hosting-vm-uptime@.{service,timer}` template units, the `rsyslog` receiver config, and its `ufw` rule (if present), gated by the same "refuse if other VMs still exist" guard already used for package purge
- [x] 8.2 Add a step removing the `update-motd.d` alert script
- [x] 8.3 Add a confirmation prompt (present even under `--purge-all`) for whether to delete `/var/log/self-hosting-vms/` entirely
- [x] 8.4 Add the corresponding interactive-mode confirm() steps mirroring `--purge-all`'s behavior

## 9. Documentation

- [x] 9.1 Confirm no `README.md` changes are needed for `--monitor` beyond task 1.2's `--help` text and this change's `vm-uptime-monitoring`/`vm-centralized-logging`/`vm-local-alerting` specs, per the existing `repository-readme` requirement to point to those instead of restating flag behavior or log locations

## 10. Verification

- [ ] 10.1 Manually verify: `--monitor` on a fresh NAT VM enables the timer instance and `systemctl list-timers` shows it
- [ ] 10.2 Manually verify: stopping the VM (`virsh destroy`) triggers a down alert (`journalctl -t self-hosting-alert`, `wall` visible, motd shows it at next login) within one check interval, and restarting it triggers a recovered alert
- [ ] 10.3 Manually verify: a hung guest (domstate running, SSH unreachable) is also treated as down
- [ ] 10.4 Manually verify: log lines emitted inside a NAT VM appear under `/var/log/self-hosting-vms/<hostname>/` on the host
- [ ] 10.5 Manually verify: `--monitor` with `--bridge` enables uptime monitoring but not log forwarding, and prints the explanatory note
- [ ] 10.6 Manually verify: with `--harden-host-firewall` also active, log forwarding still works (the scoped `ufw` rule is present and sufficient)
- [ ] 10.7 Manually verify: `--vm-only` cleanup disables the VM's timer instance but leaves its logs in place
- [ ] 10.8 Manually verify: `--purge-all` removes the host-wide template units, receiver, firewall rule, and motd script, and prompts specifically about log deletion

(Verification tasks require an actual KVM-capable host with sudo/apt access and a real VM boot — not runnable in this sandbox. Generated cloud-init YAML and the rsyslog receiver config were validated with a standalone test harness. Run these manually on the target machine.)
