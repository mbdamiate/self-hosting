## Context

The setup script adds the caller to `libvirt` and `kvm`, then uses nested `sg` commands to restart itself. On Ubuntu, the child process can still fail the `libvirt` membership check, restarting the whole script and repeating APT operations forever.

## Goals / Non-Goals

**Goals:**

- Prevent repeated full-script execution after group membership changes.
- Avoid creating a VM until the current process has both required group memberships.
- Give the user an actionable recovery path when a new login session is required.

**Non-Goals:**

- Refresh Linux group memberships inside the current login session.
- Change package installation, libvirt service configuration, or VM networking.
- Automatically log the user out or reboot the host.

## Decisions

- Remove the nested `sg` self-reexecution. It is the source of the unbounded control flow and runs package installation repeatedly.
- Inspect the current process's effective group list for both `libvirt` and `kvm` after `usermod`. If either is absent, print that the group assignments were made, direct the user to log out/in, and exit nonzero before SSH, disk, or VM steps run. A nonzero exit accurately signals that setup is incomplete to interactive users and automation.
- Let the user run the same script again after starting a new login session. APT installation and group addition are idempotent, while the refreshed session permits the script to proceed normally.

## Risks / Trade-offs

- [First-time setup needs two invocations] → The first invocation explains the required logout/login and makes no VM changes; the second continues predictably.
- [A stale or unusual session still lacks a group after login] → Check both group names and stop with explicit diagnostics instead of looping or failing later at `virsh`.
- [Users prefer no logout] → This change favors a reliable OS-supported session refresh over attempting to emulate it with nested shells.
