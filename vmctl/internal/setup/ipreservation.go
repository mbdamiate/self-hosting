package setup

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"vmctl/internal/execrunner"
	"vmctl/internal/netxml"
)

func ipToInt(ip string) uint32 {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return 0
	}
	var result uint32
	for _, p := range parts {
		n, _ := strconv.Atoi(p)
		result = (result << 8) + uint32(n)
	}
	return result
}

func intToIP(n uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d", (n>>24)&255, (n>>16)&255, (n>>8)&255, n&255)
}

func isIPLeased(ctx context.Context, r execrunner.Runner, ip string) bool {
	output, err := r.Run(ctx, "virsh", "net-dhcp-leases", "default")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, ip+"/") {
			return true
		}
	}
	return false
}

func isIPFree(ctx context.Context, r execrunner.Runner, netXML, ip string) bool {
	if netxml.FindReservationOwnerByIP(netXML, ip) != "" {
		return false
	}
	return !isIPLeased(ctx, r, ip)
}

// generateMAC produces a MAC in the 52:54:00 (QEMU/KVM) range libvirt uses
// for auto-assigned NICs, retrying until it doesn't collide with an existing
// reservation on the 'default' network.
func generateMAC(netXML string) string {
	for {
		mac := fmt.Sprintf("52:54:00:%02x:%02x:%02x", randByte(), randByte(), randByte())
		if !netxml.HasMAC(netXML, mac) {
			return mac
		}
	}
}

func randByte() byte {
	n, _ := rand.Int(rand.Reader, big.NewInt(256))
	return byte(n.Int64())
}

// resolveStaticIP validates a user-requested --ip against existing
// reservations/leases.
func resolveStaticIP(ctx context.Context, r execrunner.Runner, netXML, ip string) (string, error) {
	if owner := netxml.FindReservationOwnerByIP(netXML, ip); owner != "" {
		return "", fmt.Errorf("address %s is already reserved for VM '%s'. Pick a different --ip, or remove that VM's reservation first", ip, owner)
	}
	if isIPLeased(ctx, r, ip) {
		return "", fmt.Errorf("address %s has no static reservation but currently has an active DHCP lease. Pick a different --ip, or wait for the lease to clear", ip)
	}
	return ip, nil
}

// autoPickIP finds the first free address in the network's DHCP range.
func autoPickIP(ctx context.Context, r execrunner.Runner, netXML string) (string, error) {
	start, end := netxml.DHCPRange(netXML)
	if start == "" || end == "" {
		return "", fmt.Errorf("could not determine the 'default' network's DHCP range. Inspect with: virsh net-dumpxml default")
	}
	startInt, endInt := ipToInt(start), ipToInt(end)
	for i := startInt; i <= endInt; i++ {
		candidate := intToIP(i)
		if isIPFree(ctx, r, netXML, candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free address found in the 'default' network's DHCP range (%s - %s)", start, end)
}

// clearStaleReservation removes an orphaned reservation for hostname, left
// behind by an earlier run that registered it and then failed before
// creating the VM.
func clearStaleReservation(ctx context.Context, r execrunner.Runner, netXML, hostname string) {
	stale := netxml.FindHostEntryByName(netXML, hostname)
	if stale == "" {
		return
	}
	_, _ = r.Run(ctx, "virsh", "net-update", "default", "delete", "ip-dhcp-host", stale, "--live", "--config")
}

func registerReservation(ctx context.Context, r execrunner.Runner, ip, mac, hostname string) error {
	entry := fmt.Sprintf("<host mac='%s' name='%s' ip='%s'/>", mac, hostname, ip)
	_, err := r.Run(ctx, "virsh", "net-update", "default", "add", "ip-dhcp-host", entry, "--live", "--config")
	return err
}
