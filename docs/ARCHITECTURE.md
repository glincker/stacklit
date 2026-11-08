# Architecture

Stacklit is a single Go binary. It walks a repository, parses each source file into a small record of imports and exports, groups files into modules, resolves imports into module-level edges, and renders the result in several formats. There is no daemon, database or network client in the core path.

## Pipeline

`stacklit generate` (and `init`) run one pipeline, implemented in `internal/engine/engine.go`:

```
walk files -> parse each file -> build module graph -> git activity -> assemble index -> render outputs
```

| Step | Package | What it does |
|------|---------|--------------|
| Config | `internal/config` | Loads optional `.stacklitrc.json` over defaults (depth, module and export caps, ignore patterns, output names) |
| Monorepo detection | `internal/monorepo` | Detects pnpm, npm and yarn workspaces, `go.work`, Turborepo, Nx, Lerna, Cargo workspaces and convention directories |
| Walk | `internal/walker` | Lists indexable source files in sorted order. Honors `.gitignore`, config ignores and built-in skips (`node_modules`, `vendor`, `dist`, `build`, `target`, virtualenvs). Keeps only known source extensions. |
| Parse | `internal/parser` | Turns each file into a `FileInfo`: language, imports, exports with signatures, type definitions, line count, entrypoint flag |
| Graph | `internal/graph` | Groups files into modules by directory, resolves imports to modules, produces edges, most-depended and isolated lists |
| Detect | `internal/detect` | Frameworks (from imports and config files), framework patterns (routes, API, middleware, entry), environment variable names |
| Git | `internal/git` | 90-day commit counts per file (hot files), recent files, Merkle content hash, post-commit hook installer |
| Assemble | `internal/engine` | Builds the `schema.Index`, derives purpose strings, hints (add feature, test command, do-not-touch) |
| Summary (optional) | `internal/summary` | With `--summary`, runs a user-configured command on an index snapshot |
| Render | `internal/renderer` | `stacklit.json`, `DEPENDENCIES.md` (Mermaid), `stacklit.html`, markdown export |

Other consumers of the index:

| Package | Role |
|---------|------|
| `internal/derive` | Builds the compact navigation map and injects it between marker comments into agent config files |
| `internal/setup` | Detects Claude Code, Cursor and Aider and writes their config and MCP entries |
| `internal/mcp` | MCP server over stdio, backed by `stacklit.json` |
| `internal/cli` | Cobra commands that wire the above together |
| `internal/schema` | The `Index` types shared by every package |

## Parsers

`internal/parser/parser.go` keeps an ordered registry and uses the first parser whose `CanParse` accepts the file:

1. `GoParser` uses the standard library `go/parser`. It gives exact signatures and struct fields with no external dependency.
2. `TreeSitterParser` handles TypeScript, JavaScript, TSX, JSX, Python, Rust, Java, C#, Ruby, PHP, Kotlin, Swift, C and C++. It uses a pure-Go tree-sitter runtime (no cgo), so the binary cross-compiles to every release target without a C toolchain. Each language has an extractor in `internal/parser/ts_*.go`.
3. `GenericParser` is the fallback. It records line count and language only.

Adding a language means adding an extractor and registering it in `tsLangMap`; see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Modules and dependency resolution

A module is a directory. A file's module is its parent directory, collapsed to at most `max_depth` path segments; files at the repo root belong to `root`. Modules with a path segment named `test`, `tests`, `__tests__`, `testdata`, `fixtures`, `e2e`, `benchmarks`, `bench` or `docs_src` are left out of the graph. When the module count would exceed `max_modules`, deeper modules fold into their parents.

Edges are built from import statements: each import string is matched against the known module names, preferring the longest match. Relative imports are resolved against the importing file's directory; Python dot-style imports are normalized first; Go imports are matched on the module path. Anything that matches no module is treated as external and dropped.

This is deliberately approximate. It works from import text, not from a type-checked symbol graph, so it can miss dynamic imports or re-exports, and it can add edges that come from test files. [bench/RESULTS.md](../bench/RESULTS.md) measures how often the resulting "what depends on X" answers are exactly right.

## The index

`internal/schema` defines `stacklit.json`. Top-level sections: `project`, `tech`, `structure`, `modules`, `dependencies`, `git`, `hints`, `architecture` and `merkle_hash`. Field-by-field description: [USAGE.md](../USAGE.md#reading-stacklitjson).

Determinism is a design goal: files are walked in sorted order, maps are written with stable key order, and paths are normalized to forward slashes. The same source tree should produce the same index on any OS, which is what makes it safe to commit and to diff.

## Staleness detection

`ComputeMerkle` hashes the sorted list of source files and their contents into a single root hash, stored as `merkle_hash`. `stacklit diff` recomputes it and compares. It covers only indexed source files, so edits to docs or configs do not make the index look stale.

## Compact map and injection

`derive.CompactMap` prints one line per module (purpose, up to four dependencies, up to four dependents when it has three or more), plus header, hot files and hints. `InjectIntoFile` wraps that in start and end marker comments and replaces the block in place on re-runs, so agent config files stay clean across regenerations. The map grows with module count, which is why `max_modules` also bounds its size.

## MCP server

`internal/mcp` registers seven read-only tools with the official Go MCP SDK and serves them over stdio. Each call loads `stacklit.json` from the working directory, caching it until the file's modification time changes.

## Network use

None in the core path. `internal/summary` only runs a local command (`claude -p` by default, or `STACKLIT_SUMMARY_CMD`) and only when `--summary` is passed. The repository also contains `internal/summary/ai.go`, a direct API client that no command currently calls. The HTML output embeds everything it needs and loads nothing remotely. The npm wrapper downloads the release binary from GitHub at install time.

## Release and CI

- `.github/workflows/ci.yml`: `go vet`, `go test -race` and a build on Linux, macOS and Windows.
- `.github/workflows/release.yml` with `.goreleaser.yml`: cross-compiled binaries for Linux, macOS and Windows on amd64 and arm64, plus npm publish with trusted publishing.
- `.github/workflows/stacklit-index.yml`: regenerates this repository's own index on pushes to the default branch.
- `action/action.yml`: a composite action that installs stacklit and runs `generate`.
- `npm/`: the `npx stacklit` wrapper that downloads the matching release binary.

## Benchmarks

`bench/` is a separate `package main` inside the same module. It shells out to the built `stacklit` binary and reads `stacklit.json` as generic JSON, so it does not import the internal packages and cannot affect the shipped binary.
