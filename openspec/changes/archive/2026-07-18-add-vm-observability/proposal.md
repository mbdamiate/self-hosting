## Why

Once a VM is running as something closer to a real server, "is it up?" and "what happened right before it went down?" stop being questions the operator can answer just by remembering to `ssh` in and look. Today, nothing in this repo tracks whether a VM is actually alive over time, nothing collects its logs anywhere but the guest itself, and nothing tells the operator when something goes wrong unless they happen to be watching. This proposal adds a minimal, host-side, no-internet-required version of the three: uptime monitoring, centralized logging, and local alerting.

## What Changes

- Add a `--monitor` flag to `debian-vm-setup.sh`. When passed, it host-wide installs (once, idempotently) a templated systemd timer/service pair that periodically checks a VM's health, and enables an instance of it for the VM being created.
- The health check combines `virsh domstate` (is the QEMU process actually running) with a TCP reachability check on the guest's SSH port (is the guest OS actually responding, not just the hypervisor process) — either failing counts as "down."
- On an up→down or down→up state transition, write a local alert: `logger -t self-hosting-alert`, a `wall` broadcast to logged-in host users, and a summary of recent alerts shown via an `/etc/update-motd.d/` script at the next host login. No remote notification (email/webhook/etc.) — deferred until internet connectivity is addressed separately.
- Also install (once, host-wide, gated by the same flag) an `rsyslog` receiver on the host bound to the libvirt NAT bridge (`virbr0`) only, storing each VM's forwarded logs under `/var/log/self-hosting-vms/<vm-name>/`, with `logrotate` to bound growth. Configure the guest (via cloud-init) to forward its logs to the host over the NAT network.
- Centralized logging only covers NAT-family VMs (plain NAT or `--forward`) — `--bridge` mode's macvtap isolation means the host and guest can't reach each other directly, so there's no path for the guest to forward logs to the host in that mode. This is a known, documented limitation, not a bug to fix here.
- When the host firewall is hardened (`harden-host-firewall`'s `--harden-host-firewall`, or any pre-existing `ufw`), add a narrowly-scoped allow rule (`virbr0` interface only) for the log receiver's port, so the guest can still reach it.
- Extend `debian-vm-cleanup.sh`: per-VM removal (`--vm-only`, and per-VM interactive) disables that VM's uptime-check timer instance but preserves its historical logs; full teardown (`--purge-all`, interactive) also removes the host-wide template units, the log receiver, its firewall rule (if added), the motd script, and offers to delete the accumulated logs.
- `--help` documents the flag; what "local alerting" does and doesn't cover and the `--bridge` limitation are captured in the new capability specs. Per the existing `repository-readme` spec (README SHALL NOT restate flag reference or behavioral guarantees already in `openspec/specs/`), `README.md` itself needs no new prose.

## Capabilities

### New Capabilities
- `vm-uptime-monitoring`: the host-side periodic health check per VM (domstate + SSH reachability), state-transition tracking, and its systemd timer/service template.
- `vm-centralized-logging`: guest-to-host log forwarding over the NAT network, per-VM log storage on the host, log rotation, and the `--bridge`-mode limitation.
- `vm-local-alerting`: how a detected state transition becomes a locally-visible alert (journal entry, `wall`, motd summary), with no remote/internet-dependent channel in scope.

### Modified Capabilities
- `vm-cleanup-scope`: `--vm-only`/per-VM interactive removal gains a step to disable (not delete) that VM's monitoring timer instance and preserve its logs; `--purge-all`/interactive full teardown gains a step to remove the host-wide monitoring/logging artifacts (template units, log receiver, its firewall rule, motd script) and offer to delete accumulated logs. **Sequencing:** this delta's text already assumes `harden-host-firewall`'s own `vm-cleanup-scope` delta has archived first — see design.md.

## Impact

- `scripts/debian-vm-setup.sh`: argument parsing (`--monitor`), host-wide installation of the systemd timer/service template and the rsyslog receiver + optional `ufw` rule, per-VM timer instance enablement, cloud-init `user-data` changes (guest-side rsyslog forwarding config), `--help` text.
- `scripts/debian-vm-cleanup.sh`: per-VM timer-instance disable step; host-wide removal step for the shared monitoring/logging artifacts, gated the same way package purge already is (refuses if other VMs still exist).
- No impact on `README.md` — covered by the existing `--help`/`openspec/specs/` pointers per `repository-readme`.
- New host-side files: systemd unit templates, the rsyslog receiver config, a `logrotate` config, and the `update-motd.d` script — all installed under standard system paths, not inside the repo.
