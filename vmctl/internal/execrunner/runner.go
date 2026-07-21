// Package execrunner is the single seam through which vmctl invokes external
// tools (virsh, virt-install, qemu-img, cloud-localds, genisoimage, iptables,
// ...). Production code uses Real; tests inject Fake so subcommand logic is
// verifiable without a KVM host.
package execrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Runner runs an external command and returns its standard output.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	// RunWithStdin is like Run but feeds stdin to the child process, used
	// for the handful of setup steps that write root-owned files via
	// `sudo tee <path>`.
	RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error)
}

// Real runs commands via os/exec.
type Real struct{}

func (Real) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return out, wrapWithStderr(err)
}

func (Real) RunWithStdin(ctx context.Context, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	out, err := cmd.Output()
	return out, wrapWithStderr(err)
}

// wrapWithStderr surfaces the failed command's stderr in the error message.
// Bash gets this for free (stderr flows straight to the terminal); a bare
// Go *exec.ExitError.Error() is just "exit status 1" otherwise, discarding
// exactly the diagnostic text virsh/qemu-img/etc. print on failure. Found
// via real-host testing (2026-07-20): a `virsh snapshot-create-as` failure
// surfaced only as "ERROR: exit status 1", with no way to tell what
// actually went wrong.
func wrapWithStderr(err error) error {
	var exitErr *exec.ExitError
	if err == nil || !errors.As(err, &exitErr) {
		return err
	}
	stderr := bytes.TrimSpace(exitErr.Stderr)
	if len(stderr) == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, stderr)
}
