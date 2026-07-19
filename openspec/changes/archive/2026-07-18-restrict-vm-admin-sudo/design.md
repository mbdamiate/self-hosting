## Context

`debian-vm-setup.sh` generates cloud-init `user-data` with a single admin user that has `sudo: ALL=(ALL) NOPASSWD:ALL` and `ssh_pwauth: false` (SSH is key-only). This is the only account in the VM and the only path to root. The script has no existing mechanism for handling secrets beyond the SSH keypair it already generates/reuses at `~/.ssh/id_ed25519`, and no post-boot SSH automation runs against the guest today — all provisioning happens through cloud-init directives (`packages`, `runcmd`) executed as root during first boot, before the operator ever logs in.

## Goals / Non-Goals

**Goals:**
- Make password-required sudo available as an explicit, opt-in choice (`--admin-password[=PASSWORD]`) for VMs meant to run as real, reachable servers.
- Keep the change additive: default behavior (no flag) stays exactly as it is today.
- Give the operator a reliable, one-time way to retrieve the sudo password without adding an unbounded-lifetime plaintext secret to the host beyond what already exists for the SSH key.
- Warn (not fail) when a rerun's requested policy can't be honored because the target VM already exists.

**Non-Goals:**
- Flipping the default away from `NOPASSWD:ALL`. That's a bigger behavioral change than this proposal covers, and this repo's local-testing use case still benefits from zero-friction sudo.
- Password rotation, expiry, or any post-boot mechanism to change the password after first boot — cloud-init only applies `user-data` once, so this is a creation-time-only setting, same as the network mode.
- Any change to how future in-guest hardening (firewall, fail2ban, unattended-upgrades — the sibling TODO items) is delivered. As noted in Context, those are expected to run as root via cloud-init `packages`/`runcmd`, not through the interactive admin user's `sudo`, so this change does not constrain them.

## Decisions

**1. Hash the password with `openssl passwd -6`, don't rely on cloud-init's `plain_text_passwd`.**
cloud-init's `passwd` field expects a crypted hash; its `plain_text_passwd` alternative is explicitly documented upstream as insecure (stored and logged in the clear) and deprecated. `openssl` is already a transitive dependency of virtually every Debian/Ubuntu install (pulled in by `wget`/apt itself via libssl) and needs no new package. Generate with `openssl passwd -6 -salt "$(openssl rand -hex 8)" "$PASSWORD"` (SHA-512 crypt), matching what `mkpasswd --method=SHA-512` would produce, without adding a `whois` package dependency just for `mkpasswd`.

**2. Set `lock_passwd: false` alongside the hash, keep `ssh_pwauth: false` untouched.**
cloud-init locks the account password (`!` in `/etc/shadow`) by default, which would make the hash useless for local `sudo` prompts too, not just SSH. `lock_passwd: false` unlocks it for local authentication only; `ssh_pwauth` is the independent sshd-level switch that keeps remote password login off. Both settings target different layers (PAM/shadow vs. sshd), so this is the only combination that achieves "password gates sudo, not SSH."

**3. Generate a random password by default; accept an explicit one via `--admin-password=VALUE`.**
Mirrors the existing `--ip`/auto-pick pattern (explicit value if given, sensible default generation otherwise). Random generation avoids operators picking weak passwords under time pressure. Use `openssl rand -base64 18` for the plaintext (before hashing) — sufficient entropy, no new dependency.

**4. Persist the plaintext once to a guarded file in `WORK_DIR`, in addition to printing it.**
Terminal-only output risks being lost to scrollback or non-interactive log capture. The SSH private key is already a standing secret persisted under the same `WORK_DIR` tree philosophy (`~/.ssh`, adjacent to `~/vms/<name>/`); writing `~/vms/<name>/admin-password` with `chmod 600` at creation time follows the same precedent rather than introducing a new one. It is deleted along with the rest of `WORK_DIR` by `debian-vm-cleanup.sh --purge-all`, and left in place by `--vm-only` (consistent with that script preserving reusable state) — cleanup does not need new logic to single out this file, only note it in its existing "what's preserved" messaging.

**5. Track the applied policy in a marker file, not by introspecting the VM.**
Unlike network mode (readable from `virsh domiflist` after the fact), sudo policy leaves no trace libvirt exposes. Write `${WORK_DIR}/.admin-sudo-policy` (`nopasswd` or `password-required`) at creation time. On rerun against an existing VM: if the marker is present and disagrees with the current invocation's flag, warn using the same phrasing pattern as the existing bridge/IP mismatch warnings (state the fact, point at `virsh undefine --remove-all-storage` as the only way to change it, continue with the VM's actual policy). If the marker is absent (VM predates this change), warn that the policy can't be determined and that recreation is required to apply `--admin-password`.

## Risks / Trade-offs

- **[Risk] Operator loses the printed/stored password and locks themselves out of sudo.** → Mitigation: `virsh console <name>` gives the host's root user a guest console independent of SSH/sudo, which can reset the password (`passwd admin`) as an escape hatch; mention this explicitly in the setup script's final output when `--admin-password` is used.
- **[Risk] A future in-guest automation feature (e.g., the planned firewall/fail2ban proposal) turns out to need non-interactive `sudo` over SSH after all, which `--admin-password` would break.** → Mitigation: called out as a Non-Goal above; if a future proposal needs that, it should either run its provisioning via cloud-init (root context, unaffected) or explicitly document/handle the password-required case rather than assuming `NOPASSWD`.
- **[Risk] Plaintext password file on disk (`WORK_DIR/admin-password`) is itself a secret at rest.** → Mitigation: `chmod 600`, same threat model already accepted for the SSH private key sitting in `~/.ssh`; both require host account compromise to read.
- **[Trade-off] This is opt-in, so it does nothing for operators who don't know to pass the flag.** Accepted for this change; documented in the README as the recommended setting for any VM meant to be reachable beyond the host itself. Whether to flip the default is left as an open question below.

## Migration Plan

N/A — purely additive, opt-in flag. No change to already-created VMs' behavior. To apply hardened sudo to a VM created before this change (or without the flag), recreate it: `virsh undefine <name> --remove-all-storage` followed by `./scripts/debian-vm-setup.sh --name=<name> --admin-password`.

## Open Questions

- Should `--admin-password` (or an equivalent hardened default) eventually become the default once the sibling security/observability/resilience TODO items land, given this is meant to support real production use? Left open for a later change once more of those items are in place.
- Is a generated 18-byte base64 password (~24 chars) an acceptable default, or should length/policy be configurable? Not exposed as a flag for now to keep the surface small.
