package setup

import (
	"context"
	"testing"

	"vmctl/internal/execrunner"
)

func TestDetermineEffectiveNetworkMode_NAT(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domiflist", "debian-vm")] = execrunner.Response{
		Stdout: []byte(" Interface   Type      Source   Model    MAC\n -------------------------------------------------\n vnet0       network   default  virtio   52:54:00:aa:bb:cc\n"),
	}
	mode, iface, err := determineEffectiveNetworkMode(context.Background(), f, "debian-vm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "nat" || iface != "" {
		t.Errorf("got (%q, %q), want (nat, \"\")", mode, iface)
	}
}

func TestDetermineEffectiveNetworkMode_Bridged(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domiflist", "app-01")] = execrunner.Response{
		Stdout: []byte(" Interface   Type     Source   Model    MAC\n ------------------------------------------------\n macvtap0    direct   eth0     virtio   52:54:00:11:22:33\n"),
	}
	mode, iface, err := determineEffectiveNetworkMode(context.Background(), f, "app-01")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != "bridged" || iface != "eth0" {
		t.Errorf("got (%q, %q), want (bridged, eth0)", mode, iface)
	}
}

func TestDetermineEffectiveNetworkMode_NoInterface(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domiflist", "debian-vm")] = execrunner.Response{
		Stdout: []byte(" Interface   Type   Source   Model   MAC\n"),
	}
	_, _, err := determineEffectiveNetworkMode(context.Background(), f, "debian-vm")
	if err == nil {
		t.Fatal("expected an error when no interface is reported")
	}
}

func TestDetermineEffectiveWatchdog(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dumpxml", "debian-vm")] = execrunner.Response{
		Stdout: []byte("<domain><devices><watchdog model='i6300esb' action='reset'/></devices></domain>"),
	}
	if !determineEffectiveWatchdog(context.Background(), f, "debian-vm") {
		t.Error("expected watchdog to be detected")
	}

	f2 := execrunner.NewFake()
	f2.Responses[execrunner.Key("virsh", "dumpxml", "debian-vm")] = execrunner.Response{
		Stdout: []byte("<domain><devices></devices></domain>"),
	}
	if determineEffectiveWatchdog(context.Background(), f2, "debian-vm") {
		t.Error("expected no watchdog to be detected")
	}
}

func TestDetermineEffectiveWatchdog_IgnoresAutoAttachedItco(t *testing.T) {
	// virt-install's debian12 os-variant defaults auto-attach an 'itco'
	// watchdog to every VM regardless of --watchdog; only 'i6300esb' (the
	// model --watchdog actually requests) should count as "effective".
	// Found via real-host testing (2026-07-20).
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dumpxml", "debian-vm")] = execrunner.Response{
		Stdout: []byte("<domain><devices><watchdog model='itco' action='reset'><alias name='watchdog0'/></watchdog></devices></domain>"),
	}
	if determineEffectiveWatchdog(context.Background(), f, "debian-vm") {
		t.Error("expected the auto-attached itco watchdog to not count as an effective --watchdog")
	}
}

func TestDetermineEffectiveCrashRestart(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "dumpxml", "debian-vm")] = execrunner.Response{
		Stdout: []byte("<domain><on_crash>restart</on_crash></domain>"),
	}
	if !determineEffectiveCrashRestart(context.Background(), f, "debian-vm") {
		t.Error("expected on_crash=restart to be detected")
	}
}

func TestDetermineEffectiveConfig_MissingMetadataIsUnconfigured(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "domiflist", "debian-vm")] = execrunner.Response{
		Stdout: []byte(" Interface   Type      Source   Model    MAC\n vnet0       network   default  virtio   52:54:00:aa:bb:cc\n"),
	}
	eff, err := determineEffectiveConfig(context.Background(), f, "debian-vm", t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eff.AdminSudo != "" || eff.LogForward {
		t.Errorf("expected fully-unconfigured metadata for a VM with no record, got %+v", eff)
	}
}
