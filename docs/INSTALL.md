# Install

Every way to get Stacklit, with the exact command. Paths marked "after" are not live yet; the status column says what they wait for.

## Quick pick

| Path | Command | Gives you | Status |
| --- | --- | --- | --- |
| npx | `npx stacklit init` | CLI, one run, no install | live |
| npm global | `npm install -g stacklit` | CLI | live |
| Homebrew | `brew install glincker/tap/stacklit` | CLI | live |
| go install | `go install github.com/glincker/stacklit/cmd/stacklit@latest` | CLI | live |
| install.sh | see below | CLI, checksum verified | live once this file is on the default branch |
| Release archive | download from GitHub Releases | CLI | live |
| Docker | `docker run --rm -v "$PWD":/work ghcr.io/glincker/stacklit init` | CLI in a container | after the image is published (see below) |
| Claude Code plugin | two commands below | hooks, skill and MCP server | live once this file is on the default branch |
| MCP registry | `io.github.glincker/stacklit` | MCP server entry for registry-aware clients | after the next npm release and a registry publish |
| skills.sh | `npx skills add glincker/stacklit` | the skill only | should work from the GitHub repo; listing on skills.sh is unverified |

What each one gives:

| | CLI | Hooks | Skill | MCP server |
| --- | --- | --- | --- | --- |
| npx, npm, brew, go install, install.sh, archive, Docker | yes | via `stacklit setup` | via `stacklit setup` | via `stacklit setup` |
| Claude Code plugin | needs the CLI on PATH | yes | yes | yes (minimal profile) |
| MCP registry | needs npm or the CLI | no | no | yes |
| skills.sh | no | no | yes | no |

## Command line

```bash
npx stacklit init                 # no install
npm install -g stacklit
brew install glincker/tap/stacklit
go install github.com/glincker/stacklit/cmd/stacklit@latest
```

Then, in your repo:

```bash
stacklit init
```

## install.sh

Detects OS and architecture (Linux and macOS, amd64 and arm64), downloads the release archive and `checksums.txt`, verifies the sha256 and installs to `~/.local/bin` without sudo.

```bash
curl -fsSL https://raw.githubusercontent.com/glincker/stacklit/master/scripts/install.sh | sh
```

Options go after `sh -s --`:

```bash
curl -fsSL https://raw.githubusercontent.com/glincker/stacklit/master/scripts/install.sh | sh -s -- --version 0.5.0 --prefix /usr/local
```

`--dry-run` downloads and verifies but installs nothing. If you would rather read it first, open `scripts/install.sh`; it is short. Windows is not supported by the script: use npm or the zip from Releases.

## Docker

The repository has a multi-stage `docker/Dockerfile` (distroless final image, entrypoint `stacklit`). Build it yourself:

```bash
docker build -f docker/Dockerfile --build-arg VERSION=0.5.0 -t stacklit .
docker run --rm -v "$PWD":/work stacklit init
```

Once the image is published to ghcr.io, the same works without building:

```bash
docker run --rm -v "$PWD":/work ghcr.io/glincker/stacklit init
```

On Linux, add `--user "$(id -u):$(id -g)"` so generated files belong to you. The image has no shell and no git binary, so features that call git (hot files, the post-commit hook installer) do not work in it. Use the native binary for those. The MCP server over Docker (`docker run -i --rm -v "$PWD":/work ghcr.io/glincker/stacklit serve`) works for the index tools.

## Claude Code plugin

The repo is a Claude Code plugin marketplace. It needs the `stacklit` binary on PATH (any CLI path above).

```bash
claude plugin marketplace add glincker/stacklit
claude plugin install stacklit@stacklit
```

Inside a session the same are `/plugin marketplace add glincker/stacklit` and `/plugin install stacklit@stacklit`.

The plugin adds:

- the Stacklit skill
- a SessionStart hook (`stacklit hook session`) that injects the map
- an async PostToolUse hook (`stacklit hook posttool`) that refreshes the map for edited files
- an MCP server running `stacklit serve --profile minimal`

Both hooks exit quietly if `stacklit` is not installed. The crawl guard is not included; enable it with `stacklit setup claude --crawl-guard`. The hook wrapper uses `sh`, so on Windows it needs Git Bash.

## MCP registry

`server.json` at the repo root describes the server for the official MCP registry under the name `io.github.glincker/stacklit`. It points at the npm package, which declares the same name in `mcpName`. That field ships with the first npm release after it was added, and the registry entry is published by the maintainers after that. Until then, use `stacklit setup` or the plugin to wire the MCP server.

## Skill only

```bash
npx skills add glincker/stacklit
```

This installs `skills/stacklit/SKILL.md` into the agents it detects. The skill follows the open Agent Skills format.

## Not available yet

Scoop, WinGet, Nix, AUR, mise and a devcontainer feature are not published. Windows users can use `npm install -g stacklit` or the zip from Releases.
