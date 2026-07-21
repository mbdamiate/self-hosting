package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
	"vmctl/internal/fleet"
)

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl list")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Lists every VM currently defined in libvirt, querying it live (no cached data).")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if err := cli.RequireVirsh(); err != nil {
		return err
	}

	infos, err := fleet.List(context.Background(), execrunner.Real{})
	if err != nil {
		return err
	}
	fleet.RenderList(os.Stdout, infos)
	return nil
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to show (default: debian-vm)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl status [--name=NAME]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Shows one VM's live status, querying it fresh (no cached data).")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if err := cli.RequireVirsh(); err != nil {
		return err
	}

	target := cli.ResolveTarget(*name)
	info, err := fleet.Get(context.Background(), execrunner.Real{}, target.Name)
	if err != nil {
		return err
	}
	fleet.RenderStatus(os.Stdout, info)
	return nil
}
