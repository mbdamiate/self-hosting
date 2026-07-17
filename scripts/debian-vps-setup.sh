#!/usr/bin/env bash
#
# debian-vps-setup.sh
# Checks prerequisites, installs KVM/QEMU/libvirt, and creates a Debian 12 VM
# using the official cloud image + cloud-init, simulating a real VPS
# (the same technique used by providers like DigitalOcean/Linode).
#
# Usage:
#   chmod +x debian-vps-setup.sh
#   ./debian-vps-setup.sh                              # NAT networking (default, simplest)
#   ./debian-vps-setup.sh --bridge=eth0                # bridged networking (macvtap) over interface eth0
#   ./debian-vps-setup.sh --forward=2222:22,8080:80    # NAT + port forwarding from the host
#   ./debian-vps-setup.sh --help
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
VM_NAME="debian-vps"
VM_RAM_MB=2048          # RAM in MB (e.g. a basic VPS plan)
VM_VCPUS=2              # vCPUs
VM_DISK_GB=20           # Disk size in GB
VM_USER="admin"         # User created inside the VM
VM_HOSTNAME="debian-vps"
SSH_KEY_PATH="$HOME/.ssh/id_ed25519"   # Public key used for SSH access
WORK_DIR="$HOME/vms/${VM_NAME}"
CLOUD_IMG_URL="https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2"

# ============================================================
# 0. Argument parsing
# ============================================================
BRIDGE_IFACE=""
FORWARD_RULES=""

print_help() {
    cat <<HELP
Usage: $0 [options]

Options:
  --bridge=IFACE     Use bridged networking (macvtap) over the physical
                      interface IFACE (e.g. --bridge=eth0). The VM gets an IP
                      from your router via DHCP. Requires a WIRED interface —
                      not supported over Wi-Fi (hardware/driver limitation).
  --forward=RULES     NAT + port forwarding. RULES is a comma-separated list
                      of HOST_PORT:VM_PORT pairs (e.g. 2222:22,8080:80).
                      Works fine over Wi-Fi. Applied after the VM gets its
                      NAT IP from libvirt's DHCP.
  -h, --help          Show this help.

If neither --bridge nor --forward is given, the VM only gets a NAT IP
reachable from the host itself (e.g. via 'ssh admin@<nat-ip>' from this
machine).
HELP
}

for arg in "$@"; do
    case "$arg" in
        --bridge=*)
            BRIDGE_IFACE="${arg#*=}"
            ;;
        --forward=*)
            FORWARD_RULES="${arg#*=}"
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

if [ -n "$BRIDGE_IFACE" ] && [ -n "$FORWARD_RULES" ]; then
    echo "ERROR: --bridge and --forward are mutually exclusive (forwarding only makes sense on NAT)."
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
    libvirt-daemon-system \
    libvirt-clients \
    bridge-utils \
    virtinst \
    cloud-image-utils \
    genisoimage \
    wget \
    openssh-client

echo "==> Adding your user to the libvirt and kvm groups..."
sudo usermod -aG libvirt "$(whoami)"
sudo usermod -aG kvm "$(whoami)"

echo "==> Enabling and starting the libvirtd service..."
sudo systemctl enable --now libvirtd

# If the user was just added to the groups, that only takes effect in a
# new session. We use sg to run the rest of the script with the groups
# already applied, without needing a logout/login.
if ! groups | grep -qw libvirt; then
    echo "==> Groups not applied to this session yet. Re-running with sg..."
    printf -v QUOTED_ARGS '%q ' "$0" "$@"
    exec sg libvirt -c "sg kvm -c \"${QUOTED_ARGS}\""
fi

# ============================================================
# 3. SSH key check
# ============================================================
if [ ! -f "${SSH_KEY_PATH}.pub" ]; then
    echo "==> No SSH key found at ${SSH_KEY_PATH}.pub"
    echo "==> Generating a new key pair..."
    ssh-keygen -t ed25519 -f "${SSH_KEY_PATH}" -N "" -C "${VM_USER}@${VM_HOSTNAME}"
fi
SSH_PUB_KEY=$(cat "${SSH_KEY_PATH}.pub")

# ============================================================
# 4. Downloading the Debian 12 cloud image
# ============================================================
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

IMG_FILE="debian-12-generic-amd64.qcow2"
VM_DISK="${VM_NAME}.qcow2"

if [ ! -f "$IMG_FILE" ]; then
    echo "==> Downloading the official Debian 12 cloud image..."
    wget -O "$IMG_FILE" "$CLOUD_IMG_URL"
else
    echo "==> Cloud image already downloaded, skipping."
fi

echo "==> Creating the VM disk (copy from the base image) and resizing to ${VM_DISK_GB}G..."
cp "$IMG_FILE" "$VM_DISK"
qemu-img resize "$VM_DISK" "${VM_DISK_GB}G"

# ============================================================
# 5. Creating the cloud-init configuration (user-data / meta-data)
# ============================================================
echo "==> Generating cloud-init configuration..."

cat > user-data <<EOF
#cloud-config
hostname: ${VM_HOSTNAME}
manage_etc_hosts: true
users:
  - name: ${VM_USER}
    groups: sudo
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ${SSH_PUB_KEY}
ssh_pwauth: false
package_update: true
package_upgrade: false
packages:
  - qemu-guest-agent
runcmd:
  - systemctl enable --now qemu-guest-agent
EOF

cat > meta-data <<EOF
instance-id: ${VM_NAME}-$(date +%s)
local-hostname: ${VM_HOSTNAME}
EOF

echo "==> Generating the cloud-init seed ISO..."
cloud-localds seed.iso user-data meta-data

# ============================================================
# 6. Creating the VM via virt-install
# ============================================================
echo "==> Checking if a VM with this name already exists..."
if virsh dominfo "$VM_NAME" >/dev/null 2>&1; then
    echo "ERROR: a VM named '${VM_NAME}' already exists. Remove it first (virsh undefine ${VM_NAME} --remove-all-storage) or change VM_NAME in the script."
    exit 1
fi

if [ -n "$BRIDGE_IFACE" ]; then
    NETWORK_ARG="type=direct,source=${BRIDGE_IFACE},source_mode=bridge,model=virtio"
    echo "==> Creating the VM (bridged networking via macvtap over ${BRIDGE_IFACE})..."
else
    NETWORK_ARG="network=default,model=virtio"
    echo "==> Creating the VM (NAT networking via virbr0)..."
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
    --noautoconsole

echo ""
echo "=================================================="
echo " VM '${VM_NAME}' created successfully!"
echo "=================================================="
echo ""
echo "Waiting for cloud-init to finish and the VM to get a DHCP IP..."

VM_IP=""
for i in $(seq 1 30); do
    VM_IP=$(virsh domifaddr "$VM_NAME" 2>/dev/null | awk '/ipv4/ {print $4}' | cut -d/ -f1 | head -n1)
    if [ -n "$VM_IP" ]; then
        break
    fi
    sleep 2
done

if [ -z "$VM_IP" ] && [ -z "$BRIDGE_IFACE" ]; then
    # Fallback for NAT: try the DHCP lease table directly.
    VM_IP=$(virsh net-dhcp-leases default 2>/dev/null | awk -v mac="" '/'"$VM_NAME"'/ {print $5}' | cut -d/ -f1 | head -n1)
fi

# ============================================================
# 7. Port forwarding (--forward mode only)
# ============================================================
if [ -n "$FORWARD_RULES" ]; then
    if [ -z "$VM_IP" ]; then
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
            echo "    ${HOST_PORT} -> ${VM_IP}:${VM_PORT}"
            sudo iptables -t nat -A PREROUTING -p tcp --dport "$HOST_PORT" -j DNAT --to-destination "${VM_IP}:${VM_PORT}"
            sudo iptables -I FORWARD -p tcp -d "$VM_IP" --dport "$VM_PORT" -j ACCEPT
        done
        echo ""
        echo "NOTE: these iptables rules are NOT persistent across host reboots."
        echo "      To make them permanent, install 'iptables-persistent' (apt) and save with"
        echo "      'sudo netfilter-persistent save', or re-run this script with the same"
        echo "      --forward flag after a reboot."
        echo "      Also note: if the VM's DHCP lease changes, these rules will point to the"
        echo "      wrong IP — check with 'virsh domifaddr ${VM_NAME}' if forwarding stops working."
    fi
fi

echo ""
if [ -n "$BRIDGE_IFACE" ]; then
    echo "Bridged mode: to find the VM's IP (via qemu-guest-agent), run:"
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
