# Stacklit usage guide

## First time setup

### 1. Install

Pick one:

```bash
npx stacklit init              # easiest, downloads and runs automatically
npm install -g stacklit        # install globally for repeated use
go install github.com/glincker/stacklit/cmd/stacklit@latest  # if you have Go
```

Or download a binary from [Releases](https://github.com/glincker/stacklit/releases).

### 2. Generate your index

```bash
cd your-project
stacklit init
```

Output:

```
[stacklit] found 342 files
[stacklit] parsed 342 files (0 errors)
[stacklit] done in 89ms -- wrote stacklit.json, DEPENDENCIES.md, stacklit.html

Opening visual map...
```

### 3. Commit the index

```bash
git add stacklit.json DEPENDENCIES.md
git commit -m "add stacklit codebase index"
git push
```

`stacklit.html` is gitignored. It regenerates locally with `stacklit view`.

### 4. Tell your AI tool about it

**Claude Code** -- add to `CLAUDE.md`:
```
Read stacklit.json before exploring files. Use modules to locate code, hints for conventions.
```

**Claude Desktop / Cursor** -- add to MCP config:
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

**Copilot** -- add to `.github/copilot-instructions.md`:
```
Read stacklit.json first to understand codebase structure before exploring files.
```

**Any other agent** -- `stacklit.json` is a plain JSON file. Point the agent at it.

---

## Daily use

### Regenerate after making changes

```bash
stacklit generate
```

Or automate it with a git hook:

```bash
stacklit init --hook
```

This adds a post-commit hook. Every commit auto-regenerates the index if source files changed. Docs-only changes are skipped (Merkle hash detection).

### Check if the index is stale

```bash
stacklit diff
```

Prints "Index is up to date" or tells you what changed.

### Open the visual map

```bash
stacklit view
```

Regenerates `stacklit.html` and opens it in your browser.

---

## Command reference

| Command | What it does |
|---------|-------------|
| `stacklit init` | Full scan, generate all outputs, open HTML |
| `stacklit init --hook` | Same as init + install git post-commit hook |
| `stacklit init --multi repos.txt` | Scan multiple repos listed in a file |
| `stacklit generate` | Regenerate index from current source |
| `stacklit generate --quiet` | Silent regeneration (for scripts, CI, hooks) |
| `stacklit view` | Regenerate HTML and open in browser |
| `stacklit diff` | Check if index is stale |
| `stacklit serve` | Start MCP server for AI agent integration |
| `stacklit derive` | Print compact navigation map (~250 tokens) to stdout |
| `stacklit derive --inject claude` | Inject map into CLAUDE.md |
| `stacklit derive --inject cursor` | Inject map into .cursorrules |
| `stacklit export` | Print a readable markdown overview to stdout |
| `stacklit export -o stacklit.md` | Write the markdown overview to a file |
| `stacklit setup` | Auto-detect and configure all AI tools |
| `stacklit setup claude` | Configure Claude Code (CLAUDE.md + MCP) |
| `stacklit setup cursor` | Configure Cursor (.cursorrules + MCP) |
| `stacklit setup aider` | Configure Aider (.aider.conf.yml) |
| `stacklit --version` | Print version |

---

## MCP server

Start it:

```bash
stacklit serve
```

Seven tools available to your AI agent:

| Tool | What it returns |
|------|----------------|
| `get_overview` | Full project summary: modules, language, frameworks, entrypoints |
| `get_module` | One module: exports, types, dependencies, files, activity |
| `find_module` | Search modules by keyword |
| `list_modules` | All modules with name, purpose, files, lines |
| `get_dependencies` | What a module depends on and what depends on it |
| `get_hot_files` | Most-changed files in the last 90 days |
| `get_hints` | Where to add features, test commands, env vars |

The server auto-reloads when `stacklit.json` changes on disk.

---

## Exporting to markdown

`stacklit.json` is built for AI agents and tooling. When you want something a person can skim — a PR description, a GitHub issue, a Slack message — use the markdown export:

```bash
stacklit export                # print to stdout
stacklit export -o stacklit.md # write to a file
```

The markdown contains:

- A title with the project name, language, file count, and total lines
- A language and framework summary table
- Entrypoints and key directories (when present)
- A modules table with purpose, size, and direct dependencies
- Per-module collapsible `<details>` blocks listing exports and types
- A dependency edge list, plus most-depended-on and isolated modules
- Hot files from the last 90 days (when git data is present)
- Hints: how to add a feature, the test command, env vars

Output is deterministic: the same `stacklit.json` produces byte-identical markdown across runs and machines, so it's safe to commit.

---

## Configuration

Create `.stacklitrc.json` in your project root (optional):

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

Keys: `ignore` (extra paths on top of `.gitignore`), `max_depth` (module detection depth, default 4), `max_modules` (collapse threshold, default 200), `max_exports` (per module, default 10), and `output` (override generated file names).

---

## Reading stacklit.json

### Project info

```json
{
  "project": { "name": "my-app", "type": "monorepo" },
  "tech": {
    "primary_language": "typescript",
    "frameworks": ["React", "Express"]
  }
}
```

### Modules

Each module represents a directory of related source files:

```json
"src/auth": {
  "purpose": "Authentication and session management",
  "language": "typescript",
  "files": 8,
  "lines": 1200,
  "file_list": ["service.ts", "middleware.ts", "types.ts"],
  "exports": ["AuthProvider", "useSession()", "loginAction(email, password)"],
  "type_defs": {
    "AuthState": "user User, token string, isLoading boolean"
  },
  "depends_on": ["src/db", "src/config"],
  "depended_by": ["src/api", "src/middleware"],
  "activity": "high"
}
```

**purpose** -- What the module does (auto-generated from directory name and contents).

**exports** -- Public functions, classes, and types with signatures. For Go, includes full parameter and return types.

**type_defs** -- Struct/interface/class field definitions. Lets agents understand data shapes without reading source.

**depends_on / depended_by** -- Module-level dependency graph. Tells you what breaks if you change this module.

**activity** -- high/medium/low based on 90-day git commit frequency.

### Dependencies

```json
"dependencies": {
  "edges": [["src/api", "src/auth"], ["src/auth", "src/db"]],
  "most_depended": ["src/db", "src/config"],
  "isolated": ["scripts"]
}
```

**most_depended** -- Modules with the most dependents. Change these carefully.

**isolated** -- Modules with no incoming or outgoing dependencies.

### Hints

```json
"hints": {
  "add_feature": "Create handler in src/api/, add route in src/routes/index.ts",
  "test_command": "npm test",
  "env_vars": ["DATABASE_URL", "JWT_SECRET"]
}
```

Agent-actionable instructions generated from codebase analysis.

---

## Monorepo support

Stacklit auto-detects workspaces:

| Tool | Detection |
|------|-----------|
| pnpm | `pnpm-workspace.yaml` |
| npm/yarn | `package.json` workspaces field |
| Go | `go.work` |
| Turborepo | `turbo.json` |
| Nx | `nx.json` |
| Lerna | `lerna.json` |
| Cargo | `Cargo.toml` workspace section |
| Convention | `apps/`, `packages/`, `services/`, `libs/` directories |

Each workspace appears as a group of modules in the index.

---

## Polyrepo scanning

Scan multiple repos at once:

```bash
echo "/path/to/repo-a" > repos.txt
echo "/path/to/repo-b" >> repos.txt
stacklit init --multi repos.txt
```

Generates `stacklit-multi.json` with full module data for each repo plus cross-repo totals.

---

## CI/CD

### GitHub Action

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

### Pre-commit hook

```bash
stacklit init --hook
```

Regenerates on every commit. Skips if only docs/configs changed (Merkle hash detection).
