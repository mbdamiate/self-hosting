## Context

`vmctl` (`vmctl/cmd/vmctl/main.go`) is a single, dependency-free (no cgo) Go binary dispatched via a `switch os.Args[1]` on subcommand name — the same hand-rolled pattern used throughout: no Cobra for flags (stdlib `flag` instead), no libvirt Go bindings (`exec.Command` instead). The repo has no `LICENSE`, no `.github/workflows/`, no git tags, and the binary has no version-reporting capability at all. The user is preparing to make the repo public and wants a `curl -fsSL <url> | sh` install experience backed by GitHub Releases.

## Goals / Non-Goals

**Goals:**
- `vmctl version` (and `--version`) reports a build-time-embedded version string, falling back to a clear `dev` marker for unstamped local builds.
- A tag push (`v*`) fully automates building, packaging, checksumming, and publishing `linux/amd64` + `linux/arm64` release assets.
- `install.sh` at a stable `main`-branch URL fetches, verifies, and installs the right binary with zero interaction beyond a `sudo` password prompt.

**Non-Goals:**
- Flipping the GitHub repo to public (manual, user-only action).
- Any change to `vmctl doctor`'s host-prerequisite behavior — `install.sh` only ever touches the `vmctl` binary itself.
- macOS/Windows support, or any package-manager formula (Homebrew/apt/etc.) beyond the script + GitHub Releases path.
- An `INSTALL_DIR`/prefix override flag for `install.sh` — the decided location is `/usr/local/bin`, fixed; adding a configurable override wasn't asked for and would be scope creep for a first cut.

## Decisions

**1. Hand-rolled release workflow, not GoReleaser.**
GoReleaser is the popular off-the-shelf tool for exactly this (cross-compile, archive, checksum, publish), but it introduces a new config file format (`.goreleaser.yaml`) and an extra layer of abstraction between the YAML and what actually runs. This repo has consistently chosen the more inspectable, fewer-moving-parts option at every prior fork in the road (stdlib `flag` over Cobra, `exec.Command` over libvirt bindings — see the archived `migrate-vm-scripts-to-go-cli` design doc's Decisions). A hand-rolled workflow using only `go build`, `tar`, `sha256sum`, and the `gh` CLI (pre-installed on GitHub-hosted runners) keeps every step visible in one YAML file with no new tool to learn. Revisit if the release matrix grows enough that GoReleaser's boilerplate reduction starts to outweigh the inspectability cost.

**2. Asset naming and layout.**
`vmctl_linux_amd64.tar.gz` / `vmctl_linux_arm64.tar.gz`, each containing just the `vmctl` binary; a single `sha256sums.txt` covers both archives. This is the de facto convention (matches what GoReleaser itself would produce, so `install.sh`'s logic would still work if the workflow were ever swapped to GoReleaser later).

**3. `install.sh` resolves "latest" via GitHub's stable redirect URL.**
`https://github.com/<owner>/<repo>/releases/latest/download/<asset>` always resolves to the newest non-prerelease release without the script needing to call the GitHub API or parse JSON (no `jq` dependency). Falls back to failing with a clear message if `curl` gets a 404 (e.g. no release published yet).

**4. Version embedding via `-ldflags`, with a `dev` fallback.**
`main.version` (a package-level `var version = "dev"` in `main.go`) is overridden at build time with `-ldflags "-X main.version=$TAG"`. A plain local `go build` (no ldflags) keeps the `"dev"` default, so `vmctl version` never prints an empty string.

**5. `vmctl version` follows the existing subcommand pattern exactly.**
A new `cmd/vmctl/version.go` with `runVersion(args []string) error`, wired into `main.go`'s switch as `case "version", "--version":` (the `--version` spelling is accepted directly in the switch, mirroring how `-h`/`--help`/`help` are all accepted today) — no new dispatch mechanism introduced.

**6. `sudo` inside `curl | sh`.**
`sudo`'s password prompt reads from the controlling terminal (`/dev/tty`), not from the script's stdin — so it works normally even when the script itself arrived via a pipe (this is why `curl ... | sudo sh`-style installers are common in the wild, e.g. Docker's own convenience script). `install.sh` only invokes `sudo` for the single `install -m 755` step; everything before that (download, checksum verify, extract to a temp dir) runs unprivileged.

## Risks / Trade-offs

- **[Hand-rolled workflow has more YAML than a GoReleaser-based one]** → Mitigation: accepted trade-off per Decision 1 — every step stays auditable without learning a new tool; the YAML is small (build matrix of 2, package, checksum, publish).
- **[`install.sh` piped into `sh` can't be reviewed by the user before running, a general risk of the curl-pipe pattern]** → Mitigation: inherent to the pattern the user explicitly asked for (matches rustup/homebrew/deno); the script is also directly readable at its stable URL before running it, and is small enough to audit in seconds.
- **[GitHub's `/releases/latest/download/` redirect 404s if no release has ever been published]** → Mitigation: `install.sh` checks the curl exit code / HTTP status and fails with a clear "no release found, has one been published yet?" message rather than silently installing garbage.
- **[Architecture detection false negatives on unusual `uname -m` output]** → Mitigation: explicit `case` mapping (`x86_64`→`amd64`, `aarch64`/`arm64`→`arm64`) with a clear failure message naming the unrecognized value on anything else, rather than guessing.

## Migration Plan

1. Add `LICENSE` (MIT).
2. Add `cmd/vmctl/version.go` + wire `main.go`'s switch/usage text.
3. Add `.github/workflows/release.yml` (triggers on `v*` tag push; needs `permissions: contents: write` for `gh release create`).
4. Add `install.sh` at repo root.
5. Add README's "Install" section.
6. Tag a first release (e.g. `v0.1.0`) once the user has flipped the repo to public, to validate the whole pipeline end-to-end.

## Open Questions

None outstanding.
