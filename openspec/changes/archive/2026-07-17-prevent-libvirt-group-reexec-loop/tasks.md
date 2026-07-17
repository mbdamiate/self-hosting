## 1. Replace session-group control flow

- [x] 1.1 Remove the nested `sg` self-reexecution from the local VPS setup script.
- [x] 1.2 Add an effective-session check for both `libvirt` and `kvm` after host group membership is updated.
- [x] 1.3 Stop with a clear logout/login and rerun instruction when either required group is inactive; do not reach VM-file or VM-creation steps.

## 2. Verify the behavior

- [x] 2.1 Check the setup script for shell syntax errors.
- [x] 2.2 Verify statically that the script has no self-reexecution path and requires both groups before invoking the unprivileged libvirt workflow.
