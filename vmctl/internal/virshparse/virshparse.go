// Package virshparse parses the handful of virsh text outputs that both
// `setup` and `fleet` need (domiflist, domifaddr, net-dhcp-leases,
// dominfo), shared instead of duplicated across those two packages.
package virshparse

import "strings"

// Domiflist parses `virsh domiflist` output for the effective network mode
// ("nat" or "bridged") and, for bridged mode, the underlying interface.
func Domiflist(output string) (mode, iface string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		switch fields[1] {
		case "direct":
			return "bridged", fields[2], true
		case "network":
			return "nat", "", true
		}
	}
	return "", "", false
}

// DomifaddrIPv4 extracts the first ipv4 address from `virsh domifaddr`
// output.
func DomifaddrIPv4(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "ipv4") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		return stripPrefixLen(fields[3])
	}
	return ""
}

// DHCPLeaseIP finds vmName's leased IP in `virsh net-dhcp-leases default`
// output.
func DHCPLeaseIP(output, vmName string) string {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, vmName) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		return stripPrefixLen(fields[4])
	}
	return ""
}

func stripPrefixLen(addr string) string {
	if idx := strings.Index(addr, "/"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// Dominfo parses `virsh dominfo` output for run state, vCPU count, and RAM
// in MB (converted from the "Max memory: N KiB" field).
func Dominfo(output string) (state string, vcpus int, ramMB int) {
	for _, line := range strings.Split(output, "\n") {
		if v, ok := field(line, "State:"); ok {
			state = v
		}
		if v, ok := field(line, "CPU(s):"); ok {
			vcpus = atoi(v)
		}
		if v, ok := field(line, "Max memory:"); ok {
			parts := strings.Fields(v)
			if len(parts) > 0 {
				ramMB = atoi(parts[0]) / 1024
			}
		}
	}
	return state, vcpus, ramMB
}

func field(line, prefix string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, prefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix)), true
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
