## 1. License

- [x] 1.1 Add `LICENSE` (MIT) at the repo root

## 2. `vmctl version`

- [x] 2.1 Add `version` package-level var (default `"dev"`) in `cmd/vmctl/main.go` or a new `cmd/vmctl/version.go`
- [x] 2.2 Add `cmd/vmctl/version.go` with `runVersion(args []string) error` following the existing subcommand pattern (supports `-h`/`--help`)
- [x] 2.3 Wire `case "version", "--version":` into `main.go`'s switch, and add `version` to `usage()`'s subcommand list

## 3. Release workflow

- [x] 3.1 Add `.github/workflows/release.yml`, triggered on push of tags matching `v*`
- [x] 3.2 Workflow step: build `linux/amd64` and `linux/arm64` with `-ldflags "-X main.version=${{ github.ref_name }}"`
- [x] 3.3 Workflow step: package each binary as `vmctl_linux_amd64.tar.gz` / `vmctl_linux_arm64.tar.gz`
- [x] 3.4 Workflow step: generate `sha256sums.txt` covering both archives
- [x] 3.5 Workflow step: publish the tag, archives, and checksums file as a GitHub Release via `gh release create` (grant the job `permissions: contents: write`)

## 4. `install.sh`

- [x] 4.1 Add `install.sh` at the repo root
- [x] 4.2 OS check: fail clearly if not Linux
- [x] 4.3 Architecture detection: `uname -m` → `amd64`/`arm64`, fail clearly on anything else
- [x] 4.4 Download the matching asset from `https://github.com/mbdamiate/self-hosting/releases/latest/download/<asset>`, failing clearly on a 404 (no release yet)
- [x] 4.5 Download `sha256sums.txt` from the same release and verify the downloaded archive against it before extracting
- [x] 4.6 Extract to a temp dir and `sudo install -m 755` the `vmctl` binary to `/usr/local/bin`
- [x] 4.7 Print the installed version (`vmctl version`) at the end to confirm success

## 5. Docs

- [x] 5.1 Add an "Install" section to `README.md`'s Quick Start (curl-install as the primary path, existing `go build` path kept as "build from source")

## 6. Verification

- [x] 6.1 Run `go build ./...` and existing unit tests in `vmctl/`
- [x] 6.2 Locally build with `-ldflags "-X main.version=v0.0.0-test"` and confirm `vmctl version`/`vmctl --version` print it
- [x] 6.3 Confirm a plain `go build` (no ldflags) makes `vmctl version` print `dev`
- [x] 6.4 Locally cross-compile `GOOS=linux GOARCH=arm64` and confirm the build succeeds (already verified manually during exploration; re-confirm once `-ldflags` is wired in)
- [x] 6.5 Once the repo is public: push a `v0.1.0`-style tag and confirm the GitHub Release is created with both archives + `sha256sums.txt`, each binary reporting the tag via `vmctl version`
- [x] 6.6 Run `install.sh` end-to-end on a real Linux host (fresh install, confirm `/usr/local/bin/vmctl` works) and again (confirm re-run upgrades in place without error)
- [x] 6.7 Deliberately corrupt/mismatch a local copy of `sha256sums.txt` (or simulate) to confirm `install.sh` aborts before installing on a checksum mismatch
