// Package setup implements `vmctl setup`, porting debian-vm-setup.sh per the
// vm-fleet-provisioning and vm-setup-rerun-recovery specs. All virsh/
// virt-install/qemu-img/cloud-localds/apt/systemctl/ufw/iptables calls go
// through execrunner.Runner.
package setup

// Options mirrors the flags debian-vm-setup.sh accepts.
type Options struct {
	Name                   string
	RAMMB                  int
	VCPUs                  int
	DiskGB                 int
	StaticIP               string
	BridgeIface            string
	ForwardRules           string
	AdminPasswordRequested bool
	AdminPasswordValue     string
	NoAutoUpdates          bool
	AllowPorts             string
	NoGuestFirewall        bool
	HardenHostFirewall     bool
	Monitor                bool
	Watchdog               bool
	NoCrashRestart         bool
}

const (
	DefaultRAMMB   = 2048
	DefaultVCPUs   = 2
	DefaultDiskGB  = 20
	vmUser         = "admin"
	cloudImgURL    = "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2"
	monitorLogPort = "5140"
)
