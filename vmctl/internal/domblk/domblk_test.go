package domblk

import "testing"

func TestFindDisk(t *testing.T) {
	output := "Type  Device  Target  Source\nfile  cdrom   sda     /vms/debian-vm/seed.iso\nfile  disk    vda     /vms/debian-vm/debian-vm.qcow2\n"
	target, path := FindDisk(output)
	if target != "vda" || path != "/vms/debian-vm/debian-vm.qcow2" {
		t.Errorf("got (%q, %q), want (vda, /vms/debian-vm/debian-vm.qcow2)", target, path)
	}
}

func TestFindDisk_NoMatch(t *testing.T) {
	target, path := FindDisk("Type  Device  Target  Source\nfile  cdrom   sda     /vms/debian-vm/seed.iso\n")
	if target != "" || path != "" {
		t.Errorf("got (%q, %q), want empty", target, path)
	}
}
