## Why

`debian-vm-setup.sh` provisions the admin user with `sudo: ALL=(ALL) NOPASSWD:ALL` (debian-vm-setup.sh:483). That is fine for a disposable local VM that only the operator's own SSH key can ever reach, but it means anyone who obtains the SSH private key (or hijacks an authenticated session) gets unrestricted root with no second gate. If this same tooling is used to provision a VM that acts as a real, internet-reachable server, that privilege-escalation shortcut becomes a real risk rather than a convenience.

## What Changes

- Add a `--admin-password[=PASSWORD]` flag to `debian-vm-setup.sh`. When passed, the generated cloud-init `user-data` switches the admin user's sudo entry from `NOPASSWD:ALL` to password-required (`ALL=(ALL) ALL`), and sets a login password for that user via cloud-init (auto-generated and printed once at creation time if no explicit value is given, or the supplied value if one is).
- SSH access remains key-only in all cases — `ssh_pwauth: false` is untouched. The password only gates local `sudo` elevation after an SSH session is already established with the key; it does not open a new remote-login path.
- Default behavior (flag omitted) is unchanged: `NOPASSWD:ALL`, preserving today's frictionless local-testing UX.
- Since cloud-init only applies `user-data` on first boot, the flag has no effect when rerunning setup against an already-existing VM. Rerunning with `--admin-password` against an existing VM that was created without it (or vice versa) prints a warning and continues, mirroring the existing `--bridge`/`--ip` mismatch handling.
- `--help` documents the new flag, the generated-password output, and the security trade-off it addresses. Per the existing `repository-readme` spec (README SHALL NOT restate the flag reference, only point to `--help`/`openspec/specs/`), `README.md` itself needs no new prose for this flag.

## Capabilities

### New Capabilities
- `vm-admin-sudo-policy`: governs how the cloud-init admin user's sudo access is provisioned (password-less vs password-required), how a generated password is surfaced to the operator, and how a mismatch between the requested policy and an already-existing VM's actual policy is handled on rerun.

### Modified Capabilities
(none — this does not change the requirements of any existing spec; it introduces an orthogonal, opt-in setting)

## Impact

- `scripts/debian-vm-setup.sh`: argument parsing, cloud-init `user-data` generation, rerun-reuse path, `--help` text, final connection-info summary (surfacing the generated password once).
- No impact on `README.md` — `repository-readme` already directs readers to `--help` (this change updates) and `openspec/specs/` (this change adds `vm-admin-sudo-policy` to) for anything beyond the quick-start commands.
- No impact on `scripts/debian-vm-cleanup.sh` (sudo policy is a guest-only setting with no host-side artifacts to clean up).
