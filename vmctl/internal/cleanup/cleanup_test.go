package cleanup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vmctl/internal/execrunner"
	"vmctl/internal/metadata"
	"vmctl/internal/netxml"
)

func TestRun_VMOnlyAndPurgeAllMutuallyExclusive(t *testing.T) {
	f := execrunner.NewFake()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{VMOnly: true, PurgeAll: true}, Tools{})
	if err == nil {
		t.Fatal("expected an error for mutually exclusive flags, got nil")
	}
}

func TestRun_PurgeAllRefusesWhenOtherVMsExist(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "list", "--all", "--name")] = execrunner.Response{
		Stdout: []byte("debian-vm\napp-02\n"),
	}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Name: "debian-vm", PurgeAll: true}, Tools{Virsh: true})
	if err == nil {
		t.Fatal("expected an error when other VMs exist under --purge-all, got nil")
	}
	if !strings.Contains(out.String(), "app-02") {
		t.Errorf("expected output to name the other VM, got %q", out.String())
	}
}

func TestRun_PurgeAllProceedsWhenNoOtherVMsExist(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "list", "--all", "--name")] = execrunner.Response{
		Stdout: []byte("debian-vm\n"),
	}
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Name: "debian-vm", PurgeAll: true}, Tools{Virsh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_VMOnlySkipsHostLevelSteps(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Name: "debian-vm", VMOnly: true}, Tools{Virsh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, call := range f.Calls {
		if call.Name == "sudo" && len(call.Args) > 0 && call.Args[0] == "apt" {
			t.Errorf("--vm-only should not remove packages, but got call: %+v", call)
		}
	}
}

func TestRun_MissingVMSkipsRemovalWithoutError(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{
		Err: errNotFound,
	}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Name: "debian-vm", VMOnly: true}, Tools{Virsh: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No VM named 'debian-vm' found") {
		t.Errorf("expected a 'not found' message, got %q", out.String())
	}
}

func TestRun_VMOnlyPreservesMetadataRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workDir := filepath.Join(home, "vms", "debian-vm")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(workDir, metadata.Record{AdminSudoPolicy: "nopasswd"}); err != nil {
		t.Fatal(err)
	}

	f := execrunner.NewFake()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Name: "debian-vm", VMOnly: true}, Tools{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, "meta.json")); err != nil {
		t.Errorf("expected metadata record to survive --vm-only, got: %v", err)
	}
}

func TestRun_FullCleanupRemovesMetadataRecordWithWorkDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	workDir := filepath.Join(home, "vms", "debian-vm")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := metadata.Save(workDir, metadata.Record{AdminSudoPolicy: "nopasswd"}); err != nil {
		t.Fatal(err)
	}

	f := execrunner.NewFake()
	out := &bytes.Buffer{}
	// PurgeAll auto-approves every confirmation, including work-dir removal.
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Name: "debian-vm", PurgeAll: true}, Tools{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("expected WORK_DIR (and its metadata record) to be removed under --purge-all, stat err: %v", err)
	}
}

func TestFindHostEntryByName(t *testing.T) {
	xml := `<network><ip><dhcp><host mac='52:54:00:aa:bb:cc' name='debian-vm' ip='192.168.122.10'/><host mac='52:54:00:11:22:33' name='app-01' ip='192.168.122.11'/></dhcp></ip></network>`
	got := netxml.FindHostEntryByName(xml, "app-01")
	if !strings.Contains(got, "name='app-01'") {
		t.Errorf("FindHostEntryByName = %q, want an entry for app-01", got)
	}
	if netxml.FindHostEntryByName(xml, "no-such-vm") != "" {
		t.Error("expected empty result for a name with no reservation")
	}
}

func TestFindNumberedRuleContaining(t *testing.T) {
	status := `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere                   # self-hosting: host SSH baseline
[ 2] 80/tcp                     ALLOW IN    Anywhere`
	got := findNumberedRuleContaining(status, ufwSSHTag)
	if got != "1" {
		t.Errorf("findNumberedRuleContaining = %q, want %q", got, "1")
	}
}

var errNotFound = &fakeErr{"not found"}

type fakeErr struct{ msg string }

func (e *fakeErr) Error() string { return e.msg }
