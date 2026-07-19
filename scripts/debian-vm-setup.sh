#!/usr/bin/env bash
#
# debian-vm-setup.sh
# Checks prerequisites, installs KVM/QEMU/libvirt, and creates a Debian 12 VM
# using the official cloud image + cloud-init, simulating a real VPS
# (the same technique used by providers like DigitalOcean/Linode).
#
# Usage:
#   chmod +x debian-vm-setup.sh
#   ./debian-vm-setup.sh                              # NAT networking (default, simplest)
#   ./debian-vm-setup.sh --bridge=eth0                # bridged networking (macvtap) over interface eth0
#   ./debian-vm-setup.sh --forward=2222:22,8080:80    # NAT + port forwarding from the host
#   ./debian-vm-setup.sh --name=app-01 --ip=192.168.122.50   # fleet VM with a stable name/IP
#   ./debian-vm-setup.sh --ram=4096 --vcpus=4 --disk=40      # custom sizing
#   ./debian-vm-setup.sh --help
#
# Fleet flags (for creating multiple independently-addressable VMs):
#   --name=NAME   Name/hostname for this VM (default: debian-vm). Each fleet
#                 VM needs its own --name.
#   --ram=MB      RAM in MB (default: 2048).
#   --vcpus=N     vCPU count (default: 2).
#   --disk=GB     Disk size in GB (default: 20).
#   --ip=ADDRESS  Reserve a stable IP + resolvable hostname on the 'default'
#                 NAT network (plain NAT or --forward only; usage error with
#                 --bridge). Rejected if already reserved/leased. Omit to
#                 auto-pick the first free address in the network's DHCP
#                 range. Other fleet VMs can then reach this one by hostname.
#
# --bridge mode (requires a WIRED interface):
#   Uses macvtap in bridge mode over the given physical interface (e.g. eth0, enp3s0).
#   The VM gets an IP straight from your router via DHCP, as if it were its own
#   device on the network — without touching the host's own network config
#   (safer than creating a real bridge, which can break the host's connection
#   if misconfigured).
#   NOT SUPPORTED ON WI-FI: most Wi-Fi chipsets/drivers only allow a single
#   MAC address per association with the access point, so macvtap (and real
#   bridging) generally cannot work over wlan0/similar. This is a
#   hardware/driver limitation, not something fixable in software. The
#   script detects wireless interfaces and refuses --bridge on them.
#   Limitation: host and VM can't see each other directly in this mode
#   (macvtap isolation); this does not affect SSH access over the network.
#
# --forward mode (works fine over Wi-Fi):
#   Stays on the default NAT network, but adds host -> VM port forwarding
#   rules via iptables, so other devices on your LAN can reach services
#   running in the VM through the host's IP. Format: HOST_PORT:VM_PORT,...
#   Example: --forward=2222:22,8080:80 lets you SSH into
#   <host-ip>:2222 and reach a web server at <host-ip>:8080.
#
# Once it's done, connect with:
#   ssh YOUR_USER@VM_IP   (or ssh -p HOST_PORT YOUR_USER@HOST_IP with --forward)
#
set -euo pipefail

# ============================================================
# CONFIGURATION — adjust as needed
# ============================================================
VM_NAME="debian-vm"
VM_RAM_MB=2048          # RAM in MB (e.g. a basic VPS plan)
VM_VCPUS=2              # vCPUs
VM_DISK_GB=20           # Disk size in GB
VM_USER="admin"         # User created inside the VM
VM_HOSTNAME="debian-vm"
SSH_KEY_PATH="$HOME/.ssh/id_ed25519"   # Public key used for SSH access
CLOUD_IMG_URL="https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2"

# ============================================================
# 0. Argument parsing
# ============================================================
BRIDGE_IFACE=""
FORWARD_RULES=""
STATIC_IP=""
ADMIN_PASSWORD_REQUESTED=0
ADMIN_PASSWORD_VALUE=""
NO_AUTO_UPDATES=0
ALLOW_PORTS=""
NO_GUEST_FIREWALL=0
HARDEN_HOST_FIREWALL=0
MONITOR=0
WATCHDOG=0
NO_CRASH_RESTART=0

print_help() {
    cat <<HELP
Usage: $0 [options]

Options:
  --name=NAME         Name/hostname for the VM (default: debian-vm). Use a
                      distinct --name per VM to run a fleet of VMs at once.
  --ram=MB            RAM in MB (default: 2048).
  --vcpus=N           vCPU count (default: 2).
  --disk=GB           Disk size in GB (default: 20).
  --ip=ADDRESS        Reserve a stable IP and resolvable hostname for this VM
                      on the 'default' NAT network (plain NAT or --forward
                      only — usage error when combined with --bridge, since
                      bridged VMs get their address from your router, not
                      libvirt's DHCP). Rejected if already reserved for
                      another VM or under an active lease. Omit to
                      auto-pick the first free address in the network's
                      configured DHCP range. Once reserved, other fleet VMs
                      can resolve this VM by its --name/hostname.
  --bridge=IFACE      Use bridged networking (macvtap) over the physical
                      interface IFACE (e.g. --bridge=eth0). The VM gets an IP
                      from your router via DHCP. Requires a WIRED interface —
                      not supported over Wi-Fi (hardware/driver limitation).
  --forward=RULES     NAT + port forwarding. RULES is a comma-separated list
                      of HOST_PORT:VM_PORT pairs (e.g. 2222:22,8080:80).
                      Works fine over Wi-Fi. Applied after the VM gets its
                      NAT IP from libvirt's DHCP.
  --admin-password[=PASSWORD]
                      Require a password for sudo (ALL=(ALL) ALL instead of
                      NOPASSWD:ALL) for the admin user. Without a value, a
                      random password is generated. SSH stays key-only either
                      way (ssh_pwauth is never enabled) — the password only
                      gates local sudo elevation after an SSH session is
                      already established. Only applies at VM creation;
                      rerunning against an existing VM cannot change it.
                      Default: passwordless sudo (NOPASSWD:ALL).
  --no-auto-updates   Disable automatic security updates inside the VM.
                      By default, freshly created VMs install and enable
                      unattended-upgrades, restricted to the security origin,
                      with automatic reboot left off. Use this for a pinned,
                      reproducible package snapshot instead.
  --allow-port=PORT[,PORT...]
                      Open additional guest-side TCP ports through the VM's
                      firewall (ufw), on top of SSH (always allowed) and any
                      VM-side ports from --forward (allowed automatically).
                      No effect when combined with --no-guest-firewall.
  --no-guest-firewall Skip installing/enabling ufw inside the VM. By default,
                      freshly created VMs get a default-deny inbound / allow
                      outbound firewall (SSH always allowed). fail2ban's sshd
                      jail is unaffected by this flag — it's always enabled.
  --harden-host-firewall
                      Install and enable ufw on the HOST (not the VM), with a
                      default-deny inbound / allow outbound policy. Host SSH
                      is always allowed first. The forward-chain policy is
                      kept permissive so libvirt NAT and --forward keep
                      working. Host-wide, opt-in, idempotent — independent of
                      which VM is being created.
  --monitor           Enable host-side uptime monitoring (systemd timer,
                      checks domstate + SSH reachability every ~2 minutes),
                      local alerting (journal + wall + motd, no
                      network/email), and — for NAT-family VMs only, not
                      --bridge — centralized log forwarding to
                      /var/log/self-hosting-vms/<hostname>/ on the host.
                      Host-wide infrastructure installs once; each VM gets
                      its own monitoring timer instance.
  --watchdog          Attach a virtual watchdog device (i6300esb, action
                      reset) and enable systemd's RuntimeWatchdogSec inside
                      the guest, so an unresponsive kernel/PID 1 triggers an
                      automatic VM reset. Distinct from on_crash (QEMU
                      process crash) and vm-uptime-monitoring (detects and
                      alerts, doesn't act). Fixed at VM creation.
  --no-crash-restart  Disable automatic restart when the QEMU process itself
                      crashes (on_crash). By default, freshly created VMs
                      set on_crash=restart, since a genuine process crash has
                      no false-positive risk. Use this to leave a crashed VM
                      stopped for inspection instead. Fixed at VM creation.
  -h, --help          Show this help.

If neither --bridge nor --forward is given, the VM only gets a NAT IP
reachable from the host itself (e.g. via 'ssh admin@<nat-ip>' from this
machine).
HELP
}

for arg in "$@"; do
    case "$arg" in
        --name=*)
            VM_NAME="${arg#*=}"
            VM_HOSTNAME="${arg#*=}"
            ;;
        --ram=*)
            VM_RAM_MB="${arg#*=}"
            ;;
        --vcpus=*)
            VM_VCPUS="${arg#*=}"
            ;;
        --disk=*)
            VM_DISK_GB="${arg#*=}"
            ;;
        --ip=*)
            STATIC_IP="${arg#*=}"
            ;;
        --bridge=*)
            BRIDGE_IFACE="${arg#*=}"
            ;;
        --forward=*)
            FORWARD_RULES="${arg#*=}"
            ;;
        --admin-password)
            ADMIN_PASSWORD_REQUESTED=1
            ;;
        --admin-password=*)
            ADMIN_PASSWORD_REQUESTED=1
            ADMIN_PASSWORD_VALUE="${arg#*=}"
            ;;
        --no-auto-updates)
            NO_AUTO_UPDATES=1
            ;;
        --allow-port=*)
            ALLOW_PORTS="${arg#*=}"
            ;;
        --no-guest-firewall)
            NO_GUEST_FIREWALL=1
            ;;
        --harden-host-firewall)
            HARDEN_HOST_FIREWALL=1
            ;;
        --monitor)
            MONITOR=1
            ;;
        --watchdog)
            WATCHDOG=1
            ;;
        --no-crash-restart)
            NO_CRASH_RESTART=1
            ;;
        -h|--help)
            print_help
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $arg"
            print_help
            exit 1
            ;;
    esac
done

WORK_DIR="$HOME/vms/${VM_NAME}"

if [ -n "$BRIDGE_IFACE" ] && [ -n "$FORWARD_RULES" ]; then
    echo "ERROR: --bridge and --forward are mutually exclusive (forwarding only makes sense on NAT)."
    exit 1
fi

if [ -n "$BRIDGE_IFACE" ] && [ -n "$STATIC_IP" ]; then
    echo "ERROR: --ip and --bridge are mutually exclusive (bridged VMs get their address from"
    echo "       your router's DHCP, not libvirt's 'default' network, so there is nothing to reserve)."
    exit 1
fi

if [ -n "$BRIDGE_IFACE" ]; then
    echo "==> Checking if interface '${BRIDGE_IFACE}' exists..."
    if ! ip link show "$BRIDGE_IFACE" >/dev/null 2>&1; then
        echo "ERROR: interface '${BRIDGE_IFACE}' not found. Run 'ip link' to see available interfaces."
        exit 1
    fi

    echo "==> Checking if '${BRIDGE_IFACE}' is a wireless interface..."
    if [ -d "/sys/class/net/${BRIDGE_IFACE}/wireless" ] || command -v iw >/dev/null 2>&1 && iw dev "$BRIDGE_IFACE" info >/dev/null 2>&1; then
        echo "ERROR: '${BRIDGE_IFACE}' is a Wi-Fi interface. Bridged/macvtap networking does not"
        echo "       work over Wi-Fi on virtually any hardware, because the wireless chipset"
        echo "       only allows one MAC address per association with the access point."
        echo "       Use --forward=HOST_PORT:VM_PORT,... instead to expose services from the VM"
        echo "       to your LAN over the default NAT network."
        exit 1
    fi
    echo "    OK: '${BRIDGE_IFACE}' is a wired interface, using bridged mode (macvtap)."
elif [ -n "$FORWARD_RULES" ]; then
    echo "==> Using NAT networking with port forwarding: ${FORWARD_RULES}"
else
    echo "==> No --bridge or --forward given, using plain NAT (virbr0)."
fi

# ============================================================
# 0.1 Static IP/hostname reservation helpers (NAT-family only)
# ============================================================
ip_to_int() {
    local a b c d
    IFS='.' read -r a b c d <<< "$1"
    echo "$(( (a << 24) + (b << 16) + (c << 8) + d ))"
}

int_to_ip() {
    local ip_int="$1"
    echo "$(( (ip_int >> 24) & 255 )).$(( (ip_int >> 16) & 255 )).$(( (ip_int >> 8) & 255 )).$(( ip_int & 255 ))"
}

# Prints the VM name a given IP is already statically reserved for on the
# 'default' network, or nothing if it has no reservation.
#
# Uses process substitution (not a pipe into the while loop) and explicit
# `return 0`s throughout: with `set -eo pipefail`, a pipe ending in a `while
# read` that never matches anything (e.g. no reservations exist yet) exits
# non-zero, which would otherwise silently kill the whole script when this
# function is called via command substitution (`VAR=$(find_ip_reservation_owner ...)`).
find_ip_reservation_owner() {
    local host_line host_ip host_name
    while IFS= read -r host_line; do
        host_ip=$(echo "$host_line" | grep -oP "ip='\K[^']+" || true)
        host_name=$(echo "$host_line" | grep -oP "name='\K[^']+" || true)
        if [ "$host_ip" = "$1" ]; then
            echo "$host_name"
            return 0
        fi
    done < <(virsh net-dumpxml default 2>/dev/null | grep -oP "<host [^>]*/>" || true)
    return 0
}

is_ip_leased() {
    virsh net-dhcp-leases default 2>/dev/null \
        | grep -oP '\d{1,3}(\.\d{1,3}){3}(?=/)' \
        | grep -qx "$1"
}

is_ip_free() {
    [ -z "$(find_ip_reservation_owner "$1")" ] && ! is_ip_leased "$1"
}

# Generates a MAC address in the same 52:54:00 (QEMU/KVM) range libvirt uses
# for auto-assigned NICs, retrying until it doesn't collide with an existing
# reservation on the 'default' network.
generate_mac() {
    local mac
    while true; do
        mac=$(printf '52:54:00:%02x:%02x:%02x' $((RANDOM % 256)) $((RANDOM % 256)) $((RANDOM % 256)))
        if ! virsh net-dumpxml default 2>/dev/null | grep -qi "mac='${mac}'"; then
            echo "$mac"
            return
        fi
    done
}

# ============================================================
# 1. Prerequisite checks
# ============================================================
echo "==> Checking for hardware virtualization support..."
if [ "$(egrep -c '(vmx|svm)' /proc/cpuinfo)" -eq 0 ]; then
    echo "ERROR: your CPU doesn't report VT-x/AMD-V support, or it's disabled in the BIOS."
    echo "Enable virtualization in the BIOS/UEFI and run this script again."
    exit 1
fi
echo "    OK: hardware virtualization supported."

if [ "$(id -u)" -eq 0 ]; then
    echo "ERROR: don't run this script as root. Run it as your normal user (it will use sudo when needed)."
    exit 1
fi

echo "==> Checking if the system is Debian/Ubuntu-based (apt)..."
if ! command -v apt >/dev/null 2>&1; then
    echo "ERROR: this script assumes a system with apt (Ubuntu/Debian). Adapt it for your distro."
    exit 1
fi
echo "    OK."

# ============================================================
# 2. Installing required packages
# ============================================================
echo "==> Installing KVM, QEMU, libvirt, and cloud-init tools..."
sudo apt update
sudo apt install -y \
    qemu-system-x86 \
    qemu-utils \
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils \
    virtinst \
    cloud-image-utils \
    genisoimage \
    wget \
    openssh-client \
    acl

echo "==> Adding your user to the libvirt and kvm groups..."
sudo usermod -aG libvirt "$(whoami)"
sudo usermod -aG kvm "$(whoami)"

echo "==> Enabling and starting the libvirtd service..."
sudo systemctl enable --now libvirtd

# Group membership changes take effect only in a new login session. Do not
# re-run this script through sg: nested group-switching shells can leave the
# group check false and restart the whole setup indefinitely.
CURRENT_GROUPS="$(id -nG)"
MISSING_GROUPS=()
for required_group in libvirt kvm; do
    if [[ " ${CURRENT_GROUPS} " != *" ${required_group} "* ]]; then
        MISSING_GROUPS+=("${required_group}")
    fi
done

if [ "${#MISSING_GROUPS[@]}" -gt 0 ]; then
    echo "ERROR: the current session is missing required groups: ${MISSING_GROUPS[*]}"
    echo "       Your user was added to these groups, but the change only takes effect"
    echo "       after a new login session. Log out and back in, then run this script again."
    echo "       Verify with: id -nG"
    exit 1
fi

# ============================================================
# 3. NAT network readiness (default libvirt network)
# ============================================================
if [ -z "$BRIDGE_IFACE" ]; then
    echo "==> Checking the libvirt 'default' network (required for NAT networking)..."
    if ! virsh net-info default >/dev/null 2>&1; then
        echo "ERROR: the libvirt network 'default' is not defined."
        echo "       Inspect available networks with: virsh net-list --all"
        echo "       Define/restore it, e.g.: virsh net-define /usr/share/libvirt/networks/default.xml"
        exit 1
    fi

    if [ "$(virsh net-info default | awk '/^Active:/ {print $2}')" = "yes" ]; then
        echo "    OK: 'default' network is already active."
    else
        echo "==> Starting the inactive 'default' network..."
        if ! virsh net-start default; then
            echo "ERROR: failed to start the libvirt 'default' network. Inspect with: virsh net-info default"
            exit 1
        fi
    fi

    echo "==> Enabling autostart for the 'default' network..."
    if ! virsh net-autostart default; then
        echo "ERROR: failed to enable autostart for the libvirt 'default' network."
        exit 1
    fi
else
    echo "==> Bridged mode selected: leaving the 'default' network untouched."
fi

# ============================================================
# 4. QEMU storage access (ACL on $HOME)
# ============================================================
echo "==> Granting the 'libvirt-qemu' service account traversal access to \$HOME..."
if ! id -u libvirt-qemu >/dev/null 2>&1; then
    echo "ERROR: the 'libvirt-qemu' service account was not found."
    echo "       This script targets Debian/Ubuntu libvirt packages, which create this"
    echo "       account when libvirt-daemon-system is installed. Verify with: id libvirt-qemu"
    exit 1
fi

if ! sudo setfacl -m u:libvirt-qemu:--x "$HOME"; then
    echo "ERROR: failed to grant 'libvirt-qemu' execute-only access to \$HOME via setfacl."
    echo "       Ensure the filesystem hosting \$HOME supports POSIX ACLs."
    exit 1
fi
echo "    OK: 'libvirt-qemu' can now traverse \$HOME (execute-only, no read/listing)."

# ============================================================
# 4.1 Host firewall hardening (opt-in, host-wide, idempotent —
#     independent of which VM is being created/reused below)
# ============================================================
UFW_SSH_TAG="self-hosting: host SSH baseline"
if [ "$HARDEN_HOST_FIREWALL" -eq 1 ]; then
    echo "==> Hardening the host firewall (ufw)..."

    if ! command -v ufw >/dev/null 2>&1; then
        echo "==> Installing ufw..."
        sudo apt install -y ufw
    fi

    if sudo ufw status | grep -qF "$UFW_SSH_TAG"; then
        echo "    OK: host SSH baseline rule already present."
    else
        echo "==> Adding host SSH baseline rule (tcp/22)..."
        sudo ufw allow 22/tcp comment "$UFW_SSH_TAG"
    fi

    if grep -q '^DEFAULT_FORWARD_POLICY="DROP"' /etc/default/ufw 2>/dev/null; then
        echo "==> Setting DEFAULT_FORWARD_POLICY to ACCEPT (required for libvirt NAT / --forward)..."
        sudo sed -i 's/^DEFAULT_FORWARD_POLICY="DROP"/DEFAULT_FORWARD_POLICY="ACCEPT"/' /etc/default/ufw
        HOST_FORWARD_POLICY_CHANGED=1
    else
        echo "    OK: DEFAULT_FORWARD_POLICY is already permissive (or already ACCEPT)."
    fi

    if sudo ufw status | grep -q "^Status: active"; then
        if [ "${HOST_FORWARD_POLICY_CHANGED:-0}" -eq 1 ]; then
            sudo ufw reload
        fi
    else
        sudo ufw --force enable
    fi
    echo "    OK: host firewall hardened (ufw active, host SSH allowed, forwarding preserved)."
fi

# ============================================================
# 4.2 Monitoring infrastructure (opt-in via --monitor, host-wide,
#     idempotent — independent of which VM is being created/reused)
# ============================================================
MONITOR_LOG_PORT=5140
if [ "$MONITOR" -eq 1 ]; then
    echo "==> Installing host-wide monitoring infrastructure..."

    echo "==> Installing the uptime health-check script..."
    sudo tee /usr/local/bin/self-hosting-vm-uptime-check >/dev/null <<'SCRIPT'
#!/usr/bin/env bash
# self-hosting-vm-uptime-check <vm-name>
# Checks whether a VM is up (domstate running AND SSH port reachable) and
# alerts locally (journal + wall) only on an up<->down transition.
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
SCRIPT
    sudo chmod 755 /usr/local/bin/self-hosting-vm-uptime-check

    echo "==> Installing the uptime timer/service templates..."
    sudo tee /etc/systemd/system/self-hosting-vm-uptime@.service >/dev/null <<'UNIT'
[Unit]
Description=self-hosting uptime check for VM %i
After=libvirtd.service

[Service]
Type=oneshot
ExecStart=/usr/local/bin/self-hosting-vm-uptime-check %i
UNIT

    sudo tee /etc/systemd/system/self-hosting-vm-uptime@.timer >/dev/null <<'UNIT'
[Unit]
Description=Periodic self-hosting uptime check for VM %i

[Timer]
OnBootSec=1min
OnUnitActiveSec=2min
Unit=self-hosting-vm-uptime@%i.service

[Install]
WantedBy=timers.target
UNIT

    echo "==> Installing the login alert summary (update-motd.d)..."
    sudo tee /etc/update-motd.d/95-self-hosting-alerts >/dev/null <<'MOTD'
#!/usr/bin/env bash
RECENT=$(journalctl -t self-hosting-alert -n 5 --no-pager --since "-24 hours" 2>/dev/null)
if [ -n "$RECENT" ]; then
    echo ""
    echo "Recent self-hosting alerts:"
    echo "$RECENT"
    echo ""
fi
MOTD
    sudo chmod 755 /etc/update-motd.d/95-self-hosting-alerts

    sudo systemctl daemon-reload

    echo "==> Installing log storage and rotation..."
    sudo mkdir -p /var/log/self-hosting-vms
    sudo tee /etc/logrotate.d/self-hosting-vms >/dev/null <<'LOGROTATE'
/var/log/self-hosting-vms/*/*.log {
    weekly
    rotate 8
    compress
    missingok
    notifempty
    copytruncate
}
LOGROTATE

    VIRBR0_IP=$(ip -4 addr show virbr0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1)
    if [ -n "$VIRBR0_IP" ]; then
        echo "==> Installing the centralized-logging receiver (bound to virbr0: ${VIRBR0_IP})..."
        if ! command -v rsyslogd >/dev/null 2>&1; then
            sudo apt install -y rsyslog
        fi
        sudo tee /etc/rsyslog.d/60-self-hosting-vm-receiver.conf >/dev/null <<RSYSLOG
module(load="imtcp")
input(type="imtcp" port="${MONITOR_LOG_PORT}" address="${VIRBR0_IP}")

template(name="selfHostingVMPerHost" type="string" string="/var/log/self-hosting-vms/%HOSTNAME%/messages.log")
if \$inputname == "imtcp" then {
    action(type="omfile" dynaFile="selfHostingVMPerHost" createDirs="on")
    stop
}
RSYSLOG
        sudo systemctl restart rsyslog

    else
        echo "    NOTE: virbr0 not found (no NAT-family VM has been set up yet on this host)."
        echo "          Centralized logging will start working once one is; uptime monitoring"
        echo "          is unaffected."
    fi
    echo "    OK: monitoring infrastructure ready."
fi

# ============================================================
# 4.3 Log receiver firewall rule (order-independent re-check).
#     Runs every invocation, regardless of --monitor/--harden-host-firewall
#     on THIS run, so the rule self-heals no matter which flag was used to
#     enable ufw and which run originally installed the receiver: e.g.
#     `--monitor` today, `--harden-host-firewall` (no --monitor) later would
#     otherwise newly default-deny the receiver's port with nothing to
#     re-add the exception for it.
# ============================================================
if [ -f /etc/rsyslog.d/60-self-hosting-vm-receiver.conf ] && command -v ufw >/dev/null 2>&1 \
    && sudo ufw status | grep -q "^Status: active"; then
    if ! sudo ufw status | grep -q "${MONITOR_LOG_PORT}/tcp.*virbr0"; then
        echo "==> Ensuring the log receiver's firewall rule is present..."
        sudo ufw allow in on virbr0 to any port "${MONITOR_LOG_PORT}" proto tcp
    fi
fi

# ============================================================
# 5. VM existence check
# ============================================================
echo "==> Checking if a VM with this name already exists..."
VM_EXISTS=0
if virsh dominfo "$VM_NAME" >/dev/null 2>&1; then
    VM_EXISTS=1
fi

if [ "$VM_EXISTS" -eq 0 ]; then
    echo "    OK: no existing VM named '${VM_NAME}', proceeding with setup."

    # ============================================================
    # 5.1 Static IP/hostname reservation (NAT-family only)
    # ============================================================
    RESERVED_MAC=""
    if [ -z "$BRIDGE_IFACE" ]; then
        echo "==> Resolving a static IP/hostname reservation on the 'default' network..."

        # A reservation can only exist for '${VM_HOSTNAME}' here if an earlier run
        # registered it and then failed before actually creating the VM (we're in
        # the VM_EXISTS=0 branch, so no VM currently owns it). libvirt/dnsmasq
        # rejects a second host entry with the same name even at a different IP,
        # so clear the orphan first rather than erroring out on every retry.
        STALE_RESERVATION=$(virsh net-dumpxml default 2>/dev/null | grep -oP "<host [^>]*/>" | grep -F "name='${VM_HOSTNAME}'" || true)
        if [ -n "$STALE_RESERVATION" ]; then
            echo "    Found an orphaned reservation from an earlier incomplete run, replacing it:"
            echo "      ${STALE_RESERVATION}"
            virsh net-update default delete ip-dhcp-host "$STALE_RESERVATION" --live --config >/dev/null 2>&1 || true
        fi

        if [ -n "$STATIC_IP" ]; then
            RESERVATION_OWNER=$(find_ip_reservation_owner "$STATIC_IP")
            if [ -n "$RESERVATION_OWNER" ]; then
                echo "ERROR: address ${STATIC_IP} is already reserved for VM '${RESERVATION_OWNER}'."
                echo "       Pick a different --ip, or remove that VM's reservation first."
                exit 1
            fi
            if is_ip_leased "$STATIC_IP"; then
                echo "ERROR: address ${STATIC_IP} has no static reservation but currently has an"
                echo "       active DHCP lease. Pick a different --ip, or wait for the lease to clear."
                exit 1
            fi
            RESOLVED_IP="$STATIC_IP"
            echo "    OK: ${RESOLVED_IP} is free."
        else
            RANGE_LINE=$(virsh net-dumpxml default 2>/dev/null | grep -oP '<range[^/]*/>' | head -n1)
            RANGE_START=$(echo "$RANGE_LINE" | grep -oP "start='\K[^']+" || true)
            RANGE_END=$(echo "$RANGE_LINE" | grep -oP "end='\K[^']+" || true)
            if [ -z "$RANGE_START" ] || [ -z "$RANGE_END" ]; then
                echo "ERROR: could not determine the 'default' network's DHCP range."
                echo "       Inspect with: virsh net-dumpxml default"
                exit 1
            fi

            RESOLVED_IP=""
            START_INT=$(ip_to_int "$RANGE_START")
            END_INT=$(ip_to_int "$RANGE_END")
            for (( ip_int = START_INT; ip_int <= END_INT; ip_int++ )); do
                CANDIDATE_IP=$(int_to_ip "$ip_int")
                if is_ip_free "$CANDIDATE_IP"; then
                    RESOLVED_IP="$CANDIDATE_IP"
                    break
                fi
            done

            if [ -z "$RESOLVED_IP" ]; then
                echo "ERROR: no free address found in the 'default' network's DHCP range (${RANGE_START} - ${RANGE_END})."
                exit 1
            fi
            echo "    Auto-picked free address: ${RESOLVED_IP}"
        fi

        RESERVED_MAC=$(generate_mac)
        echo "==> Reserving ${RESOLVED_IP} for '${VM_HOSTNAME}' (mac ${RESERVED_MAC}) on the 'default' network..."
        if ! virsh net-update default add ip-dhcp-host \
            "<host mac='${RESERVED_MAC}' name='${VM_HOSTNAME}' ip='${RESOLVED_IP}'/>" \
            --live --config >/dev/null; then
            echo "ERROR: failed to register the DHCP host reservation on the 'default' network."
            exit 1
        fi
        echo "    OK: reservation registered."
    fi

    # ============================================================
    # 6. SSH key check
    # ============================================================
    if [ ! -f "${SSH_KEY_PATH}.pub" ]; then
        echo "==> No SSH key found at ${SSH_KEY_PATH}.pub"
        echo "==> Generating a new key pair..."
        ssh-keygen -t ed25519 -f "${SSH_KEY_PATH}" -N "" -C "${VM_USER}@${VM_HOSTNAME}"
    fi
    SSH_PUB_KEY=$(cat "${SSH_KEY_PATH}.pub")

    # ============================================================
    # 7. Downloading the Debian 12 cloud image
    # ============================================================
    mkdir -p "$WORK_DIR"
    cd "$WORK_DIR"

    IMG_FILE="debian-12-generic-amd64.qcow2"
    VM_DISK="${VM_NAME}.qcow2"

    if [ ! -s "$IMG_FILE" ]; then
        echo "==> Downloading the official Debian 12 cloud image..."
        wget -O "$IMG_FILE" "$CLOUD_IMG_URL"
    else
        echo "==> Cloud image already downloaded, skipping."
    fi

    echo "==> Creating the VM disk (copy from the base image) and resizing to ${VM_DISK_GB}G..."
    cp "$IMG_FILE" "$VM_DISK"
    qemu-img resize "$VM_DISK" "${VM_DISK_GB}G"

    # ============================================================
    # 7.1 Admin sudo policy (password-less by default, opt-in password-required)
    # ============================================================
    ADMIN_SUDO_ENTRY="ALL=(ALL) NOPASSWD:ALL"
    ADMIN_PASSWD_BLOCK=""
    ADMIN_SUDO_POLICY="nopasswd"
    if [ "$ADMIN_PASSWORD_REQUESTED" -eq 1 ]; then
        echo "==> Configuring password-required sudo for '${VM_USER}'..."
        if ! command -v openssl >/dev/null 2>&1; then
            echo "ERROR: 'openssl' is required to hash the admin password but was not found on the host."
            echo "       Install it and re-run."
            exit 1
        fi

        if [ -n "$ADMIN_PASSWORD_VALUE" ]; then
            ADMIN_PASSWORD_PLAINTEXT="$ADMIN_PASSWORD_VALUE"
        else
            ADMIN_PASSWORD_PLAINTEXT=$(openssl rand -base64 18)
        fi
        ADMIN_PASSWORD_HASH=$(openssl passwd -6 -salt "$(openssl rand -hex 8)" "$ADMIN_PASSWORD_PLAINTEXT")

        ADMIN_SUDO_ENTRY="ALL=(ALL) ALL"
        ADMIN_PASSWD_BLOCK="    passwd: ${ADMIN_PASSWORD_HASH}
    lock_passwd: false"
        ADMIN_SUDO_POLICY="password-required"

        echo "${ADMIN_PASSWORD_PLAINTEXT}" > "${WORK_DIR}/admin-password"
        chmod 600 "${WORK_DIR}/admin-password"
        echo "    OK: password configured (will be shown once when setup finishes)."
    fi
    echo "$ADMIN_SUDO_POLICY" > "${WORK_DIR}/.admin-sudo-policy"

    # ============================================================
    # 7.2 cloud-init accumulators (extra packages/runcmd/write_files
    #     appended to by the sections below; qemu-guest-agent is the
    #     baseline every VM gets regardless of flags)
    # ============================================================
    CLOUDINIT_PACKAGES="  - qemu-guest-agent"
    CLOUDINIT_RUNCMD="  - systemctl enable --now qemu-guest-agent"
    CLOUDINIT_WRITE_FILES=""

    # ============================================================
    # 7.3 Automatic security updates (unattended-upgrades, on by default)
    # ============================================================
    if [ "$NO_AUTO_UPDATES" -eq 0 ]; then
        echo "==> Enabling automatic security updates (unattended-upgrades)..."
        CLOUDINIT_PACKAGES="${CLOUDINIT_PACKAGES}
  - unattended-upgrades"
        UU_WRITE_FILES=$(cat <<'BLOCK'
  - path: /etc/apt/apt.conf.d/20auto-upgrades
    content: |
      APT::Periodic::Update-Package-Lists "1";
      APT::Periodic::Unattended-Upgrade "1";
  - path: /etc/apt/apt.conf.d/51unattended-upgrades-security-only
    content: |
      Unattended-Upgrade::Allowed-Origins {
          "${distro_id}:${distro_codename}-security";
          "${distro_id}ESM:${distro_codename}-security";
      };
      Unattended-Upgrade::Automatic-Reboot "false";
BLOCK
)
        CLOUDINIT_WRITE_FILES="${CLOUDINIT_WRITE_FILES}
${UU_WRITE_FILES}"
    fi

    # ============================================================
    # 7.4 Guest firewall (ufw, default-deny, on by default) and
    #     fail2ban's sshd jail (always on, independent of the flag)
    # ============================================================
    GUEST_FIREWALL_POLICY="disabled"
    if [ "$NO_GUEST_FIREWALL" -eq 0 ]; then
        GUEST_FIREWALL_POLICY="enabled"

        GUEST_FW_PORTS="22"
        if [ -n "$ALLOW_PORTS" ]; then
            GUEST_FW_PORTS="${GUEST_FW_PORTS},${ALLOW_PORTS}"
        fi
        if [ -n "$FORWARD_RULES" ]; then
            IFS=',' read -ra FWD_RULES_FOR_FW_ARR <<< "$FORWARD_RULES"
            for rule in "${FWD_RULES_FOR_FW_ARR[@]}"; do
                FWD_VM_PORT="${rule##*:}"
                # Skip malformed rules (e.g. a trailing colon with no port)
                # instead of feeding an empty/non-numeric entry into ufw.
                if [[ "$FWD_VM_PORT" =~ ^[0-9]+$ ]]; then
                    GUEST_FW_PORTS="${GUEST_FW_PORTS},${FWD_VM_PORT}"
                fi
            done
        fi
        GUEST_FW_PORTS=$(echo "$GUEST_FW_PORTS" | tr ',' '\n' | sort -n -u | tr '\n' ',' | sed 's/,$//')

        echo "==> Enabling guest firewall (ufw), allowed ports: ${GUEST_FW_PORTS}..."
        CLOUDINIT_PACKAGES="${CLOUDINIT_PACKAGES}
  - ufw"
        CLOUDINIT_RUNCMD="${CLOUDINIT_RUNCMD}
  - ufw default deny incoming
  - ufw default allow outgoing"
        IFS=',' read -ra GUEST_FW_PORTS_ARR <<< "$GUEST_FW_PORTS"
        for port in "${GUEST_FW_PORTS_ARR[@]}"; do
            CLOUDINIT_RUNCMD="${CLOUDINIT_RUNCMD}
  - ufw allow ${port}/tcp"
        done
        CLOUDINIT_RUNCMD="${CLOUDINIT_RUNCMD}
  - ufw --force enable"
    fi
    echo "$GUEST_FIREWALL_POLICY" > "${WORK_DIR}/.guest-firewall-policy"

    echo "==> Enabling fail2ban's sshd jail (always on)..."
    CLOUDINIT_PACKAGES="${CLOUDINIT_PACKAGES}
  - fail2ban"
    F2B_WRITE_FILES=$(cat <<'BLOCK'
  - path: /etc/fail2ban/jail.local
    content: |
      [sshd]
      enabled = true
BLOCK
)
    CLOUDINIT_WRITE_FILES="${CLOUDINIT_WRITE_FILES}
${F2B_WRITE_FILES}"
    CLOUDINIT_RUNCMD="${CLOUDINIT_RUNCMD}
  - systemctl enable --now fail2ban"

    # ============================================================
    # 7.5 Centralized logging (guest-side forwarding, NAT-family only —
    #     --bridge's macvtap isolation means the guest can't reach virbr0)
    # ============================================================
    LOG_FORWARDING_CONFIGURED=0
    if [ "$MONITOR" -eq 1 ] && [ -z "$BRIDGE_IFACE" ] && [ -n "${VIRBR0_IP:-}" ]; then
        echo "==> Configuring guest log forwarding to ${VIRBR0_IP}:${MONITOR_LOG_PORT}..."
        CLOUDINIT_PACKAGES="${CLOUDINIT_PACKAGES}
  - rsyslog"
        LOG_FWD_WRITE_FILES=$(cat <<BLOCK
  - path: /etc/rsyslog.d/60-forward-to-host.conf
    content: |
      *.* @@${VIRBR0_IP}:${MONITOR_LOG_PORT}
BLOCK
)
        CLOUDINIT_WRITE_FILES="${CLOUDINIT_WRITE_FILES}
${LOG_FWD_WRITE_FILES}"
        LOG_FORWARDING_CONFIGURED=1
    fi
    echo "$LOG_FORWARDING_CONFIGURED" > "${WORK_DIR}/.log-forwarding-configured"

    # ============================================================
    # 7.6 Guest watchdog petting (systemd RuntimeWatchdogSec)
    # ============================================================
    if [ "$WATCHDOG" -eq 1 ]; then
        echo "==> Configuring the guest to pet the virtual watchdog (systemd)..."
        WD_WRITE_FILES=$(cat <<'BLOCK'
  - path: /etc/systemd/system.conf.d/90-watchdog.conf
    content: |
      [Manager]
      RuntimeWatchdogSec=20s
BLOCK
)
        CLOUDINIT_WRITE_FILES="${CLOUDINIT_WRITE_FILES}
${WD_WRITE_FILES}"
    fi

    # ============================================================
    # 8. Creating the cloud-init configuration (user-data / meta-data)
    # ============================================================
    echo "==> Generating cloud-init configuration..."

    WRITE_FILES_SECTION=""
    if [ -n "$CLOUDINIT_WRITE_FILES" ]; then
        WRITE_FILES_SECTION="write_files:${CLOUDINIT_WRITE_FILES}"
    fi

    cat > user-data <<EOF
#cloud-config
hostname: ${VM_HOSTNAME}
manage_etc_hosts: true
users:
  - name: ${VM_USER}
    groups: sudo
    shell: /bin/bash
    sudo: ${ADMIN_SUDO_ENTRY}
${ADMIN_PASSWD_BLOCK}
    ssh_authorized_keys:
      - ${SSH_PUB_KEY}
ssh_pwauth: false
package_update: true
package_upgrade: false
packages:
${CLOUDINIT_PACKAGES}
${WRITE_FILES_SECTION}
runcmd:
${CLOUDINIT_RUNCMD}
EOF

    cat > meta-data <<EOF
instance-id: ${VM_NAME}-$(date +%s)
local-hostname: ${VM_HOSTNAME}
EOF

    echo "==> Generating the cloud-init seed ISO..."
    cloud-localds seed.iso user-data meta-data

    # ============================================================
    # 9. Creating the VM via virt-install
    # ============================================================
    if [ -n "$BRIDGE_IFACE" ]; then
        NETWORK_ARG="type=direct,source=${BRIDGE_IFACE},source_mode=bridge,model=virtio"
        echo "==> Creating the VM (bridged networking via macvtap over ${BRIDGE_IFACE})..."
    else
        NETWORK_ARG="network=default,model=virtio,mac=${RESERVED_MAC}"
        echo "==> Creating the VM (NAT networking via virbr0)..."
    fi

    VIRT_INSTALL_EXTRA_ARGS=()
    if [ "$WATCHDOG" -eq 1 ]; then
        echo "==> Attaching a virtual watchdog device (i6300esb, action=reset)..."
        VIRT_INSTALL_EXTRA_ARGS+=(--watchdog "model=i6300esb,action=reset")
    fi
    if [ "$NO_CRASH_RESTART" -eq 0 ]; then
        echo "==> Enabling automatic restart on QEMU crash (on_crash=restart)..."
        VIRT_INSTALL_EXTRA_ARGS+=(--events "on_crash=restart")
    fi

    virt-install \
        --name "$VM_NAME" \
        --memory "$VM_RAM_MB" \
        --vcpus "$VM_VCPUS" \
        --disk path="${WORK_DIR}/${VM_DISK}",format=qcow2 \
        --disk path="${WORK_DIR}/seed.iso",device=cdrom \
        --os-variant debian12 \
        --network "$NETWORK_ARG" \
        --graphics none \
        --import \
        --noautoconsole \
        ${VIRT_INSTALL_EXTRA_ARGS[@]+"${VIRT_INSTALL_EXTRA_ARGS[@]}"}

    echo ""
    echo "=================================================="
    echo " VM '${VM_NAME}' created successfully!"
    echo "=================================================="

    if [ "$NO_AUTO_UPDATES" -eq 0 ]; then
        echo ""
        echo "NOTE: security-origin updates will be applied automatically inside the VM"
        echo "      (unattended-upgrades). Kernel/library updates that require a reboot do"
        echo "      NOT reboot automatically — check and reboot manually when needed."
    fi

    if [ "$GUEST_FIREWALL_POLICY" = "enabled" ]; then
        echo ""
        echo "NOTE: the guest firewall (ufw) is active, default-deny inbound. Allowed TCP"
        echo "      ports: ${GUEST_FW_PORTS}. Use --allow-port=PORT[,PORT...] to open more"
        echo "      at creation time. fail2ban's sshd jail is also active."
    fi

    if [ "$MONITOR" -eq 1 ]; then
        echo ""
        echo "NOTE: uptime monitoring is active for '${VM_NAME}' (checks every ~2 minutes)."
        echo "      Alerts go to the host journal (journalctl -t self-hosting-alert), a wall"
        echo "      broadcast, and the host's login banner — no remote/email notification."
        if [ "$LOG_FORWARDING_CONFIGURED" -eq 1 ]; then
            echo "      Logs are forwarded to /var/log/self-hosting-vms/${VM_HOSTNAME}/ on the host."
        elif [ -n "$BRIDGE_IFACE" ]; then
            echo "      Centralized logging is NOT available in bridged mode (macvtap isolation);"
            echo "      only uptime monitoring applies."
        fi
    fi
else
    echo ""
    echo "=================================================="
    echo " Reusing existing VM '${VM_NAME}'"
    echo "=================================================="
fi

# ============================================================
# 10. Effective network mode introspection
# ============================================================
IFACE_LINE=$(virsh domiflist "$VM_NAME" 2>/dev/null | awk '$2=="network" || $2=="direct" {print; exit}')
if [ -z "$IFACE_LINE" ]; then
    echo "ERROR: could not determine the network interface for VM '${VM_NAME}'."
    echo "       Inspect manually with: virsh domiflist ${VM_NAME}"
    exit 1
fi
IFACE_TYPE=$(echo "$IFACE_LINE" | awk '{print $2}')
IFACE_SOURCE=$(echo "$IFACE_LINE" | awk '{print $3}')

if [ "$IFACE_TYPE" = "direct" ]; then
    EFFECTIVE_MODE="bridged"
    EFFECTIVE_BRIDGE_IFACE="$IFACE_SOURCE"
else
    EFFECTIVE_MODE="nat"
    EFFECTIVE_BRIDGE_IFACE=""
fi

# ============================================================
# 10.1 Effective admin sudo policy introspection
# ============================================================
EFFECTIVE_ADMIN_SUDO_POLICY=""
if [ -f "${WORK_DIR}/.admin-sudo-policy" ]; then
    EFFECTIVE_ADMIN_SUDO_POLICY=$(cat "${WORK_DIR}/.admin-sudo-policy")
fi

# ============================================================
# 10.2 Effective watchdog configuration introspection
# ============================================================
EFFECTIVE_WATCHDOG=0
if virsh dumpxml "$VM_NAME" 2>/dev/null | grep -q '<watchdog'; then
    EFFECTIVE_WATCHDOG=1
fi

# ============================================================
# 10.3 Effective on_crash policy introspection
# ============================================================
EFFECTIVE_CRASH_RESTART=0
if virsh dumpxml "$VM_NAME" 2>/dev/null | grep -q '<on_crash>restart</on_crash>'; then
    EFFECTIVE_CRASH_RESTART=1
fi

# ============================================================
# 10.4 Effective log-forwarding configuration introspection
# ============================================================
EFFECTIVE_LOG_FORWARDING=""
if [ -f "${WORK_DIR}/.log-forwarding-configured" ]; then
    EFFECTIVE_LOG_FORWARDING=$(cat "${WORK_DIR}/.log-forwarding-configured")
fi

# ============================================================
# 11. Reusing an existing VM: mode-mismatch warning and auto-start
# ============================================================
if [ "$VM_EXISTS" -eq 1 ]; then
    REQUESTED_BRIDGED=0
    [ -n "$BRIDGE_IFACE" ] && REQUESTED_BRIDGED=1
    EFFECTIVE_BRIDGED=0
    [ "$EFFECTIVE_MODE" = "bridged" ] && EFFECTIVE_BRIDGED=1

    if [ "$REQUESTED_BRIDGED" -ne "$EFFECTIVE_BRIDGED" ]; then
        echo ""
        if [ "$EFFECTIVE_MODE" = "bridged" ]; then
            echo "WARNING: VM '${VM_NAME}' already exists using bridged networking (via '${EFFECTIVE_BRIDGE_IFACE}'),"
            echo "         but this run did not request --bridge."
        else
            echo "WARNING: VM '${VM_NAME}' already exists using NAT networking (virbr0),"
            echo "         but this run requested --bridge."
        fi
        echo "         Network mode is fixed when the VM is created and cannot be changed by rerunning"
        echo "         this script. To use a different mode, remove the VM first and run again:"
        echo "           virsh undefine ${VM_NAME} --remove-all-storage"
        echo "         Continuing with the VM's actual (${EFFECTIVE_MODE}) networking."
    fi

    EFFECTIVE_ADMIN_PASSWORD=0
    [ "$EFFECTIVE_ADMIN_SUDO_POLICY" = "password-required" ] && EFFECTIVE_ADMIN_PASSWORD=1
    if [ -z "$EFFECTIVE_ADMIN_SUDO_POLICY" ]; then
        if [ "$ADMIN_PASSWORD_REQUESTED" -eq 1 ]; then
            echo ""
            echo "WARNING: VM '${VM_NAME}' already exists, but its sudo policy cannot be determined"
            echo "         (no record found — it may predate --admin-password)."
            echo "         Sudo policy is fixed when the VM is created and cannot be changed by rerunning"
            echo "         this script. To apply --admin-password, remove the VM first and run again:"
            echo "           virsh undefine ${VM_NAME} --remove-all-storage"
        fi
    elif [ "$ADMIN_PASSWORD_REQUESTED" -ne "$EFFECTIVE_ADMIN_PASSWORD" ]; then
        echo ""
        if [ "$EFFECTIVE_ADMIN_PASSWORD" -eq 1 ]; then
            echo "WARNING: VM '${VM_NAME}' already exists with password-required sudo,"
            echo "         but this run did not request --admin-password."
        else
            echo "WARNING: VM '${VM_NAME}' already exists with passwordless sudo (NOPASSWD:ALL),"
            echo "         but this run requested --admin-password."
        fi
        echo "         Sudo policy is fixed when the VM is created and cannot be changed by rerunning"
        echo "         this script. To use a different policy, remove the VM first and run again:"
        echo "           virsh undefine ${VM_NAME} --remove-all-storage"
        echo "         Continuing with the VM's actual sudo policy."
    fi

    if [ "$WATCHDOG" -ne "$EFFECTIVE_WATCHDOG" ]; then
        echo ""
        if [ "$EFFECTIVE_WATCHDOG" -eq 1 ]; then
            echo "WARNING: VM '${VM_NAME}' already exists with a watchdog device,"
            echo "         but this run did not request --watchdog."
        else
            echo "WARNING: VM '${VM_NAME}' already exists with no watchdog device,"
            echo "         but this run requested --watchdog."
        fi
        echo "         Watchdog configuration is fixed when the VM is created and cannot be"
        echo "         changed by rerunning this script. To use a different configuration,"
        echo "         remove the VM first and run again:"
        echo "           virsh undefine ${VM_NAME} --remove-all-storage"
        echo "         Continuing with the VM's actual watchdog configuration."
    fi

    REQUESTED_CRASH_RESTART=1
    [ "$NO_CRASH_RESTART" -eq 1 ] && REQUESTED_CRASH_RESTART=0
    if [ "$REQUESTED_CRASH_RESTART" -ne "$EFFECTIVE_CRASH_RESTART" ]; then
        echo ""
        if [ "$EFFECTIVE_CRASH_RESTART" -eq 1 ]; then
            echo "WARNING: VM '${VM_NAME}' already exists with on_crash=restart,"
            echo "         but this run requested --no-crash-restart."
        else
            echo "WARNING: VM '${VM_NAME}' already exists without on_crash=restart,"
            echo "         but this run did not request --no-crash-restart."
        fi
        echo "         Crash-recovery policy is fixed when the VM is created and cannot be"
        echo "         changed by rerunning this script. To use a different policy, remove"
        echo "         the VM first and run again:"
        echo "           virsh undefine ${VM_NAME} --remove-all-storage"
        echo "         Continuing with the VM's actual crash-recovery policy."
    fi

    if [ "$MONITOR" -eq 1 ] && [ -z "$BRIDGE_IFACE" ] && [ "$EFFECTIVE_LOG_FORWARDING" != "1" ]; then
        echo ""
        echo "WARNING: --monitor was requested, but log forwarding for VM '${VM_NAME}' was not"
        echo "         configured at creation (cloud-init only applies at first boot) — it"
        echo "         predates --monitor, was created with --bridge, or virbr0 didn't exist yet."
        echo "         Uptime monitoring still applies (host-side, reapplied below), but logs"
        echo "         from this VM will NOT reach /var/log/self-hosting-vms/. To fix, remove"
        echo "         and recreate the VM: virsh undefine ${VM_NAME} --remove-all-storage"
    fi

    VM_STATE=$(virsh domstate "$VM_NAME" 2>/dev/null || echo "unknown")
    if [ "$VM_STATE" != "running" ]; then
        echo "==> VM '${VM_NAME}' is currently '${VM_STATE}', starting it..."
        if ! virsh start "$VM_NAME"; then
            echo "ERROR: failed to start VM '${VM_NAME}'. Inspect with: virsh domstate ${VM_NAME}"
            exit 1
        fi
    else
        echo "    OK: VM '${VM_NAME}' is already running."
    fi
fi

# ============================================================
# 12. VM autostart
# ============================================================
echo "==> Configuring '${VM_NAME}' to autostart on host boot..."
if ! virsh autostart "$VM_NAME" >/dev/null 2>&1; then
    echo "WARNING: failed to enable autostart for VM '${VM_NAME}'. Retry manually with:"
    echo "         virsh autostart ${VM_NAME}"
fi

# ============================================================
# 12.1 Per-VM uptime monitoring timer instance (host-side, so unlike
#      cloud-init-driven features this can be (re)applied on reruns too)
# ============================================================
if [ "$MONITOR" -eq 1 ]; then
    echo "==> Enabling the uptime monitoring timer for '${VM_NAME}'..."
    sudo systemctl enable --now "self-hosting-vm-uptime@${VM_NAME}.timer" >/dev/null 2>&1 || \
        echo "WARNING: failed to enable the uptime monitoring timer for '${VM_NAME}'."
fi

echo ""
echo "Waiting for the VM to report a DHCP IP..."

VM_IP=""
for i in $(seq 1 30); do
    VM_IP=$(virsh domifaddr "$VM_NAME" 2>/dev/null | awk '/ipv4/ {print $4}' | cut -d/ -f1 | head -n1)
    if [ -n "$VM_IP" ]; then
        break
    fi
    sleep 2
done

if [ -z "$VM_IP" ] && [ "$EFFECTIVE_MODE" = "nat" ]; then
    # Fallback for NAT: try the DHCP lease table directly.
    VM_IP=$(virsh net-dhcp-leases default 2>/dev/null | awk -v mac="" '/'"$VM_NAME"'/ {print $5}' | cut -d/ -f1 | head -n1)
fi

# ============================================================
# 13. Port forwarding (--forward mode only)
# ============================================================
if [ -n "$FORWARD_RULES" ]; then
    if [ "$EFFECTIVE_MODE" = "bridged" ]; then
        echo ""
        echo "WARNING: --forward was requested, but VM '${VM_NAME}' uses bridged networking."
        echo "         Port forwarding only applies to the NAT network; skipping."
    elif [ -z "$VM_IP" ]; then
        echo ""
        echo "WARNING: could not detect the VM's IP automatically, so port forwarding"
        echo "         rules were NOT applied. Once the VM is up, get its IP with:"
        echo "           virsh domifaddr ${VM_NAME}"
        echo "         and add rules manually, e.g. for host port 2222 -> VM port 22:"
        echo "           sudo iptables -t nat -A PREROUTING -p tcp --dport 2222 -j DNAT --to-destination <VM_IP>:22"
        echo "           sudo iptables -I FORWARD -p tcp -d <VM_IP> --dport 22 -j ACCEPT"
    else
        echo ""
        echo "==> Applying port forwarding rules (host -> ${VM_IP})..."
        IFS=',' read -ra RULES_ARR <<< "$FORWARD_RULES"
        for rule in "${RULES_ARR[@]}"; do
            HOST_PORT="${rule%%:*}"
            VM_PORT="${rule##*:}"
            if sudo iptables -t nat -C PREROUTING -p tcp --dport "$HOST_PORT" -j DNAT --to-destination "${VM_IP}:${VM_PORT}" 2>/dev/null; then
                echo "    ${HOST_PORT} -> ${VM_IP}:${VM_PORT} (DNAT rule already present, skipping)"
            else
                echo "    ${HOST_PORT} -> ${VM_IP}:${VM_PORT}"
                sudo iptables -t nat -A PREROUTING -p tcp --dport "$HOST_PORT" -j DNAT --to-destination "${VM_IP}:${VM_PORT}"
            fi

            if ! sudo iptables -C FORWARD -p tcp -d "$VM_IP" --dport "$VM_PORT" -j ACCEPT 2>/dev/null; then
                sudo iptables -I FORWARD -p tcp -d "$VM_IP" --dport "$VM_PORT" -j ACCEPT
            fi
        done

        if [ "$VM_EXISTS" -eq 1 ] && [ -f "${WORK_DIR}/.guest-firewall-policy" ]; then
            RECORDED_GUEST_FW_POLICY=$(cat "${WORK_DIR}/.guest-firewall-policy")
            if [ "$RECORDED_GUEST_FW_POLICY" = "enabled" ]; then
                echo ""
                echo "WARNING: this VM's guest firewall (ufw) was enabled at creation, but its allow"
                echo "         list can't be updated by rerunning this script (cloud-init only applies"
                echo "         at first boot). The newly forwarded port(s) may still be blocked inside"
                echo "         the guest. Allow them manually, e.g. over SSH:"
                for rule in "${RULES_ARR[@]}"; do
                    echo "           ssh ${VM_USER}@${VM_IP} sudo ufw allow ${rule##*:}/tcp"
                done
            fi
        fi

        echo ""
        echo "NOTE: these iptables rules are NOT persistent across host reboots."
        echo "      To make them permanent, install 'iptables-persistent' (apt) and save with"
        echo "      'sudo netfilter-persistent save', or re-run this script with the same"
        echo "      --forward flag after a reboot."
        echo "      Also note: if the VM's DHCP lease changes, these rules will point to the"
        echo "      wrong IP — check with 'virsh domifaddr ${VM_NAME}' if forwarding stops working."
    fi
fi

# ============================================================
# 13.1 Admin sudo policy summary
# ============================================================
if [ "$VM_EXISTS" -eq 1 ]; then
    FINAL_ADMIN_SUDO_POLICY="$EFFECTIVE_ADMIN_SUDO_POLICY"
else
    FINAL_ADMIN_SUDO_POLICY="$ADMIN_SUDO_POLICY"
fi

if [ "$VM_EXISTS" -eq 0 ] && [ "$ADMIN_PASSWORD_REQUESTED" -eq 1 ]; then
    echo ""
    echo "=================================================="
    echo " Admin sudo password (shown once, also saved to"
    echo " ${WORK_DIR}/admin-password, chmod 600)"
    echo "=================================================="
    echo " ${ADMIN_PASSWORD_PLAINTEXT}"
    echo "=================================================="
fi

if [ "$FINAL_ADMIN_SUDO_POLICY" = "password-required" ]; then
    echo ""
    echo "NOTE: sudo on this VM requires the password above. If it's lost, 'virsh console"
    echo "      ${VM_NAME}' gives host-root guest access independent of SSH/sudo, which can"
    echo "      reset it: run 'passwd ${VM_USER}' from the console (Ctrl+] to exit)."
fi

echo ""
if [ "$EFFECTIVE_MODE" = "bridged" ]; then
    echo "Bridged mode (via '${EFFECTIVE_BRIDGE_IFACE}'): to find the VM's IP (via qemu-guest-agent), run:"
    echo "  virsh domifaddr ${VM_NAME} --source agent"
    echo ""
    echo "Then connect via SSH:"
    echo "  ssh ${VM_USER}@<VM_IP>"
elif [ -n "$FORWARD_RULES" ]; then
    echo "From other devices on your LAN, connect via the host's IP and the forwarded port, e.g.:"
    echo "  ssh -p <HOST_PORT> ${VM_USER}@<HOST_IP>"
    echo ""
    echo "From this host itself, you can still connect directly to the NAT IP:"
    echo "  ssh ${VM_USER}@${VM_IP:-<VM_IP>}"
    echo ""
    echo "To find (or re-check) the VM's IP, run:"
    echo "  virsh domifaddr ${VM_NAME}"
else
    echo "To find the VM's IP, run:"
    echo "  virsh domifaddr ${VM_NAME}"
    echo ""
    echo "Then connect via SSH (from this host):"
    echo "  ssh ${VM_USER}@<VM_IP>"
fi
echo ""
echo "Useful commands:"
echo "  virsh list --all                # list VMs"
echo "  virsh console ${VM_NAME}        # view console (Ctrl+] to exit)"
echo "  virsh shutdown ${VM_NAME}       # power off"
echo "  virsh start ${VM_NAME}          # power on"
echo "  virsh undefine ${VM_NAME} --remove-all-storage   # delete everything"
echo ""
