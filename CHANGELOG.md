# Changelog

All notable changes, reconstructed from the git history and release tags. Dates are tag dates (UTC). Versions follow semantic versioning while below 1.0, so minor releases may include behavior changes.

## Unreleased

- Docs: README rewritten around measured numbers, full command reference in USAGE.md, new ARCHITECTURE.md, CONTRIBUTING.md, SECURITY.md, ROADMAP.md, issue and PR templates.
- Added a reproducible benchmark harness in `bench/` (three pinned repos, 30 navigation tasks, no LLM calls) and its results.

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
