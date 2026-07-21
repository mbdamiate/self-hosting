package cli

import (
	"errors"
	"testing"
)

func TestRequireNonRoot(t *testing.T) {
	if err := requireNonRoot(func() int { return 1000 }); err != nil {
		t.Errorf("non-root euid: got error %v, want nil", err)
	}
	if err := requireNonRoot(func() int { return 0 }); err == nil {
		t.Error("root euid: got nil error, want an error")
	}
}

func TestRequireVirsh(t *testing.T) {
	ok := func(string) (string, error) { return "/usr/bin/virsh", nil }
	missing := func(string) (string, error) { return "", errors.New("not found") }

	if err := requireVirsh(ok); err != nil {
		t.Errorf("virsh present: got error %v, want nil", err)
	}
	if err := requireVirsh(missing); err == nil {
		t.Error("virsh missing: got nil error, want an error")
	}
}
