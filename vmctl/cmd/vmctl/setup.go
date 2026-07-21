package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"vmctl/internal/execrunner"
	"vmctl/internal/setup"
)

// adminPasswordFlag lets --admin-password behave like the bash script's
// --admin-password[=PASSWORD]: valid bare (random password generated) or
// with an explicit value. flag.FlagSet only allows a bare "-flag" (no
// following value) for flags whose Value implements IsBoolFlag() bool.
type adminPasswordFlag struct {
	requested bool
	value     string
}

func (f *adminPasswordFlag) String() string {
	if f == nil {
		return ""
	}
	return f.value
}

func (f *adminPasswordFlag) Set(s string) error {
	f.requested = true
	// A bare "--admin-password" is delivered here as Set("true") by the
	// flag package's boolFlag handling; only treat it as an explicit value
	// when an actual "--admin-password=X" was given.
	if s != "true" {
		f.value = s
	}
	return nil
}

func (f *adminPasswordFlag) IsBoolFlag() bool { return true }

func setupUsage(fs *flag.FlagSet) func() {
	return func() {
		fmt.Fprintln(os.Stdout, "Usage: vmctl setup [options]")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Options:")
		fs.PrintDefaults()
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "If neither --bridge nor --forward is given, the VM only gets a NAT IP")
		fmt.Fprintln(os.Stdout, "reachable from the host itself.")
	}
}

func runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	name := fs.String("name", "", "Name/hostname for the VM (default: debian-vm)")
	ram := fs.Int("ram", setup.DefaultRAMMB, "RAM in MB")
	vcpus := fs.Int("vcpus", setup.DefaultVCPUs, "vCPU count")
	disk := fs.Int("disk", setup.DefaultDiskGB, "Disk size in GB")
	ip := fs.String("ip", "", "Reserve a stable IP/hostname on the 'default' NAT network")
	bridge := fs.String("bridge", "", "Use bridged networking (macvtap) over the given physical interface")
	forward := fs.String("forward", "", "NAT + port forwarding, comma-separated HOST_PORT:VM_PORT pairs")
	adminPassword := &adminPasswordFlag{}
	fs.Var(adminPassword, "admin-password", "Require a password for sudo. Without a value, a random password is generated")
	noAutoUpdates := fs.Bool("no-auto-updates", false, "Disable automatic security updates inside the VM")
	allowPort := fs.String("allow-port", "", "Open additional guest-side TCP ports (comma-separated)")
	noGuestFirewall := fs.Bool("no-guest-firewall", false, "Skip installing/enabling ufw inside the VM")
	hardenHostFirewall := fs.Bool("harden-host-firewall", false, "Install and enable ufw on the HOST")
	monitor := fs.Bool("monitor", false, "Enable host-side uptime monitoring, local alerting, and log forwarding")
	watchdog := fs.Bool("watchdog", false, "Attach a virtual watchdog device and enable guest-side petting")
	noCrashRestart := fs.Bool("no-crash-restart", false, "Disable automatic restart when the QEMU process crashes")
	fs.Usage = setupUsage(fs)

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	opts := setup.Options{
		Name:                   *name,
		RAMMB:                  *ram,
		VCPUs:                  *vcpus,
		DiskGB:                 *disk,
		StaticIP:               *ip,
		BridgeIface:            *bridge,
		ForwardRules:           *forward,
		AdminPasswordRequested: adminPassword.requested,
		AdminPasswordValue:     adminPassword.value,
		NoAutoUpdates:          *noAutoUpdates,
		AllowPorts:             *allowPort,
		NoGuestFirewall:        *noGuestFirewall,
		HardenHostFirewall:     *hardenHostFirewall,
		Monitor:                *monitor,
		Watchdog:               *watchdog,
		NoCrashRestart:         *noCrashRestart,
	}

	return setup.Run(context.Background(), execrunner.Real{}, os.Stdout, opts)
}
