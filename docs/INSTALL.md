# Install

Every way to get Stacklit, with the exact command. The status column says whether a path works today. Paths marked "after" are not live yet, and the column says what they wait for.

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
| skills.sh | `npx skills add glincker/stacklit` | the skill only | should work from the GitHub repo. Listing on skills.sh is unverified. |

What each path gives you:

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

The script detects your OS and architecture (Linux and macOS, amd64 and arm64). It downloads the release archive and `checksums.txt`, checks the sha256, and installs to `~/.local/bin`. It does not need sudo.

```bash
curl -fsSL https://raw.githubusercontent.com/glincker/stacklit/master/scripts/install.sh | sh
```

Put options after `sh -s --`:

```bash
curl -fsSL https://raw.githubusercontent.com/glincker/stacklit/master/scripts/install.sh | sh -s -- --version 0.5.0 --prefix /usr/local
```

- `--dry-run` downloads and verifies the archive, but installs nothing.
- To read the script first, open `scripts/install.sh`. It is short.
- The script does not support Windows. Use npm, or the zip from Releases.

## Docker

The repository has a multi-stage `docker/Dockerfile`. The final image is distroless, and its entrypoint is `stacklit`. Build it yourself:

```bash
docker build -f docker/Dockerfile --build-arg VERSION=0.5.0 -t stacklit .
docker run --rm -v "$PWD":/work stacklit init
```

Once the image is published to ghcr.io, you can skip the build:

```bash
docker run --rm -v "$PWD":/work ghcr.io/glincker/stacklit init
```

On Linux, add `--user "$(id -u):$(id -g)"` so the generated files belong to you.

> [!WARNING]
> The image has no shell and no git binary. Features that call git do not work in it: hot files, and the post-commit hook installer. Use the native binary for those. The MCP server works over Docker for the index tools: `docker run -i --rm -v "$PWD":/work ghcr.io/glincker/stacklit serve`.

## Claude Code plugin

The repo is a Claude Code plugin marketplace. The plugin needs the `stacklit` binary on your PATH. Any CLI path above works.

1. Add the marketplace.

   ```bash
   claude plugin marketplace add glincker/stacklit
   ```

2. Install the plugin.

   ```bash
   claude plugin install stacklit@stacklit
   ```

Inside a session, the same two steps are `/plugin marketplace add glincker/stacklit` and `/plugin install stacklit@stacklit`.

The plugin adds:

- the Stacklit skill
- a SessionStart hook (`stacklit hook session`) that injects the map
- an async PostToolUse hook (`stacklit hook posttool`) that refreshes the map for edited files
- an MCP server that runs `stacklit serve --profile minimal`

Both hooks exit quietly if `stacklit` is not installed. The crawl guard is not included. Turn it on with `stacklit setup claude --crawl-guard`. The hook wrapper uses `sh`, so on Windows it needs Git Bash.

## MCP registry

`server.json` at the repo root describes the server for the official MCP registry, under the name `io.github.glincker/stacklit`. It points at the npm package, which declares the same name in `mcpName`.

That field ships with the first npm release after it was added. The maintainers publish the registry entry after that release. Until then, use `stacklit setup` or the plugin to wire up the MCP server.

## Skill only

```bash
npx skills add glincker/stacklit
```

This installs `skills/stacklit/SKILL.md` into the agents it detects. The skill follows the open Agent Skills format.

## Not available yet

Scoop, WinGet, Nix, AUR, mise and a devcontainer feature are not published. Windows users can use `npm install -g stacklit` or the zip from Releases.

Next: [Quickstart](USAGE.md#first-run), or the full [usage guide](USAGE.md).
