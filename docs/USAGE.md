# Stacklit usage guide

This is the full reference for the `stacklit` command, the MCP server, the configuration file and the `stacklit.json` format. For a short introduction, read the [README](../README.md).

On this page: [Install](#install) | [First run](#first-run) | [Command reference](#command-reference) | [MCP server](#mcp-server) | [Configuration](#configuration) | [Environment variables](#environment-variables) | [Reading stacklit.json](#reading-stacklitjson) | [Monorepos](#monorepo-support) | [Polyrepo scanning](#polyrepo-scanning) | [CI/CD](#cicd) | [Troubleshooting](#troubleshooting)

---

## Install

Pick one:

```bash
npx stacklit init                                              # no install, downloads the binary on first use
npm install -g stacklit                                        # global install
go install github.com/glincker/stacklit/cmd/stacklit@latest    # from source, needs Go
```

You can also download a binary for macOS, Linux or Windows (amd64 and arm64) from [Releases](https://github.com/glincker/stacklit/releases).

Check the install:

```bash
stacklit --version
```

---

## First run

1. Go to your project.

   ```bash
   cd your-project
   ```

2. Build the index.

   ```bash
   stacklit init
   ```

   The output looks like this:

   ```
   [stacklit] found 342 files
   [stacklit] parsed 342 files (0 errors)
   [stacklit] done in 89ms -- wrote stacklit.json, DEPENDENCIES.md, stacklit.html

   Opening visual map...
   ```

   File counts and timings vary. Go repos index in milliseconds. Python and TypeScript repos can take seconds. See [bench/RESULTS.md](../bench/RESULTS.md) for numbers.

3. Commit the portable outputs. `stacklit.html` is gitignored and comes back with `stacklit view`.

   ```bash
   git add stacklit.json DEPENDENCIES.md
   git commit -m "add stacklit codebase index"
   ```

4. Tell your tools about the index. Run `stacklit setup` to do it for you, or add the lines by hand.

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

Every command runs against the current directory. Run `stacklit <command> --help` for the built-in help.

| Command | Purpose |
|---------|---------|
| [`init`](#stacklit-init) | First scan, write all outputs, open the visual map |
| [`generate`](#stacklit-generate) | Regenerate the outputs from the current source |
| [`view`](#stacklit-view) | Rebuild `stacklit.html` from `stacklit.json` and open it |
| [`diff`](#stacklit-diff) | Check whether the index is stale |
| [`derive`](#stacklit-derive) | Print the compact navigation map, or inject it into an agent config file |
| [`export`](#stacklit-export) | Render the index as markdown, or export the graph as GraphML, Cypher, JSON, DOT, Mermaid, an Obsidian vault or a markdown wiki |
| [`setup`](#stacklit-setup) | Configure detected AI tools and install the git hook |
| [`serve`](#stacklit-serve) | Run the MCP server over stdio |
| [`graph`, `query`, `explain`, `impact`, `path`, `hubs`, `communities`, `report`, `slice`, `watch`](#symbol-graph-commands) | Symbol-level graph and token-budgeted queries |
| [`doctor`](#stacklit-doctor) | Diagnose config, hooks, a stale index and parse errors |
| [`ingest`](#stacklit-ingest) | Add docs, OpenAPI, SQL schemas and ownership to `stacklit.context.json` |
| [`check`](#stacklit-check) | Enforce layer boundaries, cycles, fan-in and fan-out limits and orphans (exit code 3 on violation) |
| [`merge-graphs`](#stacklit-merge-graphs) | Merge symbol graphs from several repos and link them by package names |
| `completion` | Shell completions for bash, zsh, fish and PowerShell |

Global flags: `--version`, `--help`.

### stacklit init

```
stacklit init [--hook] [--summary] [--multi FILE] [--workspace PATH]
```

Scans the current directory, writes `stacklit.json`, `DEPENDENCIES.md` and `stacklit.html`, then opens the HTML in your default browser.

| Flag | Description |
|------|-------------|
| `--hook` | Installs a git `post-commit` hook that runs `stacklit generate --quiet` and stages `stacklit.json` and the configured mermaid file (`DEPENDENCIES.md` by default). An existing hook is appended to, never replaced. |
| `--summary` | Adds an AI-written architecture narrative to the index. See [Summary](#summary-opt-in). |
| `--multi FILE` | Polyrepo mode. Scans every repo path listed in FILE. See [Polyrepo scanning](#polyrepo-scanning). |
| `--workspace PATH` | Accepted, but it has no effect on the scan yet. Monorepo workspaces are detected automatically. |

> [!NOTE]
> The built-in help for `--hook` says "pre-commit". The hook that gets installed is `post-commit`.

> [!NOTE]
> `--summary` is the only opt-in feature that sends data anywhere. Stacklit itself opens no network connection. Whether the command you configure does is up to that command.

### stacklit generate

```
stacklit generate [--quiet]
```

Runs the same scan and writes the same outputs as `init`, without opening a browser. Use it in scripts, hooks and CI.

| Flag | Description |
|------|-------------|
| `--quiet` | Hides progress output |

### stacklit view

```
stacklit view
```

Reads `stacklit.json`, rewrites `stacklit.html` and opens it. It does not rescan your source. Run `generate` first if the code changed.

The HTML is one self-contained file with four views:

- **Graph**: a force-directed dependency map. Click a node to see its exports, types and files.
- **Tree**: a collapsible directory hierarchy with file and line counts.
- **Table**: a sortable module table with search.
- **Flow**: a top-down dependency flow, from entrypoints to leaves.

The Share button downloads a standalone copy.

### stacklit diff

```
stacklit diff
```

Hashes the current source files (a Merkle hash) and compares the result with `merkle_hash` in `stacklit.json`. It prints either "Index is up to date" or a note telling you to run `generate`.

The command exits with status 0 in both cases. A script has to check the output text, or use the GitHub Action's check mode. The command needs an index that has a `merkle_hash` field.

### stacklit derive

```
stacklit derive [--inject claude|cursor|aider]
```

Prints a compact navigation map built from `stacklit.json`. The map has:

- one header line
- an entry and test line
- one line per module, with its purpose and key dependencies
- hot files
- the add-feature hint
- do-not-touch paths

The size grows with the number of modules. A 6-module Go repo gives 122 tokens. A 72-module TypeScript repo gives 1,122. See [bench/RESULTS.md](../bench/RESULTS.md).

| Flag | Description |
|------|-------------|
| `--inject TARGET` | Inserts or replaces the map between marker comments in `CLAUDE.md` (`claude`), `.cursorrules` (`cursor`) or `.aider.conf.yml` (`aider`). The file is created if it is missing. Running it again replaces the previous block. |

### stacklit export

```
stacklit export [--format md] [--source FILE] [-o FILE]
```

Renders the index as markdown for people. Use it for a PR description, an issue or a chat message.

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `md` | `md`, or a graph format: `graphml`, `cypher`, `json`, `dot`, `mermaid`, `obsidian`, `markdown-wiki` |
| `--scope` | auto | Graph scope for single-file graph formats: `modules` or `symbols`. Auto picks symbols when `stacklit.graph.json` exists. |
| `--graph` | `stacklit.graph.json` | Symbol graph file for graph formats |
| `--budget` | 2000 | Token budget per `markdown-wiki` page. Env: `STACKLIT_EXPORT_PAGE_TOKENS`. |
| `--source` | `stacklit.json` | Index file to read |
| `-o`, `--output` | stdout | Write to this file (one-file formats) or directory (`obsidian`, `markdown-wiki`) instead |

The markdown includes:

- the project title and totals
- a language and framework table
- entrypoints and key directories
- a modules table
- collapsible export lists for each module
- dependency edges, with the most depended on and isolated modules
- hot files, when git data exists
- the hints

The output is deterministic. The same `stacklit.json` always gives byte-identical markdown.

#### Graph formats

Graph formats print to stdout unless you pass `-o`. They never write into the repo on their own. Every node carries `source_file`. Symbol edges carry their confidence (`EXTRACTED`, `INFERRED` or `EXACT`) and a score.

| Format | Output |
|--------|--------|
| `graphml` | One GraphML file (Gephi, yEd, NetworkX) |
| `cypher` | `CREATE` and `MATCH ... CREATE` statements for Neo4j and compatible stores |
| `json` | `{scope, nodes, edges}` |
| `dot` | Graphviz. Inferred edges are dashed. Capped to the highest-degree nodes (`STACKLIT_EXPORT_MAX_NODES`, default 300). |
| `mermaid` | Mermaid flowchart with the same cap |
| `obsidian` | A vault folder. It has `Index.md` plus one note per module, with `[[wikilinks]]` to dependencies. Symbol headings get their own links, as `[[module#Symbol]]`. |
| `markdown-wiki` | `index.md` plus `modules/<module>.md`. Each page is cut off at the token budget with a "... N more" line. The same input gives identical files. |

```
stacklit export --format graphml -o /tmp/graph.graphml
stacklit export --format obsidian -o ~/vault/myproject
stacklit export --format markdown-wiki --budget 1500 -o /tmp/wiki
```

### stacklit setup

```
stacklit setup [claude|cursor|aider]
```

Uses the existing `stacklit.json`, or generates one if none exists. Then it configures your AI tools.

| Target | Detected by | What it changes |
|--------|-------------|-----------------|
| `claude` | `claude` on PATH, or a `CLAUDE.md` file | Injects the map into `CLAUDE.md`, adds the stacklit server to `.mcp.json` |
| `cursor` | a `.cursor` directory, or `.cursorrules` | Injects the map into `.cursorrules`, adds `.cursor/mcp.json` |
| `aider` | `aider` on PATH, or `.aider.conf.yml` | Injects the map into `.aider.conf.yml` |

With no argument, `setup` configures every tool it detects and installs the post-commit hook. If no tool is detected, it prints the map to stdout so you can paste it yourself.

`.mcp.json` is in this repository's `.gitignore` because it is specific to one user. Decide for your own repo whether to commit it.

#### Claude Code crawl guard

> [!WARNING]
> Not recommended. In the first real-agent A/B, the strict arm (`--crawl-guard=redirect` with steering `d`) cost 1.12x and was right 82.7% of the time. Without Stacklit it was right 94.0% of the time. See [docs/SESSIONS.md](SESSIONS.md#what-the-first-real-agent-ab-measured).

The guard is opt in:

```
stacklit setup claude --crawl-guard[=nudge|redirect|off]
```

It installs a `PreToolUse` hook. The hook spots repo-wide crawls, such as `find .`, `ls -R`, `tree`, `grep -r` or `rg` on the root, `Glob **/*`, a Read of a directory, or a generic "explore the codebase" subagent prompt.

- A bare `--crawl-guard` means `nudge`. Pass a mode after `=` to choose another.
- `nudge` adds a pointer to the stacklit command.
- `redirect` denies the call with a reason that names the command. It does this at most twice in a row per session, and only while the index is current.
- `off` removes the hook. `--uninstall` does the same.

The command supports `--dry-run`. The details are in [docs/SESSIONS.md](SESSIONS.md#crawl-guard).

### stacklit guide

```
stacklit guide [--json]
```

Prints the agent usage guide (rules, command list and examples). You can paste it into any tool.

The rules use the routed wording (steering `e`):

- Use `query` when you do not know where something is.
- Use grep or `impact` when you know the exact name.
- Check the neighbours when the answer is several files.

`STACKLIT_STEERING=d` prints the older strict no-crawl wording. That wording measured worse.

### stacklit refresh, digest and stats

These are session plumbing commands. For the design, see [docs/SESSIONS.md](SESSIONS.md).

```
stacklit refresh --files a.go,b.go [--json]    # or: --stdin with a hook payload
stacklit digest [--check] [--session ID]
stacklit stats [--json] [--since 7d]
```

- `refresh` updates `stacklit.json` (and `stacklit.graph.json` when it exists) for the named files only. Files that have not changed are skipped by content hash. Concurrent callers coalesce behind a lock. Writes are atomic. The result matches a full `generate`.
- `digest` prints the structure hash. It covers module names, edges and exported names only. With `--check`, it exits 0 when the hash matches what the session was given, 1 when it changed, and 2 when nothing was recorded.
- `stats` reports the opt-in local usage log (`STACKLIT_STATS=1`). `served` is measured as bytes/4. `replaced_est` is an estimate of what reading the whole files would have cost. It is not a measured saving.

Claude Code hooks are opt in. They are written to `.claude/settings.json`, or to `~/.claude` with `--global`.

| Flag | Installs |
|---|---|
| `--session-hook` | `SessionStart` for startup, resume, clear and compact. `SubagentStart` for Explore and Plan. |
| `--refresh-hook` | An async `PostToolUse` hook on Edit, Write, MultiEdit and NotebookEdit |

### stacklit serve

```
stacklit serve
```

Starts the MCP server on stdio. Run it from the repo root, because it reads `stacklit.json` from the working directory. See [MCP server](#mcp-server).

### Symbol graph commands

```
stacklit graph                          # build stacklit.graph.json (query commands build it in memory if it is missing)
stacklit query "where is auth checked?" # scoped subgraph for a question, hard limit via --budget (default 2000 tokens)
stacklit explain <symbol>               # definition, location, callers, callees (--budget, default 1500)
stacklit impact <name|file|dir>         # importers of a file or package with file:line, or dependents of a symbol (--depth, --calls)
stacklit path <from> <to>               # shortest path between two symbols
stacklit hubs -n 15                     # most connected symbols
stacklit communities                    # deterministic file clusters with labels
stacklit report -o GRAPH_REPORT.md      # key concepts, surprising links, suggested questions
stacklit slice <symbol>                 # only that symbol's source, with line numbers
stacklit watch                          # keep the graph current as files change
```

**`query`** prints files in relevance order:

- Each file group is numbered (`1. path`). The number comes from the best score among its symbols, with ties broken by path.
- Symbols inside a file are sorted by score, then by line.
- A `best match: <file>:<line> <symbol>` line names the top result.
- The output is byte-stable between runs. When `--budget` cuts results, the lowest ranked ones go first.
- `--order relevance|path` picks the layout. `path` is the older alphabetical grouping, with no numbers. The default comes from `STACKLIT_QUERY_ORDER`, and otherwise it is `relevance`.
- With `--json`, each file has `rank` (1 is best) and `score` (0 to about 1.2, relative to the best seed). Each symbol has its own `score`. Files stay in the printed order.
- The MCP `query_graph` and `query` tools return the same text.

Two scoring knobs:

- `STACKLIT_AUX_PENALTY` (default 0.6) multiplies the score of files under test, example, benchmark or fixture directories. The penalty does not apply when the question names those directories. Set it to 1 to turn it off.
- `STACKLIT_IDENT_TOKEN_BOOST` (default 0.5) raises symbols whose names match a camelCase or snake_case word in the question exactly.

When the question suggests a flow, `query` adds one more line: "several files may take part; also ranked: ...". That line lists the next ranked files, with the `explain` and `path` commands to expand them. It appears in two cases:

- The question contains flow words, such as trace, flow, through, path, calls, wake or lifecycle. `STACKLIT_FLOW_WORDS` replaces that list.
- The top files sit in many different directories with similar scores. The test uses `STACKLIT_QUERY_SPREAD_DIRS` (default 4 of the top `STACKLIT_QUERY_SPREAD_TOP` 5 files, within `STACKLIT_QUERY_FLATNESS` 0.5 of the best score).

The line counts against `--budget`. Single-answer questions do not get it. `STACKLIT_QUERY_NEXT` sets how many files it lists (default 4, at most 5; 0 turns the line off). With `--json`, the files appear in `next`.

**`impact`** answers "who imports X" the way grep would, in fewer tokens. Give it a file path (`src/core.ts`, a unique suffix or a bare name) or a directory (a Go package, such as `internal/appsleep`). It lists the files that import it, one per line as `path:line`. Tests are excluded. The list stays inside `--budget`, with a `[truncated: ...]` line when it is cut.

- A directory means the package in that directory. A subpackage is separate.
- For script languages (TypeScript, JavaScript, Vue, Svelte, Astro), the sources are also read. That catches imports the extractor missed, so the list matches grep. `STACKLIT_IMPACT_SCAN=0` turns this off. `STACKLIT_IMPACT_SCAN_MAX` caps the files read (default 20000).
- An exact symbol name keeps the call-level view, by depth. `--calls` forces that view for files and directories.
- Measured on this repository and three others, the list for a package took about 40 to 160 tokens. `grep -rn` of the same import and the same number of lines took 110 to 390. Before this change, `impact` on a directory returned an error, and on a file it returned a 1,400 token call-level list.

All of these commands accept `--root`, `--graph`, and `--include-tests` (keeps test files in the graph; `STACKLIT_GRAPH_INCLUDE_TESTS=1` does the same). Except for `watch`, they also accept `--json`.

Twenty languages produce symbols and call, import and inheritance edges. The per-language table is in [docs/ARCHITECTURE.md](ARCHITECTURE.md#symbol-graph-extraction).

- `STACKLIT_GRAPH_MAX_PARSE_BYTES` (default 196608) sets the largest file parsed with tree-sitter.
- `STACKLIT_PARSE_MEMORY_MB` bounds the memory used by concurrent parses.
- `watch` uses fsnotify. Above `STACKLIT_WATCH_MAX` directories (default 4096), it falls back to polling. `STACKLIT_WATCH_POLL_MS` sets the polling interval.

### stacklit doctor

```
stacklit doctor [path] [--json] [--strict]
```

Checks the config file, the git hook, index freshness and parse errors. `--strict` also fails on warnings.

### stacklit ingest

```
stacklit ingest [--docs] [--api] [--schema] [--owners] [--git-owners]
```

Writes `stacklit.context.json` next to the index. It leaves `stacklit.json` alone. With no connector flag, every connector runs except `--git-owners`.

Limits: `STACKLIT_INGEST_MAX_BYTES` and `STACKLIT_INGEST_MAX_ITEMS`.

#### Adapters

```
stacklit ingest --adapter "<command>" [--adapter-name NAME] [--adapter-files GLOBS] [--into-graph]
```

An adapter is your own command. Stacklit passes it file paths on stdin, one per line, relative to the root. The command writes JSON Lines back:

- `{"type":"node","id","name","kind","summary","source_file"}`
- `{"type":"edge","from","to","kind"}`

Use adapters for concepts from PDFs, images, audio or anything else Stacklit does not parse. Results go into `stacklit.context.json` as `concepts` and `concept_links`. They are always tagged `INFERRED` and carry a `source` field. A rerun replaces that adapter's earlier output. `--into-graph` also adds them to `stacklit.graph.json` as `concept` nodes.

Stacklit makes no network or model calls itself.

| Variable | Limit |
|----------|-------|
| `STACKLIT_ADAPTER_TIMEOUT_SEC` | 60 seconds |
| `STACKLIT_ADAPTER_MAX_BYTES` | 4 MiB |
| `STACKLIT_ADAPTER_MAX_ITEMS` | 5000 |

The full contract and an example are in [examples/adapters](../examples/adapters/README.md).

#### SCIP

```
stacklit ingest --scip index.scip [--graph stacklit.graph.json]
```

Reads a SCIP index, from `scip-go`, `scip-typescript` or a similar tool, with a built-in decoder. It upgrades `INFERRED` graph edges to `EXACT` where the index shows a reference from the edge's source to a symbol defined at the edge's target. It never adds or removes edges, and it rewrites the graph file in place. The size cap is `STACKLIT_SCIP_MAX_BYTES` (512 MiB).

### stacklit check

```
stacklit check [--root DIR] [--source stacklit.json] [--json]
```

Checks the `check` block of `.stacklitrc.json` against the module graph. It exits with code 3 when a rule is broken and 0 otherwise, so you can use it as a CI step.

```json
{
  "check": {
    "forbidden": [{ "from": "ui/**", "to": "core/db", "reason": "go through the service layer" }],
    "cycles": true,
    "max_fan_in": 12,
    "max_fan_out": 8,
    "orphans": true
  }
}
```

Patterns are module paths with `*` wildcards. `dir/**` matches `dir` and everything below it. With no `check` block, the command passes. `--json` prints `{ok, violations:[{rule, message, modules}]}`.

### stacklit merge-graphs

```
stacklit merge-graphs a/stacklit.graph.json b/stacklit.graph.json -o merged.json [--multi stacklit-multi.json]
```

Combines symbol graphs from several repos. Node IDs get a `repo:` prefix. Each node keeps its `source_file` and gets a `repo` field. To name a repo yourself, use `name=path.json`.

With `--multi`, which takes the output of `generate --multi`, the command adds:

- repo nodes and `depends_on` edges, for repos that depend on each other through manifest package names
- import edges from the importing files to the provider repo

---

## MCP server

```bash
stacklit serve
```

The server exposes fifteen read-only tools. The first seven read `stacklit.json`. The last eight read the symbol graph (`stacklit.graph.json`, built in memory when it is missing).

| Tool | Input | Returns |
|------|-------|---------|
| `get_overview` | none | project name and type, languages, frameworks, entrypoints, file and line totals |
| `get_module` | `name` (exact module name) | one module: purpose, exports, types, dependencies, files, activity |
| `find_module` | `query` (substring) | up to 5 modules whose name or purpose contains the query, case-insensitive. It does not search exports or file names. |
| `list_modules` | none | every module with purpose, file and line counts, activity, sorted by name |
| `get_dependencies` | `module` | the dependency edges that touch that module |
| `get_hot_files` | none | the most changed files in the last 90 days |
| `get_hints` | none | add-feature guidance, test command, env var names, do-not-touch paths |
| `query_graph` | `query`, optional `budget` | scoped symbol subgraph grouped by file, hard token budget (default 2000) |
| `shortest_path` | `from`, `to` | shortest path between two symbols, with the weakest hop's confidence |
| `explain_symbol` | `symbol` | definition, file and line range, signature, callers and callees |
| `impact` | `symbol` (name, `Type.Method`, file path or directory), optional `depth`, `budget` | importers of a file or package with file:line, or the dependents of a symbol |
| `get_hubs` | optional `top` | most connected symbols |
| `get_communities` | none | deterministic file clusters with labels |
| `graph_report` | none | key concepts, surprising links, suggested questions |
| `slice_symbol` | `symbol` | only that symbol's source lines |

The server re-reads `stacklit.json` when its modification time changes. You do not need to restart it after `generate`.

---

## Configuration

Create `.stacklitrc.json` in the project root. The file is optional, and every key in it is optional.

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
| `ignore` | none | Extra gitignore-style patterns to skip. They apply on top of `.gitignore` and the built-in skips (`node_modules`, `vendor`, `dist`, `build`, `target`, `.venv`, `__pycache__` and similar). |
| `max_depth` | 4 | How deep module detection goes in the directory tree |
| `max_modules` | 200 | Above this many modules, deeper ones collapse into their parents |
| `max_exports` | 10 | Exports kept per module, taken from the files with the most exports first. Symbols past the cap are not in the index. Raise it if lookups by symbol name miss. Env: `STACKLIT_MAX_EXPORTS`. |
| `max_module_files` | 30 | File names listed per module, files with the most exports first. Env: `STACKLIT_MAX_MODULE_FILES`. |
| `module_split_files` | 40 | A module with more files than this is split by subdirectory, repeatedly, so one big directory does not hide its files. Env: `STACKLIT_MODULE_SPLIT_FILES` (`-1` turns splitting off). |
| `output.json`, `output.mermaid`, `output.html` | `stacklit.json`, `DEPENDENCIES.md`, `stacklit.html` | Output file names. The outputs are excluded from the next scan automatically. |
| `timestamps` | false | Writes `generated_at`. Off by default, so a committed index changes only when the code does. Env: `STACKLIT_TIMESTAMPS=1`. |
| `git_activity` | false | Adds git hot and recent files to the index. Off by default for the same reason. Turn it on with this key or with `STACKLIT_GIT=1`. |
| `steering` | `a` | Wording of the rules at the top of the generated `CLAUDE.md` or `AGENTS.md` block: `a`, `b`, `c`, `d` (strict, measured worse, not recommended) or `e` (routed, not yet measured). See [docs/SESSIONS.md](docs/SESSIONS.md). Env: `STACKLIT_STEERING`. |
| `quiet_repo_files` | 5000 | A repo with fewer indexed source files than this gets no hook text at all from the session and subagent hooks. 0 turns it off. Env: `STACKLIT_QUIET_REPO_FILES`. |
| `small_repo_files` | 600 | A repo with fewer indexed source files than this gets a one-line note instead of the digest, in the session hook and in `setup`. 0 turns it off. Env: `STACKLIT_SMALL_REPO_FILES`. |
| `crawl_guard` | (installed mode) | Crawl guard mode: `off`, `nudge` or `redirect`. Env: `STACKLIT_CRAWL_GUARD`. |
| `stats` | false | Appends a local usage log of `query`, `slice`, `explain` and `impact` calls. Env: `STACKLIT_STATS=1`. Never sent over the network. |
| `map_icons` | remote | `remote` lets the HTML map request language logos from thesvg.org when you open it. `off` removes every remote URL and shows letters instead. Env: `STACKLIT_MAP_ICONS=off`. |
| `map_layout` | flat | Module list layout in the map from `stacklit derive`. `flat` prints one line per module with its full path. `tree` prints one line per parent directory with the module names listed after it, which is shorter on repos with many sibling modules. Env: `STACKLIT_MAP_LAYOUT=tree`. |
| `cache` | true | On-disk parse cache. `STACKLIT_NO_CACHE=1` disables it. |
| `workers` | CPU count | Parser pool size. Env: `STACKLIT_WORKERS`. |
| `parse_memory_mb` | 256 | Soft cap on the estimated memory of concurrent tree-sitter parses. A single file above the cap still parses, alone. Env: `STACKLIT_PARSE_MEMORY_MB` (0 disables the cap). |
| `parse_timeout_ms` | 3000 | Wall-clock budget per file for tree-sitter. A file that goes over is indexed with its line count only, and listed in `scan.lightweight`. Env: `STACKLIT_PARSE_TIMEOUT_MS` (0 disables). |
| `ts_max_bytes` | 131072 | Files larger than this skip tree-sitter (reason `large`). Env: `STACKLIT_TS_MAX_BYTES` (0 disables). |
| `max_line_len` | 2000 | A line longer than this marks the file as minified and skips tree-sitter. Env: `STACKLIT_MAX_LINE_LEN` (0 disables). |
| `skip_generated` | true | Files named `.min.`, `.pb.` or `.gen.`, or marked "generated" or "do not edit" in the first KB, skip tree-sitter. Env: `STACKLIT_SKIP_GENERATED`. |
| `memory_limit_mb` | 448 | Soft Go heap limit during a scan, to keep RSS near the live set. An explicit `GOMEMLIMIT` wins. Env: `STACKLIT_MEMORY_LIMIT_MB` (0 disables). |
| `check` | none | Architecture rules for `stacklit check`: `forbidden`, `cycles`, `max_fan_in`, `max_fan_out`, `orphans` (see [stacklit check](#stacklit-check)) |

Diagnostics:

- `STACKLIT_TRACE=1` prints files whose parse took over 250 ms.
- `STACKLIT_PPROF=<dir>` writes `cpu.pprof`, `heap.pprof` and `heap-peak.pprof`.

A file that is missing, unreadable or invalid JSON is ignored, and the defaults are used.

Only files with recognised source extensions are scanned. Directories named `test`, `tests`, `testdata`, `fixtures`, `e2e`, `bench` or `benchmarks` are left out of the module list. Each module lists up to about 20 file names.

---

## Environment variables

| Variable | Used by | Meaning |
|----------|---------|---------|
| `STACKLIT_SUMMARY_CMD` | `init --summary` | Command (split on whitespace) that reads a prompt on stdin and prints the summary. Default: `claude -p`. |
| `STACKLIT_SUMMARY_TIMEOUT` | `init --summary` | Timeout in seconds. Default: 120. |
| `STACKLIT_EXPORT_PAGE_TOKENS` | `export --format markdown-wiki`, `obsidian` | Token budget per page. Default: 2000. |
| `STACKLIT_EXPORT_MAX_NODES` | `export --format dot`, `mermaid` | Node cap, highest degree kept. Default: 300. |
| `STACKLIT_ADAPTER_TIMEOUT_SEC`, `STACKLIT_ADAPTER_MAX_BYTES`, `STACKLIT_ADAPTER_MAX_ITEMS` | `ingest --adapter` | Adapter timeout, output size and item caps. Defaults: 60, 4194304, 5000. |
| `STACKLIT_DIGEST_TOKENS` | `setup`, hooks | Token cap of the injected digest. Default 500. |
| `STACKLIT_STEERING` | `setup`, `guide`, `serve` | Steering wording variant `a`, `b`, `c`, `d` or `e`. `d` also selects the older MCP and guide wording. |
| `STACKLIT_QUIET_REPO_FILES` | hooks | Source file count below which the session and subagent hooks stay silent. Default 5000, 0 off. |
| `STACKLIT_SMALL_REPO_FILES` | `setup`, hooks | Source file count below which a repo gets the small repo note instead of the digest. Default 600, 0 off. |
| `STACKLIT_QUERY_NEXT`, `STACKLIT_FLOW_WORDS`, `STACKLIT_QUERY_SPREAD_DIRS`, `STACKLIT_QUERY_SPREAD_TOP`, `STACKLIT_QUERY_FLATNESS` | `query` | The "also ranked" line: files listed (default 4, 0 off), flow word list, and the spread test (defaults 4, 5, 0.5). |
| `STACKLIT_IMPACT_SCAN`, `STACKLIT_IMPACT_SCAN_MAX` | `impact` | `0` stops `impact` from reading script sources to confirm importers. The cap sets how many files are read (default 20000). |
| `STACKLIT_CRAWL_GUARD` | `hook crawl` | `off`, `nudge` or `redirect`. Overrides the `crawl_guard` config key and the mode the hook was installed with. Off by default when nothing sets it. |
| `STACKLIT_CRAWL_REDIRECTS`, `STACKLIT_CRAWL_FRESH_TTL` | `hook crawl` | Consecutive redirects per session before the guard stands down (default 2). Seconds a staleness verdict is reused (default 10). |
| `STACKLIT_CRAWL_MAX_DEPTH`, `STACKLIT_CRAWL_PROMPT_CHARS` | `hook crawl` | Deepest `find -maxdepth` or `tree -L` still treated as narrow (default 1). Longest subagent prompt judged generic (default 600). |
| `STACKLIT_CRAWL_POINTER_MAX_BYTES`, `STACKLIT_CRAWL_STATE_DAYS` | `hook crawl` | Largest graph loaded to build an exact pointer (default 8 MiB). Days before session state files are pruned (default 7). |
| `STACKLIT_STATS` | `query`, `slice`, `explain`, `impact` | `1` logs usage locally. `0` forces logging off. |
| `STACKLIT_CACHE_DIR` | cache, hooks, stats | Where the parse cache, session state and usage log live. |
| `STACKLIT_REFRESH_DEBOUNCE_MS`, `STACKLIT_LOCK_STALE_SEC` | `refresh` | Debounce before a refresh (default 30). Age at which a lock counts as abandoned (default 30). |
| `STACKLIT_SUBAGENT_TYPES` | `hook subagent` | Comma-separated list of agent types that get the digest. Default `Explore,Plan`. |
| `STACKLIT_SCIP_MAX_BYTES` | `ingest --scip` | Largest SCIP file read. Default: 512 MiB. |

### Summary (opt-in)

`stacklit init --summary` runs the command above with a prompt. The prompt has a short instruction and a JSON snapshot of the index: project, tech, modules with exports and dependencies, and entrypoints. No file contents are included.

The output is stored under `architecture.ai_summary` in `stacklit.json`. If the command fails, Stacklit prints a warning and writes the index without a summary.

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

`tech.framework_patterns` adds detected routes, API, middleware and entry locations for known frameworks. `merkle_hash` is the content hash that `stacklit diff` and the git hook use to skip regeneration that is not needed.

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

- **purpose** comes from the directory name and a built-in vocabulary. It is generic. For example, a `json` directory gets "Json". It is not a semantic summary.
- **exports** are public functions, classes and types, with signatures where the language provides them. The count is capped by `max_exports`.
- **type_defs** are field definitions of key structs, interfaces and classes.
- **depends_on** and **depended_by** come from the module graph, which is built from import statements. Entries are sometimes shortened to the last path segment.
- **activity** is `high`, `medium` or `low`, based on git commit frequency over 90 days.

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

`env_vars` lists names found in `os.Getenv`, `process.env` and `os.environ` style accesses. Stacklit never reads the values. Hints are heuristics. The test command comes from well-known build files. The add-feature hint comes from module names such as `api` or `handler`.

---

## Monorepo support

Workspaces are detected automatically. They show up as module groups in the index.

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

Scan several separate repositories at once.

1. Write one repo path per line into a file.

   ```bash
   echo "/path/to/repo-a" > repos.txt
   echo "/path/to/repo-b" >> repos.txt
   ```

2. Run the scan with that file.

   ```bash
   stacklit init --multi repos.txt
   ```

Blank lines and `#` comments in the file are skipped. The result is `stacklit-multi.json` in the current directory. It holds module data for each repo and combined totals. Cross-repo dependency edges are not computed yet. That is on the [roadmap](ROADMAP.md).

---

## CI/CD

### GitHub Action

[glincker/stacklit-action](https://github.com/glincker/stacklit-action) keeps the index fresh:

```yaml
- uses: actions/checkout@v4
- uses: glincker/stacklit-action@v1            # auto-commit (default)
# or: with: { mode: check }                   # fail the PR if the index is stale
```

In auto-commit mode, give the job `permissions: contents: write`.

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

The hook runs `stacklit generate --quiet` after each commit. It stages the two committed outputs for the next commit.

---

## Troubleshooting

**The session hook does not fire, the index looks stale, or two sessions collide.** See the troubleshooting section of [docs/SESSIONS.md](docs/SESSIONS.md). Quick checks:

- `/hooks` in Claude Code
- `echo '{"source":"startup","cwd":"'$PWD'"}' | stacklit hook session`
- `stacklit status`
- `stacklit refresh --files <paths>`

**`could not read stacklit.json`.** Run `stacklit init` or `stacklit generate` first, and run the command from the repo root.

**A symbol is missing from the index.** Exports are capped per module. Raise `max_exports` in `.stacklitrc.json`, then regenerate. Unexported (private) symbols are never listed.

**A dependency looks wrong.** The graph comes from import statements resolved to modules. Test files and relative imports can add or drop edges. Confirm with a read, and open an issue with a minimal example.

**Generation is slow or uses a lot of memory on Python or TypeScript.** This is a known cost of the pure-Go tree-sitter parsers. Add large generated or vendored directories to `ignore`.

**Windows install problems.** Use `go install`, or download the binary from Releases and put it on your PATH.

---

Next: [How it works](ARCHITECTURE.md), or the [FAQ](FAQ.md) if something looks off.
