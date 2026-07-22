# self-hosting

Provisions a local Debian VM via libvirt/KVM/QEMU + cloud-init, mimicking a rented VPS (the same technique used by providers like DigitalOcean or Linode). Use it to try out self-hosting setups locally before deploying them to a real VPS.

## Prerequisites

- A Linux host with KVM support. `vmctl doctor` (no flag) reports host readiness on any distro, but the automated installer, `vmctl doctor --fix`/`--unfix`, only supports apt-based hosts (Ubuntu/Debian) — on other distros (Fedora, Arch, openSUSE, ...), run `vmctl doctor` to see exactly what's missing and install the equivalents yourself.
- Host-level prerequisites (packages, libvirt/kvm group membership, the libvirtd service, the libvirt `default` network, the QEMU storage ACL) present before running `vmctl create`. `vmctl create` only checks these; it does not install or configure them — run `vmctl doctor` to check, or `vmctl doctor --fix` to install/configure what's missing on an apt-based host.
- A Go toolchain (build-time only) to build `vmctl` itself.

See `openspec/specs/ubuntu-qemu-prerequisites/` for the exact package list and why `qemu-kvm` is deliberately not one of them on Ubuntu.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/mbdamiate/self-hosting/main/install.sh | sh
```

Downloads the latest release binary (`linux/amd64`/`linux/arm64`) from [GitHub Releases](https://github.com/mbdamiate/self-hosting/releases), verifies its checksum, and installs it to `/usr/local/bin/vmctl`. Safe to re-run to upgrade.

Latest release: [v0.1.0](https://github.com/mbdamiate/self-hosting/releases/tag/v0.1.0).

Building from source instead (e.g. to contribute, or to track `main`):

```sh
cd vmctl && go build -o vmctl ./cmd/vmctl && cd ..
# use ./vmctl/vmctl in place of vmctl in the commands below
```

## Quick start

```sh
# Check host readiness, and install/configure anything missing (packages,
# group membership, libvirtd, the default network, the QEMU storage ACL)
vmctl doctor
vmctl doctor --fix   # if 'doctor' reported anything missing

# Create a VM with default NAT networking
vmctl create

# Tear it down
vmctl delete
```

### Power state

```sh
vmctl stop           # graceful ACPI shutdown
vmctl start          # power on
vmctl reboot         # graceful ACPI reboot
vmctl stop --force   # hard power-off (virsh destroy) if it won't shut down gracefully
```

### Running more than one VM (fleet)

Give each VM its own `--name`, and optionally reserve a stable `--ip` so other fleet VMs can reach it by hostname:

```sh
vmctl create --name=app-01 --ip=192.168.122.50
vmctl delete --name=app-01 --vm-only
```

List every VM currently defined, or check one in detail — both query libvirt live, nothing is cached:

```sh
vmctl list
vmctl info --name=app-01
```

### Protecting a VM's disk (snapshot/backup)

`vmctl snapshot` takes a fast local rollback point; `vmctl backup` writes a compressed copy of a VM's disk to a separate destination:

```sh
vmctl snapshot create --name=debian-vm
vmctl backup create --name=debian-vm
```

## More options

`vmctl` documents its full flag/subcommand set (`--bridge`, `--forward`, `--purge-all`, snapshot/backup verbs, and more) in its own `--help`:

```sh
vmctl create --help
vmctl delete --help
vmctl start --help
vmctl stop --help
vmctl reboot --help
vmctl snapshot --help
vmctl backup --help
vmctl doctor --help
```

## Detailed behavior

The guarantees behind VM creation/removal, fleet behavior, and host-prerequisite checking — rerun safety, cleanup scope, port-forward idempotency, IP reservation, and more — are specified as OpenSpec capabilities under `openspec/specs/`.
