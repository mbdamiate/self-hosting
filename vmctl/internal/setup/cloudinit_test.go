package setup

import (
	"strings"
	"testing"
)

func TestGuestFirewallPorts_AlwaysIncludesSSH(t *testing.T) {
	got := guestFirewallPorts("", "")
	if got != "22" {
		t.Errorf("got %q, want %q", got, "22")
	}
}

func TestGuestFirewallPorts_DedupesAndSorts(t *testing.T) {
	// Forward rules are HOST_PORT:VM_PORT; only the VM (guest-facing) port
	// belongs in the guest firewall, matching bash's `${rule##*:}`.
	got := guestFirewallPorts("80,22,443", "2222:2022,8080:8888")
	want := "22,80,443,2022,8888"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestGuestFirewallPorts_SkipsMalformedForwardRule(t *testing.T) {
	got := guestFirewallPorts("", "2222:")
	if got != "22" {
		t.Errorf("got %q, want %q (malformed rule should be skipped)", got, "22")
	}
}

func TestCloudInitBuilder_BaselineAlwaysPresent(t *testing.T) {
	b := newCloudInitBuilder()
	userData, metaData := b.render("app-01", "app-01", "admin", "ALL=(ALL) NOPASSWD:ALL", "", "ssh-ed25519 AAAA...")
	if !strings.Contains(userData, "qemu-guest-agent") {
		t.Error("expected qemu-guest-agent package in baseline cloud-init")
	}
	if !strings.Contains(userData, "hostname: app-01") {
		t.Error("expected hostname in user-data")
	}
	if !strings.Contains(metaData, "local-hostname: app-01") {
		t.Error("expected local-hostname in meta-data")
	}
}

func TestCloudInitBuilder_AccumulatesOptionalFeatures(t *testing.T) {
	b := newCloudInitBuilder()
	b.addUnattendedUpgrades()
	b.addGuestFirewall("22,8080")
	b.addFail2ban()
	b.addLogForwarding("192.168.122.1")
	b.addWatchdogPetting()

	userData, _ := b.render("app-01", "app-01", "admin", "ALL=(ALL) ALL", "    passwd: hash\n    lock_passwd: false", "ssh-ed25519 AAAA...")

	for _, want := range []string{
		"unattended-upgrades",
		"ufw allow 22/tcp",
		"ufw allow 8080/tcp",
		"fail2ban",
		// Debian 12's cloud image doesn't write /var/log/auth.log, so
		// fail2ban's default log-file autodetection fails ("Have not found
		// any log file for sshd jail") unless the systemd/journald backend
		// is set explicitly. Found via real-host testing (2026-07-20).
		"backend = systemd",
		"*.* @@192.168.122.1:5140",
		"RuntimeWatchdogSec=20s",
		"passwd: hash",
	} {
		if !strings.Contains(userData, want) {
			t.Errorf("expected user-data to contain %q, got:\n%s", want, userData)
		}
	}
}
