package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTarget_Default(t *testing.T) {
	home, _ := os.UserHomeDir()
	target := ResolveTarget("")
	if target.Name != DefaultVMName {
		t.Errorf("Name = %q, want %q", target.Name, DefaultVMName)
	}
	want := filepath.Join(home, "vms", DefaultVMName)
	if target.WorkDir != want {
		t.Errorf("WorkDir = %q, want %q", target.WorkDir, want)
	}
}

func TestResolveTarget_ExplicitName(t *testing.T) {
	home, _ := os.UserHomeDir()
	target := ResolveTarget("app-01")
	if target.Name != "app-01" {
		t.Errorf("Name = %q, want %q", target.Name, "app-01")
	}
	want := filepath.Join(home, "vms", "app-01")
	if target.WorkDir != want {
		t.Errorf("WorkDir = %q, want %q", target.WorkDir, want)
	}
}
