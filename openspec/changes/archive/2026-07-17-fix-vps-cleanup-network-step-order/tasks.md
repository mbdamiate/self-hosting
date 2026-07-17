## 1. Reorder steps

- [x] 1.1 Move the default-network removal block in `scripts/debian-vps-cleanup.sh` to run immediately after the VM removal step (1) and before the working-directory removal step
- [x] 1.2 Renumber the step comments/output (VM → network → working directory → packages → groups → ACL) to match the new order
- [x] 1.3 Confirm the network-removal block is still gated so it does not run at all when `--vm-only` is set

## 2. Visibility for the "nothing to do" case

- [x] 2.1 Add an explicit message when `command -v virsh` or `virsh net-info default` fails at the network step (e.g. "no default network found, skipping"), matching the style of steps 1 and 2's existing "not found, skipping" messages

## 3. Verification

- [x] 3.1 Manually run the no-flags interactive path, confirming every prompt (including the default-network prompt) appears and that answering "y" actually removes the `default` network — check with `virsh net-info default` before packages are purged
- [x] 3.2 Manually run `--purge-all` and confirm the default network is gone at the end (not just the packages), by checking `virsh net-list --all` before packages disappear, or by observing the network-removal message in the run's output
- [x] 3.3 Manually run cleanup a second time after libvirt is already fully removed, and confirm the network step now prints a clear "nothing to remove" message instead of producing no output
