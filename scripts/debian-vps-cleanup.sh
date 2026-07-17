#!/usr/bin/env bash
#
# debian-vps-cleanup.sh
# Removes what debian-vps-setup.sh created or installed. Three modes:
#   (no flags)    Interactive walkthrough, asking for confirmation before
#                 each step below.
#   --vm-only     Non-interactive. Removes only the VM, its attached storage
#                 (disk + cloud-init seed ISO), and its network reservation
#                 (if any). Preserves the downloaded base cloud image,
#                 installed packages, group membership, the default network,
#                 and the QEMU storage ACL — so a rerun of debian-vps-setup.sh
#                 is fast (no re-download, no reinstall).
#   --purge-all   Non-interactive. Removes everything: the VM, the full
#                 working directory (including the base image), installed
#                 packages, group membership, the default network, and the
#                 QEMU storage ACL granted on $HOME. Refuses to run if any
#                 VM other than the one named by --name still exists, since
#                 purging shared packages/network/groups would break it.
#
# Usage:
#   chmod +x debian-vps-cleanup.sh
#   ./debian-vps-cleanup.sh
#   ./debian-vps-cleanup.sh --vm-only
#   ./debian-vps-cleanup.sh --purge-all
#   ./debian-vps-cleanup.sh --name=app-01 --vm-only   # target one fleet VM
#
set -euo pipefail

VM_NAME="debian-vps"
VM_ONLY=0
PURGE_ALL=0

print_help() {
    cat <<HELP
Usage: $0 [--name=NAME] [--vm-only|--purge-all]

Options:
  --name=NAME  VM to target (default: debian-vps). Use a distinct --name per
               VM when managing a fleet of VMs.
  --vm-only    Non-interactive. Removes only the named VM, its attached
               storage (disk + cloud-init seed ISO), and its network
               reservation (if any). Preserves the downloaded base cloud
               image, installed packages, group membership, the default
               network, and the QEMU storage ACL — so a rerun of
               debian-vps-setup.sh is fast.
  --purge-all  Non-interactive. Removes everything: the VM, the full working
               directory (including the base image), installed packages,
               group membership, the default network, and the QEMU storage
               ACL on \$HOME. Refuses to run — before touching anything — if
               any VM other than the one named by --name still exists;
               remove those first with --vm-only (per VM).
  -h, --help   Show this help.

Without a flag, the script walks through each removal step interactively,
asking for confirmation before each one (including revoking the QEMU
storage ACL).
HELP
}

for arg in "$@"; do
    case "$arg" in
        --name=*)
            VM_NAME="${arg#*=}"
            ;;
        --vm-only)
            VM_ONLY=1
            ;;
        --purge-all)
            PURGE_ALL=1
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

if [ "$VM_ONLY" -eq 1 ] && [ "$PURGE_ALL" -eq 1 ]; then
    echo "ERROR: --vm-only and --purge-all are mutually exclusive."
    exit 1
fi

confirm() {
    local prompt="$1"
    if [ "$VM_ONLY" -eq 1 ] || [ "$PURGE_ALL" -eq 1 ]; then
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

# ============================================================
# 0. --purge-all fleet-safety check
#    Refuses to touch anything if another VM still exists, since purging
#    shared packages/network/groups would break it.
# ============================================================
if [ "$PURGE_ALL" -eq 1 ] && command -v virsh >/dev/null 2>&1; then
    OTHER_VMS=$(virsh list --all --name 2>/dev/null | grep -v '^$' | grep -Fvx "$VM_NAME" || true)
    if [ -n "$OTHER_VMS" ]; then
        echo "ERROR: --purge-all refuses to run while other VMs still exist:"
        echo "$OTHER_VMS" | sed 's/^/  - /'
        echo "       Purging shared packages/network/groups would break them."
        echo "       Remove each one first with: $0 --name=<name> --vm-only"
        exit 1
    fi
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

# The reservation lives on the network, not on the VM instance, so it can be
# released even when the VM itself was never found above (e.g. an earlier
# debian-vps-setup.sh run registered the reservation and then failed before
# actually creating the VM, leaving it orphaned).
if [ "$VM_ONLY" -eq 1 ] && command -v virsh >/dev/null 2>&1 && virsh net-info default >/dev/null 2>&1; then
    echo "==> Releasing '${VM_NAME}'s network reservation (if any)..."
    RESERVATION_LINE=$(virsh net-dumpxml default 2>/dev/null | grep -oP "<host [^>]*/>" | grep -F "name='${VM_NAME}'" || true)
    if [ -n "$RESERVATION_LINE" ]; then
        if virsh net-update default delete ip-dhcp-host "$RESERVATION_LINE" --live --config >/dev/null 2>&1; then
            echo "    Reservation released; the IP/hostname are available for reuse."
        else
            echo "WARNING: could not release the network reservation for '${VM_NAME}'. Inspect with: virsh net-dumpxml default"
        fi
    else
        echo "    No network reservation found for '${VM_NAME}', nothing to release."
    fi
fi
echo ""

# ============================================================
# 2. libvirt's default network (optional)
#    Runs before package purge (step 4) so 'virsh' is still
#    available — purging libvirt-clients removes the binary
#    this step depends on.
# ============================================================
if [ "$VM_ONLY" -ne 1 ]; then
    if command -v virsh >/dev/null 2>&1 && virsh net-info default >/dev/null 2>&1; then
        if confirm "Also remove libvirt's default virtual network (virbr0/'default')?"; then
            virsh net-destroy default 2>/dev/null || true
            virsh net-undefine default 2>/dev/null || true
            echo "    'default' network removed."
        else
            echo "==> Keeping the default virtual network."
        fi
    else
        echo "==> No 'default' libvirt network found, skipping."
    fi
    echo ""
fi

# ============================================================
# 3. Remove the working directory (cloud image, disk, cloud-init)
# ============================================================
if [ "$VM_ONLY" -ne 1 ]; then
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
fi

# ============================================================
# 4. Remove installed packages
# ============================================================
if [ "$VM_ONLY" -ne 1 ]; then
    PACKAGES="qemu-system-x86 qemu-utils libvirt-daemon-system libvirt-clients bridge-utils virtinst cloud-image-utils genisoimage"

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
fi

# ============================================================
# 5. Remove user from the libvirt/kvm groups
# ============================================================
if [ "$VM_ONLY" -ne 1 ]; then
    if confirm "Remove your user ($(whoami)) from the 'libvirt' and 'kvm' groups?"; then
        sudo gpasswd -d "$(whoami)" libvirt 2>/dev/null || true
        sudo gpasswd -d "$(whoami)" kvm 2>/dev/null || true
        echo "    Done (full effect only after logout/login)."
    else
        echo "==> Skipping group removal."
    fi
    echo ""
fi

# ============================================================
# 6. QEMU storage ACL on $HOME
# ============================================================
if [ "$VM_ONLY" -ne 1 ]; then
    if confirm "Revoke the 'libvirt-qemu' traversal ACL on \$HOME? (skip if other local VMs might still need it)"; then
        if sudo setfacl -x u:libvirt-qemu "$HOME" 2>/dev/null; then
            echo "    ACL entry removed."
        else
            echo "WARNING: could not revoke the 'libvirt-qemu' ACL entry on \$HOME (it may not exist, or the filesystem may not support ACLs)."
        fi
    else
        echo "==> Keeping the 'libvirt-qemu' ACL entry on \$HOME."
    fi
    echo ""
fi

echo ""
echo "=================================================="
echo " Cleanup finished."
echo "=================================================="
echo ""
if [ "$VM_ONLY" -eq 1 ]; then
    echo "Notes:"
    echo "  - Only the VM and its attached storage were removed."
    echo "  - The downloaded base cloud image, installed packages, group membership,"
    echo "    the default network, and the QEMU storage ACL on \$HOME were left in place."
    echo "  - Rerun debian-vps-setup.sh to recreate the VM without re-downloading or"
    echo "    reinstalling anything."
else
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
fi
echo ""
