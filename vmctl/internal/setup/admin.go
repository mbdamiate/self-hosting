package setup

import (
	"context"
	"fmt"
	"strings"

	"vmctl/internal/execrunner"
)

// adminSudoResult carries the outcome of admin sudo policy configuration.
type adminSudoResult struct {
	SudoEntry      string
	PasswdBlock    string
	Policy         string // "nopasswd" or "password-required"
	PlaintextShown string // only set when a password was generated/provided this run
}

// configureAdminSudo mirrors section 7.1: password-less sudo by default,
// opt-in password-required sudo with a generated or provided password.
// Matches the existing bash script's approach of passing the plaintext
// password as an openssl argument; this is a faithful, behavior-preserving
// port, not a new design.
func configureAdminSudo(ctx context.Context, r execrunner.Runner, requested bool, providedValue string) (adminSudoResult, error) {
	if !requested {
		return adminSudoResult{SudoEntry: "ALL=(ALL) NOPASSWD:ALL", Policy: "nopasswd"}, nil
	}

	plaintext := providedValue
	if plaintext == "" {
		out, err := r.Run(ctx, "openssl", "rand", "-base64", "18")
		if err != nil {
			return adminSudoResult{}, fmt.Errorf("'openssl' is required to generate the admin password but was not found on the host")
		}
		plaintext = strings.TrimSpace(string(out))
	}

	saltOut, err := r.Run(ctx, "openssl", "rand", "-hex", "8")
	if err != nil {
		return adminSudoResult{}, fmt.Errorf("'openssl' is required to hash the admin password but was not found on the host")
	}
	salt := strings.TrimSpace(string(saltOut))

	hashOut, err := r.Run(ctx, "openssl", "passwd", "-6", "-salt", salt, plaintext)
	if err != nil {
		return adminSudoResult{}, fmt.Errorf("failed to hash the admin password with openssl")
	}
	hash := strings.TrimSpace(string(hashOut))

	return adminSudoResult{
		SudoEntry:      "ALL=(ALL) ALL",
		PasswdBlock:    fmt.Sprintf("    passwd: %s\n    lock_passwd: false", hash),
		Policy:         "password-required",
		PlaintextShown: plaintext,
	}, nil
}
