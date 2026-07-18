## Context

`README.md` is currently a one-line stub. The two scripts (`debian-vm-setup.sh`, `debian-vm-cleanup.sh`) already carry thorough header comments (usage, every flag, mode-specific caveats), and each OpenSpec capability under `openspec/specs/` already documents detailed behavior/guarantees. The risk in writing a README is duplicating that content and having it drift out of sync with the scripts as flags change.

## Goals / Non-Goals

**Goals:**
- Give a first-time reader (including future-self) a working mental model of the repo and a copy-pasteable path to a running VM in under a minute of reading.
- Keep the README as the single entry point, but treat script `--help` output and `openspec/specs/` as the detailed source of truth — link to them instead of restating their content.

**Non-Goals:**
- Full flag reference (already in `--help`).
- Contribution guidelines, license, badges — this is a single-operator personal repo.
- Reconciling `AGENTS.md`'s aspirational structure (`src/`, `tests/`, etc.) with reality — separate concern.

## Decisions

- **Pointer-based over duplicated content**: the README states each script's *purpose* and the 2-3 most common invocations (default NAT setup, cleanup), then links to `--help` for the full flag list and to `openspec/specs/` for detailed guarantees. Alternative considered: copy full usage blocks into the README — rejected because it creates two places to update every time a flag changes (the scripts already show signs of this: prior commits updated defaults in both script headers and specs together).
- **No architecture/tooling sections**: since there's no build/test/lint tooling yet (confirmed empty besides `scripts/` and `openspec/`), the README omits those sections entirely rather than including placeholder text, avoiding staleness the moment real tooling is added.
- **Single flat structure**: given the small scope (2 scripts, 1 workflow), the README uses a flat set of sections (What this is / Prerequisites / Quick start / Fleet & advanced modes / Where to look for details) rather than a deep table of contents.

## Risks / Trade-offs

- [Risk] README quick-start commands drift from actual script defaults over time → [Mitigation] Quick-start only shows the no-flags invocation and one fleet example; flag-level detail is delegated to `--help`, which is generated from the same source as the behavior itself.
- [Risk] Omitting AGENTS.md reconciliation leaves a visible inconsistency between the two docs → [Mitigation] Explicitly called out as out-of-scope in the proposal; tracked as a follow-up rather than silently ignored.
