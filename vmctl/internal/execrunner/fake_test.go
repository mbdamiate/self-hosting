package execrunner

import (
	"context"
	"errors"
	"testing"
)

func TestFake_RecordsCallsAndReturnsCannedResponse(t *testing.T) {
	f := NewFake()
	f.Responses[Key("virsh", "domstate", "app-01")] = Response{Stdout: []byte("running\n")}

	out, err := f.Run(context.Background(), "virsh", "domstate", "app-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "running\n" {
		t.Errorf("Stdout = %q, want %q", out, "running\n")
	}
	if len(f.Calls) != 1 || f.Calls[0].Name != "virsh" {
		t.Errorf("Calls = %+v, want one recorded virsh call", f.Calls)
	}
}

func TestFake_UnregisteredCallReturnsZeroValue(t *testing.T) {
	f := NewFake()
	out, err := f.Run(context.Background(), "virsh", "domstate", "unknown-vm")
	if out != nil || err != nil {
		t.Errorf("unregistered call: got (%v, %v), want (nil, nil)", out, err)
	}
}

func TestFake_ReturnsCannedError(t *testing.T) {
	f := NewFake()
	wantErr := errors.New("boom")
	f.Responses[Key("virsh", "dominfo", "missing")] = Response{Err: wantErr}

	_, err := f.Run(context.Background(), "virsh", "dominfo", "missing")
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
}
