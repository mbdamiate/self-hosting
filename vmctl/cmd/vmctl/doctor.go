package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"vmctl/internal/cli"
	"vmctl/internal/execrunner"
	"vmctl/internal/hostready"
)

func runDoctor(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fix := fs.Bool("fix", false, "Install and configure every missing host prerequisite (packages, libvirt/kvm group membership, libvirtd, the 'default' NAT network, the QEMU storage ACL).")
	unfix := fs.Bool("unfix", false, "Revert everything --fix establishes. Refuses to run while any VM is still defined on the host.")
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl doctor [--fix|--unfix]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Without a flag, reports every host-readiness check as OK or MISSING and")
		fmt.Fprintln(os.Stdout, "makes no changes. 'vmctl create' relies on these same checks and fails")
		fmt.Fprintln(os.Stdout, "fast on the first one that's missing.")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if *fix && *unfix {
		return fmt.Errorf("--fix and --unfix are mutually exclusive")
	}

	if err := cli.RequireNonRoot(); err != nil {
		return err
	}

	ctx := context.Background()
	r := execrunner.Real{}

	switch {
	case *fix:
		return hostready.Fix(ctx, r, os.Stdout)
	case *unfix:
		return hostready.Unfix(ctx, r, os.Stdout)
	default:
		return runDoctorReport(ctx, r)
	}
}

func runDoctorReport(ctx context.Context, r execrunner.Runner) error {
	results := hostready.Check(ctx, r)
	missing := 0
	for _, res := range results {
		if res.OK {
			fmt.Printf("[OK]      %s\n", res.Name)
			continue
		}
		missing++
		fmt.Printf("[MISSING] %s: %s\n", res.Name, res.Detail)
	}
	if missing > 0 {
		return fmt.Errorf("%d of %d host prerequisite(s) missing (see above). Run 'vmctl doctor --fix' to install/configure them", missing, len(results))
	}
	fmt.Println("Host is ready.")
	return nil
}
