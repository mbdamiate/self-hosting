package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm replaces the two divergent bash implementations (confirm() in
// cleanup, confirm_destructive() in backup) with one, taking the bypass
// decision as an explicit parameter instead of reading global flag state.
func Confirm(out io.Writer, in io.Reader, prompt string, autoApprove bool) bool {
	if autoApprove {
		return true
	}
	fmt.Fprintf(out, "%s [y/N] ", prompt)
	resp, _ := bufio.NewReader(in).ReadString('\n')
	resp = strings.ToLower(strings.TrimSpace(resp))
	return resp == "y" || resp == "yes"
}
