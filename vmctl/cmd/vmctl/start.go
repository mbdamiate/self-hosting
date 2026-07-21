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

func runStart(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to start (default: debian-vm)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl start [--name=NAME]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Starts the VM if it isn't already running.")
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
	return power.Start(context.Background(), execrunner.Real{}, os.Stdout, target.Name)
}
