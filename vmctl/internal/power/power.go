// Package power implements the VM power-state mutations behind `vmctl
// start`/`vmctl stop`/`vmctl reboot`: distinct from internal/fleet's
// read-only status querying, and from internal/backup's stop/start of a VM
// as a means to disk-consistency rather than a user-facing power command.
package power

import (
	"context"
	"fmt"
	"io"
	"strings"

	"vmctl/internal/execrunner"
)

func domState(ctx context.Context, r execrunner.Runner, name string) (string, error) {
	output, err := r.Run(ctx, "virsh", "domstate", name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Start starts name if it isn't already running.
func Start(ctx context.Context, r execrunner.Runner, out io.Writer, name string) error {
	state, err := domState(ctx, r, name)
	if err != nil {
		return err
	}
	if state == "running" {
		fmt.Fprintf(out, "'%s' is already running.\n", name)
		return nil
	}
	if _, err := r.Run(ctx, "virsh", "start", name); err != nil {
		return err
	}
	fmt.Fprintf(out, "'%s' started.\n", name)
	return nil
}

// Stop gracefully shuts down name (ACPI, via `virsh shutdown`) unless force
// is set, in which case it powers it off immediately (`virsh destroy`). It
// no-ops if the VM is already stopped.
func Stop(ctx context.Context, r execrunner.Runner, out io.Writer, name string, force bool) error {
	state, err := domState(ctx, r, name)
	if err != nil {
		return err
	}
	if state == "shut off" {
		fmt.Fprintf(out, "'%s' is already stopped.\n", name)
		return nil
	}
	if force {
		if _, err := r.Run(ctx, "virsh", "destroy", name); err != nil {
			return err
		}
		fmt.Fprintf(out, "'%s' powered off (--force).\n", name)
		return nil
	}
	if _, err := r.Run(ctx, "virsh", "shutdown", name); err != nil {
		return err
	}
	fmt.Fprintf(out, "'%s' is shutting down gracefully.\n", name)
	return nil
}

// Reboot gracefully reboots name (ACPI, via `virsh reboot`) unless force is
// set, in which case it performs a hard reset (`virsh reset`). It performs
// no pre-check on the VM's state, letting virsh's own error surface
// verbatim if the VM isn't running.
func Reboot(ctx context.Context, r execrunner.Runner, out io.Writer, name string, force bool) error {
	if force {
		if _, err := r.Run(ctx, "virsh", "reset", name); err != nil {
			return err
		}
		fmt.Fprintf(out, "'%s' hard-reset (--force).\n", name)
		return nil
	}
	if _, err := r.Run(ctx, "virsh", "reboot", name); err != nil {
		return err
	}
	fmt.Fprintf(out, "'%s' is rebooting gracefully.\n", name)
	return nil
}
