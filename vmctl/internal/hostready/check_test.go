package hostready

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vmctl/internal/execrunner"
)

// withOnlyBinaries points PATH at a fresh directory containing only the
// given (empty, executable) binary names, so exec.LookPath results are
// deterministic regardless of what's actually installed on the machine
// running the tests.
func withOnlyBinaries(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("writing stub binary %q: %v", name, err)
		}
	}
	t.Setenv("PATH", dir)
}

func findResult(t *testing.T, results []CheckResult, name string) CheckResult {
	t.Helper()
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("no CheckResult named %q found among %d results", name, len(results))
	return CheckResult{}
}

func TestCheck_FormerDpkgPackagesVerifiedByBinaryOnPath(t *testing.T) {
	withOnlyBinaries(t, "qemu-system-x86_64", "brctl", "genisoimage")

	results := Check(context.Background(), execrunner.NewFake())

	for _, name := range []string{"qemu-system-x86", "bridge-utils", "genisoimage"} {
		res := findResult(t, results, name)
		if !res.OK {
			t.Errorf("%s: expected OK when its binary is on PATH, got Detail=%q", name, res.Detail)
		}
	}
}

func TestCheck_FormerDpkgPackagesMissingWhenBinaryAbsent(t *testing.T) {
	withOnlyBinaries(t) // empty PATH

	results := Check(context.Background(), execrunner.NewFake())

	cases := map[string]string{
		"qemu-system-x86": "qemu-system-x86_64",
		"bridge-utils":    "brctl",
		"genisoimage":     "genisoimage",
	}
	for name, binary := range cases {
		res := findResult(t, results, name)
		if res.OK {
			t.Errorf("%s: expected MISSING when %q is not on PATH", name, binary)
		}
		if !strings.Contains(res.Detail, binary) || !strings.Contains(res.Detail, "not found on PATH") {
			t.Errorf("%s: Detail %q does not name the missing binary", name, res.Detail)
		}
	}
}

func TestCheck_NoAptBasedHostResult(t *testing.T) {
	results := Check(context.Background(), execrunner.NewFake())

	for _, res := range results {
		if res.Name == "apt-based host" {
			t.Fatalf("Check() must not report an apt-based-host result; the report is distro-agnostic now")
		}
	}
}

func TestCheckApt_ErrorPointsToDoctorReportWhenAptMissing(t *testing.T) {
	withOnlyBinaries(t) // empty PATH, no apt

	err := checkApt()
	if err == nil {
		t.Fatal("expected an error when apt is not on PATH")
	}
	if !strings.Contains(err.Error(), "vmctl doctor") {
		t.Errorf("checkApt() error does not point to 'vmctl doctor': %q", err.Error())
	}
	if strings.Contains(err.Error(), "Adapt it for your distro") {
		t.Errorf("checkApt() still returns the old dead-end message: %q", err.Error())
	}
}
