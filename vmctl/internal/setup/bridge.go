package setup

import (
	"context"
	"fmt"
	"os"

	"vmctl/internal/execrunner"
)

// validateBridgeIface mirrors section 0's --bridge checks: the interface
// must exist and must not be a Wi-Fi interface (macvtap cannot work over
// Wi-Fi on virtually any hardware).
func validateBridgeIface(ctx context.Context, r execrunner.Runner, iface string) error {
	if _, err := r.Run(ctx, "ip", "link", "show", iface); err != nil {
		return fmt.Errorf("interface '%s' not found. Run 'ip link' to see available interfaces", iface)
	}

	isWireless := false
	if _, err := os.Stat("/sys/class/net/" + iface + "/wireless"); err == nil {
		isWireless = true
	} else if _, err := r.Run(ctx, "iw", "dev", iface, "info"); err == nil {
		isWireless = true
	}
	if isWireless {
		return fmt.Errorf(`'%s' is a Wi-Fi interface. Bridged/macvtap networking does not
       work over Wi-Fi on virtually any hardware, because the wireless chipset
       only allows one MAC address per association with the access point.
       Use --forward=HOST_PORT:VM_PORT,... instead to expose services from the VM
       to your LAN over the default NAT network`, iface)
	}
	return nil
}
