package setup

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// writeFile is one entry of cloud-init's write_files list.
type writeFile struct {
	path    string
	content string
}

// cloudInitBuilder accumulates the packages/runcmd/write_files sections as
// setup's optional features are configured, mirroring the bash script's
// CLOUDINIT_* accumulator variables.
type cloudInitBuilder struct {
	packages   []string
	runcmd     []string
	writeFiles []writeFile
}

func newCloudInitBuilder() *cloudInitBuilder {
	return &cloudInitBuilder{
		packages: []string{"qemu-guest-agent"},
		runcmd:   []string{"systemctl enable --now qemu-guest-agent"},
	}
}

func (b *cloudInitBuilder) addPackage(pkg string)     { b.packages = append(b.packages, pkg) }
func (b *cloudInitBuilder) addRuncmd(cmd string)      { b.runcmd = append(b.runcmd, cmd) }
func (b *cloudInitBuilder) addWriteFile(wf writeFile) { b.writeFiles = append(b.writeFiles, wf) }

func (b *cloudInitBuilder) addUnattendedUpgrades() {
	b.addPackage("unattended-upgrades")
	b.addWriteFile(writeFile{
		path: "/etc/apt/apt.conf.d/20auto-upgrades",
		content: `APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
`,
	})
	b.addWriteFile(writeFile{
		path: "/etc/apt/apt.conf.d/51unattended-upgrades-security-only",
		content: `Unattended-Upgrade::Allowed-Origins {
    "${distro_id}:${distro_codename}-security";
    "${distro_id}ESM:${distro_codename}-security";
};
Unattended-Upgrade::Automatic-Reboot "false";
`,
	})
}

// guestFirewallPorts computes the deduplicated, sorted TCP port list for
// ufw: SSH plus --allow-port plus any --forward VM-side ports.
func guestFirewallPorts(allowPorts, forwardRules string) string {
	seen := map[int]bool{22: true}
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if n, err := strconv.Atoi(raw); err == nil {
			seen[n] = true
		}
	}
	if allowPorts != "" {
		for _, p := range strings.Split(allowPorts, ",") {
			add(p)
		}
	}
	if forwardRules != "" {
		for _, rule := range strings.Split(forwardRules, ",") {
			idx := strings.LastIndex(rule, ":")
			if idx == -1 {
				continue
			}
			add(rule[idx+1:])
		}
	}
	ports := make([]int, 0, len(seen))
	for p := range seen {
		ports = append(ports, p)
	}
	sort.Ints(ports)
	strs := make([]string, len(ports))
	for i, p := range ports {
		strs[i] = strconv.Itoa(p)
	}
	return strings.Join(strs, ",")
}

func (b *cloudInitBuilder) addGuestFirewall(ports string) {
	b.addPackage("ufw")
	b.addRuncmd("ufw default deny incoming")
	b.addRuncmd("ufw default allow outgoing")
	for _, p := range strings.Split(ports, ",") {
		b.addRuncmd(fmt.Sprintf("ufw allow %s/tcp", p))
	}
	b.addRuncmd("ufw --force enable")
}

func (b *cloudInitBuilder) addFail2ban() {
	b.addPackage("fail2ban")
	b.addWriteFile(writeFile{
		path: "/etc/fail2ban/jail.local",
		// backend=systemd: the Debian 12 cloud image doesn't write
		// /var/log/auth.log by default, so fail2ban's default log-file
		// autodetection ("backend = auto") fails with "Have not found any
		// log file for sshd jail". Reading via journald works regardless
		// of whether traditional syslog files exist.
		content: `[sshd]
enabled = true
backend = systemd
`,
	})
	b.addRuncmd("systemctl enable --now fail2ban")
}

func (b *cloudInitBuilder) addLogForwarding(virbr0IP string) {
	b.addPackage("rsyslog")
	b.addWriteFile(writeFile{
		path:    "/etc/rsyslog.d/60-forward-to-host.conf",
		content: fmt.Sprintf("*.* @@%s:%s\n", virbr0IP, monitorLogPort),
	})
}

func (b *cloudInitBuilder) addWatchdogPetting() {
	b.addWriteFile(writeFile{
		path: "/etc/systemd/system.conf.d/90-watchdog.conf",
		content: `[Manager]
RuntimeWatchdogSec=20s
`,
	})
}

// render produces the user-data and meta-data cloud-init files.
func (b *cloudInitBuilder) render(vmName, vmHostname, vmUser, adminSudoEntry, adminPasswdBlock, sshPubKey string) (userData, metaData string) {
	var buf strings.Builder
	buf.WriteString("#cloud-config\n")
	fmt.Fprintf(&buf, "hostname: %s\n", vmHostname)
	buf.WriteString("manage_etc_hosts: true\n")
	buf.WriteString("users:\n")
	fmt.Fprintf(&buf, "  - name: %s\n", vmUser)
	buf.WriteString("    groups: sudo\n")
	buf.WriteString("    shell: /bin/bash\n")
	fmt.Fprintf(&buf, "    sudo: %s\n", adminSudoEntry)
	if adminPasswdBlock != "" {
		buf.WriteString(adminPasswdBlock)
		buf.WriteString("\n")
	}
	buf.WriteString("    ssh_authorized_keys:\n")
	fmt.Fprintf(&buf, "      - %s\n", sshPubKey)
	buf.WriteString("ssh_pwauth: false\n")
	buf.WriteString("package_update: true\n")
	buf.WriteString("package_upgrade: false\n")
	buf.WriteString("packages:\n")
	for _, p := range b.packages {
		fmt.Fprintf(&buf, "  - %s\n", p)
	}
	if len(b.writeFiles) > 0 {
		buf.WriteString("write_files:\n")
		for _, wf := range b.writeFiles {
			fmt.Fprintf(&buf, "  - path: %s\n", wf.path)
			buf.WriteString("    content: |\n")
			for _, line := range strings.Split(strings.TrimRight(wf.content, "\n"), "\n") {
				fmt.Fprintf(&buf, "      %s\n", line)
			}
		}
	}
	buf.WriteString("runcmd:\n")
	for _, c := range b.runcmd {
		fmt.Fprintf(&buf, "  - %s\n", c)
	}
	userData = buf.String()

	metaData = fmt.Sprintf("instance-id: %s-%d\nlocal-hostname: %s\n", vmName, time.Now().Unix(), vmHostname)
	return userData, metaData
}
