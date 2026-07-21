package metadata

import "testing"

func TestLoad_MissingRecordReturnsZeroValue(t *testing.T) {
	rec, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec != (Record{}) {
		t.Errorf("got %+v, want zero value", rec)
	}
}

func TestSaveThenLoad_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	want := Record{AdminSudoPolicy: "password-required", LogForwarding: true, GuestFirewallPolicy: "enabled"}
	if err := Save(dir, want); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSave_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Record{AdminSudoPolicy: "nopasswd"}); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, Record{AdminSudoPolicy: "password-required"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.AdminSudoPolicy != "password-required" {
		t.Errorf("got %q, want password-required", got.AdminSudoPolicy)
	}
}
