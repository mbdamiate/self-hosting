## 1. Argument parsing (setup)

- [x] 1.1 Add `--harden-host-firewall` parsing to `debian-vm-setup.sh`
- [x] 1.2 Add the flag to `print_help`, documenting the default-deny host policy, the always-allowed SSH rule, and the forward-policy preservation

## 2. Host firewall hardening step (setup)

- [x] 2.1 Add a new host-prerequisite step (alongside package install / NAT-network readiness) that runs when `--harden-host-firewall` is passed, independent of `VM_EXISTS`
- [x] 2.2 Install `ufw` if not already present
- [x] 2.3 Add the tagged SSH allow rule (`ufw allow 22/tcp comment '<identifying tag>'`), idempotently
- [x] 2.4 Check `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY`; if `DROP`, set it to `ACCEPT` and reload `ufw`; if already `ACCEPT`, leave unchanged
- [x] 2.5 Enable `ufw` (`ufw --force enable`) after the SSH rule and forward-policy check are in place

## 3. Cleanup: removal of host firewall hardening

- [x] 3.1 Add a `remove_host_firewall_hardening` step to `debian-vm-cleanup.sh`, mirroring the existing step structure (non-interactive under `--vm-only`/`--purge-all` semantics, `confirm()`-gated interactively)
- [x] 3.2 Confirm `--vm-only` does not invoke this step (host-wide state stays out of scope, matching packages/groups/network/ACL)
- [x] 3.3 Under `--purge-all` and the interactive walkthrough, look up the tagged `ufw` rule (`ufw status numbered` + grep on the comment) and delete it if found, report "nothing to remove" if not
- [x] 3.4 Under the same modes, check `/etc/default/ufw`'s `DEFAULT_FORWARD_POLICY`; if `ACCEPT`, revert to `DROP` and reload `ufw`; if already `DROP`, leave unchanged
- [x] 3.5 Confirm this step never disables or uninstalls `ufw` itself

## 4. Documentation

- [x] 4.1 Confirm no `README.md` changes are needed for `--harden-host-firewall` beyond task 1.2's `--help` text and this change's `host-firewall-hardening`/`vm-cleanup-scope` specs, per the existing `repository-readme` requirement to point to those instead of restating flag behavior or removal guarantees

## 5. Verification

- [ ] 5.1 Manually verify: `--harden-host-firewall` on a host with no prior `ufw` state results in `ufw` active, default-deny incoming, host SSH still reachable
- [ ] 5.2 Manually verify: an existing VM's `--forward` rule (e.g., `8080:80`) still reaches the VM after applying `--harden-host-firewall`, and after an explicit `ufw reload`
- [ ] 5.3 Manually verify: re-running setup with `--harden-host-firewall` a second time does not duplicate the SSH rule or error
- [ ] 5.4 Manually verify: `DEFAULT_FORWARD_POLICY` is left unchanged if already `ACCEPT` before the flag is used
- [ ] 5.5 Manually verify: cleanup's removal step deletes only the tagged rule, reverts the forward policy only if it was changed by this repo's flag, and leaves `ufw` installed/enabled
- [ ] 5.6 Manually verify: `--vm-only` cleanup leaves all host firewall state untouched

(Verification tasks require an actual host with sudo/apt access to install and reconfigure ufw — not runnable in this sandbox. Code paths were syntax-checked with `bash -n`. Run these manually on the target machine.)
