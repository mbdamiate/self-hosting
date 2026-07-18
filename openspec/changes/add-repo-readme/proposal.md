## Why

`README.md` is a single line (`# self-hosting`) and gives a new reader (including future-self) no way to tell what this repo does or how to run it without opening both scripts and reading their headers. This is already tracked as an open `TODO.md` item ("Create an explanatory README.md").

## What Changes

- Rewrite `README.md` with: a short project description (local libvirt/KVM/QEMU VM provisioning that mimics a rented VPS, for self-hosting experiments before deploying to a real VPS), prerequisites (apt-based Linux host with KVM support), and quick-start commands for `debian-vm-setup.sh` and `debian-vm-cleanup.sh` covering the main modes (default NAT, `--bridge`, `--forward`, and fleet usage via `--name`/`--ip`).
- Point to `openspec/specs/` as the source of truth for detailed behavior and to each script's `--help` as the source of truth for flags, instead of duplicating that content in the README.
- Keep the tone personal/single-operator: no badges, no external-contribution section, no license section.
- Mark the "Create an explanatory README.md" item in `TODO.md` as resolved by this change.
- Out of scope: `AGENTS.md` currently describes an aspirational project structure (`src/`, `tests/`, `config/`, `docs/`, `assets/`) that doesn't match the repo. Reconciling that is a separate, later concern — not touched here.

No **BREAKING** changes: this is documentation only, no script behavior changes.

## Capabilities

### New Capabilities
- `repository-readme`: defines what `README.md` must cover (project description, prerequisites, quick start for setup/cleanup, and pointers to `openspec/specs/` and script `--help` as the detailed sources of truth) so the repo is self-explanatory without duplicating existing script/spec documentation.

### Modified Capabilities
(none — no existing capability's requirements change)

## Impact

- Docs: `README.md` (rewritten), `TODO.md` (one item marked resolved).
- Code: none.
- Excluded: `AGENTS.md` (separate concern, not addressed here).
