package main

import (
	"flag"
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=<tag>".
// A plain local `go build` keeps this default, so `vmctl version` never
// prints an empty string.
var version = "dev"

func runVersion(args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl version")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Prints the version vmctl was built with ('dev' for a local, unstamped build).")
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	fmt.Println(version)
	return nil
}
