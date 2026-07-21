package setup

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"vmctl/internal/execrunner"
)

const ufwSSHTag = "self-hosting: host SSH baseline"

// hardenHostFirewall mirrors section 4.1: opt-in, host-wide, idempotent,
// independent of which VM is being created/reused.
func hardenHostFirewall(ctx context.Context, r execrunner.Runner, out io.Writer) error {
	fmt.Fprintln(out, "==> Hardening the host firewall (ufw)...")

	if _, err := r.Run(ctx, "sh", "-c", "command -v ufw"); err != nil {
		fmt.Fprintln(out, "==> Installing ufw...")
		if _, err := r.Run(ctx, "sudo", "apt", "install", "-y", "ufw"); err != nil {
			return err
		}
	}

	status, _ := r.Run(ctx, "sudo", "ufw", "status")
	forwardPolicyChanged := false
	if strings.Contains(string(status), ufwSSHTag) {
		fmt.Fprintln(out, "    OK: host SSH baseline rule already present.")
	} else {
		fmt.Fprintln(out, "==> Adding host SSH baseline rule (tcp/22)...")
		if _, err := r.Run(ctx, "sudo", "ufw", "allow", "22/tcp", "comment", ufwSSHTag); err != nil {
			return err
		}
	}

	if contents, err := os.ReadFile("/etc/default/ufw"); err == nil && strings.Contains(string(contents), `DEFAULT_FORWARD_POLICY="DROP"`) {
		fmt.Fprintln(out, "==> Setting DEFAULT_FORWARD_POLICY to ACCEPT (required for libvirt NAT / --forward)...")
		if _, err := r.Run(ctx, "sudo", "sed", "-i",
			`s/^DEFAULT_FORWARD_POLICY="DROP"/DEFAULT_FORWARD_POLICY="ACCEPT"/`, "/etc/default/ufw"); err != nil {
			return err
		}
		forwardPolicyChanged = true
	} else {
		fmt.Fprintln(out, "    OK: DEFAULT_FORWARD_POLICY is already permissive (or already ACCEPT).")
	}

	if strings.Contains(string(status), "Status: active") {
		if forwardPolicyChanged {
			if _, err := r.Run(ctx, "sudo", "ufw", "reload"); err != nil {
				return err
			}
		}
	} else {
		if _, err := r.Run(ctx, "sudo", "ufw", "--force", "enable"); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "    OK: host firewall hardened (ufw active, host SSH allowed, forwarding preserved).")
	return nil
}

var uptimeCheckScript = `#!/usr/bin/env bash
set -uo pipefail
NAME="$1"
STATE_DIR="/run/self-hosting-vm-uptime"
STATE_FILE="${STATE_DIR}/${NAME}.state"
mkdir -p "$STATE_DIR"

DOMSTATE=$(virsh domstate "$NAME" 2>/dev/null || echo "unknown")
STATUS="down"
if [ "$DOMSTATE" = "running" ]; then
    VM_IP=$(virsh domifaddr "$NAME" 2>/dev/null | awk '/ipv4/ {print $4}' | cut -d/ -f1 | head -n1)
    if [ -n "$VM_IP" ] && timeout 3 bash -c "echo > /dev/tcp/${VM_IP}/22" 2>/dev/null; then
        STATUS="up"
    fi
fi

PREV_STATUS=""
if [ -f "$STATE_FILE" ]; then
    PREV_STATUS=$(cat "$STATE_FILE")
fi

if [ -n "$PREV_STATUS" ] && [ "$PREV_STATUS" != "$STATUS" ]; then
    if [ "$STATUS" = "down" ]; then
        MSG="VM '${NAME}' is DOWN (domstate=${DOMSTATE}, SSH unreachable)"
    else
        MSG="VM '${NAME}' has RECOVERED (up)"
    fi
    logger -t self-hosting-alert "$MSG"
    wall "self-hosting-alert: ${MSG}" 2>/dev/null || true
fi

echo "$STATUS" > "$STATE_FILE"
`

var uptimeServiceUnit = `[Unit]
Description=self-hosting uptime check for VM %i
After=libvirtd.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/self-hosting-vm-uptime-check %i
`

var uptimeTimerUnit = `[Unit]
Description=Periodic self-hosting uptime check for VM %i

[Timer]
OnBootSec=1min
OnUnitActiveSec=2min
Unit=self-hosting-vm-uptime@%i.service

[Install]
WantedBy=timers.target
`

var motdScript = `#!/usr/bin/env bash
RECENT=$(journalctl -t self-hosting-alert -n 5 --no-pager --since "-24 hours" 2>/dev/null)
if [ -n "$RECENT" ]; then
    echo ""
    echo "Recent self-hosting alerts:"
    echo "$RECENT"
    echo ""
fi
`

var logrotateConfig = `/var/log/self-hosting-vms/*/*.log {
    weekly
    rotate 8
    compress
    missingok
    notifempty
    copytruncate
}
`

// installMonitoringInfra mirrors section 4.2: opt-in via --monitor,
// host-wide, idempotent. Returns the detected virbr0 IP (used by the
// caller for per-VM log-forwarding configuration), or "" if virbr0 doesn't
// exist yet.
func installMonitoringInfra(ctx context.Context, r execrunner.Runner, out io.Writer) (virbr0IP string, err error) {
	fmt.Fprintln(out, "==> Installing host-wide monitoring infrastructure...")

	fmt.Fprintln(out, "==> Installing the uptime health-check script...")
	if _, err := r.RunWithStdin(ctx, []byte(uptimeCheckScript), "sudo", "tee", "/usr/local/bin/self-hosting-vm-uptime-check"); err != nil {
		return "", err
	}
	if _, err := r.Run(ctx, "sudo", "chmod", "755", "/usr/local/bin/self-hosting-vm-uptime-check"); err != nil {
		return "", err
	}

	fmt.Fprintln(out, "==> Installing the uptime timer/service templates...")
	if _, err := r.RunWithStdin(ctx, []byte(uptimeServiceUnit), "sudo", "tee", "/etc/systemd/system/self-hosting-vm-uptime@.service"); err != nil {
		return "", err
	}
	if _, err := r.RunWithStdin(ctx, []byte(uptimeTimerUnit), "sudo", "tee", "/etc/systemd/system/self-hosting-vm-uptime@.timer"); err != nil {
		return "", err
	}

	fmt.Fprintln(out, "==> Installing the login alert summary (update-motd.d)...")
	if _, err := r.RunWithStdin(ctx, []byte(motdScript), "sudo", "tee", "/etc/update-motd.d/95-self-hosting-alerts"); err != nil {
		return "", err
	}
	if _, err := r.Run(ctx, "sudo", "chmod", "755", "/etc/update-motd.d/95-self-hosting-alerts"); err != nil {
		return "", err
	}

	if _, err := r.Run(ctx, "sudo", "systemctl", "daemon-reload"); err != nil {
		return "", err
	}

	fmt.Fprintln(out, "==> Installing log storage and rotation...")
	if _, err := r.Run(ctx, "sudo", "mkdir", "-p", "/var/log/self-hosting-vms"); err != nil {
		return "", err
	}
	// Debian's default /etc/rsyslog.conf drops rsyslogd's file-writing
	// privileges to the syslog user (via $PrivDropToUser/$PrivDropToGroup),
	// so a root:root 0755 directory blocks it from creating each VM's
	// per-hostname subdirectory ("Permission denied" in rsyslog's own log).
	// Found via real-host testing (2026-07-20).
	if _, err := r.Run(ctx, "sudo", "chown", "syslog:adm", "/var/log/self-hosting-vms"); err != nil {
		return "", err
	}
	if _, err := r.RunWithStdin(ctx, []byte(logrotateConfig), "sudo", "tee", "/etc/logrotate.d/self-hosting-vms"); err != nil {
		return "", err
	}

	ipOut, _ := r.Run(ctx, "sh", "-c", "ip -4 addr show virbr0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1")
	virbr0IP = trimNewline(string(ipOut))
	if virbr0IP != "" {
		fmt.Fprintf(out, "==> Installing the centralized-logging receiver (bound to virbr0: %s)...\n", virbr0IP)
		if _, err := r.Run(ctx, "sh", "-c", "command -v rsyslogd"); err != nil {
			if _, err := r.Run(ctx, "sudo", "apt", "install", "-y", "rsyslog"); err != nil {
				return virbr0IP, err
			}
		}
		// Deliberately NOT setting dirCreateMode/fileCreateMode/dirGroup/
		// fileGroup here: on real-host testing (2026-07-20, rsyslog
		// 8.2512.0), adding ANY of them (with or without an explicit
		// group) made the dynaFile action fail completely and silently —
		// no directory, no file, no error anywhere, despite `rsyslogd -N1`
		// validating the config and the TCP payload provably arriving
		// (confirmed with tcpdump). This plain form (matching the
		// original bash script byte-for-byte) is the one confirmed
		// working end-to-end. It creates each VM's log directory
		// owner-only (0700, syslog:syslog) — reachable via sudo, not
		// directly by an unprivileged admin user. If tightening that is
		// worth revisiting, do it as a follow-up with a real rsyslog
		// instance to iterate against, not blind.
		receiverConf := fmt.Sprintf(`module(load="imtcp")
input(type="imtcp" port="%s" address="%s")

template(name="selfHostingVMPerHost" type="string" string="/var/log/self-hosting-vms/%%HOSTNAME%%/messages.log")
if $inputname == "imtcp" then {
    action(type="omfile" dynaFile="selfHostingVMPerHost" createDirs="on")
    stop
}
`, monitorLogPort, virbr0IP)
		if _, err := r.RunWithStdin(ctx, []byte(receiverConf), "sudo", "tee", "/etc/rsyslog.d/60-self-hosting-vm-receiver.conf"); err != nil {
			return virbr0IP, err
		}
		if _, err := r.Run(ctx, "sudo", "systemctl", "restart", "rsyslog"); err != nil {
			return virbr0IP, err
		}
	} else {
		fmt.Fprintln(out, "    NOTE: virbr0 not found (no NAT-family VM has been set up yet on this host).")
		fmt.Fprintln(out, "          Centralized logging will start working once one is; uptime monitoring")
		fmt.Fprintln(out, "          is unaffected.")
	}
	fmt.Fprintln(out, "    OK: monitoring infrastructure ready.")
	return virbr0IP, nil
}

// ensureLogReceiverFirewallRule mirrors section 4.3: an order-independent
// re-check that runs every invocation regardless of --monitor/
// --harden-host-firewall on THIS run, so the rule self-heals no matter
// which run originally installed the receiver vs. enabled ufw.
func ensureLogReceiverFirewallRule(ctx context.Context, r execrunner.Runner, out io.Writer) {
	if _, err := os.Stat("/etc/rsyslog.d/60-self-hosting-vm-receiver.conf"); err != nil {
		return
	}
	if _, err := r.Run(ctx, "sh", "-c", "command -v ufw"); err != nil {
		return
	}
	status, _ := r.Run(ctx, "sudo", "ufw", "status")
	if !strings.Contains(string(status), "Status: active") {
		return
	}
	if strings.Contains(string(status), monitorLogPort+"/tcp") && strings.Contains(string(status), "virbr0") {
		return
	}
	fmt.Fprintln(out, "==> Ensuring the log receiver's firewall rule is present...")
	_, _ = r.Run(ctx, "sudo", "ufw", "allow", "in", "on", "virbr0", "to", "any", "port", monitorLogPort, "proto", "tcp")
}

// detectVirbr0IP is used on reruns (when installMonitoringInfra isn't
// called) to still know virbr0's IP for the rerun-mismatch warning.
func detectVirbr0IP(ctx context.Context, r execrunner.Runner) string {
	ipOut, _ := r.Run(ctx, "sh", "-c", "ip -4 addr show virbr0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1")
	return trimNewline(string(ipOut))
}
