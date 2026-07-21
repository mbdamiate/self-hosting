package setup

import (
	"bytes"
	"context"
	"testing"

	"vmctl/internal/execrunner"
)

func TestRun_BridgeAndForwardMutuallyExclusive(t *testing.T) {
	f := execrunner.NewFake()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, Options{BridgeIface: "eth0", ForwardRules: "2222:22"})
	if err == nil {
		t.Fatal("expected an error for --bridge + --forward")
	}
	if len(f.Calls) != 0 {
		t.Errorf("expected no external calls before validation fails, got %+v", f.Calls)
	}
}

func TestRun_BridgeAndIPMutuallyExclusive(t *testing.T) {
	f := execrunner.NewFake()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, Options{BridgeIface: "eth0", StaticIP: "192.168.122.50"})
	if err == nil {
		t.Fatal("expected an error for --bridge + --ip")
	}
	if len(f.Calls) != 0 {
		t.Errorf("expected no external calls before validation fails, got %+v", f.Calls)
	}
}
