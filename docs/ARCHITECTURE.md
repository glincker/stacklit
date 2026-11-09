# Architecture

Stacklit is one Go binary. It walks a repository, parses each source file into a small record of imports and exports, groups files into modules, resolves imports into module-level edges, and writes the result in several formats.

The core path has no daemon, no database and no network client.

## Pipeline

`stacklit generate` (and `init`) run one pipeline, implemented in `internal/engine/engine.go`:

```
walk files -> parse each file -> build module graph -> git activity -> assemble index -> render outputs
```

| Step | Package | What it does |
|------|---------|--------------|
| Config | `internal/config` | Loads an optional `.stacklitrc.json` over the defaults (depth, module and export caps, ignore patterns, output names) |
| Monorepo detection | `internal/monorepo` | Detects pnpm, npm and yarn workspaces, `go.work`, Turborepo, Nx, Lerna, Cargo workspaces and convention directories |
| Walk | `internal/walker` | Lists indexable source files in sorted order. Honors `.gitignore`, config ignores and built-in skips (`node_modules`, `vendor`, `dist`, `build`, `target`, virtualenvs). Keeps only known source extensions. |
| Parse | `internal/parser` | Turns each file into a `FileInfo`: language, imports, exports with signatures, type definitions, line count and an entrypoint flag |
| Graph | `internal/graph` | Groups files into modules by directory, resolves imports to modules, and produces edges, most-depended and isolated lists |
| Detect | `internal/detect` | Frameworks (from imports and config files), framework patterns (routes, API, middleware, entry), and environment variable names |
| Git | `internal/git` | 90-day commit counts per file (hot files), recent files, the Merkle content hash, and the post-commit hook installer |
| Assemble | `internal/engine` | Builds the `schema.Index`, derives purpose strings and hints (add feature, test command, do-not-touch) |
| Summary (optional) | `internal/summary` | With `--summary`, runs a user-configured command on an index snapshot |
| Render | `internal/renderer` | Writes `stacklit.json`, `DEPENDENCIES.md` (Mermaid), `stacklit.html` and the markdown export |

Other consumers of the index:

| Package | Role |
|---------|------|
| `internal/derive` | Builds the compact navigation map and injects it between marker comments in agent config files |
| `internal/setup` | Detects Claude Code, Cursor and Aider, and writes their config and MCP entries |
| `internal/mcp` | MCP server over stdio, backed by `stacklit.json` |
| `internal/cli` | Cobra commands that wire the above together |
| `internal/schema` | The `Index` types shared by every package |

## Parsers

`internal/parser/parser.go` keeps an ordered registry. For each file, it uses the first parser whose `CanParse` accepts the file.

1. **`GoParser`** uses the standard library `go/parser`. It gives exact signatures and struct fields, with no external dependency.
2. **`TreeSitterParser`** handles TypeScript, JavaScript, TSX, JSX, Python, Rust, Java, C#, Ruby, PHP, Kotlin, Swift, C and C++. It uses a pure-Go tree-sitter runtime (no cgo). The binary therefore cross-compiles to every release target without a C toolchain. Each language has an extractor in `internal/parser/ts_*.go`.
3. **`GenericParser`** is the fallback. It records only the line count and language.

To add a language, write an extractor and register it in `tsLangMap`. See [CONTRIBUTING.md](../.github/CONTRIBUTING.md).

## Modules and dependency resolution

A module is a directory.

- A file belongs to its parent directory, cut to at most `max_depth` path segments.
- Files at the repo root belong to `root`.
- Modules with a path segment named `test`, `tests`, `__tests__`, `testdata`, `fixtures`, `e2e`, `benchmarks`, `bench` or `docs_src` are left out of the graph.
- When the module count would go over `max_modules`, deeper modules fold into their parents.

Edges come from import statements:

- Each import string is matched against the known module names. The longest match wins.
- Relative imports resolve against the importing file's directory.
- Python dot-style imports are normalized first.
- Go imports match on the module path.
- Anything that matches no module is treated as external and dropped.

This is deliberately approximate. It works from import text, not from a type-checked symbol graph. It can miss dynamic imports and re-exports, and it can add edges that come from test files. [bench/RESULTS.md](../bench/RESULTS.md) measures how often the "what depends on X" answers are exactly right.

## The index

`internal/schema` defines `stacklit.json`. Its top-level sections are `project`, `tech`, `structure`, `modules`, `dependencies`, `git`, `hints`, `architecture` and `merkle_hash`. For a field-by-field description, see [USAGE.md](USAGE.md#reading-stacklitjson).

Determinism is a design goal. Files are walked in sorted order, maps are written with stable key order, and paths are normalized to forward slashes. The same source tree should give the same index on any OS. That is what makes the index safe to commit and to diff.

## Symbol graph extraction

`internal/graph` turns source files into symbols (functions, methods and types) and edges between them.

- Go uses the standard library AST.
- Every other language goes through one shared tree-sitter driver, `ts_driver.go`. The driver walks the parse tree once. A small table per language sets the details: which node types define symbols, which are calls, and how imports and base types are read. The tables live in `ts_lang_core.go` and `ts_lang_more.go`.
- Each symbol carries its start and end line, a one-line signature, the first sentence of its doc comment, and its visibility.

### Memory limits

- Concurrent parses share a weighted gate, sized by `STACKLIT_PARSE_MEMORY_MB` (default 512).
- A file larger than `STACKLIT_GRAPH_MAX_PARSE_BYTES` (default 192 KiB) skips tree-sitter.
- Python, TypeScript and JavaScript, Rust and Java fall back to the older line scanner in that case. Other languages keep only their file node.
- The line scanner also takes over when a Python tree disagrees with the file's indentation. The grammar runtime can drop a block without an error.
- Test files are skipped unless you pass `--include-tests` or set `STACKLIT_GRAPH_INCLUDE_TESTS=1`.

### Edge kinds by language

- **def**: the symbol has a span, a signature, a doc comment and a visibility.
- **typed**: receiver types are inferred from annotated parameters, typed locals, typed fields or constructor assignments. So `repo.find()` links to `Repo.find`, not to every `find`.

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

When a clause lists bases without saying which are classes (C#, Swift, Scala, Solidity), the first non-interface name counts as `inherits`. The rest, and any name shaped like `IName`, count as `implements`.

### Call resolution and confidence

Resolution is one pass over hash maps that are built once. So the cost grows linearly with the number of references.

A call is resolved in this order. The first rule that matches wins.

1. The same file, including the enclosing type's own methods.
2. Names imported from a file in the repo, then the package or namespace of the file.
3. Wildcard and namespace imports, in sorted order, so the output is stable.
4. A typed receiver (`@Type`), then that type's ancestors. For `this`, `self` and `super`, the enclosing type and its ancestors.
5. A qualifier bound to an import (`mod.fn`, `Alias.method`).
6. A last resort: a name guess across the repo.

A receiver whose type is known but lives outside the repo (`String`, or a library class) links to nothing. It does not link to a project method with the same name.

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
| 0.45 | INFERRED | Receiver type is in the repo, but no method was found on it |
| 0.25 to 0.50 | INFERRED | Several candidates (at most four). The score is split by candidate count. |
| 0.30 | INFERRED | Generic names such as `get` or `close` with an unknown receiver |

Prototype-only declarations (C and C++ headers) are dropped as call targets when a definition with the same name is also a candidate.

## Staleness detection

`ComputeMerkle` hashes the sorted list of source files and their contents into one root hash. The hash is stored as `merkle_hash`. `stacklit diff` recomputes it and compares.

The hash covers only indexed source files. Edits to docs or config files do not make the index look stale.

## Compact map and injection

`derive.CompactMap` prints one line per module. The line holds the purpose, up to four dependencies, and up to four dependents when there are three or more. The map also has a header, hot files and hints.

`InjectIntoFile` wraps the map in start and end marker comments. Re-runs replace the block in place, so agent config files stay clean across regenerations. The map grows with the module count, which is why `max_modules` also bounds its size.

## MCP server

`internal/mcp` registers fifteen read-only tools with the official Go MCP SDK. Seven read the index and eight read the symbol graph. It serves them over stdio. Each call loads `stacklit.json` from the working directory and caches it until the file's modification time changes.

## Network use

The core path makes no network calls.

- `internal/summary` runs a local command only when you pass `--summary`. The command is `claude -p` by default, or whatever `STACKLIT_SUMMARY_CMD` names.
- The repository also has `internal/summary/ai.go`, a direct API client. No command calls it today.
- The HTML output embeds everything it needs and loads nothing remotely.
- The npm wrapper downloads the release binary from GitHub when you install.

## Release and CI

- `.github/workflows/ci.yml`: `go vet`, `go test -race`, and a build on Linux, macOS and Windows.
- `.github/workflows/release.yml` with `.goreleaser.yml`: cross-compiled binaries for Linux, macOS and Windows on amd64 and arm64, plus an npm publish with trusted publishing.
- `.github/workflows/stacklit-index.yml`: regenerates this repository's own index on pushes to the default branch.
- `action/action.yml`: a composite action that installs stacklit and runs `generate`.
- `npm/`: the `npx stacklit` wrapper that downloads the matching release binary.

## Benchmarks

`bench/` is a separate `package main` inside the same module. It runs the built `stacklit` binary and reads `stacklit.json` as generic JSON. It does not import the internal packages, so it cannot change the shipped binary.
