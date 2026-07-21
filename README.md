# self-hosting

Provisions a local Debian VM via libvirt/KVM/QEMU + cloud-init, mimicking a rented VPS (the same technique used by providers like DigitalOcean or Linode). Use it to try out self-hosting setups locally before deploying them to a real VPS.

## Prerequisites

- An apt-based Linux host (Ubuntu/Debian) with KVM support.
- Host-level prerequisites (packages, libvirt/kvm group membership, the libvirtd service, the libvirt `default` network, the QEMU storage ACL) present before running `vmctl create`. `vmctl create` only checks these; it does not install or configure them — run `vmctl doctor` to check, or `vmctl doctor --fix` to install/configure what's missing.
- A Go toolchain (build-time only) to build `vmctl` itself.

See `openspec/specs/ubuntu-qemu-prerequisites/` for the exact package list and why `qemu-kvm` is deliberately not one of them on Ubuntu.

## Quick start

```sh
cd vmctl && go build -o vmctl ./cmd/vmctl && cd ..

# Check host readiness, and install/configure anything missing (packages,
# group membership, libvirtd, the default network, the QEMU storage ACL)
./vmctl/vmctl doctor
./vmctl/vmctl doctor --fix   # if 'doctor' reported anything missing

# Create a VM with default NAT networking
./vmctl/vmctl create

# Tear it down
./vmctl/vmctl delete
```

### Power state

```sh
./vmctl/vmctl stop           # graceful ACPI shutdown
./vmctl/vmctl start          # power on
./vmctl/vmctl reboot         # graceful ACPI reboot
./vmctl/vmctl stop --force   # hard power-off (virsh destroy) if it won't shut down gracefully
```

### Running more than one VM (fleet)

Give each VM its own `--name`, and optionally reserve a stable `--ip` so other fleet VMs can reach it by hostname:

```sh
./vmctl/vmctl create --name=app-01 --ip=192.168.122.50
./vmctl/vmctl delete --name=app-01 --vm-only
```

List every VM currently defined, or check one in detail — both query libvirt live, nothing is cached:

```sh
./vmctl/vmctl list
./vmctl/vmctl info --name=app-01
```

### Protecting a VM's disk (snapshot/backup)

`vmctl snapshot` takes a fast local rollback point; `vmctl backup` writes a compressed copy of a VM's disk to a separate destination:

```sh
./vmctl/vmctl snapshot create --name=debian-vm
./vmctl/vmctl backup create --name=debian-vm
```

## More options

`vmctl` documents its full flag/subcommand set (`--bridge`, `--forward`, `--purge-all`, snapshot/backup verbs, and more) in its own `--help`:

```sh
./vmctl/vmctl create --help
./vmctl/vmctl delete --help
./vmctl/vmctl start --help
./vmctl/vmctl stop --help
./vmctl/vmctl reboot --help
./vmctl/vmctl snapshot --help
./vmctl/vmctl backup --help
./vmctl/vmctl doctor --help
```

## Detailed behavior

The guarantees behind VM creation/removal, fleet behavior, and host-prerequisite checking — rerun safety, cleanup scope, port-forward idempotency, IP reservation, and more — are specified as OpenSpec capabilities under `openspec/specs/`.
