## Context

`scripts/debian-vps-setup.sh` and `scripts/debian-vps-cleanup.sh` provision local KVM/QEMU/libvirt VMs from the official Debian 12 cloud image — a hardcoded, Debian-specific image URL, not a distro-agnostic tool. "vps" was used throughout to signal that the local VM mimics a rented-provider VPS, but the term stuck to the wrong noun: it now names the object (`VM_NAME="debian-vps"`, capability names) rather than the concept it's imitating. This surfaced as an open TODO item and, once the setup script grew fleet support (`--name`, `--ip`, per-VM sizing), the "single simulated VPS" framing stopped matching what the tool actually does — provision one or more named local VMs.

This is a pure rename: no behavior, flag, or requirement changes ride along. The OpenSpec `spec-driven` schema has no first-class "rename a capability" delta operator (only `RENAMED Requirements`, which renames a requirement's title within the same capability, not the capability/directory itself), so the rename of the 4 `vps-*` capabilities has to be expressed as a paired removal + addition.

## Goals / Non-Goals

**Goals:**
- Every active (non-archived) file uses "vm" consistently in place of "vps" as the name of the locally provisioned VM and its capabilities.
- Preserve the 4 lines where "VPS" legitimately refers to the external rented-server concept being mimicked — rewording those would produce nonsensical text ("simulating a real VM").
- Keep the change purely mechanical: no requirement text changes behavior, no script logic changes.

**Non-Goals:**
- Not fixing the pre-existing `cleanup.sh:279` word-order typo (`setup-debian-vps.sh` vs. the real `debian-vps-setup.sh`) — already tracked separately in `TODO.md`, orthogonal to this rename.
- Not fixing the pre-existing network/package purge step-ordering bug also noted in `TODO.md` — unrelated to naming.
- Not touching `openspec/changes/archive/**` — historical record of what was actually built at the time; rewriting it would falsify history.
- Not generalizing the scripts beyond Debian (still hardcoded to the Debian 12 cloud image) — "vm" replaces "vps", not "debian".

## Decisions

**Rename target: "vm", not another term.** Alternatives considered: `guest`, `local-vm`, `kvm-vm`. "vm" is what every other capability in this repo already calls the object (`qemu-vm-storage-access`, `libvirt-nat-network-readiness`'s body text) — matching existing convention beats introducing a third term.

**Capability rename via ADDED deltas + manual directory removal (revised during apply).** The original plan paired a `REMOVED Requirements` delta on each old `vps-*` capability with an `ADDED Requirements` delta on the new `vm-*` name. That plan doesn't survive contact with the tool: `openspec archive` validates every rebuilt spec and rejects one left with zero requirements ("Spec must have at least one requirement"), so a delta that removes 100% of a capability's requirements can never be archived — it aborts atomically before writing anything. The working mechanism is: the change's `specs/` only carries `ADDED Requirements` deltas for the 4 `vm-*` capabilities (plus the 2 `MODIFIED` wording deltas); `openspec archive` creates the 4 new capability directories; the 4 old `vps-*` capability directories are then deleted directly from `openspec/specs/` as a plain filesystem step (`git rm -r`) right after a successful archive, not through the delta mechanism at all.

**New capability Purpose text.** The `specs` artifact template has no `## Purpose` section for `ADDED Requirements` deltas — the archive tooling auto-generates a `TBD` placeholder for a brand-new capability directory. Since these 4 "new" capabilities are really renames with a well-known Purpose (the old capability's Purpose, reworded), `tasks.md` includes an explicit step to replace each `TBD` Purpose immediately after archiving, copying the prior capability's Purpose text with "vps"→"vm" applied, rather than leaving it as a placeholder.

**Wording-only capabilities use MODIFIED, not a rename.** `ubuntu-qemu-prerequisites` and `libvirt-group-session-handling` keep their directory names (never had "vps" in the name) — only their requirement body text changes ("Debian VPS setup" → "Debian VM setup"). This is a `MODIFIED Requirements` delta with full requirement content per the schema's rules, even though nothing about system behavior changes — there's no lighter-weight "wording-only" delta operator available.

## Risks / Trade-offs

- **[Risk]** Deleting the 4 old capability directories outside the delta mechanism loses the requirement's history/identity in tooling that tracks capabilities by name across time (e.g. `openspec list --specs` will show 4 new capability names appearing the same day 4 others disappear, with no built-in link between them). → **Mitigation**: this proposal/design document the 1:1 mapping (`vps-cleanup-scope`→`vm-cleanup-scope`, etc.) for anyone reading change history later; the archived change's own `specs/` directory (still containing the 4 `ADDED` deltas) remains the durable record of what moved where.
- **[Risk]** Manually re-typing 19 requirements (6 + 5 + 2 + 6) across the 4 renamed capabilities risks transcription drift from the originals. → **Mitigation**: apply-time step diffs each new spec file against `git show HEAD:openspec/specs/<old-name>/spec.md` before archiving, to confirm only the intended strings changed.
- **[Risk]** Forgetting to update a cross-reference (e.g., a spec that mentions another capability's default) leaves stale "vps" text behind despite the rename's goal of full consistency. → **Mitigation**: `tasks.md` ends with a repo-wide `grep -ri vps` sweep (excluding `openspec/changes/archive/**`) that must return no results before the change is considered done.

## Migration Plan

1. Apply the script edits (rename files, swap defaults/help text, preserve the 4 excluded lines).
2. Apply the two wording-only spec edits (`ubuntu-qemu-prerequisites`, `libvirt-group-session-handling`) directly to `openspec/specs/`.
3. Archive this change so the CLI applies the `ADDED`/`MODIFIED` deltas to `openspec/specs/`, creating the 4 `vm-*` directories.
4. Immediately after archiving, delete the 4 old `vps-*` capability directories from `openspec/specs/` directly (`git rm -r`) — the tool has no delta operator for retiring a capability entirely, so this is a plain filesystem step, not part of the archive.
5. Replace the auto-generated `TBD` Purpose text on each of the 4 new capability spec files with the reworded original Purpose.
6. Update `TODO.md`.
7. Run the repo-wide `vps` grep sweep to confirm nothing active was missed.

No rollback beyond `git revert` — this is a same-day, low-risk, local-only rename with no external consumers.

## Open Questions

None — scope and exceptions were confirmed during exploration before this proposal was written.
