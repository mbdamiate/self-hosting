package fleet

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// RenderList writes the `vmctl list` table. Per vm-fleet-status, an empty
// fleet prints a message instead of an empty table.
func RenderList(w io.Writer, infos []VMInfo) {
	if len(infos) == 0 {
		fmt.Fprintln(w, "No VMs found. Create one with: vmctl create")
		return
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATE\tRAM\tVCPUS\tDISK\tMODE\tIP")
	for _, info := range infos {
		fmt.Fprintln(tw, formatRow(info))
	}
	_ = tw.Flush()
}

// RenderStatus writes the `vmctl info` single-VM detail view.
func RenderStatus(w io.Writer, info VMInfo) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSTATE\tRAM\tVCPUS\tDISK\tMODE\tIP")
	fmt.Fprintln(tw, formatRow(info))
	_ = tw.Flush()
}

func formatRow(info VMInfo) string {
	disk := "-"
	if info.DiskGB > 0 {
		disk = fmt.Sprintf("%.0fG", info.DiskGB)
	}
	mode := info.NetworkMode
	if mode == "" {
		mode = "-"
	}
	ip := info.IP
	if ip == "" {
		ip = "-"
	}
	ram := "-"
	if info.RAMMB > 0 {
		ram = fmt.Sprintf("%d", info.RAMMB)
	}
	vcpus := "-"
	if info.VCPUs > 0 {
		vcpus = fmt.Sprintf("%d", info.VCPUs)
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", info.Name, info.State, ram, vcpus, disk, mode, ip)
}
