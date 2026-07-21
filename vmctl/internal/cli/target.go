// Package cli holds the plumbing shared by every vmctl subcommand: the
// --name/working-directory convention, preflight checks, and confirmation
// semantics. It replaces logic that was previously copy-pasted (with drift)
// across the three bash scripts.
package cli

import (
	"os"
	"path/filepath"
)

// DefaultVMName is used when --name is not given, matching the bash scripts.
const DefaultVMName = "debian-vm"

// Target identifies a VM a subcommand operates on and its working directory.
type Target struct {
	Name    string
	WorkDir string
}

// ResolveTarget applies the --name default and the $HOME/vms/<name>
// working-directory convention used by every subcommand.
func ResolveTarget(name string) Target {
	if name == "" {
		name = DefaultVMName
	}
	home, _ := os.UserHomeDir()
	return Target{Name: name, WorkDir: filepath.Join(home, "vms", name)}
}
