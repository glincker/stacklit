# Stacklit

**108,000 lines of code. 4,000 tokens of index.**

One command makes any repo AI-agent-ready. No server, no setup.

[![CI](https://github.com/glincker/stacklit/actions/workflows/ci.yml/badge.svg)](https://github.com/glincker/stacklit/actions)
[![Release](https://img.shields.io/github/v/release/glincker/stacklit)](https://github.com/glincker/stacklit/releases)
[![npm](https://img.shields.io/npm/v/stacklit)](https://www.npmjs.com/package/stacklit)
[![License](https://img.shields.io/badge/license-MIT-green)](https://opensource.org/licenses/MIT)

## Install and run

```bash
npx stacklit init
```

That is it. Downloads the binary, scans your codebase, generates the index, opens the visual map. One command.

Other install options:

```bash
npm install -g stacklit              # install globally, then run: stacklit init
go install github.com/glincker/stacklit/cmd/stacklit@latest
```

Or grab a binary from [GitHub Releases](https://github.com/glincker/stacklit/releases) (macOS, Linux, Windows).

## CI / GitHub Action

Use [glincker/stacklit-action](https://github.com/glincker/stacklit-action) to keep the index fresh automatically. Auto-commit on push, or gate PRs with check mode:

```yaml
- uses: actions/checkout@v4
- uses: glincker/stacklit-action@v1        # auto-commit (default)
# or: with: { mode: check }               # fail PR if index is stale
```

Add `permissions: contents: write` to the job when using `auto-commit` mode.

![Stacklit demo](demo.gif)

## What happens when you run it

```
$ stacklit init
[stacklit] found 342 files
[stacklit] parsed 342 files (0 errors)
[stacklit] done in 89ms -- wrote stacklit.json, DEPENDENCIES.md, stacklit.html

Opening visual map...
```

Three files appear in your project:

| File | What it is | Commit it? |
|------|-----------|------------|
| `stacklit.json` | Codebase index for AI agents | **Yes** |
| `DEPENDENCIES.md` | Mermaid dependency diagram | **Yes** (renders on GitHub) |
| `stacklit.html` | Interactive visual map (4 views) | No (gitignored, regenerates) |

```bash
git add stacklit.json DEPENDENCIES.md
git commit -m "add stacklit codebase index"
```

Done. Every AI agent that opens this repo can now read `stacklit.json` instead of scanning files.

## Why

AI coding agents burn most of their context window figuring out where things live. Reading one large file to find a function signature costs thousands of tokens. Five agents on the same repo each rebuild the same mental model from scratch.

**Without stacklit:** Agent reads 8-12 files. ~400,000 tokens. 45 seconds before writing a line.

**With stacklit:** Agent reads `stacklit.json`. ~4,000 tokens. Knows the structure instantly.

### Token efficiency (measured on real projects)

| Project | Language | Lines of code | Index tokens |
|---------|----------|---------------|-------------|
| Express.js | JavaScript | 21,346 | 3,765 |
| FastAPI | Python | 108,075 | 4,142 |
| Gin | Go | 23,829 | 3,361 |
| Axum | Rust | 43,997 | 14,371 |

See [examples/](examples/) for full outputs.

## What is in stacklit.json

```json
{
  "modules": {
    "src/auth": {
      "purpose": "Authentication and session management",
      "files": 8, "lines": 1200,
      "exports": ["AuthProvider", "useSession()", "loginAction()"],
      "depends_on": ["src/db", "src/config"],
      "activity": "high"
    }
  },
  "hints": {
    "add_feature": "Create handler in src/api/, add route in src/index.ts",
    "test_command": "npm test"
  }
}
```

Modules, dependencies, exports with signatures, type definitions, git activity heatmap, framework detection, and hints for where to add features and how to run tests.

## Set up your AI tools

### One command (recommended)

```bash
stacklit setup
```

Auto-detects Claude Code, Cursor, and Aider. For each:
- Injects a compact ~250-token codebase map into the tool's config file
- Configures MCP server integration
- Installs a git hook to keep the map fresh on every commit

Or configure a specific tool:

```bash
stacklit setup claude   # updates CLAUDE.md + .mcp.json
stacklit setup cursor   # updates .cursorrules + .cursor/mcp.json
stacklit setup aider    # updates .aider.conf.yml
```

### Compact navigation map

```bash
stacklit derive         # print to stdout
```

Generates a ~250-token navigation map that replaces 3,000-8,000 tokens of agent exploration:

```
myapp | go | 14 modules | 8,420 lines
entry: cmd/api/main.go | test: go test ./...

modules:
  cmd/api/          entrypoint, routes, middleware
  internal/auth/    jwt, session | depends: store, config
  internal/store/   postgres | depended-by: auth, handler
```

### Manual setup

<details>
<summary>Configure manually instead</summary>

**Claude Code**  - add to `CLAUDE.md`:

```
Read stacklit.json before exploring files. Use modules to locate code, hints for conventions.
```

**Claude Desktop / Cursor (MCP)**  - add to MCP config:

```json
{
  "mcpServers": {
    "stacklit": {
      "command": "stacklit",
      "args": ["serve"]
    }
  }
}
```

MCP server exposes 7 tools: `get_overview`, `get_module`, `find_module`, `list_modules`, `get_dependencies`, `get_hot_files`, `get_hints`.

**Any other agent** - `stacklit.json` is a plain JSON file. Any tool that reads files can use it.

</details>

## Keep it updated

```bash
stacklit init --hook
```

Installs a git hook that regenerates the index on every commit. Uses Merkle hashing to skip regeneration when only docs or configs changed.

Other ways to keep it fresh:

```bash
stacklit generate          # manual regeneration
stacklit generate --quiet  # silent (for scripts/CI)
stacklit diff              # check if the index is stale
```

<details>
<summary>GitHub Action for auto-updates</summary>

```yaml
name: Update stacklit index
on:
  push:
    branches: [main]
    paths-ignore: ['stacklit.json', 'DEPENDENCIES.md', '**.md']

jobs:
  stacklit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go install github.com/glincker/stacklit/cmd/stacklit@latest
      - run: stacklit generate --quiet
      - uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "chore: update stacklit index"
          file_pattern: "stacklit.json DEPENDENCIES.md"
```

</details>

## Visual map

![Stacklit visual map](stacklit-og.png)

`stacklit view` opens the interactive HTML. Four views:

- **Graph** -- Force-directed dependency map. Click a node to see exports, types, files.
- **Tree** -- Collapsible directory hierarchy with file and line counts.
- **Table** -- Sortable module table with search filter.
- **Flow** -- Top-down dependency flow from entrypoints to leaves.

## 11 languages via tree-sitter

| Language | Extracts |
|----------|----------|
| Go | imports, exports with signatures, struct fields, interface methods |
| TypeScript/JS | imports (ESM, CJS, dynamic), classes, interfaces, type aliases |
| Python | imports, classes with methods, type hints, decorators |
| Rust | use/mod/crate, pub items with generics, trait methods |
| Java | imports, public classes, method signatures with types |
| C# | using directives, public types, method signatures |
| Ruby | require, classes, modules, methods |
| PHP | namespace use, classes, traits, public methods |
| Kotlin | imports, classes, objects, functions |
| Swift | imports, structs, classes, protocols |
| C/C++ | includes, functions, structs, typedefs |

Any other language gets basic support (line count + language detection).

## All CLI commands

```
stacklit init                    # scan, generate, open HTML
stacklit init --hook             # also install git post-commit hook
stacklit init --multi repos.txt  # polyrepo: scan multiple repos
stacklit generate                # regenerate from current source
stacklit view                    # regenerate HTML, open in browser
stacklit diff                    # check if index is stale
stacklit serve                   # start MCP server
stacklit derive                  # print compact nav map (~250 tokens)
stacklit derive --inject claude  # inject map into CLAUDE.md
stacklit setup                   # auto-configure all detected AI tools
stacklit setup claude            # configure Claude Code + MCP
stacklit setup cursor            # configure Cursor + MCP
```

<details>
<summary>Configuration (stacklit.toml)</summary>

```toml
ignore = ["vendor/", "generated/"]
max_depth = 3

[output]
json = "stacklit.json"
mermaid = "DEPENDENCIES.md"
html = "stacklit.html"
```

</details>

## How it compares

| Tool | Approach | Tokens | Committable | Visual map |
|------|----------|--------|-------------|------------|
| **Stacklit** | Structured index | ~250 | Yes | Yes |
| Repomix | Full dump | 50k-500k | No | No |
| code2prompt | Full dump | 50k-500k | No | No |
| Aider repo-map | Tree-sitter + PageRank | ~1k | No | No |

[Full comparison with 7 tools →](https://github.com/glincker/stacklit/discussions/13)

## Monorepo support

Auto-detects: pnpm, npm, yarn workspaces, Go workspaces, Turborepo, Nx, Lerna, Cargo workspaces, and convention directories (`apps/`, `packages/`, `services/`).

## How does Stacklit compare to Repomix?

Repomix concatenates all files into one prompt (50k-500k tokens). Stacklit parses code structure and generates a ~250-token navigation map. Use Repomix for small repos and one-shot chats. Use Stacklit for daily AI-assisted development on larger codebases. See the [full comparison](https://github.com/glincker/stacklit/discussions/13).

## FAQ

**Does Stacklit read my code?**
Yes, locally. It parses source files with tree-sitter to extract structure (imports, exports, types). No code is sent anywhere unless you use the optional `--summary` flag (which calls the Claude API).

**What if my language isn't supported?**
Stacklit falls back to basic support (line count + language detection) for any language not in the tree-sitter list. The module map, dependency graph, and git activity still work.

**Does the git hook slow down commits?**
No. Stacklit uses Merkle hashing to skip regeneration when only docs or configs changed. On a 10k-line repo, regeneration takes ~50ms.

**Can I use Stacklit with GitHub Copilot?**
Yes. Run `stacklit derive --inject claude` and rename the output to `.github/copilot-instructions.md`, or just commit `stacklit.json` and reference it in your Copilot instructions.

## Documentation

- [USAGE.md](USAGE.md) -- full usage guide, command reference, MCP tools, configuration
- [COMPARISON.md](COMPARISON.md) -- head-to-head comparison with Repomix, code2prompt, Codebase-Memory
- [SKILL.md](SKILL.md) -- instructions for AI agents on how to use stacklit.json
- [examples/](examples/) -- real stacklit.json outputs from Express.js, FastAPI, Gin, Axum
- [Discussions](https://github.com/glincker/stacklit/discussions) -- guides, Q&A, feature requests

## Contributing

```bash
make build   # build binary
make test    # run all tests
```

Contributions welcome. See [open issues](https://github.com/glincker/stacklit/issues) or start a [discussion](https://github.com/glincker/stacklit/discussions).

## License

MIT
