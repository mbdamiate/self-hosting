package netxml

import "testing"

const sampleXML = `<network>
  <name>default</name>
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.2' end='192.168.122.254'/>
      <host mac='52:54:00:aa:bb:cc' name='debian-vm' ip='192.168.122.10'/>
      <host mac='52:54:00:11:22:33' name='app-01' ip='192.168.122.11'/>
    </dhcp>
  </ip>
</network>`

func TestFindHostEntryByName(t *testing.T) {
	if got := FindHostEntryByName(sampleXML, "app-01"); Attr(got, "ip") != "192.168.122.11" {
		t.Errorf("FindHostEntryByName(app-01) ip = %q, want 192.168.122.11", Attr(got, "ip"))
	}
	if got := FindHostEntryByName(sampleXML, "no-such-vm"); got != "" {
		t.Errorf("FindHostEntryByName(no-such-vm) = %q, want empty", got)
	}
}

func TestFindReservationOwnerByIP(t *testing.T) {
	if got := FindReservationOwnerByIP(sampleXML, "192.168.122.10"); got != "debian-vm" {
		t.Errorf("FindReservationOwnerByIP(.10) = %q, want debian-vm", got)
	}
	if got := FindReservationOwnerByIP(sampleXML, "192.168.122.99"); got != "" {
		t.Errorf("FindReservationOwnerByIP(.99) = %q, want empty", got)
	}
}

func TestHasMAC(t *testing.T) {
	if !HasMAC(sampleXML, "52:54:00:AA:BB:CC") {
		t.Error("expected case-insensitive MAC match to be found")
	}
	if HasMAC(sampleXML, "52:54:00:ff:ff:ff") {
		t.Error("expected unused MAC to not be found")
	}
}

func TestDHCPRange(t *testing.T) {
	start, end := DHCPRange(sampleXML)
	if start != "192.168.122.2" || end != "192.168.122.254" {
		t.Errorf("DHCPRange = (%q, %q), want (192.168.122.2, 192.168.122.254)", start, end)
	}
}

func TestDHCPRange_Missing(t *testing.T) {
	start, end := DHCPRange(`<network></network>`)
	if start != "" || end != "" {
		t.Errorf("DHCPRange with no range = (%q, %q), want empty", start, end)
	}
}
