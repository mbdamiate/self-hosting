// Package fleet implements `vmctl list`/`vmctl info`, per vm-fleet-status:
// a live, aggregated view across every defined VM, querying libvirt fresh on
// every invocation and persisting nothing.
package fleet

import (
	"context"
	"fmt"
	"strings"

	"vmctl/internal/domblk"
	"vmctl/internal/execrunner"
	"vmctl/internal/virshparse"
)

// VMInfo is one VM's live status, as reported by libvirt right now.
type VMInfo struct {
	Name        string
	State       string
	RAMMB       int
	VCPUs       int
	DiskGB      float64
	NetworkMode string // "nat", "bridged", or "" if undetermined
	IP          string // "" if not running or not yet reachable
}

// List enumerates every VM currently defined in libvirt.
func List(ctx context.Context, r execrunner.Runner) ([]VMInfo, error) {
	output, err := r.Run(ctx, "virsh", "list", "--all", "--name")
	if err != nil {
		return nil, fmt.Errorf("could not list VMs. Inspect with: virsh list --all")
	}
	var infos []VMInfo
	for _, name := range strings.Split(string(output), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		info, err := Get(ctx, r, name)
		if err != nil {
			// Isolate one VM's introspection failure from the rest of the
			// listing: still show it, with whatever fields we do have.
			info = VMInfo{Name: name, State: "unknown"}
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// Get returns a single VM's live status.
func Get(ctx context.Context, r execrunner.Runner, name string) (VMInfo, error) {
	dominfoOut, err := r.Run(ctx, "virsh", "dominfo", name)
	if err != nil {
		return VMInfo{}, fmt.Errorf("no VM named '%s' found. Check with: virsh list --all", name)
	}
	state, vcpus, ramMB := virshparse.Dominfo(string(dominfoOut))

	info := VMInfo{Name: name, State: state, RAMMB: ramMB, VCPUs: vcpus}

	if diskGB, err := diskSizeGB(ctx, r, name); err == nil {
		info.DiskGB = diskGB
	}

	if iflistOut, err := r.Run(ctx, "virsh", "domiflist", name); err == nil {
		if mode, iface, ok := virshparse.Domiflist(string(iflistOut)); ok {
			info.NetworkMode = mode
			if mode == "bridged" {
				info.NetworkMode = "bridged (" + iface + ")"
			}
		}
	}

	if state == "running" {
		if addrOut, err := r.Run(ctx, "virsh", "domifaddr", name); err == nil {
			info.IP = virshparse.DomifaddrIPv4(string(addrOut))
		}
		if info.IP == "" {
			if leases, err := r.Run(ctx, "virsh", "net-dhcp-leases", "default"); err == nil {
				info.IP = virshparse.DHCPLeaseIP(string(leases), name)
			}
		}
	}

	return info, nil
}

func diskSizeGB(ctx context.Context, r execrunner.Runner, name string) (float64, error) {
	blkOut, err := r.Run(ctx, "virsh", "domblklist", name, "--details")
	if err != nil {
		return 0, err
	}
	_, path := domblk.FindDisk(string(blkOut))
	if path == "" {
		return 0, fmt.Errorf("no disk found")
	}
	infoOut, err := r.Run(ctx, "qemu-img", "info", "-U", "--output=json", path)
	if err != nil {
		return 0, err
	}
	bytes, ok := parseVirtualSize(string(infoOut))
	if !ok {
		return 0, fmt.Errorf("could not parse qemu-img output")
	}
	return float64(bytes) / (1 << 30), nil
}

// parseVirtualSize extracts "virtual-size": N from `qemu-img info
// --output=json` without a full JSON dependency for one field.
func parseVirtualSize(jsonOutput string) (int64, bool) {
	const key = `"virtual-size":`
	idx := strings.Index(jsonOutput, key)
	if idx == -1 {
		return 0, false
	}
	rest := jsonOutput[idx+len(key):]
	rest = strings.TrimLeft(rest, " ")
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	var n int64
	for _, c := range rest[:end] {
		n = n*10 + int64(c-'0')
	}
	return n, true
}
