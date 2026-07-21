package execrunner

import (
	"context"
	"strings"
	"testing"
)

func TestReal_Run_SurfacesStderrOnFailure(t *testing.T) {
	_, err := Real{}.Run(context.Background(), "sh", "-c", "echo 'domain not found' >&2; exit 1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "domain not found") {
		t.Errorf("expected error to include stderr output, got: %v", err)
	}
}

func TestReal_Run_NoStderrFallsBackToPlainError(t *testing.T) {
	_, err := Real{}.Run(context.Background(), "sh", "-c", "exit 1")
	if err == nil {
		t.Fatal("expected an error")
	}
	// No stderr output: should still get a usable error (bare exit status),
	// not panic or return nil.
	if !strings.Contains(err.Error(), "exit status") {
		t.Errorf("expected a plain exit-status error, got: %v", err)
	}
}

func TestReal_Run_SuccessReturnsNoError(t *testing.T) {
	out, err := Real{}.Run(context.Background(), "echo", "-n", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "hello" {
		t.Errorf("got %q, want %q", out, "hello")
	}
}
