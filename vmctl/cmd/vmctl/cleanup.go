package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"vmctl/internal/cleanup"
	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
)

func runCleanup(args []string) error {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	name := fs.String("name", "", "VM to target (default: debian-vm). Use a distinct --name per VM when managing a fleet of VMs.")
	vmOnly := fs.Bool("vm-only", false, "Non-interactive. Removes only the named VM, its attached storage, and its network reservation. Preserves everything else so a rerun of 'vmctl setup' is fast.")
	purgeAll := fs.Bool("purge-all", false, "Non-interactive. Removes everything this VM's setup installed. Refuses to run if another VM still exists.")
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl cleanup [--name=NAME] [--vm-only|--purge-all]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Without a flag, walks through each removal step interactively.")
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

	tools := cleanup.Tools{}
	if _, err := exec.LookPath("virsh"); err == nil {
		tools.Virsh = true
	}
	if _, err := exec.LookPath("ufw"); err == nil {
		tools.UFW = true
	}

	return cleanup.Run(context.Background(), execrunner.Real{}, os.Stdout, os.Stdin, cleanup.Options{
		Name:     *name,
		VMOnly:   *vmOnly,
		PurgeAll: *purgeAll,
	}, tools)
}
