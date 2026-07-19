#!/usr/bin/env bash
#
# debian-vm-backup.sh
# Snapshot/backup mechanism for a VM's disk, on top of what
# debian-vm-setup.sh/debian-vm-cleanup.sh already manage.
#
# Two distinct, related operations:
#   snapshot          A single, external, disk-only libvirt snapshot — a
#                      fast local rollback point. Lives on the SAME disk as
#                      the VM: it protects against "I broke something," not
#                      against disk/host loss. One at a time, not a stack.
#   backup             A compressed, point-in-time copy of the VM's disk
#                      written to a separate destination (default
#                      $HOME/vm-backups/<name>/, overridable with --dest).
#                      Works whether the VM is running or stopped. This is
#                      what actually protects against losing the VM.
#
# Subcommands:
#   snapshot          --name=<vm>
#   snapshot-restore  --name=<vm>            (destructive, discards writes
#                                              since the snapshot; prompts)
#   snapshot-delete   --name=<vm>            (merges writes back, keeps them)
#   backup            --name=<vm> [--dest=<dir>] [--keep=N]
#   backup-list       --name=<vm> [--dest=<dir>]
#   backup-restore    --name=<vm> --file=<path> [--dest=<dir>]
#                                              (destructive, prompts)
#
# Usage:
#   chmod +x debian-vm-backup.sh
#   ./debian-vm-backup.sh snapshot --name=app-01
#   ./debian-vm-backup.sh backup --name=app-01 --keep=5
#   ./debian-vm-backup.sh backup-list --name=app-01
#   ./debian-vm-backup.sh --help
#
set -euo pipefail

DEFAULT_DEST_ROOT="$HOME/vm-backups"
SNAPSHOT_NAME="self-hosting-snapshot"
BACKUP_TMP_SNAPSHOT_NAME="self-hosting-backup-tmp"

print_help() {
    cat <<HELP
Usage: $0 <subcommand> --name=<vm> [options]

Subcommands:
  snapshot          Create a single external, disk-only rollback point for
                    the VM. Fails if one already exists (one at a time).
  snapshot-restore  Revert the VM's disk to the pre-snapshot state,
                    discarding all writes made since. Prompts for
                    confirmation. Stops the VM first if running, restarts
                    it afterward if it was running before.
  snapshot-delete   Merge the active snapshot's overlay back into the VM's
                    disk, KEEPING all writes made since (unlike
                    snapshot-restore).
  backup            Write a compressed, point-in-time copy of the VM's disk
                    to a separate destination. Works live or stopped.
  backup-list       List backups available for the VM at its destination.
  backup-restore    Replace the VM's current disk with the contents of a
                    chosen backup. Prompts for confirmation. Stops the VM
                    first if running, restarts it afterward if it was
                    running before.

Options:
  --name=NAME   VM to target (default: debian-vm).
  --dest=DIR    Backup destination directory (default: \$HOME/vm-backups/<name>/).
  --keep=N      With 'backup': after a successful backup, delete this VM's
                own backups beyond the N most recent. No pruning by default.
  --file=PATH   With 'backup-restore': the backup file to restore from.
  -h, --help    Show this help.

Notes:
  - The default backup destination is typically on the same physical disk
    as the VM's own storage. It does NOT protect against a disk failure
    unless --dest points somewhere else (e.g. an external/network mount).
  - No remote/cloud upload is built in; --dest accepts any local path.
  - snapshot-restore and backup-restore always prompt; there is no
    non-interactive bypass flag.
HELP
}

if [ "$#" -eq 0 ]; then
    print_help
    exit 1
fi

SUBCOMMAND="$1"
shift

if [ "$SUBCOMMAND" = "-h" ] || [ "$SUBCOMMAND" = "--help" ]; then
    print_help
    exit 0
fi

case "$SUBCOMMAND" in
    snapshot|snapshot-restore|snapshot-delete|backup|backup-list|backup-restore)
        ;;
    *)
        echo "ERROR: unknown subcommand: $SUBCOMMAND"
        print_help
        exit 1
        ;;
esac

VM_NAME="debian-vm"
DEST_DIR=""
KEEP_N=""
BACKUP_FILE=""

for arg in "$@"; do
    case "$arg" in
        --name=*)
            VM_NAME="${arg#*=}"
            ;;
        --dest=*)
            DEST_DIR="${arg#*=}"
            ;;
        --keep=*)
            KEEP_N="${arg#*=}"
            ;;
        --file=*)
            BACKUP_FILE="${arg#*=}"
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

if [ -z "$DEST_DIR" ]; then
    DEST_DIR="${DEFAULT_DEST_ROOT}/${VM_NAME}"
fi

if [ "$(id -u)" -eq 0 ]; then
    echo "ERROR: don't run this script as root. Run it as your normal user (it will use sudo when needed)."
    exit 1
fi

if ! command -v virsh >/dev/null 2>&1; then
    echo "ERROR: 'virsh' was not found. Run debian-vm-setup.sh first."
    exit 1
fi

# backup-list is a plain filesystem listing — deliberately allowed even for
# a VM that no longer exists, since "what backups do I have for a VM I
# already deleted" is exactly the case backups exist for.
if [ "$SUBCOMMAND" != "backup-list" ] && ! virsh dominfo "$VM_NAME" >/dev/null 2>&1; then
    echo "ERROR: no VM named '${VM_NAME}' found. Check with: virsh list --all"
    exit 1
fi

# ============================================================
# Shared helpers
# ============================================================

# Prints "<target> <path>" for the VM's current active disk (its "disk"
# device, e.g. vda — as opposed to the "cdrom" device used for the
# cloud-init seed ISO). Columns from `virsh domblklist --details` are:
# Type Device Target Source.
get_disk_info() {
    virsh domblklist "$VM_NAME" --details 2>/dev/null | awk '$1=="file" && $2=="disk"{print $3, $4; exit}'
}

confirm_destructive() {
    local prompt="$1"
    read -r -p "${prompt} [y/N] " resp
    case "$resp" in
        [yY]|[yY][eE][sS]) return 0 ;;
        *) return 1 ;;
    esac
}

# Stops the VM if running, recording prior state in PRIOR_STATE, so the
# caller can restart it afterward with restore_prior_state.
stop_if_running() {
    PRIOR_STATE=$(virsh domstate "$VM_NAME" 2>/dev/null || echo "unknown")
    if [ "$PRIOR_STATE" = "running" ]; then
        echo "==> Stopping '${VM_NAME}'..."
        virsh destroy "$VM_NAME" >/dev/null 2>&1 || true
    fi
}

restore_prior_state() {
    if [ "$PRIOR_STATE" = "running" ]; then
        echo "==> Restarting '${VM_NAME}'..."
        virsh start "$VM_NAME"
    fi
}

# ============================================================
# snapshot
# ============================================================
cmd_snapshot() {
    if virsh snapshot-list "$VM_NAME" --name 2>/dev/null | grep -qF "$SNAPSHOT_NAME"; then
        echo "ERROR: VM '${VM_NAME}' already has an active snapshot ('${SNAPSHOT_NAME}')."
        echo "       Run 'snapshot-restore' or 'snapshot-delete' first."
        exit 1
    fi

    echo "==> Creating an external, disk-only snapshot of '${VM_NAME}'..."
    virsh snapshot-create-as --domain "$VM_NAME" "$SNAPSHOT_NAME" --disk-only --atomic
    echo "    OK: snapshot '${SNAPSHOT_NAME}' created. VM keeps running (if it was)."
    echo "    Restore with: $0 snapshot-restore --name=${VM_NAME}"
    echo "    Discard (keep changes) with: $0 snapshot-delete --name=${VM_NAME}"
}

# ============================================================
# snapshot-restore (destructive: discards writes since the snapshot)
# ============================================================
cmd_snapshot_restore() {
    if ! virsh snapshot-list "$VM_NAME" --name 2>/dev/null | grep -qF "$SNAPSHOT_NAME"; then
        echo "ERROR: VM '${VM_NAME}' has no active snapshot to restore."
        exit 1
    fi

    echo "This will DISCARD all writes made to '${VM_NAME}' since the snapshot was taken."
    if ! confirm_destructive "Revert '${VM_NAME}' to its pre-snapshot state?"; then
        echo "==> Aborted, no changes made."
        exit 0
    fi

    stop_if_running
    echo "==> Reverting to the pre-snapshot state..."
    virsh snapshot-revert "$VM_NAME" "$SNAPSHOT_NAME"
    virsh snapshot-delete "$VM_NAME" "$SNAPSHOT_NAME" --metadata >/dev/null 2>&1 || true
    restore_prior_state
    echo "    OK: reverted."
}

# ============================================================
# snapshot-delete (keeps writes made since the snapshot)
# ============================================================
cmd_snapshot_delete() {
    if ! virsh snapshot-list "$VM_NAME" --name 2>/dev/null | grep -qF "$SNAPSHOT_NAME"; then
        echo "==> No active snapshot for '${VM_NAME}', nothing to delete."
        exit 0
    fi

    read -r DISK_TARGET _ < <(get_disk_info)
    if [ -z "$DISK_TARGET" ]; then
        echo "ERROR: could not determine '${VM_NAME}'s disk target device."
        exit 1
    fi

    echo "==> Merging the snapshot overlay back into '${VM_NAME}'s disk (keeping all changes)..."
    virsh blockcommit "$VM_NAME" "$DISK_TARGET" --active --pivot --wait
    virsh snapshot-delete "$VM_NAME" "$SNAPSHOT_NAME" --metadata >/dev/null 2>&1 || true
    echo "    OK: snapshot merged and removed; current state preserved."
}

# ============================================================
# backup (works live or stopped)
# ============================================================
cmd_backup() {
    # A live backup's blockcommit has no --base, so it flattens the ENTIRE
    # disk chain down to the root. If a named rollback snapshot (from the
    # `snapshot` subcommand) is already active, that would silently merge
    # it away — refuse instead, matching `snapshot`'s own "one at a time"
    # rule, rather than corrupting a rollback point the operator is relying on.
    if virsh snapshot-list "$VM_NAME" --name 2>/dev/null | grep -qF "$SNAPSHOT_NAME"; then
        echo "ERROR: VM '${VM_NAME}' has an active rollback snapshot ('${SNAPSHOT_NAME}')."
        echo "       Running 'backup' now would merge it away. Run 'snapshot-restore' or"
        echo "       'snapshot-delete' first, then back up."
        exit 1
    fi

    mkdir -p "$DEST_DIR"
    TIMESTAMP=$(date +%Y%m%d-%H%M%S)
    DEST_FILE="${DEST_DIR}/${VM_NAME}-${TIMESTAMP}.qcow2"

    STATE=$(virsh domstate "$VM_NAME" 2>/dev/null || echo "unknown")
    read -r DISK_TARGET DISK_PATH < <(get_disk_info)
    if [ -z "$DISK_PATH" ]; then
        echo "ERROR: could not determine '${VM_NAME}'s disk path."
        exit 1
    fi

    if [ "$STATE" != "running" ]; then
        echo "==> VM is stopped; copying its disk directly..."
        qemu-img convert -O qcow2 -c "$DISK_PATH" "$DEST_FILE"
    else
        echo "==> VM is running; taking a live, consistent backup..."
        FROZEN=0
        if timeout 10 virsh domfsfreeze "$VM_NAME" >/dev/null 2>&1; then
            FROZEN=1
            echo "    Guest filesystems frozen for a consistent snapshot point."
        else
            echo "    WARNING: could not freeze guest filesystems (agent unresponsive?)."
            echo "             Proceeding with a crash-consistent (not fully consistent) backup."
        fi

        SOURCE_DISK="$DISK_PATH"
        virsh snapshot-create-as --domain "$VM_NAME" "$BACKUP_TMP_SNAPSHOT_NAME" --disk-only --atomic

        if [ "$FROZEN" -eq 1 ]; then
            virsh domfsthaw "$VM_NAME" >/dev/null 2>&1 || true
        fi

        echo "==> Copying the frozen disk state to ${DEST_FILE}..."
        qemu-img convert -O qcow2 -c "$SOURCE_DISK" "$DEST_FILE"

        echo "==> Merging the temporary snapshot back into the live disk..."
        if ! virsh blockcommit "$VM_NAME" "$DISK_TARGET" --active --pivot --wait; then
            echo "ERROR: blockcommit failed. The VM may be running on a lingering overlay."
            echo "       Inspect with: virsh blockjob ${VM_NAME} ${DISK_TARGET} --info"
            exit 1
        fi
        virsh snapshot-delete "$VM_NAME" "$BACKUP_TMP_SNAPSHOT_NAME" --metadata >/dev/null 2>&1 || true
    fi

    echo "    OK: backup written to ${DEST_FILE}"

    if [ -n "$KEEP_N" ]; then
        echo "==> Applying retention (keep ${KEEP_N} most recent)..."
        # shellcheck disable=SC2012
        ls -1t "${DEST_DIR}/${VM_NAME}-"*.qcow2 2>/dev/null | tail -n +"$((KEEP_N + 1))" | while IFS= read -r old; do
            echo "    Removing old backup: ${old}"
            rm -f "$old"
        done
    fi
}

# ============================================================
# backup-list
# ============================================================
cmd_backup_list() {
    if [ ! -d "$DEST_DIR" ]; then
        echo "No backups found for '${VM_NAME}' at ${DEST_DIR}."
        exit 0
    fi
    FOUND=0
    for f in "${DEST_DIR}/${VM_NAME}-"*.qcow2; do
        [ -e "$f" ] || continue
        FOUND=1
        printf '%s\t%s\n' "$(date -r "$f" '+%Y-%m-%d %H:%M:%S')" "$f"
    done
    if [ "$FOUND" -eq 0 ]; then
        echo "No backups found for '${VM_NAME}' at ${DEST_DIR}."
    fi
}

# ============================================================
# backup-restore (destructive: replaces the current disk)
# ============================================================
cmd_backup_restore() {
    if [ -z "$BACKUP_FILE" ]; then
        echo "ERROR: --file=<path> is required for backup-restore."
        exit 1
    fi
    if [ ! -f "$BACKUP_FILE" ]; then
        echo "ERROR: backup file not found: ${BACKUP_FILE}"
        exit 1
    fi

    echo "This will REPLACE '${VM_NAME}'s current disk with the contents of:"
    echo "  ${BACKUP_FILE}"
    if ! confirm_destructive "Proceed?"; then
        echo "==> Aborted, no changes made."
        exit 0
    fi

    read -r _ DISK_PATH < <(get_disk_info)
    if [ -z "$DISK_PATH" ]; then
        echo "ERROR: could not determine '${VM_NAME}'s disk path."
        exit 1
    fi

    stop_if_running
    echo "==> Replacing the disk..."
    qemu-img convert -O qcow2 "$BACKUP_FILE" "$DISK_PATH"
    restore_prior_state
    echo "    OK: restored from ${BACKUP_FILE}"
}

case "$SUBCOMMAND" in
    snapshot)
        cmd_snapshot
        ;;
    snapshot-restore)
        cmd_snapshot_restore
        ;;
    snapshot-delete)
        cmd_snapshot_delete
        ;;
    backup)
        cmd_backup
        ;;
    backup-list)
        cmd_backup_list
        ;;
    backup-restore)
        cmd_backup_restore
        ;;
esac
