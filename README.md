# Stacklit

**A committed map of your codebase, built for AI coding agents.**

Stacklit parses your repo with tree-sitter and writes `stacklit.json`: modules, what each exports, who depends on whom, which files are hot in git, and hints for running tests and adding features. A coding agent reads that file (or a smaller derived map of it) instead of rediscovering the layout every session.

[![CI](https://github.com/glincker/stacklit/actions/workflows/ci.yml/badge.svg)](https://github.com/glincker/stacklit/actions)
[![Release](https://img.shields.io/github/v/release/glincker/stacklit)](https://github.com/glincker/stacklit/releases)
[![npm](https://img.shields.io/npm/v/stacklit)](https://www.npmjs.com/package/stacklit)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

> **Fully open source. MIT licensed. No paid tier, no gated features, no account.** Everything in this repository is everything there is: the CLI, the MCP server, the visual map, the GitHub Action and the npm installer.

![Stacklit demo](demo.gif)

## Quickstart

```bash
npx stacklit init
```

That downloads the binary, scans the current directory, writes the index and opens the visual map. Then commit the two portable outputs:

```bash
git add stacklit.json DEPENDENCIES.md
git commit -m "add stacklit codebase index"
```

Other ways to install:

```bash
npm install -g stacklit
go install github.com/glincker/stacklit/cmd/stacklit@latest
```

Or download a binary for macOS, Linux or Windows from [GitHub Releases](https://github.com/glincker/stacklit/releases).

To wire your AI tools up in one step:

```bash
stacklit setup        # detects Claude Code, Cursor and Aider, configures each, installs a git hook
```

## What you get

| File | What it is | Commit it? |
|------|-----------|------------|
| `stacklit.json` | The index agents read | Yes |
| `DEPENDENCIES.md` | Mermaid dependency diagram, renders on GitHub | Yes |
| `stacklit.html` | Self-contained interactive map (graph, tree, table, flow) | No, it is gitignored and regenerates |

## Measured numbers

All figures below come from the reproducible harness in [bench/](bench/) (full methodology and limitations in [bench/RESULTS.md](bench/RESULTS.md)). Three pinned public repos, 30 navigation tasks, no LLM calls. Tokens are approximated as bytes / 4.

| Repo | Language | Source lines | Modules | `stacklit.json` | `stacklit derive` map | Generate (median) |
|------|----------|-------------:|--------:|----------------:|----------------------:|------------------:|
| gin v1.10.0 | Go | 19,161 | 6 | 2,410 tokens | 122 tokens | 39 ms |
| flask 3.1.2 | Python | 17,909 | 9 | 2,045 tokens | 164 tokens | 3.7 s |
| hono v4.6.0 | TypeScript | 55,896 | 72 | 11,814 tokens | 1,122 tokens | 2.3 s |

What that says, and what it does not:

- The index is roughly 2,000 to 12,000 tokens on these repos, and the derived map is roughly 120 to 1,100 tokens. Both grow with the number of modules, not with lines of code. The map is about 250 tokens only for repos of around 10 to 25 modules.
- On the 30 tasks, a simulated agent that greps and then reads its top 3 files spent about 2.4x more tokens end to end than one that read the map, looked up modules in the index and read 1 file. The ratio ranged from roughly 1x on hono to about 7x on gin.
- For lookups where you already know the exact symbol name, grep alone is cheaper than the map (a few hundred tokens versus roughly 700 including the map).
- The grep and read path found the verified answer on 30 of 30 tasks. The stacklit path found it on 25 of 30. The misses are mostly unexported symbols, truncated export lists and imperfect dependency edges. They are listed in the results file and tracked on the [roadmap](ROADMAP.md).
- Generating the index is fast on Go and slower on Python and TypeScript, with peak memory around 1 to 2 GB on the larger two. That is a known cost and is on the roadmap.

This is a token-cost proxy, not an agent evaluation. See the limitations section of the results file before quoting any figure.

## Supported AI tools

| Tool | How | Command |
|------|-----|---------|
| Claude Code | Injects the compact map into `CLAUDE.md`, adds an MCP server entry to `.mcp.json` | `stacklit setup claude` |
| Cursor | Injects the map into `.cursorrules`, adds `.cursor/mcp.json` | `stacklit setup cursor` |
| Aider | Injects the map into `.aider.conf.yml` | `stacklit setup aider` |
| Any MCP client | Run `stacklit serve` (stdio) | see [USAGE.md](USAGE.md#mcp-server) |
| Anything else | `stacklit.json` is plain JSON, point the agent at it | |

`stacklit setup` with no argument configures every tool it detects and installs a post-commit git hook that regenerates the index.

The MCP server exposes seven tools: `get_overview`, `get_module`, `find_module`, `list_modules`, `get_dependencies`, `get_hot_files`, `get_hints`.

## Supported languages

| Language | Parser | Extracts |
|----------|--------|----------|
| Go | Go standard library AST | imports, exports with signatures, struct fields, interface methods |
| TypeScript, JavaScript (incl. TSX, JSX) | tree-sitter | imports (ESM, CJS, dynamic), classes, interfaces, type aliases |
| Python | tree-sitter | imports, classes with methods, type hints, decorators |
| Rust | tree-sitter | use, mod, crate, pub items with generics, trait methods |
| Java | tree-sitter | imports, public classes, method signatures |
| C# | tree-sitter | using directives, public types, method signatures |
| Ruby | tree-sitter | require, classes, modules, methods |
| PHP | tree-sitter | namespace use, classes, traits, public methods |
| Kotlin | tree-sitter | imports, classes, objects, functions |
| Swift | tree-sitter | imports, structs, classes, protocols |
| C, C++ | tree-sitter | includes, functions, structs, typedefs |

Other source files (Scala, Dart, Lua, SQL, Vue, Svelte and similar) are counted and language-detected but not parsed for exports or imports. More languages are on the [roadmap](ROADMAP.md).

Monorepos are detected automatically: pnpm, npm and yarn workspaces, Go workspaces, Turborepo, Nx, Lerna, Cargo workspaces, and `apps/`, `packages/`, `services/`, `libs/` conventions.

## What is in stacklit.json

```json
{
  "modules": {
    "src/auth": {
      "purpose": "Authentication and authorization",
      "files": 8, "lines": 1200,
      "exports": ["AuthProvider", "useSession()", "loginAction()"],
      "depends_on": ["src/db", "src/config"],
      "depended_by": ["src/api"],
      "activity": "high"
    }
  },
  "hints": {
    "add_feature": "Add handler in src/api, register in src/index.ts",
    "test_command": "npm test"
  }
}
```

Project and tech summary, modules with exports and type definitions, module-level dependency edges, git hot files, framework patterns and hints. See [USAGE.md](USAGE.md#reading-stacklitjson) for every field.

The compact map from `stacklit derive` looks like this (gin, 122 tokens):

```
gin | go | 6 modules | 19,161 lines | Gin, GitHub Actions, net/http
test: go test ./... | env: PORT

modules:
  binding             binding | depends: bytesconv, json
  ginS                gins
  internal/bytesconv  bytesconv | depended-by: binding, render, root
  internal/json       json | depended-by: binding, render, root
  render              render | depends: bytesconv, json
  root                root package | depends: binding, bytesconv, json, render
do-not-touch: .github/
```

## Keeping the index fresh

```bash
stacklit generate            # regenerate now
stacklit generate --quiet    # silent, for scripts and CI
stacklit diff                # is the index stale? (compares a content hash)
stacklit init --hook         # post-commit hook that regenerates on every commit
```

In CI, [glincker/stacklit-action](https://github.com/glincker/stacklit-action) either auto-commits a refreshed index or fails a PR when it is stale. A hand-written workflow is in [USAGE.md](USAGE.md#cicd).

## Privacy

Stacklit runs locally. It reads your source files on your machine, writes its outputs into your repository, and makes no network requests.

- No telemetry, no analytics, no account, no upload.
- The generated HTML map is self-contained and loads nothing from the network.
- The only network activity in the whole project happens outside the program: `npx stacklit` and `npm install -g stacklit` download the release binary from GitHub when you install.
- The one opt-in exception is `stacklit init --summary`. It pipes a snapshot of the index (module names, exports, dependency edges, entrypoints; no file contents) to a command you control, `claude -p` by default or whatever `STACKLIT_SUMMARY_CMD` names. Whether that command talks to a network is up to that command. Without the flag, nothing is sent anywhere.

`stacklit.json` lists names of modules, files and exported symbols, and detected environment variable names (not values). Review it before committing a private repo's index to a public place.

## FAQ

**Does it read my code?**
Yes, locally, to extract structure with tree-sitter. Nothing leaves your machine unless you pass `--summary` (see Privacy).

**Does it help on small repos?**
Less. If the whole repo fits comfortably in a prompt, an agent can just read it. The index pays off once exploring costs more than reading a map, and it saves the most when the agent would otherwise read several large files to confirm a location.

**When is grep better?**
When you already know the exact identifier. A single grep is cheaper than loading a map. The index helps with questions grep cannot answer in one step, such as "what depends on this module" or "where would a new adapter go".

**How accurate are the dependency edges?**
Module-level edges are built from import statements and are good enough to orient an agent, but not exact. In the benchmark, 4 of 7 "what depends on X" lookups were fully correct. Known weak spots: test files counted as dependents, relative imports resolved to the wrong module, and short module names that can be ambiguous. Treat the graph as a guide and confirm with a read.

**What if my language is not supported?**
Files are still counted and language-detected, and the module map, git activity and hints still work. Exports and dependency edges need a parser.

**Does the git hook slow commits down?**
It runs `stacklit generate --quiet` after each commit. On Go repos of this size that is tens of milliseconds; on Python and TypeScript repos in the benchmark it took a few seconds. If that is too slow, run `stacklit generate` manually or in CI instead.

**Can I use it with Copilot?**
Yes. Commit `stacklit.json` and tell Copilot to read it in `.github/copilot-instructions.md`, or paste the output of `stacklit derive` there.

**How is it different from dumping the whole repo into a prompt?**
A dump is the full source. Stacklit is structure only: it is orders of magnitude smaller but cannot answer questions that need code bodies. See [COMPARISON.md](COMPARISON.md) for a factual feature comparison.

## Documentation

- [USAGE.md](USAGE.md): full command reference, MCP tools, configuration, index schema
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): how the pipeline works
- [bench/RESULTS.md](bench/RESULTS.md): benchmark methodology and results
- [ROADMAP.md](ROADMAP.md): what is planned and what is in progress
- [CHANGELOG.md](CHANGELOG.md): release history
- [COMPARISON.md](COMPARISON.md): feature comparison with other context tools
- [SKILL.md](SKILL.md): instructions an agent can follow to use `stacklit.json`
- [examples/](examples/): sample outputs (generated with earlier versions)
- [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md)

## Contributing

```bash
make build   # build the binary
make test    # run all tests
```

See [CONTRIBUTING.md](CONTRIBUTING.md). Questions and ideas are welcome in [Discussions](https://github.com/glincker/stacklit/discussions).

## License

MIT. See [LICENSE](LICENSE).
