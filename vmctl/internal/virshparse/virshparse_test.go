package virshparse

import "testing"

func TestDomiflist_NAT(t *testing.T) {
	output := " Interface   Type      Source   Model    MAC\n -------------------------------------------------\n vnet0       network   default  virtio   52:54:00:aa:bb:cc\n"
	mode, iface, ok := Domiflist(output)
	if !ok || mode != "nat" || iface != "" {
		t.Errorf("got (%q, %q, %v), want (nat, \"\", true)", mode, iface, ok)
	}
}

func TestDomiflist_Bridged(t *testing.T) {
	output := " Interface   Type     Source   Model    MAC\n ------------------------------------------------\n macvtap0    direct   eth0     virtio   52:54:00:11:22:33\n"
	mode, iface, ok := Domiflist(output)
	if !ok || mode != "bridged" || iface != "eth0" {
		t.Errorf("got (%q, %q, %v), want (bridged, eth0, true)", mode, iface, ok)
	}
}

func TestDomiflist_NoInterface(t *testing.T) {
	_, _, ok := Domiflist(" Interface   Type   Source   Model   MAC\n")
	if ok {
		t.Error("expected ok=false when no interface line matches")
	}
}

func TestDomifaddrIPv4(t *testing.T) {
	output := " Name    MAC address         Protocol   Address\n---------------------------------------------------\n vnet0   52:54:00:aa:bb:cc   ipv4       192.168.122.50/24\n"
	if got := DomifaddrIPv4(output); got != "192.168.122.50" {
		t.Errorf("got %q, want 192.168.122.50", got)
	}
}

func TestDHCPLeaseIP(t *testing.T) {
	// The expiry column is "<date> <time>" (two space-separated tokens),
	// matching real `virsh net-dhcp-leases` output and bash's `awk '{print $5}'`.
	output := " Expiry Time           MAC address         Protocol   IP address               Hostname\n-------------------------------------------------------------------------------------------\n 2026-01-01 12:00:00   52:54:00:aa:bb:cc   ipv4       192.168.122.50/24        debian-vm\n"
	if got := DHCPLeaseIP(output, "debian-vm"); got != "192.168.122.50" {
		t.Errorf("got %q, want 192.168.122.50", got)
	}
}

func TestDominfo(t *testing.T) {
	output := `Id:             3
Name:           debian-vm
UUID:           00000000-0000-0000-0000-000000000000
OS Type:        hvm
State:          running
CPU(s):         2
CPU time:       12.3s
Max memory:     2097152 KiB
Used memory:    2097152 KiB
Persistent:     yes
Autostart:      disable
`
	state, vcpus, ramMB := Dominfo(output)
	if state != "running" || vcpus != 2 || ramMB != 2048 {
		t.Errorf("got (%q, %d, %d), want (running, 2, 2048)", state, vcpus, ramMB)
	}
}
