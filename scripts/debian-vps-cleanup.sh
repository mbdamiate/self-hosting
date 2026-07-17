#!/usr/bin/env bash
#
# debian-vps-cleanup.sh
# Removes everything that setup-debian-vps.sh created or installed:
#   - the VM (definition + disks)
#   - the working directory with the cloud image, VM disk, and cloud-init files
#   - the installed KVM/QEMU/libvirt packages
#   - your user's membership in the libvirt/kvm groups
#
# Runs in interactive mode by default, asking before each destructive step.
# Use --yes to skip all confirmations.
#
# Usage:
#   chmod +x debian-vps-cleanup.sh
#   ./debian-vps-cleanup.sh
#   ./debian-vps-cleanup.sh --yes
#
set -euo pipefail

VM_NAME="debian-vps"
WORK_DIR="$HOME/vms/${VM_NAME}"
ASSUME_YES=0

for arg in "$@"; do
    case "$arg" in
        --yes|-y)
            ASSUME_YES=1
            ;;
        -h|--help)
            echo "Usage: $0 [--yes]"
            echo "  --yes   Don't ask for confirmation at each step (careful!)."
            exit 0
            ;;
        *)
            echo "ERROR: unknown argument: $arg"
            exit 1
            ;;
    esac
done

confirm() {
    local prompt="$1"
    if [ "$ASSUME_YES" -eq 1 ]; then
        return 0
    fi
    read -r -p "${prompt} [y/N] " resp
    case "$resp" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) return 1 ;;
    esac
}

if [ "$(id -u)" -eq 0 ]; then
    echo "ERROR: don't run this script as root. Run it as your normal user (it will use sudo when needed)."
    exit 1
fi

echo "=================================================="
echo " Cleaning up the simulated VPS environment"
echo "=================================================="
echo ""

# ============================================================
# 1. Remove the VM (if it exists)
# ============================================================
if command -v virsh >/dev/null 2>&1 && virsh dominfo "$VM_NAME" >/dev/null 2>&1; then
    if confirm "Remove the VM '${VM_NAME}' (definition + disks)?"; then
        echo "==> Stopping the VM (if running)..."
        virsh destroy "$VM_NAME" >/dev/null 2>&1 || true

        echo "==> Removing the VM definition and all associated disks..."
        virsh undefine "$VM_NAME" --remove-all-storage --nvram >/dev/null 2>&1 \
            || virsh undefine "$VM_NAME" --remove-all-storage \
            || echo "WARNING: could not remove it automatically. Check manually with 'virsh list --all'."
        echo "    VM removed."
    else
        echo "==> Skipping VM removal."
    fi
else
    echo "==> No VM named '${VM_NAME}' found, skipping."
fi
echo ""

# ============================================================
# 2. Remove the working directory (cloud image, disk, cloud-init)
# ============================================================
if [ -d "$WORK_DIR" ]; then
    echo "The working directory holds the base image, the VM disk, and the cloud-init files:"
    echo "  ${WORK_DIR}"
    if confirm "Delete this directory and all its contents?"; then
        rm -rf "$WORK_DIR"
        echo "    Directory removed."
    else
        echo "==> Skipping working directory removal."
    fi
else
    echo "==> Working directory '${WORK_DIR}' not found, skipping."
fi
echo ""

# ============================================================
# 3. Remove installed packages
# ============================================================
PACKAGES="qemu-system-x86 libvirt-daemon-system libvirt-clients bridge-utils virtinst cloud-image-utils genisoimage"

echo "Packages that will be removed (purge):"
echo "  ${PACKAGES}"
echo "WARNING: if you use KVM/libvirt for other VMs besides this one, do NOT remove the packages."
if confirm "Remove these packages from the system?"; then
    echo "==> Stopping the libvirtd service..."
    sudo systemctl stop libvirtd 2>/dev/null || true
    sudo systemctl disable libvirtd 2>/dev/null || true

    echo "==> Removing packages..."
    sudo apt purge -y $PACKAGES
    sudo apt autoremove -y

    echo "    Packages removed."
else
    echo "==> Skipping package removal."
fi
echo ""

# ============================================================
# 4. Remove user from the libvirt/kvm groups
# ============================================================
if confirm "Remove your user ($(whoami)) from the 'libvirt' and 'kvm' groups?"; then
    sudo gpasswd -d "$(whoami)" libvirt 2>/dev/null || true
    sudo gpasswd -d "$(whoami)" kvm 2>/dev/null || true
    echo "    Done (full effect only after logout/login)."
else
    echo "==> Skipping group removal."
fi
echo ""

# ============================================================
# 5. libvirt's default network (optional)
# ============================================================
if command -v virsh >/dev/null 2>&1 && virsh net-info default >/dev/null 2>&1; then
    if confirm "Also remove libvirt's default virtual network (virbr0/'default')?"; then
        virsh net-destroy default 2>/dev/null || true
        virsh net-undefine default 2>/dev/null || true
        echo "    'default' network removed."
    else
        echo "==> Keeping the default virtual network."
    fi
fi

echo ""
echo "=================================================="
echo " Cleanup finished."
echo "=================================================="
echo ""
echo "Notes:"
echo "  - If you removed the groups, log out/in for the change to fully apply."
echo "  - Your SSH key in ~/.ssh was NOT deleted (it may be used elsewhere)."
echo "  - If you skipped package removal, KVM/libvirt remain installed on the system."
echo "  - If you used --forward in setup-debian-vps.sh, the iptables port-forwarding"
echo "    rules were NOT removed automatically (this script can't tell which are yours)."
echo "    List them with 'sudo iptables -t nat -L PREROUTING -n --line-numbers' and"
echo "    'sudo iptables -L FORWARD -n --line-numbers', then remove with:"
echo "      sudo iptables -t nat -D PREROUTING <line-number>"
echo "      sudo iptables -D FORWARD <line-number>"
echo ""
