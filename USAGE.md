# Stacklit usage guide

Complete reference for the `stacklit` command line, the MCP server, configuration and the `stacklit.json` format. For a short introduction see the [README](README.md).

Contents: [Install](#install) | [First run](#first-run) | [Command reference](#command-reference) | [MCP server](#mcp-server) | [Configuration](#configuration) | [Environment variables](#environment-variables) | [Reading stacklit.json](#reading-stacklitjson) | [Monorepos](#monorepo-support) | [Polyrepo scanning](#polyrepo-scanning) | [CI/CD](#cicd) | [Troubleshooting](#troubleshooting)

---

## Install

Pick one:

```bash
npx stacklit init                                              # no install, downloads the binary on first use
npm install -g stacklit                                        # global install
go install github.com/glincker/stacklit/cmd/stacklit@latest    # from source, needs Go
```

Or download a binary for macOS, Linux or Windows (amd64 and arm64) from [Releases](https://github.com/glincker/stacklit/releases).

Check the install:

```bash
stacklit --version
```

---

## First run

```bash
cd your-project
stacklit init
```

Typical output:

```
[stacklit] found 342 files
[stacklit] parsed 342 files (0 errors)
[stacklit] done in 89ms -- wrote stacklit.json, DEPENDENCIES.md, stacklit.html

Opening visual map...
```

(File counts and timing vary. Go repos index in milliseconds; Python and TypeScript repos can take seconds, see [bench/RESULTS.md](bench/RESULTS.md).)

Commit the portable outputs. `stacklit.html` is gitignored and regenerates with `stacklit view`.

```bash
git add stacklit.json DEPENDENCIES.md
git commit -m "add stacklit codebase index"
```

Then tell your tools about it, either automatically with `stacklit setup` or by hand:

**Claude Code**, add to `CLAUDE.md`:
```
Read stacklit.json before exploring files. Use modules to locate code, hints for conventions.
```

**Cursor and other MCP clients**, add to the MCP config:
```json
{
  "mcpServers": {
    "stacklit": { "command": "stacklit", "args": ["serve"] }
  }
}
```

**Copilot**, add to `.github/copilot-instructions.md`:
```
Read stacklit.json first to understand codebase structure before exploring files.
```

**Any other agent**: `stacklit.json` is plain JSON. Point the agent at it.

---

## Command reference

All commands run against the current directory. Run `stacklit <command> --help` for the built-in help.

| Command | Purpose |
|---------|---------|
| [`init`](#stacklit-init) | First-time scan, write all outputs, open the visual map |
| [`generate`](#stacklit-generate) | Regenerate the outputs from current source |
| [`view`](#stacklit-view) | Rebuild `stacklit.html` from `stacklit.json` and open it |
| [`diff`](#stacklit-diff) | Check whether the index is stale |
| [`derive`](#stacklit-derive) | Print the compact navigation map, or inject it into an agent config file |
| [`export`](#stacklit-export) | Render the index as a readable markdown overview |
| [`setup`](#stacklit-setup) | Configure detected AI tools and install the git hook |
| [`serve`](#stacklit-serve) | Run the MCP server over stdio |

Global flags: `--version`, `--help`.

### stacklit init

```
stacklit init [--hook] [--summary] [--multi FILE] [--workspace PATH]
```

Scans the current directory, writes `stacklit.json`, `DEPENDENCIES.md` and `stacklit.html`, then opens the HTML in your default browser.

| Flag | Description |
|------|-------------|
| `--hook` | Install a git `post-commit` hook that runs `stacklit generate --quiet` and stages `stacklit.json` and `DEPENDENCIES.md`. An existing hook is appended to, never replaced. (The flag's built-in help text says "pre-commit"; the hook that is installed is `post-commit`.) |
| `--summary` | Add an AI-written architecture narrative to the index. This is the only opt-in feature that sends data anywhere, see [Summary](#summary-opt-in). |
| `--multi FILE` | Polyrepo mode: scan every repo path listed in FILE. See [Polyrepo scanning](#polyrepo-scanning). |
| `--workspace PATH` | Accepted but currently has no effect on the scan. Monorepo workspaces are detected automatically. |

### stacklit generate

```
stacklit generate [--quiet]
```

Same scan and outputs as `init`, without opening a browser. Use this in scripts, hooks and CI.

| Flag | Description |
|------|-------------|
| `--quiet` | Suppress progress output |

### stacklit view

```
stacklit view
```

Reads `stacklit.json`, rewrites `stacklit.html` and opens it. It does not rescan the source, so run `generate` first if the code changed.

The HTML is one self-contained file with four views:

- **Graph**: force-directed dependency map. Click a node for exports, types and files.
- **Tree**: collapsible directory hierarchy with file and line counts.
- **Table**: sortable module table with search.
- **Flow**: top-down dependency flow from entrypoints to leaves.

A Share button downloads a standalone copy.

### stacklit diff

```
stacklit diff
```

Recomputes a content hash (Merkle hash) of the current source files and compares it with the `merkle_hash` stored in `stacklit.json`. It prints either "Index is up to date" or tells you to run `generate`. It exits with status 0 in both cases, so scripts should compare the output text or use the GitHub Action's check mode. Requires an index that has a `merkle_hash` field.

### stacklit derive

```
stacklit derive [--inject claude|cursor|aider]
```

Prints a compact navigation map built from `stacklit.json`: one header line, an entry and test line, one line per module with its purpose and key dependencies, hot files, the add-feature hint and do-not-touch paths. Size grows with the number of modules: 122 tokens for a 6-module Go repo, 1,122 for a 72-module TypeScript repo (see [bench/RESULTS.md](bench/RESULTS.md)).

| Flag | Description |
|------|-------------|
| `--inject TARGET` | Insert or replace the map, between marker comments, in `CLAUDE.md` (`claude`), `.cursorrules` (`cursor`) or `.aider.conf.yml` (`aider`). The file is created if missing. Re-running replaces the previous block. |

### stacklit export

```
stacklit export [--format md] [--source FILE] [-o FILE]
```

Renders the index as markdown for people: a PR description, an issue, a chat message.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `md` | Output format. Only markdown is supported. |
| `--source` | `stacklit.json` | Index file to read |
| `-o`, `--output` | stdout | Write to this file instead |

The markdown contains the project title and totals, a language and framework table, entrypoints and key directories, a modules table, collapsible per-module export lists, the dependency edges with most-depended and isolated modules, hot files when git data exists, and the hints. Output is deterministic: the same `stacklit.json` always gives byte-identical markdown.

### stacklit setup

```
stacklit setup [claude|cursor|aider]
```

Uses the existing `stacklit.json`, or generates one if none exists, then configures AI tools:

| Target | Detected by | What it changes |
|--------|-------------|-----------------|
| `claude` | `claude` on PATH or a `.claude` directory | injects the map into `CLAUDE.md`, adds the stacklit server to `.mcp.json` |
| `cursor` | a `.cursor` directory or `.cursorrules` | injects the map into `.cursorrules`, adds `.cursor/mcp.json` |
| `aider` | `aider` on PATH or `.aider.conf.yml` | injects the map into `.aider.conf.yml` |

With no argument it configures every detected tool and installs the post-commit hook. If no tool is detected it prints the map to stdout so you can paste it. `.mcp.json` is listed in this repository's `.gitignore` because it is user-specific; decide for your own repo whether to commit it.

### stacklit serve

```
stacklit serve
```

Starts the MCP server on stdio. Run it from the repo root: it reads `stacklit.json` from the working directory. See [MCP server](#mcp-server).

---

## MCP server

```bash
stacklit serve
```

Seven read-only tools:

| Tool | Input | Returns |
|------|-------|---------|
| `get_overview` | none | project name and type, languages, frameworks, entrypoints, file and line totals |
| `get_module` | `name` (exact module name) | one module: purpose, exports, types, dependencies, files, activity |
| `find_module` | `query` (substring) | up to 5 modules whose name or purpose contains the query, case-insensitive. It does not search exports or file names. |
| `list_modules` | none | every module with purpose, file and line counts, activity, sorted by name |
| `get_dependencies` | `module` | the dependency edges that touch that module |
| `get_hot_files` | none | the most-changed files in the last 90 days |
| `get_hints` | none | add-feature guidance, test command, env var names, do-not-touch paths |

The server re-reads `stacklit.json` whenever its modification time changes, so you do not need to restart it after `generate`.

---

## Configuration

Create `.stacklitrc.json` in the project root. The file is optional and every key is optional.

```json
{
  "ignore": ["vendor/", "generated/", "*.pb.go"],
  "max_depth": 3,
  "max_modules": 150,
  "max_exports": 15,
  "output": {
    "json": "stacklit.json",
    "mermaid": "DEPENDENCIES.md",
    "html": "stacklit.html"
  }
}
```

| Key | Default | Meaning |
|-----|---------|---------|
| `ignore` | none | Extra gitignore-style patterns to skip, on top of `.gitignore` and the built-in skips (`node_modules`, `vendor`, `dist`, `build`, `target`, `.venv`, `__pycache__` and similar) |
| `max_depth` | 4 | How deep module detection goes in the directory tree |
| `max_modules` | 200 | Above this many modules, deeper ones are collapsed into their parents |
| `max_exports` | 10 | Exports kept per module. Symbols beyond the cap are not in the index, so raise it if lookups by symbol name miss. |
| `output.json`, `output.mermaid`, `output.html` | `stacklit.json`, `DEPENDENCIES.md`, `stacklit.html` | Output file names. The outputs are excluded from the next scan automatically. |

A file that is missing, unreadable or invalid JSON is ignored and the defaults are used.

Only files with recognised source extensions are scanned. Test directories are filtered out of the module list, and each module lists up to about 20 file names.

---

## Environment variables

| Variable | Used by | Meaning |
|----------|---------|---------|
| `STACKLIT_SUMMARY_CMD` | `init --summary` | Command (split on whitespace) that reads a prompt on stdin and prints the summary. Default: `claude -p`. |
| `STACKLIT_SUMMARY_TIMEOUT` | `init --summary` | Timeout in seconds. Default: 120. |

### Summary (opt-in)

`stacklit init --summary` runs the command above with a prompt made of a short instruction plus a JSON snapshot of the index (project, tech, modules with exports and dependencies, entrypoints). No file contents are included. The output is stored under `architecture.ai_summary` in `stacklit.json`. If the command fails, a warning is printed and the index is written without a summary. Stacklit itself opens no network connection; whether the command you configure does is up to that command.

---

## Reading stacklit.json

### Project and tech

```json
{
  "project": { "name": "my-app", "type": "monorepo" },
  "tech": {
    "primary_language": "typescript",
    "languages": { "typescript": { "files": 120, "lines": 14000 } },
    "frameworks": ["React", "Express"]
  },
  "structure": {
    "entrypoints": ["src/index.ts"],
    "total_files": 120,
    "total_lines": 14000
  }
}
```

`tech.framework_patterns` adds detected routes, API, middleware and entry locations for known frameworks. `merkle_hash` is the content hash used by `stacklit diff` and the git hook to skip needless regeneration.

### Modules

Each module is a directory of related source files:

```json
"src/auth": {
  "purpose": "Authentication and authorization",
  "language": "typescript",
  "files": 8,
  "lines": 1200,
  "file_list": ["service.ts", "middleware.ts", "types.ts"],
  "exports": ["AuthProvider", "useSession()", "loginAction(email, password)"],
  "type_defs": { "AuthState": "user User, token string, isLoading boolean" },
  "depends_on": ["src/db", "src/config"],
  "depended_by": ["src/api", "src/middleware"],
  "activity": "high"
}
```

- **purpose**: derived from the directory name and a built-in vocabulary, so it is generic (for example "Json" for a `json` directory). It is not a semantic summary.
- **exports**: public functions, classes and types with signatures where the language provides them, capped by `max_exports`.
- **type_defs**: field definitions of key structs, interfaces and classes.
- **depends_on / depended_by**: module-level graph built from import statements. Entries are sometimes shortened to the last path segment.
- **activity**: `high`, `medium` or `low` from 90-day git commit frequency.

### Dependencies

```json
"dependencies": {
  "edges": [["src/api", "src/auth"], ["src/auth", "src/db"]],
  "most_depended": ["src/db", "src/config"],
  "isolated": ["scripts"]
}
```

`most_depended` ranks modules by number of dependents. `isolated` lists modules with no edges in or out.

### Git and hints

```json
"git": { "hot_files": [{ "path": "src/api/routes.ts", "commits_90d": 14 }] },
"hints": {
  "add_feature": "Add handler in src/api, register in src/index.ts",
  "test_command": "npm test",
  "env_vars": ["DATABASE_URL", "JWT_SECRET"],
  "do_not_touch": ["migrations/", ".github/"]
}
```

`env_vars` holds names found in `os.Getenv`, `process.env` and `os.environ` style accesses. Values are never read. Hints are heuristics: the test command comes from well-known build files and the add-feature hint from module names such as `api` or `handler`.

---

## Monorepo support

Workspaces are detected automatically and appear as module groups in the index.

| Tool | Detection |
|------|-----------|
| pnpm | `pnpm-workspace.yaml` |
| npm, yarn | `package.json` `workspaces` field |
| Go | `go.work` |
| Turborepo | `turbo.json` |
| Nx | `nx.json` |
| Lerna | `lerna.json` |
| Cargo | `Cargo.toml` `[workspace]` section |
| Convention | `apps/`, `packages/`, `services/`, `libs/` directories |

---

## Polyrepo scanning

Scan several separate repositories at once:

```bash
echo "/path/to/repo-a" > repos.txt
echo "/path/to/repo-b" >> repos.txt
stacklit init --multi repos.txt
```

Blank lines and `#` comments in the file are skipped. The result is `stacklit-multi.json` in the current directory, with module data for each repo and combined totals. Cross-repo dependency edges are not computed; that is on the [roadmap](ROADMAP.md).

---

## CI/CD

### GitHub Action

[glincker/stacklit-action](https://github.com/glincker/stacklit-action) keeps the index fresh:

```yaml
- uses: actions/checkout@v4
- uses: glincker/stacklit-action@v1            # auto-commit (default)
# or: with: { mode: check }                   # fail the PR if the index is stale
```

Give the job `permissions: contents: write` in auto-commit mode.

### Hand-written workflow

```yaml
name: Update stacklit index
on:
  push:
    branches: [main]
    paths-ignore: ['stacklit.json', 'DEPENDENCIES.md', '**.md']

jobs:
  stacklit:
    runs-on: ubuntu-latest
    permissions:
      contents: write
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

### Git hook

```bash
stacklit init --hook     # or: stacklit setup
```

Runs `stacklit generate --quiet` after each commit and stages the two committed outputs for the next commit.

---

## Troubleshooting

**`could not read stacklit.json`**: run `stacklit init` or `stacklit generate` first, and run the command from the repo root.

**A symbol is missing from the index**: exports are capped per module. Raise `max_exports` in `.stacklitrc.json` and regenerate. Unexported (private) symbols are never listed.

**A dependency looks wrong**: the graph comes from import statements resolved to modules. Test files and relative imports can produce extra or missing edges. Confirm with a read, and open an issue with a minimal example.

**Generation is slow or uses a lot of memory on Python or TypeScript**: this is a known cost of the pure-Go tree-sitter parsers. Add large generated or vendored directories to `ignore`.

**Windows install problems**: see the npm installer notes in [npm/README.md](npm/README.md), or download the binary from Releases.
