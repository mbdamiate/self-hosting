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
	case "create":
		err = runCreate(args)
	case "start":
		err = runStart(args)
	case "stop":
		err = runStop(args)
	case "reboot":
		err = runReboot(args)
	case "delete":
		err = runDelete(args)
	case "list":
		err = runList(args)
	case "info":
		err = runInfo(args)
	case "snapshot":
		err = runSnapshot(args)
	case "backup":
		err = runBackup(args)
	case "doctor":
		err = runDoctor(args)
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
  create    Create or reuse a Debian VM
  start     Start a stopped VM
  stop      Gracefully shut down a running VM (--force for a hard power-off)
  reboot    Gracefully reboot a running VM (--force for a hard reset)
  delete    Remove a VM (and, with --purge-all, opt-in host firewall/monitoring features)
  list      List all defined VMs
  info      Show a single VM's status
  snapshot  Create/restore/delete a local disk-only rollback point
  backup    Create/list/restore a compressed point-in-time disk copy
  doctor    Check, install (--fix), or remove (--unfix) host prerequisites

Run 'vmctl <subcommand> --help' for subcommand-specific options.
`)
}
