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
	fmt.Fprintln(os.Stdout, `Usage: vmctl backup <verb> --name=<vm> [options]

Verbs:
  create   Write a compressed, point-in-time copy of the VM's disk to a
           separate destination. Works live or stopped.
  list     List backups available for the VM at its destination.
  restore  Replace the VM's current disk with the contents of a chosen
           backup. Prompts for confirmation.

Options:
  --name=NAME   VM to target (default: debian-vm).
  --dest=DIR    Backup destination directory (default: $HOME/vm-backups/<name>/).
  --keep=N      With 'create': after a successful backup, delete this VM's
                own backups beyond the N most recent. No pruning by default.
  --file=PATH   With 'restore': the backup file to restore from.
  -h, --help    Show this help.

Notes:
  - 'restore' always prompts; there is no non-interactive bypass flag.`)
}

var backupVerbToSubcommand = map[string]string{
	"create":  "backup",
	"list":    "backup-list",
	"restore": "backup-restore",
}

func runBackup(args []string) error {
	if len(args) == 0 {
		backupUsage()
		return fmt.Errorf("a verb is required (create, list, or restore)")
	}

	verb := args[0]
	if verb == "-h" || verb == "--help" {
		backupUsage()
		return nil
	}
	subcommand, ok := backupVerbToSubcommand[verb]
	if !ok {
		backupUsage()
		return fmt.Errorf("unknown verb: %s", verb)
	}
	rest := args[1:]

	fs := flag.NewFlagSet("backup "+verb, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to target (default: debian-vm)")
	dest := fs.String("dest", "", "Backup destination directory (default: $HOME/vm-backups/<name>/)")
	keep := fs.String("keep", "", "With 'create': delete this VM's own backups beyond the N most recent")
	file := fs.String("file", "", "With 'restore': the backup file to restore from")
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
