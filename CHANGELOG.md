# Changelog

All notable changes, reconstructed from the git history and release tags. Dates are tag dates (UTC). Versions follow semantic versioning while below 1.0, so minor releases may include behavior changes.

## Unreleased (0.5.0)

### Added
- Query layer: a symbol graph (`stacklit graph`) with `query` (token-budgeted subgraph for a question), `path`, `explain`, `impact`, `hubs`, `communities`, `report` and `slice`, an incremental `watch` command (fsnotify with a polling fallback), graph tools in the MCP server, and a Symbols tab in the visual map.
- 48 languages and formats with tree-sitter extraction, including Elixir, Zig, Dart, Scala, Lua, shell, SQL, Vue, Svelte, Terraform, Protobuf, Haskell, OCaml, Clojure, R, Perl, Julia, Objective-C, Nim, Erlang, Groovy, F#, Fortran, Elm, Solidity, PowerShell, Gleam, CMake, Makefile, Dockerfile, YAML, TOML, JSON and GraphQL. Audited C#, Ruby, PHP, Swift, C and C++ and Kotlin extractors.
- Integrations: `stacklit setup` for 26 AI assistants with dry-run, uninstall, marker blocks and optional Claude Code hooks; `stacklit ingest` for docs, ADRs, OpenAPI, SQL schemas and CODEOWNERS; a 300 to 500 token digest and `setup --report`; verified npm downloads, Homebrew tap, shell completions.
- Stability: parallel parse pool with panic recovery, an on-disk incremental cache, safer walking (symlinks, size cap, nested ignore files, include globs, scan roots), `stacklit doctor`, stable exit codes, `schema_version`, atomic output writes, fuzz tests, monorepo package boundaries and a robust multi-repo index with cross-repo edges.
- A reproducible benchmark harness in `bench/` with a third arm for the graph query path, plus rewritten README and USAGE, ARCHITECTURE.md, CONTRIBUTING.md, SECURITY.md, ROADMAP.md and issue and PR templates.

- Retrieval: `query` now ranks with BM25 over symbol name, signature, doc comment, a capped body word list and path, blended with PageRank. Identifier queries resolve exact then prefix first, snake_case and schema-prefixed names resolve, and quoted phrases filter. A 40-question gold set over this repo gates the top-5 rate.
- `stacklit status` (index and graph age versus git HEAD, files changed since) and a one-line stale warning on `query`. `STACKLIT_NO_STALE=1` silences it.
- `stacklit serve --profile minimal`: four tools (query, slice, impact, map) in about 230 tokens of schema; `full` stays the default.
- The opt-in PreToolUse hook now answers a grepped identifier with its definition and top callers (max 120 tokens), and stays silent without a graph.
- `init --hook` and `setup` also install quiet post-checkout, post-merge and post-rewrite refresh hooks.

### Changed
- `timestamps` and `git_activity` now default to off so a committed index is byte-stable. Opt in with the config keys or `STACKLIT_TIMESTAMPS=1` and `STACKLIT_GIT=1`.
- Tree-sitter trees are released after extraction, parsers are reused, and concurrent parse memory is gated (`parse_memory_mb`, `STACKLIT_PARSE_MEMORY_MB`). Cold peak memory on the benchmark repos fell from 2.2 GB to 1.6 GB (flask) and from 2.4 GB to 0.9 GB (hono).
- `init --workspace` now sets the directory to index. The `derive` help no longer promises a fixed token count, and `init --hook` is described as a post-commit hook.

## 0.4.0 (2026-05-06)

### Added
- `stacklit export`: render the index as a deterministic markdown overview for PRs, issues and chat (`--format`, `--source`, `-o`).
- `--summary` now runs through an agent CLI (`claude -p` by default). `STACKLIT_SUMMARY_CMD` and `STACKLIT_SUMMARY_TIMEOUT` configure it.
- Framework pattern detection: routes, API, middleware and entry points for known frameworks.
- Share button in the visual map to download a standalone HTML copy.
- GitHub Action (`glincker/stacklit-action`) for auto-indexing, plus a workflow that refreshes this repository's own index.

### Fixed
- Windows install: PowerShell `Expand-Archive`, hardened quoting, no silent failures.
- The npm installer reads its version from `package.json`.
- Trimmed module references in `depends_on` and `depended_by` are filtered correctly.
- Config docs aligned with the real `.stacklitrc.json` keys.
- The index workflow runs on pushes to the default branch with auto-commit, not on pull requests.

## 0.3.2 (2026-04-12)

### Fixed
- Generated outputs are deterministic and paths are normalized to forward slashes on every OS.
- Ignore patterns are normalized across platforms, and `diff` honors the configured index path.

## 0.3.1 (2026-04-11)

### Added
- npm publish is automated on tag push, using trusted publishing.

### Fixed
- Python dot-notation and relative-import shorthand resolve in the dependency graph.
- Go imports: dot-slash conversion is guarded and the longest module match wins.

## 0.3.0 (2026-04-10)

### Added
- `stacklit derive`: a compact navigation map, with `--inject` for `CLAUDE.md`, `.cursorrules` and `.aider.conf.yml`.
- `stacklit setup`: detects Claude Code, Cursor and Aider, injects the map, configures MCP and installs a git hook.
- Comparison document and expanded README and USAGE guide.

## 0.2.0 (2026-04-09)

### Added
- Tree-sitter AST parsing replaces the regex parsers for non-Go languages.
- Real-world example indexes for Express, FastAPI, Gin and Axum.
- `list_modules` MCP tool, cache invalidation when `stacklit.json` changes, table search in the HTML map.

### Changed
- The Mermaid output is `DEPENDENCIES.md` (was `stacklit.mmd`) in a code fence with `<br/>` labels, so it renders on GitHub.

### Fixed
- npm package: missing bin wrapper.

## 0.1.0 (2026-04-09)

First release.

- Pipeline: gitignore-aware walker, parsers (Go via the standard library, TypeScript, Python, Rust, Java and a generic fallback), module dependency graph, git activity and Merkle change detection, monorepo workspace detection.
- Outputs: `stacklit.json`, Mermaid dependency diagram and a self-contained HTML map with Graph, Tree, Table and Flow views.
- Commands: `init`, `generate`, `view`, `diff`, `serve` (MCP server), `init --multi` for polyrepo scans, `--summary` for an AI-written narrative.
- Framework and environment variable detection, per-module exports with signatures, type definitions and hints.
- MIT license, GoReleaser cross-platform builds, CI, npm wrapper (`npx stacklit`) and a Claude Code skill.
