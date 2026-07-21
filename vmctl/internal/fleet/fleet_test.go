package fleet

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"vmctl/internal/execrunner"
)

const dominfoRunning = `Id:             3
Name:           debian-vm
State:          running
CPU(s):         2
Max memory:     2097152 KiB
`

const domiflistNAT = " Interface   Type      Source   Model    MAC\n vnet0       network   default  virtio   52:54:00:aa:bb:cc\n"

func TestGet_RunningVM(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{Stdout: []byte(dominfoRunning)}
	f.Responses[execrunner.Key("virsh", "domblklist", "debian-vm", "--details")] = execrunner.Response{
		Stdout: []byte("Type  Device  Target  Source\nfile  disk    vda     /vms/debian-vm/debian-vm.qcow2\n"),
	}
	f.Responses[execrunner.Key("qemu-img", "info", "-U", "--output=json", "/vms/debian-vm/debian-vm.qcow2")] = execrunner.Response{
		Stdout: []byte(`{"virtual-size": 21474836480, "filename": "x"}`),
	}
	f.Responses[execrunner.Key("virsh", "domiflist", "debian-vm")] = execrunner.Response{Stdout: []byte(domiflistNAT)}
	f.Responses[execrunner.Key("virsh", "domifaddr", "debian-vm")] = execrunner.Response{
		Stdout: []byte(" Name    MAC                 Protocol   Address\n vnet0   52:54:00:aa:bb:cc   ipv4       192.168.122.50/24\n"),
	}

	info, err := Get(context.Background(), f, "debian-vm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.State != "running" || info.RAMMB != 2048 || info.VCPUs != 2 {
		t.Errorf("got State=%q RAMMB=%d VCPUs=%d, want running/2048/2", info.State, info.RAMMB, info.VCPUs)
	}
	if info.DiskGB != 20 {
		t.Errorf("got DiskGB=%v, want 20", info.DiskGB)
	}
	if info.NetworkMode != "nat" {
		t.Errorf("got NetworkMode=%q, want nat", info.NetworkMode)
	}
	if info.IP != "192.168.122.50" {
		t.Errorf("got IP=%q, want 192.168.122.50", info.IP)
	}
}

func TestGet_StoppedVMHasNoIP(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{
		Stdout: []byte("State:          shut off\nCPU(s):         2\nMax memory:     2097152 KiB\n"),
	}
	info, err := Get(context.Background(), f, "debian-vm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.State != "shut off" {
		t.Errorf("got State=%q, want 'shut off'", info.State)
	}
	if info.IP != "" {
		t.Errorf("expected no IP lookup attempt for a stopped VM, got %q", info.IP)
	}
	for _, c := range f.Calls {
		if c.Name == "virsh" && len(c.Args) > 0 && c.Args[0] == "domifaddr" {
			t.Error("should not call domifaddr for a stopped VM")
		}
	}
}

func TestGet_MissingVMReturnsError(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dominfo", "no-such-vm")] = execrunner.Response{Err: errBoom}
	_, err := Get(context.Background(), f, "no-such-vm")
	if err == nil {
		t.Fatal("expected an error for a missing VM")
	}
}

func TestList_EmptyFleet(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "list", "--all", "--name")] = execrunner.Response{Stdout: []byte("\n")}
	infos, err := List(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Errorf("got %d VMs, want 0", len(infos))
	}
}

func TestList_IsolatesOneVMsFailureFromTheRest(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "list", "--all", "--name")] = execrunner.Response{Stdout: []byte("debian-vm\nbroken-vm\n")}
	f.Responses[execrunner.Key("virsh", "dominfo", "debian-vm")] = execrunner.Response{Stdout: []byte(dominfoRunning)}
	f.Responses[execrunner.Key("virsh", "dominfo", "broken-vm")] = execrunner.Response{Err: errBoom}

	infos, err := List(context.Background(), f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("got %d VMs, want 2", len(infos))
	}
	if infos[1].Name != "broken-vm" || infos[1].State != "unknown" {
		t.Errorf("got %+v, want broken-vm with state=unknown", infos[1])
	}
}

func TestRenderList_EmptyFleetPrintsMessage(t *testing.T) {
	out := &bytes.Buffer{}
	RenderList(out, nil)
	if !strings.Contains(out.String(), "No VMs found") {
		t.Errorf("expected a 'no VMs' message, got %q", out.String())
	}
}

func TestRenderList_PrintsAllRows(t *testing.T) {
	out := &bytes.Buffer{}
	RenderList(out, []VMInfo{
		{Name: "debian-vm", State: "running", RAMMB: 2048, VCPUs: 2, DiskGB: 20, NetworkMode: "nat", IP: "192.168.122.10"},
		{Name: "app-01", State: "shut off"},
	})
	got := out.String()
	if !strings.Contains(got, "debian-vm") || !strings.Contains(got, "app-01") {
		t.Errorf("expected both VMs listed, got:\n%s", got)
	}
	if !strings.Contains(got, "192.168.122.10") {
		t.Errorf("expected IP in output, got:\n%s", got)
	}
}

func TestParseVirtualSize(t *testing.T) {
	n, ok := parseVirtualSize(`{"virtual-size": 21474836480, "filename": "x"}`)
	if !ok || n != 21474836480 {
		t.Errorf("got (%d, %v), want (21474836480, true)", n, ok)
	}
}

func TestParseVirtualSize_Missing(t *testing.T) {
	_, ok := parseVirtualSize(`{"filename": "x"}`)
	if ok {
		t.Error("expected ok=false when virtual-size is absent")
	}
}

var errBoom = &testError{"boom"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
