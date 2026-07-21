package setup

import (
	"context"
	"fmt"
	"io"
	"strings"

	"vmctl/internal/execrunner"
)

// printRerunMismatchWarnings mirrors section 11: when reusing an existing
// VM, warn (never fail) about any flag that conflicts with what was
// actually determined from libvirt, and start the VM if it isn't running.
func printRerunMismatchWarnings(ctx context.Context, r execrunner.Runner, out io.Writer, vmName string, opts Options, eff effectiveConfig) error {
	undefineHint := fmt.Sprintf("virsh undefine %s --remove-all-storage", vmName)

	requestedBridged := opts.BridgeIface != ""
	effectiveBridged := eff.NetworkMode == "bridged"
	if requestedBridged != effectiveBridged {
		fmt.Fprintln(out)
		if effectiveBridged {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists using bridged networking (via '%s'),\n", vmName, eff.BridgeIface)
			fmt.Fprintln(out, "         but this run did not request --bridge.")
		} else {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists using NAT networking (virbr0),\n", vmName)
			fmt.Fprintln(out, "         but this run requested --bridge.")
		}
		fmt.Fprintln(out, "         Network mode is fixed when the VM is created and cannot be changed by rerunning")
		fmt.Fprintln(out, "         this. To use a different mode, remove the VM first and run again:")
		fmt.Fprintf(out, "           %s\n", undefineHint)
		fmt.Fprintf(out, "         Continuing with the VM's actual (%s) networking.\n", eff.NetworkMode)
	}

	effectiveAdminPassword := eff.AdminSudo == "password-required"
	if eff.AdminSudo == "" {
		if opts.AdminPasswordRequested {
			fmt.Fprintln(out)
			fmt.Fprintf(out, "WARNING: VM '%s' already exists, but its sudo policy cannot be determined\n", vmName)
			fmt.Fprintln(out, "         (no record found — it may predate --admin-password).")
			fmt.Fprintln(out, "         Sudo policy is fixed when the VM is created and cannot be changed by rerunning")
			fmt.Fprintln(out, "         this. To apply --admin-password, remove the VM first and run again:")
			fmt.Fprintf(out, "           %s\n", undefineHint)
		}
	} else if opts.AdminPasswordRequested != effectiveAdminPassword {
		fmt.Fprintln(out)
		if effectiveAdminPassword {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists with password-required sudo,\n", vmName)
			fmt.Fprintln(out, "         but this run did not request --admin-password.")
		} else {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists with passwordless sudo (NOPASSWD:ALL),\n", vmName)
			fmt.Fprintln(out, "         but this run requested --admin-password.")
		}
		fmt.Fprintln(out, "         Sudo policy is fixed when the VM is created and cannot be changed by rerunning")
		fmt.Fprintln(out, "         this. To use a different policy, remove the VM first and run again:")
		fmt.Fprintf(out, "           %s\n", undefineHint)
		fmt.Fprintln(out, "         Continuing with the VM's actual sudo policy.")
	}

	if opts.Watchdog != eff.Watchdog {
		fmt.Fprintln(out)
		if eff.Watchdog {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists with a watchdog device,\n", vmName)
			fmt.Fprintln(out, "         but this run did not request --watchdog.")
		} else {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists with no watchdog device,\n", vmName)
			fmt.Fprintln(out, "         but this run requested --watchdog.")
		}
		fmt.Fprintln(out, "         Watchdog configuration is fixed when the VM is created and cannot be")
		fmt.Fprintln(out, "         changed by rerunning this. To use a different configuration,")
		fmt.Fprintln(out, "         remove the VM first and run again:")
		fmt.Fprintf(out, "           %s\n", undefineHint)
		fmt.Fprintln(out, "         Continuing with the VM's actual watchdog configuration.")
	}

	requestedCrashRestart := !opts.NoCrashRestart
	if requestedCrashRestart != eff.CrashRestart {
		fmt.Fprintln(out)
		if eff.CrashRestart {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists with on_crash=restart,\n", vmName)
			fmt.Fprintln(out, "         but this run requested --no-crash-restart.")
		} else {
			fmt.Fprintf(out, "WARNING: VM '%s' already exists without on_crash=restart,\n", vmName)
			fmt.Fprintln(out, "         but this run did not request --no-crash-restart.")
		}
		fmt.Fprintln(out, "         Crash-recovery policy is fixed when the VM is created and cannot be")
		fmt.Fprintln(out, "         changed by rerunning this. To use a different policy, remove")
		fmt.Fprintln(out, "         the VM first and run again:")
		fmt.Fprintf(out, "           %s\n", undefineHint)
		fmt.Fprintln(out, "         Continuing with the VM's actual crash-recovery policy.")
	}

	if opts.Monitor && opts.BridgeIface == "" && !eff.LogForward {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "WARNING: --monitor was requested, but log forwarding for VM '%s' was not\n", vmName)
		fmt.Fprintln(out, "         configured at creation (cloud-init only applies at first boot) — it")
		fmt.Fprintln(out, "         predates --monitor, was created with --bridge, or virbr0 didn't exist yet.")
		fmt.Fprintln(out, "         Uptime monitoring still applies (host-side, reapplied below), but logs")
		fmt.Fprintln(out, "         from this VM will NOT reach /var/log/self-hosting-vms/. To fix, remove")
		fmt.Fprintf(out, "         and recreate the VM: %s\n", undefineHint)
	}

	stateOut, _ := r.Run(ctx, "virsh", "domstate", vmName)
	state := strings.TrimSpace(string(stateOut))
	if state != "running" {
		fmt.Fprintf(out, "==> VM '%s' is currently '%s', starting it...\n", vmName, state)
		if _, err := r.Run(ctx, "virsh", "start", vmName); err != nil {
			return fmt.Errorf("failed to start VM '%s'. Inspect with: virsh domstate %s", vmName, vmName)
		}
	} else {
		fmt.Fprintf(out, "    OK: VM '%s' is already running.\n", vmName)
	}
	return nil
}

func trimNewline(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
