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
	purgeAll := fs.Bool("purge-all", false, "Non-interactive. Removes the VM, the working directory, host firewall hardening, and host-wide monitoring infrastructure. Refuses to run if another VM still exists. Does NOT touch host prerequisites (packages, groups, network, ACL) — use 'vmctl doctor --unfix' for those.")
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl cleanup [--name=NAME] [--vm-only|--purge-all]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Without a flag, walks through each removal step interactively.")
		fmt.Fprintln(os.Stdout, "Host prerequisites (packages, groups, network, ACL) are managed separately")
		fmt.Fprintln(os.Stdout, "by 'vmctl doctor' / 'vmctl doctor --fix' / 'vmctl doctor --unfix'.")
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
