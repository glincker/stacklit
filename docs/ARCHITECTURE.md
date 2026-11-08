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

## Symbol graph extraction

`internal/graph` turns source files into symbols (functions, methods, types) and edges between them. Go uses the standard library AST. Every other language goes through one shared tree-sitter driver (`ts_driver.go`) that walks the parse tree once and is configured by a small table per language (`ts_lang_core.go`, `ts_lang_more.go`): which node types define symbols, which are calls, how imports and base types are read. Each symbol carries start and end line, a one-line signature, the first sentence of its doc comment and its visibility.

Memory is bounded. Concurrent parses share a weighted gate (`STACKLIT_PARSE_MEMORY_MB`, default 512), and a file larger than `STACKLIT_GRAPH_MAX_PARSE_BYTES` (default 192 KiB) skips tree-sitter. Python, TypeScript/JavaScript, Rust and Java then fall back to the older line scanner; other languages keep only their file node. The line scanner also takes over when a Python tree contradicts the file's indentation, because the grammar runtime can silently drop a block. Test files are skipped unless `--include-tests` or `STACKLIT_GRAPH_INCLUDE_TESTS=1` is set.

### Edge kinds by language

`def` means symbols with spans, signature, doc and visibility. `typed` means receiver types are inferred from annotated parameters, typed locals, typed fields or constructor assignments, so `repo.find()` links to `Repo.find` instead of every `find`.

| Language | def | calls | imports | inherits | implements | typed |
| --- | --- | --- | --- | --- | --- | --- |
| Go | yes | yes | yes | embedding | structural (inferred) | yes |
| Python | yes | yes | yes | yes | no | yes |
| TypeScript, JavaScript | yes | yes | yes (import, require, re-export) | yes | yes (TypeScript) | yes |
| Rust | yes | yes | yes (use, mod) | trait bounds | impl Trait for | yes |
| Java | yes | yes | yes | yes | yes | yes |
| C# | yes | yes | yes (using) | yes | by position and I-prefix | yes |
| Kotlin | yes | yes | yes | yes | yes | yes |
| Swift | yes | yes | module imports | yes | by position | yes |
| PHP | yes | yes | yes (use) | yes | yes | yes |
| Ruby | yes | yes | yes (require, require_relative) | yes | include, extend | yes |
| C | yes | yes | yes (#include, header and source pair) | no | no | yes |
| C++ | yes | yes | yes (#include) | yes | no | yes |
| Scala | yes | yes | yes | yes | yes | yes |
| Dart | yes | yes | yes | yes | with, implements | yes |
| Elixir | yes | yes | yes (alias, import, use, require) | no | no | no |
| Lua | yes | yes | yes (require) | no | no | no |
| Zig | yes | yes | yes (@import) | no | no | no |
| Haskell | yes | yes | yes | no | no | no |
| OCaml | yes | yes | yes (open, qualified paths) | no | no | no |
| Solidity | yes | yes | yes | yes | by I-prefix | yes |

Where a clause lists bases without saying which are classes (C#, Swift, Scala, Solidity), the first non-interface name is `inherits` and the rest, or any name shaped like `IName`, are `implements`.

### Call resolution and confidence

Resolution is one pass over hash maps built once, so cost grows linearly with references. A call is resolved in this order, and the first rule that matches wins:

1. Same file, including the enclosing type's own methods.
2. Names imported from a file in the repo, then the package or namespace of the file.
3. Wildcard and namespace imports, in sorted order so output is stable.
4. A typed receiver (`@Type`), then that type's ancestors, then (for `this`, `self`, `super`) the enclosing type and its ancestors.
5. A qualifier bound to an import (`mod.fn`, `Alias.method`).
6. Last resort, a name guess across the repo.

A receiver whose type is known but lives outside the repo (`String`, a library class) links to nothing, rather than to a project method with the same name.

| Score | Confidence | Meaning |
| --- | --- | --- |
| 1.00 | EXTRACTED | Same-file definition, or a unique type in the same file |
| 0.95 | EXTRACTED | Imported name that resolves to one definition |
| 0.90 | EXTRACTED | Qualified through an import, a wildcard import, a typed receiver or `this` |
| 0.85 | EXTRACTED | Static call on a project type |
| 0.80 | EXTRACTED | Method found on an ancestor type, or a member of a bound module whose owner name differs |
| 0.60 | INFERRED | Base type found only by unique name |
| 0.55 | INFERRED | Unknown receiver, one method of that name in the repo |
| 0.50 | INFERRED | Unqualified name, one function of that name in the repo |
| 0.45 | INFERRED | Receiver type is in the repo but no method was found on it |
| 0.25 to 0.50 | INFERRED | Several candidates (at most four), the score is split by candidate count |
| 0.30 | INFERRED | Generic names such as `get` or `close` with an unknown receiver |

Prototype-only declarations (C and C++ headers) are dropped as call targets when a definition of the same name is also a candidate.

## Staleness detection

`ComputeMerkle` hashes the sorted list of source files and their contents into a single root hash, stored as `merkle_hash`. `stacklit diff` recomputes it and compares. It covers only indexed source files, so edits to docs or configs do not make the index look stale.

## Compact map and injection

`derive.CompactMap` prints one line per module (purpose, up to four dependencies, up to four dependents when it has three or more), plus header, hot files and hints. `InjectIntoFile` wraps that in start and end marker comments and replaces the block in place on re-runs, so agent config files stay clean across regenerations. The map grows with module count, which is why `max_modules` also bounds its size.

## MCP server

`internal/mcp` registers fifteen read-only tools (seven over the index, eight over the symbol graph) with the official Go MCP SDK and serves them over stdio. Each call loads `stacklit.json` from the working directory, caching it until the file's modification time changes.

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
