package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"vmctl/internal/backup"
	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
)

func snapshotUsage() {
	fmt.Fprintln(os.Stdout, `Usage: vmctl snapshot <verb> --name=<vm> [options]

Verbs:
  create   Create a single external, disk-only rollback point for the VM.
           Fails if one already exists (one at a time).
  restore  Revert the VM's disk to the pre-snapshot state, discarding all
           writes made since. Prompts for confirmation.
  delete   Merge the active snapshot's overlay back into the VM's disk,
           KEEPING all writes made since.

Options:
  --name=NAME   VM to target (default: debian-vm).
  -h, --help    Show this help.

Notes:
  - 'restore' always prompts; there is no non-interactive bypass flag.`)
}

var snapshotVerbToSubcommand = map[string]string{
	"create":  "snapshot",
	"restore": "snapshot-restore",
	"delete":  "snapshot-delete",
}

func runSnapshot(args []string) error {
	if len(args) == 0 {
		snapshotUsage()
		return fmt.Errorf("a verb is required (create, restore, or delete)")
	}

	verb := args[0]
	if verb == "-h" || verb == "--help" {
		snapshotUsage()
		return nil
	}
	subcommand, ok := snapshotVerbToSubcommand[verb]
	if !ok {
		snapshotUsage()
		return fmt.Errorf("unknown verb: %s", verb)
	}
	rest := args[1:]

	fs := flag.NewFlagSet("snapshot "+verb, flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "VM to target (default: debian-vm)")
	fs.Usage = snapshotUsage
	if err := fs.Parse(rest); err != nil {
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

	return backup.Run(context.Background(), execrunner.Real{}, os.Stdout, os.Stdin, backup.Options{
		Subcommand: subcommand,
		Name:       *name,
	})
}
