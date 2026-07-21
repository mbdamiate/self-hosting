package setup

import (
	"context"
	"strings"

	"vmctl/internal/execrunner"
	"vmctl/internal/metadata"
	"vmctl/internal/virshparse"
)

// effectiveConfig is what setup determines by inspecting libvirt/the VM's
// recorded facts, per vm-setup-rerun-recovery: never trust the current
// invocation's flags over what's actually there.
type effectiveConfig struct {
	NetworkMode  string // "nat" or "bridged"
	BridgeIface  string // set only when NetworkMode == "bridged"
	AdminSudo    string // "", "nopasswd", or "password-required"
	Watchdog     bool
	CrashRestart bool
	LogForward   bool
}

// determineEffectiveNetworkMode mirrors section 10: inspects
// `virsh domiflist`, not the flags passed on this invocation.
func determineEffectiveNetworkMode(ctx context.Context, r execrunner.Runner, vmName string) (mode, iface string, err error) {
	output, runErr := r.Run(ctx, "virsh", "domiflist", vmName)
	if runErr != nil {
		return "", "", runErr
	}
	mode, iface, ok := virshparse.Domiflist(string(output))
	if !ok {
		return "", "", errNoInterface
	}
	return mode, iface, nil
}

var errNoInterface = &introspectionError{"could not determine the network interface"}

type introspectionError struct{ msg string }

func (e *introspectionError) Error() string { return e.msg }

func determineEffectiveWatchdog(ctx context.Context, r execrunner.Runner, vmName string) bool {
	output, err := r.Run(ctx, "virsh", "dumpxml", vmName)
	if err != nil {
		return false
	}
	// Match the specific model --watchdog attaches (i6300esb), not any
	// <watchdog> element: virt-install's debian12 os-variant defaults
	// auto-attach an unrelated 'itco' watchdog to every VM regardless of
	// --watchdog, which a generic substring match would misdetect as the
	// opt-in device. Found via real-host testing (2026-07-20).
	return strings.Contains(string(output), "<watchdog") && strings.Contains(string(output), "model='i6300esb'")
}

func determineEffectiveCrashRestart(ctx context.Context, r execrunner.Runner, vmName string) bool {
	output, err := r.Run(ctx, "virsh", "dumpxml", vmName)
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "<on_crash>restart</on_crash>")
}

// determineEffectiveConfig reads both the libvirt-owned facts (network
// mode, watchdog, on_crash) and the consolidated vm-tooling-metadata
// record for facts libvirt can't report (admin sudo policy, log
// forwarding). A missing metadata record is treated as fully unconfigured.
func determineEffectiveConfig(ctx context.Context, r execrunner.Runner, vmName, workDir string) (effectiveConfig, error) {
	mode, iface, err := determineEffectiveNetworkMode(ctx, r, vmName)
	if err != nil {
		return effectiveConfig{}, err
	}
	meta, err := metadata.Load(workDir)
	if err != nil {
		return effectiveConfig{}, err
	}
	return effectiveConfig{
		NetworkMode:  mode,
		BridgeIface:  iface,
		AdminSudo:    meta.AdminSudoPolicy,
		Watchdog:     determineEffectiveWatchdog(ctx, r, vmName),
		CrashRestart: determineEffectiveCrashRestart(ctx, r, vmName),
		LogForward:   meta.LogForwarding,
	}, nil
}
