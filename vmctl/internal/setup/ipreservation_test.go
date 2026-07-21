package setup

import (
	"context"
	"testing"

	"vmctl/internal/execrunner"
)

func TestIPToIntAndBack(t *testing.T) {
	cases := []string{"192.168.122.1", "192.168.122.254", "10.0.0.0", "255.255.255.255"}
	for _, ip := range cases {
		if got := intToIP(ipToInt(ip)); got != ip {
			t.Errorf("round-trip %s -> %s, want %s", ip, got, ip)
		}
	}
}

func TestIsIPLeased(t *testing.T) {
	f := execrunner.NewFake()
	f.Responses[execrunner.Key("virsh", "net-dhcp-leases", "default")] = execrunner.Response{
		Stdout: []byte(" Expiry Time   MAC address   Protocol  IP address           Hostname\n -------------------------------------------------------------------\n 2026-01-01   52:54:00:aa:bb:cc  ipv4      192.168.122.50/24     app-01\n"),
	}
	if !isIPLeased(context.Background(), f, "192.168.122.50") {
		t.Error("expected 192.168.122.50 to be detected as leased")
	}
	if isIPLeased(context.Background(), f, "192.168.122.99") {
		t.Error("expected 192.168.122.99 to not be leased")
	}
}

func TestResolveStaticIP_AlreadyReserved(t *testing.T) {
	xml := `<host mac='52:54:00:aa:bb:cc' name='other-vm' ip='192.168.122.10'/>`
	f := execrunner.NewFake()
	_, err := resolveStaticIP(context.Background(), f, xml, "192.168.122.10")
	if err == nil {
		t.Fatal("expected an error for an already-reserved IP")
	}
}

func TestResolveStaticIP_Free(t *testing.T) {
	f := execrunner.NewFake()
	ip, err := resolveStaticIP(context.Background(), f, "", "192.168.122.50")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "192.168.122.50" {
		t.Errorf("got %q, want 192.168.122.50", ip)
	}
}

func TestAutoPickIP_FindsFirstFree(t *testing.T) {
	xml := `<range start='192.168.122.2' end='192.168.122.4'/><host mac='52:54:00:aa:bb:cc' name='taken' ip='192.168.122.2'/>`
	f := execrunner.NewFake()
	ip, err := autoPickIP(context.Background(), f, xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ip != "192.168.122.3" {
		t.Errorf("got %q, want 192.168.122.3 (first free after .2 is taken)", ip)
	}
}

func TestAutoPickIP_NoRangeFound(t *testing.T) {
	f := execrunner.NewFake()
	_, err := autoPickIP(context.Background(), f, `<network></network>`)
	if err == nil {
		t.Fatal("expected an error when no DHCP range is found")
	}
}

func TestGenerateMAC_AvoidsCollision(t *testing.T) {
	// Sanity check: generateMAC should return a well-formed 52:54:00 MAC and
	// not loop forever against an empty (no-collision) network XML.
	mac := generateMAC("")
	if len(mac) != 17 || mac[:9] != "52:54:00:" {
		t.Errorf("generateMAC() = %q, want a 52:54:00:xx:xx:xx MAC", mac)
	}
}
