#!/bin/sh
# Installs vmctl by fetching the latest release binary from GitHub Releases.
# Usage: curl -fsSL https://raw.githubusercontent.com/mbdamiate/self-hosting/main/install.sh | sh
#
# This only installs the vmctl binary itself. It does NOT install any
# host-level package (KVM/QEMU/libvirt/etc.) — run 'vmctl doctor --fix' for
# that, after this script finishes.
set -eu

REPO="mbdamiate/self-hosting"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="vmctl"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "'$1' is required but was not found on PATH"
}

for cmd in curl tar sha256sum sudo install; do
  require_command "$cmd"
done

os="$(uname -s)"
case "$os" in
  Linux) ;;
  *) fail "vmctl only supports Linux (detected: $os) — it depends on libvirt/KVM/systemd/apt" ;;
esac

arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *) fail "unsupported CPU architecture: $arch_raw (vmctl publishes linux/amd64 and linux/arm64 only)" ;;
esac

asset="vmctl_linux_${arch}.tar.gz"
base_url="https://github.com/${REPO}/releases/latest/download"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

echo "==> Downloading ${asset}..."
curl -fsSL -o "$tmp_dir/$asset" "$base_url/$asset" \
  || fail "could not download $asset from $base_url — has a release been published yet? (https://github.com/${REPO}/releases)"

echo "==> Downloading sha256sums.txt..."
curl -fsSL -o "$tmp_dir/sha256sums.txt" "$base_url/sha256sums.txt" \
  || fail "could not download sha256sums.txt from $base_url"

echo "==> Verifying checksum..."
( cd "$tmp_dir" && grep " ${asset}\$" sha256sums.txt | sha256sum -c - >/dev/null ) \
  || fail "checksum verification failed for $asset — aborting before installing anything"

echo "==> Extracting..."
tar -xzf "$tmp_dir/$asset" -C "$tmp_dir"
[ -f "$tmp_dir/$BINARY_NAME" ] || fail "extracted archive did not contain a '$BINARY_NAME' binary"

echo "==> Installing to ${INSTALL_DIR}/${BINARY_NAME} (requires sudo)..."
sudo install -m 755 "$tmp_dir/$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

echo "==> Installed:"
"$INSTALL_DIR/$BINARY_NAME" version

echo
echo "Next steps:"
echo "  $BINARY_NAME doctor         # check host prerequisites"
echo "  $BINARY_NAME doctor --fix   # install/configure anything missing"
echo "  $BINARY_NAME create         # create your first VM"
