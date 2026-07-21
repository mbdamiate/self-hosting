## ADDED Requirements

### Requirement: `vmctl version` reports a build-time-embedded version
`vmctl version` (and its `--version` alias) SHALL print a version string embedded at build time via `-ldflags "-X main.version=<value>"`, and SHALL print a clear non-release marker (`dev`) when built without that override.

#### Scenario: Release build
- **WHEN** `vmctl` is built with `-ldflags "-X main.version=v0.1.0"` and `vmctl version` is run
- **THEN** it prints `v0.1.0`

#### Scenario: Local unstamped build
- **WHEN** `vmctl` is built with a plain `go build` (no `-ldflags` override) and `vmctl version` is run
- **THEN** it prints `dev`, never an empty string

#### Scenario: `--version` alias
- **WHEN** a user runs `vmctl --version`
- **THEN** `vmctl` behaves identically to `vmctl version`

### Requirement: Pushing a version tag triggers an automated release build
Pushing a `v*` git tag SHALL trigger a GitHub Actions workflow that builds `linux/amd64` and `linux/arm64` binaries with the version embedded, packages each as a `.tar.gz`, generates a `sha256sums.txt` covering both archives, and publishes all of them as assets on a GitHub Release for that tag, without manual steps.

#### Scenario: Tag pushed
- **WHEN** a `v*`-prefixed tag (e.g. `v0.1.0`) is pushed to the repository
- **THEN** a GitHub Release for that tag is created (or updated) with `vmctl_linux_amd64.tar.gz`, `vmctl_linux_arm64.tar.gz`, and `sha256sums.txt` as assets, each binary reporting the pushed tag via `vmctl version`

### Requirement: `install.sh` detects OS and CPU architecture before downloading anything
`install.sh` SHALL verify the host is Linux and map its CPU architecture (`uname -m`) to a supported release asset (`x86_64`→`amd64`, `aarch64`/`arm64`→`arm64`) before attempting any download, and SHALL fail with a clear message naming the unsupported OS or architecture otherwise.

#### Scenario: Supported Linux host
- **WHEN** `install.sh` runs on a Linux host reporting `x86_64` or `aarch64`/`arm64`
- **THEN** it proceeds to download the matching release asset

#### Scenario: Unsupported OS
- **WHEN** `install.sh` runs on a non-Linux host
- **THEN** it exits with an error naming the detected OS, before attempting any download

#### Scenario: Unsupported architecture
- **WHEN** `install.sh` runs on Linux but `uname -m` reports something other than `x86_64`, `aarch64`, or `arm64`
- **THEN** it exits with an error naming the unrecognized architecture, before attempting any download

### Requirement: `install.sh` downloads, verifies, and installs the binary idempotently
`install.sh` SHALL download the release asset matching the detected architecture from the GitHub Releases "latest" stable URL, verify it against the published `sha256sums.txt`, extract it, and install the `vmctl` binary to `/usr/local/bin` via `sudo install -m 755`, succeeding identically whether or not a `vmctl` binary already exists there.

#### Scenario: Fresh install
- **WHEN** `install.sh` runs on a host with no existing `vmctl` binary at `/usr/local/bin/vmctl`
- **THEN** it downloads, verifies, and installs the binary there, and printing the installed version (via `vmctl version`) confirms success

#### Scenario: Re-running to upgrade
- **WHEN** `install.sh` runs again on a host that already has `vmctl` installed at `/usr/local/bin/vmctl`
- **THEN** it overwrites the existing binary with the latest release's, without requiring the old one to be removed first

#### Scenario: Checksum mismatch
- **WHEN** the downloaded archive's checksum does not match the corresponding line in `sha256sums.txt`
- **THEN** `install.sh` aborts before extracting or installing anything, with an error identifying the mismatch

#### Scenario: No release published yet
- **WHEN** `install.sh` requests the GitHub Releases "latest" download URL and receives a 404 (no release exists)
- **THEN** it exits with a clear error indicating no release was found, rather than installing a partial or empty file
