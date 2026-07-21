package setup

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"vmctl/internal/execrunner"
)

func TestPrintRerunMismatchWarnings_BridgeMismatch(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domstate", "debian-vm")] = execrunner.Response{Stdout: []byte("running\n")}
	out := &bytes.Buffer{}
	err := printRerunMismatchWarnings(context.Background(), f, out, "debian-vm",
		Options{BridgeIface: "eth0"},
		effectiveConfig{NetworkMode: "nat"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "already exists using NAT networking") {
		t.Errorf("expected a network-mode mismatch warning, got:\n%s", out.String())
	}
}

func TestPrintRerunMismatchWarnings_NoMismatchNoWarning(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domstate", "debian-vm")] = execrunner.Response{Stdout: []byte("running\n")}
	out := &bytes.Buffer{}
	err := printRerunMismatchWarnings(context.Background(), f, out, "debian-vm",
		Options{NoCrashRestart: false},
		effectiveConfig{NetworkMode: "nat", CrashRestart: true, AdminSudo: "nopasswd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out.String(), "WARNING") {
		t.Errorf("expected no warnings when requested matches effective, got:\n%s", out.String())
	}
}

func TestPrintRerunMismatchWarnings_RunningWithExtraWhitespaceIsNotRestarted(t *testing.T) {
	// Real `virsh domstate` output observed on a real host carried extra
	// whitespace/line-ending bytes beyond a bare "running\n" — an exact
	// string comparison misclassified an already-running VM as stopped and
	// then failed calling `virsh start` on it (libvirt refuses to "start"
	// an already-running domain). Found via real-host testing (2026-07-20).
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domstate", "debian-vm")] = execrunner.Response{Stdout: []byte("running\r\n")}
	out := &bytes.Buffer{}
	err := printRerunMismatchWarnings(context.Background(), f, out, "debian-vm",
		Options{}, effectiveConfig{NetworkMode: "nat", CrashRestart: true, AdminSudo: "nopasswd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range f.Calls {
		if c.Name == "virsh" && len(c.Args) > 0 && c.Args[0] == "start" {
			t.Error("should not call virsh start on an already-running VM")
		}
	}
	if !strings.Contains(out.String(), "already running") {
		t.Errorf("expected 'already running' message, got:\n%s", out.String())
	}
}

func TestPrintRerunMismatchWarnings_StartsStoppedVM(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domstate", "debian-vm")] = execrunner.Response{Stdout: []byte("shut off\n")}
	out := &bytes.Buffer{}
	err := printRerunMismatchWarnings(context.Background(), f, out, "debian-vm",
		Options{},
		effectiveConfig{NetworkMode: "nat", CrashRestart: true, AdminSudo: "nopasswd"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range f.Calls {
		if c.Name == "virsh" && len(c.Args) > 0 && c.Args[0] == "start" {
			found = true
		}
	}
	if !found {
		t.Error("expected a stopped VM to be started")
	}
}
