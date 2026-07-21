// Package domblk parses `virsh domblklist --details` output, shared by
// `backup` and `fleet` (both need to find a VM's active disk device as
// opposed to the cdrom device used for the cloud-init seed ISO).
package domblk

import "strings"

// FindDisk returns the target device and source path of the "disk" device
// (Type=file, Device=disk) in `virsh domblklist --details` output. Columns
// are: Type Device Target Source.
func FindDisk(output string) (target, path string) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 4 && fields[0] == "file" && fields[1] == "disk" {
			return fields[2], fields[3]
		}
	}
	return "", ""
}
