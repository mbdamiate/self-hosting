package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"

	"vmctl/internal/backup"
	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
)

func backupUsage() {
	fmt.Fprintln(os.Stdout, `Usage: vmctl backup <subcommand> --name=<vm> [options]

Subcommands:
  snapshot          Create a single external, disk-only rollback point for
                    the VM. Fails if one already exists (one at a time).
  snapshot-restore  Revert the VM's disk to the pre-snapshot state,
                    discarding all writes made since. Prompts for
                    confirmation.
  snapshot-delete   Merge the active snapshot's overlay back into the VM's
                    disk, KEEPING all writes made since.
  backup            Write a compressed, point-in-time copy of the VM's disk
                    to a separate destination. Works live or stopped.
  backup-list       List backups available for the VM at its destination.
  backup-restore    Replace the VM's current disk with the contents of a
                    chosen backup. Prompts for confirmation.

Options:
  --name=NAME   VM to target (default: debian-vm).
  --dest=DIR    Backup destination directory (default: $HOME/vm-backups/<name>/).
  --keep=N      With 'backup': after a successful backup, delete this VM's
                own backups beyond the N most recent. No pruning by default.
  --file=PATH   With 'backup-restore': the backup file to restore from.
  -h, --help    Show this help.

Notes:
  - snapshot-restore and backup-restore always prompt; there is no
    non-interactive bypass flag.`)
}

func runBackup(args []string) error {
	if len(args) == 0 {
		backupUsage()
		return fmt.Errorf("a subcommand is required")
	}

	subcommand := args[0]
	if subcommand == "-h" || subcommand == "--help" {
		backupUsage()
		return nil
	}
	rest := args[1:]

	fs := flag.NewFlagSet("backup "+subcommand, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to target (default: debian-vm)")
	dest := fs.String("dest", "", "Backup destination directory (default: $HOME/vm-backups/<name>/)")
	keep := fs.String("keep", "", "With 'backup': delete this VM's own backups beyond the N most recent")
	file := fs.String("file", "", "With 'backup-restore': the backup file to restore from")
	fs.Usage = backupUsage
	if err := fs.Parse(rest); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	keepN := 0
	if *keep != "" {
		n, err := strconv.Atoi(*keep)
		if err != nil {
			return fmt.Errorf("--keep must be a number: %w", err)
		}
		keepN = n
	}

	if err := cli.RequireNonRoot(); err != nil {
		return err
	}
	if err := cli.RequireVirsh(); err != nil {
		return err
	}

	return backup.Run(context.Background(), execrunner.Real{}, os.Stdout, os.Stdin, backup.Options{
		Subcommand: subcommand,
		Name:       *name,
		Dest:       *dest,
		Keep:       keepN,
		File:       *file,
	})
}
