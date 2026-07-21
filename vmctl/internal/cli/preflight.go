package cli

import (
	"fmt"
	"os"
	"os/exec"
)

// RequireNonRoot mirrors every bash script's "don't run this as root" check.
func RequireNonRoot() error {
	return requireNonRoot(os.Geteuid)
}

func requireNonRoot(geteuid func() int) error {
	if geteuid() == 0 {
		return fmt.Errorf("don't run this as root. Run it as your normal user (it will use sudo when needed)")
	}
	return nil
}

// RequireVirsh mirrors the "virsh not found" preflight check performed by
// cleanup and backup before doing anything else.
func RequireVirsh() error {
	return requireVirsh(exec.LookPath)
}

func requireVirsh(lookPath func(string) (string, error)) error {
	if _, err := lookPath("virsh"); err != nil {
		return fmt.Errorf("'virsh' was not found. Run 'vmctl setup' first")
	}
	return nil
}
