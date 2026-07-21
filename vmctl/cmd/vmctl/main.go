// Command vmctl replaces debian-vm-setup.sh, debian-vm-cleanup.sh, and
// debian-vm-backup.sh with a single binary. Each subcommand still shells out
// to virsh/virt-install/qemu-img/cloud-localds/genisoimage/iptables; vmctl
// runs and exits like the scripts it replaces, it is not a daemon.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(1)
	}

	args := os.Args[2:]
	var err error
	switch os.Args[1] {
	case "setup":
		err = runSetup(args)
	case "cleanup":
		err = runCleanup(args)
	case "backup":
		err = runBackup(args)
	case "list":
		err = runList(args)
	case "status":
		err = runStatus(args)
	case "-h", "--help", "help":
		usage(os.Stdout)
		return
	default:
		fmt.Fprintf(os.Stderr, "ERROR: unknown subcommand: %s\n\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func usage(w *os.File) {
	fmt.Fprint(w, `Usage: vmctl <subcommand> [options]

Subcommands:
  setup     Create or reuse a Debian VM
  cleanup   Remove a VM and/or the host-level resources setup installed
  backup    Snapshot/backup/restore a VM's disk
  list      List all defined VMs
  status    Show a single VM's status

Run 'vmctl <subcommand> --help' for subcommand-specific options.
`)
}
