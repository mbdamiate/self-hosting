// Package metadata implements vm-tooling-metadata: one consolidated record
// per VM for the guest-only facts libvirt itself cannot report (admin sudo
// policy, log-forwarding configuration, guest firewall policy), replacing
// the three separate dotfiles debian-vm-setup.sh used to write.
//
// Storage: a JSON file under the VM's WORK_DIR (design.md option (a)).
// design.md's preferred option (b) — libvirt's native domain <metadata>
// XML element — needs an empirical spike against a real libvirtd that
// wasn't possible in the environment this was built in; this file option
// was chosen because it's fully testable without one, and still satisfies
// every requirement in specs/vm-tooling-metadata/spec.md (which describes
// observable behavior, not a storage mechanism). Revisit if/when the spike
// is done — see the change's design.md Open Questions.
package metadata

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const fileName = "meta.json"

// Record holds the guest-only facts tracked per VM. A missing record (e.g.
// a VM created before this existed) is treated as every field being at its
// zero value — "fully unconfigured" — per vm-tooling-metadata.
type Record struct {
	AdminSudoPolicy     string `json:"admin_sudo_policy,omitempty"`
	LogForwarding       bool   `json:"log_forwarding,omitempty"`
	GuestFirewallPolicy string `json:"guest_firewall_policy,omitempty"`
}

func path(workDir string) string {
	return filepath.Join(workDir, fileName)
}

// Load reads the record for workDir. A missing file is not an error: it
// returns a zero Record, matching "missing metadata == fully unconfigured".
func Load(workDir string) (Record, error) {
	data, err := os.ReadFile(path(workDir))
	if err != nil {
		if os.IsNotExist(err) {
			return Record{}, nil
		}
		return Record{}, err
	}
	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return Record{}, err
	}
	return rec, nil
}

// Save writes rec for workDir, overwriting any existing record.
func Save(workDir string, rec Record) error {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path(workDir), data, 0o644)
}
