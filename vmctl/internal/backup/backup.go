// Package backup implements `vmctl backup`'s six subcommands
// (snapshot, snapshot-restore, snapshot-delete, backup, backup-list,
// backup-restore), porting debian-vm-backup.sh per the vm-disk-snapshot and
// vm-disk-backup specs.
package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vmctl/internal/cli"
	"vmctl/internal/domblk"
	"vmctl/internal/execrunner"
)

const (
	snapshotName          = "self-hosting-snapshot"
	backupTmpSnapshotName = "self-hosting-backup-tmp"
)

// Options mirrors the flags debian-vm-backup.sh accepts.
type Options struct {
	Subcommand string
	Name       string
	Dest       string
	Keep       int // 0 means --keep was not given
	File       string
}

// ValidSubcommands lists the subcommands debian-vm-backup.sh accepted.
var ValidSubcommands = []string{"snapshot", "snapshot-restore", "snapshot-delete", "backup", "backup-list", "backup-restore"}

func isValidSubcommand(s string) bool {
	for _, v := range ValidSubcommands {
		if v == s {
			return true
		}
	}
	return false
}

// Run dispatches to the requested subcommand.
func Run(ctx context.Context, r execrunner.Runner, out io.Writer, in io.Reader, opts Options) error {
	if !isValidSubcommand(opts.Subcommand) {
		return fmt.Errorf("unknown subcommand: %s", opts.Subcommand)
	}

	name := opts.Name
	if name == "" {
		name = cli.DefaultVMName
	}
	dest := opts.Dest
	if dest == "" {
		home, _ := os.UserHomeDir()
		dest = filepath.Join(home, "vm-backups", name)
	}

	// backup-list is a plain filesystem listing, deliberately allowed even
	// for a VM that no longer exists.
	if opts.Subcommand != "backup-list" {
		if _, err := r.Run(ctx, "virsh", "dominfo", name); err != nil {
			return fmt.Errorf("no VM named '%s' found. Check with: virsh list --all", name)
		}
	}

	switch opts.Subcommand {
	case "snapshot":
		return cmdSnapshot(ctx, r, out, name)
	case "snapshot-restore":
		return cmdSnapshotRestore(ctx, r, out, in, name)
	case "snapshot-delete":
		return cmdSnapshotDelete(ctx, r, out, name)
	case "backup":
		return cmdBackup(ctx, r, out, name, dest, opts.Keep)
	case "backup-list":
		return cmdBackupList(out, name, dest)
	case "backup-restore":
		return cmdBackupRestore(ctx, r, out, in, name, opts.File)
	}
	return nil
}

// getDiskInfo finds the VM's active disk device via `virsh domblklist
// --details`.
func getDiskInfo(ctx context.Context, r execrunner.Runner, name string) (target, path string, err error) {
	output, err := r.Run(ctx, "virsh", "domblklist", name, "--details")
	if err != nil {
		return "", "", err
	}
	target, path = domblk.FindDisk(string(output))
	return target, path, nil
}

func hasSnapshot(ctx context.Context, r execrunner.Runner, name, snapName string) bool {
	output, err := r.Run(ctx, "virsh", "snapshot-list", name, "--name")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == snapName {
			return true
		}
	}
	return false
}

func domState(ctx context.Context, r execrunner.Runner, name string) string {
	output, err := r.Run(ctx, "virsh", "domstate", name)
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func stopIfRunning(ctx context.Context, r execrunner.Runner, out io.Writer, name string) (priorState string) {
	priorState = domState(ctx, r, name)
	if priorState == "running" {
		fmt.Fprintf(out, "==> Stopping '%s'...\n", name)
		_, _ = r.Run(ctx, "virsh", "destroy", name)
	}
	return priorState
}

func restorePriorState(ctx context.Context, r execrunner.Runner, out io.Writer, name, priorState string) error {
	if priorState != "running" {
		return nil
	}
	fmt.Fprintf(out, "==> Restarting '%s'...\n", name)
	_, err := r.Run(ctx, "virsh", "start", name)
	return err
}

func cmdSnapshot(ctx context.Context, r execrunner.Runner, out io.Writer, name string) error {
	if hasSnapshot(ctx, r, name, snapshotName) {
		return fmt.Errorf("VM '%s' already has an active snapshot ('%s'). Run 'snapshot-restore' or 'snapshot-delete' first", name, snapshotName)
	}

	fmt.Fprintf(out, "==> Creating an external, disk-only snapshot of '%s'...\n", name)
	if _, err := r.Run(ctx, "virsh", "snapshot-create-as", "--domain", name, snapshotName, "--disk-only", "--atomic"); err != nil {
		return err
	}
	fmt.Fprintf(out, "    OK: snapshot '%s' created. VM keeps running (if it was).\n", snapshotName)
	fmt.Fprintf(out, "    Restore with: vmctl backup snapshot-restore --name=%s\n", name)
	fmt.Fprintf(out, "    Discard (keep changes) with: vmctl backup snapshot-delete --name=%s\n", name)
	return nil
}

func cmdSnapshotRestore(ctx context.Context, r execrunner.Runner, out io.Writer, in io.Reader, name string) error {
	if !hasSnapshot(ctx, r, name, snapshotName) {
		return fmt.Errorf("VM '%s' has no active snapshot to restore", name)
	}

	fmt.Fprintf(out, "This will DISCARD all writes made to '%s' since the snapshot was taken.\n", name)
	if !cli.Confirm(out, in, fmt.Sprintf("Revert '%s' to its pre-snapshot state?", name), false) {
		fmt.Fprintln(out, "==> Aborted, no changes made.")
		return nil
	}

	// Capture the overlay's path before reverting: once reverted, the
	// domain's active disk points back at the base image, and
	// `snapshot-delete --metadata` only removes libvirt's own bookkeeping
	// — it never deletes the overlay file itself, leaving it orphaned on
	// disk to block a future snapshot with the same name ("external
	// snapshot file ... already exists"). Found via real-host testing
	// (2026-07-20).
	_, overlayPath, _ := getDiskInfo(ctx, r, name)

	priorState := stopIfRunning(ctx, r, out, name)
	fmt.Fprintln(out, "==> Reverting to the pre-snapshot state...")
	if _, err := r.Run(ctx, "virsh", "snapshot-revert", name, snapshotName); err != nil {
		return err
	}
	_, _ = r.Run(ctx, "virsh", "snapshot-delete", name, snapshotName, "--metadata")
	if overlayPath != "" {
		_, _ = r.Run(ctx, "rm", "-f", overlayPath)
	}
	if err := restorePriorState(ctx, r, out, name, priorState); err != nil {
		return err
	}
	fmt.Fprintln(out, "    OK: reverted.")
	return nil
}

func cmdSnapshotDelete(ctx context.Context, r execrunner.Runner, out io.Writer, name string) error {
	if !hasSnapshot(ctx, r, name, snapshotName) {
		fmt.Fprintf(out, "==> No active snapshot for '%s', nothing to delete.\n", name)
		return nil
	}

	diskTarget, overlayPath, err := getDiskInfo(ctx, r, name)
	if err != nil || diskTarget == "" {
		return fmt.Errorf("could not determine '%s's disk target device", name)
	}

	fmt.Fprintf(out, "==> Merging the snapshot overlay back into '%s's disk (keeping all changes)...\n", name)
	if _, err := r.Run(ctx, "virsh", "blockcommit", name, diskTarget, "--active", "--pivot", "--wait"); err != nil {
		return err
	}
	_, _ = r.Run(ctx, "virsh", "snapshot-delete", name, snapshotName, "--metadata")
	// blockcommit --pivot stops referencing the overlay (the domain's
	// active disk switches to the freshly-committed base file) but never
	// deletes the overlay file itself, leaving it orphaned on disk to
	// block a future snapshot with the same name. Same root cause as
	// snapshot-restore's fix. Found via real-host testing (2026-07-20).
	if overlayPath != "" {
		_, _ = r.Run(ctx, "rm", "-f", overlayPath)
	}
	fmt.Fprintln(out, "    OK: snapshot merged and removed; current state preserved.")
	return nil
}

func cmdBackup(ctx context.Context, r execrunner.Runner, out io.Writer, name, destDir string, keepN int) error {
	// A live backup's blockcommit has no --base, so it flattens the entire
	// disk chain. Refuse if a named rollback snapshot is active, matching
	// `snapshot`'s own "one at a time" rule.
	if hasSnapshot(ctx, r, name, snapshotName) {
		return fmt.Errorf("VM '%s' has an active rollback snapshot ('%s'). Running 'backup' now would merge it away. Run 'snapshot-restore' or 'snapshot-delete' first, then back up", name, snapshotName)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	timestamp := time.Now().Format("20060102-150405")
	destFile := filepath.Join(destDir, fmt.Sprintf("%s-%s.qcow2", name, timestamp))

	state := domState(ctx, r, name)
	diskTarget, diskPath, err := getDiskInfo(ctx, r, name)
	if err != nil || diskPath == "" {
		return fmt.Errorf("could not determine '%s's disk path", name)
	}

	if state != "running" {
		fmt.Fprintln(out, "==> VM is stopped; copying its disk directly...")
		if _, err := r.Run(ctx, "qemu-img", "convert", "-O", "qcow2", "-c", diskPath, destFile); err != nil {
			return err
		}
	} else {
		fmt.Fprintln(out, "==> VM is running; taking a live, consistent backup...")
		frozen := false
		freezeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if _, err := r.Run(freezeCtx, "virsh", "domfsfreeze", name); err == nil {
			frozen = true
			fmt.Fprintln(out, "    Guest filesystems frozen for a consistent snapshot point.")
		} else {
			fmt.Fprintln(out, "    WARNING: could not freeze guest filesystems (agent unresponsive?).")
			fmt.Fprintln(out, "             Proceeding with a crash-consistent (not fully consistent) backup.")
		}
		cancel()

		sourceDisk := diskPath
		if _, err := r.Run(ctx, "virsh", "snapshot-create-as", "--domain", name, backupTmpSnapshotName, "--disk-only", "--atomic"); err != nil {
			return err
		}
		// Capture the temp overlay's own path (distinct from sourceDisk,
		// the pre-snapshot base file) so it can be cleaned up after
		// blockcommit below — see the comment there.
		_, tmpOverlayPath, _ := getDiskInfo(ctx, r, name)

		if frozen {
			_, _ = r.Run(ctx, "virsh", "domfsthaw", name)
		}

		fmt.Fprintf(out, "==> Copying the frozen disk state to %s...\n", destFile)
		if _, err := r.Run(ctx, "qemu-img", "convert", "-O", "qcow2", "-c", sourceDisk, destFile); err != nil {
			return err
		}

		fmt.Fprintln(out, "==> Merging the temporary snapshot back into the live disk...")
		if _, err := r.Run(ctx, "virsh", "blockcommit", name, diskTarget, "--active", "--pivot", "--wait"); err != nil {
			return fmt.Errorf("blockcommit failed. The VM may be running on a lingering overlay. Inspect with: virsh blockjob %s %s --info", name, diskTarget)
		}
		_, _ = r.Run(ctx, "virsh", "snapshot-delete", name, backupTmpSnapshotName, "--metadata")
		// blockcommit --pivot stops referencing the temp overlay but never
		// deletes the file itself, leaving it orphaned on disk to block a
		// future backup's snapshot-create-as with the same name ("external
		// snapshot file ... already exists"). Same root cause as
		// snapshot-restore/snapshot-delete's fix. Found via real-host
		// testing (2026-07-20).
		if tmpOverlayPath != "" {
			_, _ = r.Run(ctx, "rm", "-f", tmpOverlayPath)
		}
	}

	fmt.Fprintf(out, "    OK: backup written to %s\n", destFile)

	if keepN > 0 {
		fmt.Fprintf(out, "==> Applying retention (keep %d most recent)...\n", keepN)
		if err := applyRetention(destDir, name, keepN, out); err != nil {
			return err
		}
	}
	return nil
}

func applyRetention(destDir, name string, keepN int, out io.Writer) error {
	matches, err := filepath.Glob(filepath.Join(destDir, name+"-*.qcow2"))
	if err != nil {
		return err
	}
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{m, info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.After(files[j].modTime) })
	for i, f := range files {
		if i < keepN {
			continue
		}
		fmt.Fprintf(out, "    Removing old backup: %s\n", f.path)
		_ = os.Remove(f.path)
	}
	return nil
}

func cmdBackupList(out io.Writer, name, destDir string) error {
	if _, err := os.Stat(destDir); err != nil {
		fmt.Fprintf(out, "No backups found for '%s' at %s.\n", name, destDir)
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(destDir, name+"-*.qcow2"))
	if len(matches) == 0 {
		fmt.Fprintf(out, "No backups found for '%s' at %s.\n", name, destDir)
		return nil
	}
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		fmt.Fprintf(out, "%s\t%s\n", info.ModTime().Format("2006-01-02 15:04:05"), m)
	}
	return nil
}

func cmdBackupRestore(ctx context.Context, r execrunner.Runner, out io.Writer, in io.Reader, name, backupFile string) error {
	if backupFile == "" {
		return fmt.Errorf("--file=<path> is required for backup-restore")
	}
	if _, err := os.Stat(backupFile); err != nil {
		return fmt.Errorf("backup file not found: %s", backupFile)
	}

	fmt.Fprintf(out, "This will REPLACE '%s's current disk with the contents of:\n", name)
	fmt.Fprintf(out, "  %s\n", backupFile)
	if !cli.Confirm(out, in, "Proceed?", false) {
		fmt.Fprintln(out, "==> Aborted, no changes made.")
		return nil
	}

	_, diskPath, err := getDiskInfo(ctx, r, name)
	if err != nil || diskPath == "" {
		return fmt.Errorf("could not determine '%s's disk path", name)
	}

	priorState := stopIfRunning(ctx, r, out, name)
	fmt.Fprintln(out, "==> Replacing the disk...")
	if _, err := r.Run(ctx, "qemu-img", "convert", "-O", "qcow2", backupFile, diskPath); err != nil {
		return err
	}
	if err := restorePriorState(ctx, r, out, name, priorState); err != nil {
		return err
	}
	fmt.Fprintf(out, "    OK: restored from %s\n", backupFile)
	return nil
}
