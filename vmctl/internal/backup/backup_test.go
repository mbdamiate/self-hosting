package backup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vmctl/internal/execrunner"
)

func newFakeWithVM(name string) *execrunner.Fake {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", name)] = execrunner.Response{}
	return f
}

func TestRun_UnknownSubcommand(t *testing.T) {
	f := execrunner.NewFake()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "bogus", Name: "debian-vm"})
	if err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
}

func TestRun_MissingVMErrorsExceptForBackupList(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{Err: errBoom}
	out := &bytes.Buffer{}

	if err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "snapshot", Name: "debian-vm"}); err == nil {
		t.Error("expected an error for snapshot against a missing VM")
	}

	dir := t.TempDir()
	out.Reset()
	if err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "backup-list", Name: "debian-vm", Dest: dir}); err != nil {
		t.Errorf("backup-list should not require the VM to exist, got error: %v", err)
	}
}

func TestCmdSnapshot_RefusesIfAlreadyExists(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "snapshot-list", "debian-vm", "--name")] = execrunner.Response{
		Stdout: []byte(snapshotName + "\n"),
	}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "snapshot", Name: "debian-vm"})
	if err == nil {
		t.Fatal("expected an error when a snapshot already exists")
	}
}

func TestCmdSnapshot_CreatesWhenNoneExists(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "snapshot", Name: "debian-vm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range f.Calls {
		if c.Name == "virsh" && len(c.Args) > 0 && c.Args[0] == "snapshot-create-as" {
			found = true
		}
	}
	if !found {
		t.Error("expected a snapshot-create-as call")
	}
}

func TestCmdSnapshotRestore_AbortsWithoutConfirmation(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "snapshot-list", "debian-vm", "--name")] = execrunner.Response{
		Stdout: []byte(snapshotName + "\n"),
	}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader("n\n"), Options{Subcommand: "snapshot-restore", Name: "debian-vm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, c := range f.Calls {
		if c.Name == "virsh" && len(c.Args) > 0 && c.Args[0] == "snapshot-revert" {
			t.Error("should not revert without confirmation")
		}
	}
}

func TestCmdSnapshotRestore_RemovesOrphanedOverlayFile(t *testing.T) {
	// Regression test for a real-host finding (2026-07-20): snapshot-revert
	// + snapshot-delete --metadata never deleted the overlay file itself,
	// blocking a future `snapshot` with "external snapshot file ... already
	// exists". The fix captures the overlay path before reverting and rm
	// -f's it afterward.
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "snapshot-list", "debian-vm", "--name")] = execrunner.Response{
		Stdout: []byte(snapshotName + "\n"),
	}
	f.Responses[execrunner.Key("virsh", "domblklist", "debian-vm", "--details")] = execrunner.Response{
		Stdout: []byte("Type  Device  Target  Source\nfile  disk    vda     /vms/debian-vm/debian-vm.self-hosting-snapshot\n"),
	}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader("y\n"), Options{Subcommand: "snapshot-restore", Name: "debian-vm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range f.Calls {
		if c.Name == "rm" && len(c.Args) == 2 && c.Args[0] == "-f" && c.Args[1] == "/vms/debian-vm/debian-vm.self-hosting-snapshot" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the orphaned overlay file to be removed, got calls: %+v", f.Calls)
	}
}

func TestCmdSnapshotDelete_RemovesOrphanedOverlayFile(t *testing.T) {
	// Same root cause as the snapshot-restore fix: blockcommit --pivot
	// stops referencing the overlay but never deletes the file. Found via
	// real-host testing (2026-07-20).
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "snapshot-list", "debian-vm", "--name")] = execrunner.Response{
		Stdout: []byte(snapshotName + "\n"),
	}
	f.Responses[execrunner.Key("virsh", "domblklist", "debian-vm", "--details")] = execrunner.Response{
		Stdout: []byte("Type  Device  Target  Source\nfile  disk    vda     /vms/debian-vm/debian-vm.self-hosting-snapshot\n"),
	}
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "snapshot-delete", Name: "debian-vm"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range f.Calls {
		if c.Name == "rm" && len(c.Args) == 2 && c.Args[0] == "-f" && c.Args[1] == "/vms/debian-vm/debian-vm.self-hosting-snapshot" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the orphaned overlay file to be removed, got calls: %+v", f.Calls)
	}
}

func TestCmdBackup_RunningVM_RemovesOrphanedTempOverlay(t *testing.T) {
	// Same root cause again, this time for the live-backup path's temp
	// snapshot (self-hosting-backup-tmp). Found via real-host testing
	// (2026-07-20): a second `backup` on a running VM failed with
	// "external snapshot file ... already exists" because the first
	// call's temp overlay was never deleted.
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "domstate", "debian-vm")] = execrunner.Response{Stdout: []byte("running\n")}
	f.Sequences[execrunner.Key("virsh", "domblklist", "debian-vm", "--details")] = []execrunner.Response{
		{Stdout: []byte("Type  Device  Target  Source\nfile  disk    vda     /vms/debian-vm/debian-vm.qcow2\n")},
		{Stdout: []byte("Type  Device  Target  Source\nfile  disk    vda     /vms/debian-vm/debian-vm.self-hosting-backup-tmp\n")},
	}
	dir := t.TempDir()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "backup", Name: "debian-vm", Dest: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range f.Calls {
		if c.Name == "rm" && len(c.Args) == 2 && c.Args[0] == "-f" && c.Args[1] == "/vms/debian-vm/debian-vm.self-hosting-backup-tmp" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the orphaned temp overlay file to be removed, got calls: %+v", f.Calls)
	}
}

func TestCmdSnapshotRestore_ErrorsWithoutActiveSnapshot(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader("y\n"), Options{Subcommand: "snapshot-restore", Name: "debian-vm"})
	if err == nil {
		t.Fatal("expected an error when there is no active snapshot")
	}
}

func TestCmdBackup_RefusesWhenRollbackSnapshotActive(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "snapshot-list", "debian-vm", "--name")] = execrunner.Response{
		Stdout: []byte(snapshotName + "\n"),
	}
	dir := t.TempDir()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "backup", Name: "debian-vm", Dest: dir})
	if err == nil {
		t.Fatal("expected an error when a rollback snapshot is active")
	}
}

func TestCmdBackup_StoppedVMCopiesDiskDirectly(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	f.Responses[execrunner.Key("virsh", "domstate", "debian-vm")] = execrunner.Response{Stdout: []byte("shut off\n")}
	f.Responses[execrunner.Key("virsh", "domblklist", "debian-vm", "--details")] = execrunner.Response{
		Stdout: []byte("Type  Device  Target  Source\nfile  disk    vda     /vms/debian-vm/debian-vm.qcow2\nfile  cdrom   sda     /vms/debian-vm/seed.iso\n"),
	}
	dir := t.TempDir()
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "backup", Name: "debian-vm", Dest: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range f.Calls {
		if c.Name == "qemu-img" && len(c.Args) > 0 && c.Args[0] == "convert" {
			found = true
			for _, a := range c.Args {
				if a == "/vms/debian-vm/debian-vm.qcow2" {
					continue
				}
			}
		}
	}
	if !found {
		t.Error("expected a qemu-img convert call")
	}
}

func TestCmdBackupList_NoDestDir(t *testing.T) {
	out := &bytes.Buffer{}
	err := cmdBackupList(out, "debian-vm", filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "No backups found") {
		t.Errorf("expected a 'no backups' message, got %q", out.String())
	}
}

func TestCmdBackupList_ListsMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "debian-vm-20260101-000000.qcow2"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other-vm-20260101-000000.qcow2"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	if err := cmdBackupList(out, "debian-vm", dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "debian-vm-20260101-000000.qcow2") {
		t.Errorf("expected the matching file to be listed, got %q", out.String())
	}
	if strings.Contains(out.String(), "other-vm") {
		t.Errorf("should not list backups for a different VM, got %q", out.String())
	}
}

func TestCmdBackupRestore_RequiresFile(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "backup-restore", Name: "debian-vm"})
	if err == nil {
		t.Fatal("expected an error when --file is missing")
	}
}

func TestCmdBackupRestore_ErrorsWhenFileDoesNotExist(t *testing.T) {
	f := newFakeWithVM("debian-vm")
	out := &bytes.Buffer{}
	err := Run(context.Background(), f, out, strings.NewReader(""), Options{Subcommand: "backup-restore", Name: "debian-vm", File: "/no/such/file.qcow2"})
	if err == nil {
		t.Fatal("expected an error when the backup file does not exist")
	}
}

func TestGetDiskInfo_ParsesDiskDeviceOnly(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domblklist", "debian-vm", "--details")] = execrunner.Response{
		Stdout: []byte("Type  Device  Target  Source\nfile  cdrom   sda     /vms/debian-vm/seed.iso\nfile  disk    vda     /vms/debian-vm/debian-vm.qcow2\n"),
	}
	target, path, err := getDiskInfo(context.Background(), f, "debian-vm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "vda" || path != "/vms/debian-vm/debian-vm.qcow2" {
		t.Errorf("got (%q, %q), want (vda, /vms/debian-vm/debian-vm.qcow2)", target, path)
	}
}

var errBoom = &testErr{"boom"}

type testErr struct{ msg string }

func (e *testErr) Error() string { return e.msg }
