## 1. Write README.md

- [x] 1.1 Add a title and 1-2 sentence description: local Debian VM via libvirt/KVM/QEMU + cloud-init, mimicking a rented VPS, for self-hosting experiments before deploying to a real VPS.
- [x] 1.2 Add a "Prerequisites" section: apt-based Linux host with KVM support (reference `openspec/specs/ubuntu-qemu-prerequisites/` for the exact package list).
- [x] 1.3 Add a "Quick start" section with copy-pasteable commands: `./scripts/debian-vm-setup.sh` (default NAT) and `./scripts/debian-vm-cleanup.sh`.
- [x] 1.4 Add a fleet example combining `--name` and `--ip` (e.g. `./scripts/debian-vm-setup.sh --name=app-01 --ip=192.168.122.50`).
- [x] 1.5 Add a "More options" pointer: run either script with `--help` for the full flag reference (`--bridge`, `--forward`, `--purge-all`, sizing flags, etc.) instead of listing every flag in the README.
- [x] 1.6 Add a "Detailed behavior" pointer to `openspec/specs/` for the guarantees behind setup/cleanup/fleet behavior.
- [x] 1.7 Confirm no badges, license, or external-contribution section were added (out of scope per proposal).

## 2. Update TODO.md

- [x] 2.1 Remove or check off the "Create an explanatory README.md" pending item, reflecting that it's resolved by this change.

## 3. Verify

- [x] 3.1 Read `README.md` top to bottom as a first-time reader would and confirm each requirement in `specs/repository-readme/spec.md` is satisfied.
- [x] 3.2 Confirm the quick-start commands match the scripts' actual current flags/defaults (cross-check against `scripts/debian-vm-setup.sh` and `scripts/debian-vm-cleanup.sh` headers).
- [x] 3.3 `grep -n 'src/\|tests/\|config/\|docs/\|assets/' README.md` returns nothing, confirming no AGENTS.md-aspirational structure leaked into the README.
