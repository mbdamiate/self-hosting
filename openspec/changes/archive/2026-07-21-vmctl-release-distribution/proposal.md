## Why

`vmctl` has no release or distribution story today: no `LICENSE`, no CI, no git tags, no way to install a pre-built binary, and no way for the binary itself to report what version it is. The user wants a "curl | sh"-style install experience (as used by rustup/homebrew/deno) backed by GitHub Releases, and is preparing to make the currently-private `github.com/mbdamiate/self-hosting` repo public to support it. `vmctl` is a good fit for this: it's pure Go with zero cgo (verified — cross-compiles cleanly to `linux/arm64` from this host with no extra toolchain), and produces a small (~4MB) single binary.

## What Changes

- Add `LICENSE` (MIT) at the repo root.
- Add a `vmctl version` subcommand (and a `--version` alias in the top-level dispatch) that prints a version string embedded at build time via `-ldflags "-X main.version=<tag>"`; a plain local `go build` (no ldflags) prints a clear non-release indicator (e.g. `dev`) instead of an empty string.
- Add `.github/workflows/release.yml`: triggered by pushing a `v*` tag, builds `linux/amd64` and `linux/arm64` with the version embedded, packages each as a `.tar.gz`, generates a `sha256sums.txt` covering both, and publishes everything as assets on a GitHub Release for that tag.
- Add `install.sh` at the repo root (stable `raw.githubusercontent.com` URL on `main` for `curl -fsSL <url> | sh`): detects Linux + CPU architecture (`amd64`/`arm64`, failing clearly on anything else, including non-Linux), downloads the matching archive from the GitHub Releases "latest" stable URL, verifies it against the published `sha256sums.txt`, extracts it, and installs the `vmctl` binary to `/usr/local/bin` via `sudo install -m 755`. Safe to re-run (upgrades in place). Prints the installed version (via `vmctl version`) at the end.
- Add an "Install" section to `README.md`'s Quick Start, ahead of the existing "build from source" path (which remains, for contributors/developers).

## Capabilities

### New Capabilities
- `vmctl-release-distribution`: the `vmctl version` subcommand/build-time version embedding, the GitHub Actions release workflow's contract (what a tagged push produces), and `install.sh`'s behavior (detection, download, verification, installation, idempotency).

### Modified Capabilities
(none — this is purely additive; no existing subcommand's behavior changes)

## Impact

- Code: new `vmctl/cmd/vmctl/version.go` (following the existing `run*(args []string) error` subcommand pattern in `cmd/vmctl/`), a one-line addition to `main.go`'s switch (`case "version", "--version"`) and `usage()` text.
- New root-level files: `LICENSE`, `install.sh`, `.github/workflows/release.yml`.
- Docs: `README.md` gains an install section.
- Out of scope (explicitly): actually flipping the GitHub repo to public (a manual action only the user can take in GitHub's settings); any change to `vmctl doctor`'s host-prerequisite behavior (the install script deliberately only installs the `vmctl` binary itself, never host packages); non-Linux support; package-manager formulas (Homebrew/apt/etc.) beyond the curl-script + GitHub Releases path.
