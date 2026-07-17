## Context

`debian-vps-cleanup.sh` runs its removal steps in a fixed order: VM (1), working directory (2), package purge (3), group removal (4), default network removal (5), QEMU storage ACL revocation (6, added by `add-vps-cleanup-scope-flags`). Step 3's `apt purge` removes `libvirt-clients`, which provides the `virsh` binary. Step 5 guards itself with `command -v virsh >/dev/null 2>&1 && virsh net-info default >/dev/null 2>&1` — a check that now fails whenever step 3 already ran, because the binary it depends on is gone. The outer `if` has no `else`, so the entire step — prompt included — vanishes with no output. This was confirmed live: the interactive walkthough jumped straight from the groups-removal confirmation to the ACL-revocation prompt, with nothing for the network step in between.

## Goals / Non-Goals

**Goals:**

- Make default-network removal actually run (or make an explicit, visible decision not to) regardless of whether packages are also being removed in the same invocation.
- Give the network step the same "nothing to do here" visibility that steps 1 and 2 already have, instead of silent disappearance.

**Non-Goals:**

- Changing what any step removes, or the `--vm-only`/`--purge-all`/no-flags gating logic added by `add-vps-cleanup-scope-flags`. Network removal still SHALL NOT run under `--vm-only`.
- Detecting or distinguishing "virsh was never installed," "virsh was removed in a prior run," vs. "virsh was just removed by step 3 in this run" — all three should simply work correctly or report clearly, without needing to know which case they're in.
- Any change to `debian-vps-setup.sh`.

## Decisions

### Move network removal to run before package purge

Reorder the steps so default-network removal (today's step 5) executes immediately after the VM removal step (1) and before the working-directory (2) and package-purge (3) steps. This guarantees `virsh` is still installed and functional when the network step runs, for every invocation mode (`--vm-only` skips it entirely regardless of position, `--purge-all` and no-flags both benefit from the reordering). Steps are renumbered in comments/output accordingly (VM → network → working directory → packages → groups → ACL), but no step's internal logic changes — only its position in the sequence.

Alternative considered: keep the original order but re-check for `virsh` availability by capturing whether it was present at script start, before any removal happened, and using that captured state at step 5. Rejected — this adds a second flag to track for no benefit over simply doing the network step earlier; a mid-script snapshot is more code and more state to keep in sync than a straightforward reorder.

Alternative considered: skip package purge's removal of `libvirt-clients` specifically, to keep `virsh` around throughout the whole script. Rejected — this would leave the `virsh` binary installed for a "full" purge that's supposed to remove it, undermining `--purge-all`'s and the interactive path's own guarantee that packages are gone at the end.

### Report clearly when there is nothing to remove

Add an explicit message ("no default network found, skipping" or equivalent) for the case where `command -v virsh` or `virsh net-info default` fails, mirroring steps 1 and 2's existing `else` branches for their own "nothing here" cases. This makes a second cleanup run (after libvirt is already gone from a prior `--purge-all`) behave visibly rather than silently, without needing to distinguish why `virsh` is unavailable.

## Risks / Trade-offs

- [Some other, unrelated step comes to depend on `virsh` being removed before it runs] → None currently do; the reordering only moves the network step earlier, it does not reorder anything relative to steps 4 or 6.
- [Users who read the old step numbers in issue trackers or notes now see different step positions] → This is a personal script with a single user already aware of the change; no external documentation references old step numbers besides the script's own comments, which are updated in this same change.
