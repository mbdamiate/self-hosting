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

func runStop(args []string) error {
	fs := flag.NewFlagSet("stop", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to stop (default: debian-vm)")
	force := fs.Bool("force", false, "Hard power-off (virsh destroy) instead of a graceful ACPI shutdown")
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl stop [--name=NAME] [--force]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Requests a graceful ACPI shutdown by default. --force powers it off")
		fmt.Fprintln(os.Stdout, "immediately instead.")
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
	return power.Stop(context.Background(), execrunner.Real{}, os.Stdout, target.Name, *force)
}
