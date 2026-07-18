# self-hosting

Provisions a local Debian VM via libvirt/KVM/QEMU + cloud-init, mimicking a rented VPS (the same technique used by providers like DigitalOcean or Linode). Use it to try out self-hosting setups locally before deploying them to a real VPS.

## Prerequisites

- An apt-based Linux host (Ubuntu/Debian) with KVM support.
- `sudo` access — the setup script installs packages and manages libvirt/KVM group membership on your behalf.

See `openspec/specs/ubuntu-qemu-prerequisites/` for the exact package list and why `qemu-kvm` is deliberately not one of them on Ubuntu.

## Quick start

```sh
chmod +x scripts/debian-vm-setup.sh scripts/debian-vm-cleanup.sh

# Create a VM with default NAT networking
./scripts/debian-vm-setup.sh

# Tear it down
./scripts/debian-vm-cleanup.sh
```

### Running more than one VM (fleet)

Give each VM its own `--name`, and optionally reserve a stable `--ip` so other fleet VMs can reach it by hostname:

```sh
./scripts/debian-vm-setup.sh --name=app-01 --ip=192.168.122.50
./scripts/debian-vm-cleanup.sh --name=app-01 --vm-only
```

## More options

Both scripts document their full flag set (`--bridge`, `--forward`, `--purge-all`, sizing flags, and more) in their own `--help`:

```sh
./scripts/debian-vm-setup.sh --help
./scripts/debian-vm-cleanup.sh --help
```

## Detailed behavior

The guarantees behind setup, cleanup, and fleet behavior — rerun safety, cleanup scope, port-forward idempotency, IP reservation, and more — are specified as OpenSpec capabilities under `openspec/specs/`.
