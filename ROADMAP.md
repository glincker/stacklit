# Roadmap

Stacklit is MIT licensed and stays that way: no paid tier, no gated features. This file lists what is planned and what is in progress. Nothing here is a promise or a date.

Status labels:

- **Shipped**: in the latest release. See [CHANGELOG.md](CHANGELOG.md).
- **In progress**: work exists on a branch and is not merged or released. Do not rely on it.
- **Planned**: wanted, not started.

Priorities come from measurements in [bench/RESULTS.md](bench/RESULTS.md) where possible. The honest summary of that benchmark: the map is cheap and helps most with "what depends on X" and "where would this go" questions, but exact symbol lookup and dependency accuracy have gaps.

## 1. Query layer

Goal: let an agent ask a precise question and get a small answer, instead of loading the whole index.

| Item | Status |
|------|--------|
| Symbol-level graph (functions, types and their references), beyond today's module-level edges | In progress |
| Query, path and impact commands over that graph ("what breaks if I change this") | In progress |
| Search over exports, type definitions and file names in the MCP `find_module` tool (today it matches only module names and purpose) | Planned |
| Per-module and per-symbol MCP responses sized to a token budget | Planned |
| Raise or remove the 10-export and 20-file-name caps per module without growing the default map | Planned |

Benchmark link: 2 of the 30 tasks missed because a symbol was unexported or cut by the export cap, and 3 of the 7 "what depends on X" tasks were wrong or incomplete.

## 2. Dependency accuracy

| Item | Status |
|------|--------|
| Exclude test files from dependency edges | Planned |
| Resolve relative imports to the exact module instead of a shortened name | Planned |
| Disambiguate modules that share a last path segment | Planned |
| Add the benchmark's hand-verified dependency answers as regression tests | Planned |

## 3. Languages

Shipped: Go, TypeScript, JavaScript, Python, Rust, Java, C#, Ruby, PHP, Kotlin, Swift, C and C++.

| Item | Status |
|------|--------|
| Import and export extraction for more languages (for example Scala, Dart, Elixir, Lua, Zig, shell, SQL, Vue, Svelte, Terraform, Protobuf) | In progress |
| Audit existing extractors (C#, Ruby, PHP, Swift, C and C++, Kotlin) against what the README claims, with fixtures | In progress |
| Per-language precision and recall reported in the benchmark | Planned |

## 4. Integrations

Shipped: Claude Code, Cursor and Aider setup, an MCP server, a GitHub Action, an npm installer.

| Item | Status |
|------|--------|
| Setup for more AI assistants, with dry-run and uninstall | In progress |
| A smaller digest format for injection into agent config files | In progress |
| Ingest of docs, architecture decision records, OpenAPI files, SQL schemas and ownership files | In progress |
| Verified binary downloads in the npm installer, Homebrew tap, shell completions | In progress |
| Editor extensions | Planned |

## 5. Monorepo and cross-repo

Shipped: workspace detection (pnpm, npm, yarn, Go, Turbo, Nx, Lerna, Cargo, conventions) and `--multi` scans that list several repos in one `stacklit-multi.json`.

| Item | Status |
|------|--------|
| Package boundaries and cross-package dependency edges inside a monorepo | In progress |
| Multi-repo index with cross-repo edges and no writes into member repos | In progress |
| Query across repos through the MCP server | Planned |

## 6. Performance and stability

The benchmark shows generation takes milliseconds on a Go repo but seconds and 1 to 2 GB of peak memory on the Python and TypeScript repos tested.

| Item | Status |
|------|--------|
| Parallel parsing with panic recovery and an on-disk incremental cache | In progress |
| Safer walking: symlinks, file size cap, nested ignore files, include globs | In progress |
| A `doctor` command, stable exit codes and clearer config errors | In progress |
| Fuzz tests for parsers and config | In progress |
| Lower peak memory in the tree-sitter parsers | Planned |
| `stacklit diff` exit code that signals staleness to scripts | Planned |

## 7. Benchmarks and docs

| Item | Status |
|------|--------|
| Reproducible token-cost benchmark on three pinned repos | Shipped (in `bench/`) |
| More repos, more tasks, an agent-in-the-loop variant with real model runs | Planned |
| Track benchmark results per release | Planned |

## How to influence this

Open a [feature request](https://github.com/glincker/stacklit/issues/new/choose), add a benchmark task that shows a gap, or send a pull request. See [CONTRIBUTING.md](CONTRIBUTING.md).
