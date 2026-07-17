## 1. Move the existence check earlier

- [x] 1.1 Move the `virsh dominfo "$VM_NAME"` existence check from immediately before `virt-install` to right after the QEMU storage ACL section, before SSH key handling.
- [x] 1.2 Branch on the check: the existing-VM path skips SSH key handling, `$WORK_DIR`/disk download/copy/resize, cloud-init generation, and `virt-install`; the no-VM path proceeds through them as today.

## 2. Effective network mode introspection

- [x] 2.1 After the branch converges, determine `EFFECTIVE_MODE` (`bridged` or `nat`) and `EFFECTIVE_BRIDGE_IFACE` (when bridged) by parsing `virsh domiflist "$VM_NAME"`'s interface `Type`/`Source` columns; exit with a diagnostic if no interface line is found.
- [x] 2.2 In the existing-VM path, compare the requested `--bridge` flag against `EFFECTIVE_MODE`; on mismatch, print a warning naming the VM's actual mode/interface and pointing to `virsh undefine --remove-all-storage` + rerun, then continue using `EFFECTIVE_MODE`.

## 3. Auto-start and VM autostart

- [x] 3.1 In the existing-VM path, check `virsh domstate "$VM_NAME"`; if not `running`, run `virsh start "$VM_NAME"`, exiting with an actionable error on failure.
- [x] 3.2 After the branch converges, run `virsh autostart "$VM_NAME"` unconditionally; on failure, print a warning and continue (non-fatal).

## 4. Idempotent port forwarding

- [x] 4.1 Before each `iptables -t nat -A PREROUTING ...` DNAT rule, check with `iptables -t nat -C PREROUTING ...` first and skip adding it (with a message) if it already exists.
- [x] 4.2 Before each `iptables -I FORWARD ...` accept rule, check with `iptables -C FORWARD ...` first and skip adding it (with a message) if it already exists.
- [x] 4.3 Guard forwarding application on `EFFECTIVE_MODE` being NAT-family: if `--forward` is requested but `EFFECTIVE_MODE` is bridged, skip forwarding entirely and explain that it requires the NAT network.

## 5. Unified connection-info summary

- [x] 5.1 Move VM IP detection (`virsh domifaddr` / DHCP lease fallback) into the unified tail that runs after the branch converges, so it runs for both the freshly-created and already-existing paths.
- [x] 5.2 Update the final help block to select bridged/forward/plain-NAT messaging based on `EFFECTIVE_MODE`/`EFFECTIVE_BRIDGE_IFACE` instead of the raw `$BRIDGE_IFACE`/`$FORWARD_RULES` flags, so it reflects the VM's real state on both paths.

## 6. Verification

- [x] 6.1 Run shell syntax validation (`bash -n`) for the updated setup script.
- [x] 6.2 Review the fresh-create, existing-running, existing-stopped, and mode-mismatch control paths against the new OpenSpec scenarios.
