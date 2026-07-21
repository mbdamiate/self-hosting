package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
	"vmctl/internal/power"
)

func runReboot(args []string) error {
	fs := flag.NewFlagSet("reboot", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to reboot (default: debian-vm)")
	force := fs.Bool("force", false, "Hard reset (virsh reset) instead of a graceful ACPI reboot")
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl reboot [--name=NAME] [--force]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Requests a graceful ACPI reboot by default. --force performs a hard reset")
		fmt.Fprintln(os.Stdout, "instead. Fails with virsh's own error if the VM isn't running.")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if err := cli.RequireNonRoot(); err != nil {
		return err
	}
	if err := cli.RequireVirsh(); err != nil {
		return err
	}

	target := cli.ResolveTarget(*name)
	return power.Reboot(context.Background(), execrunner.Real{}, os.Stdout, target.Name, *force)
}
