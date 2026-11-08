# AGENTS.md

Guide for agents and people working on the Stacklit repo itself. If you only want to use Stacklit on your own project, read README.md and docs/INSTALL.md.

## What this is

A Go CLI that indexes a codebase into `stacklit.json` (modules, exports, dependency edges, hints) plus a symbol graph, and serves it over MCP. Deterministic, no LLM, no network calls from the CLI. Module path `github.com/glincker/stacklit`, Go version from `go.mod`.

## Build and test

```bash
make build                 # ./stacklit
go vet ./...
go test ./... -race -count=1
go build -o /tmp/stacklit ./cmd/stacklit   # try it on another repo
```

CI runs vet and tests on Linux, macOS and Windows. Node is only needed for `npm/`.

## Where things live

| Path | Holds |
| --- | --- |
| `cmd/stacklit` | main package |
| `internal/cli` | Cobra commands |
| `internal/parser`, `internal/graph` | extractors and the dependency and symbol graph |
| `internal/engine`, `internal/renderer`, `internal/schema` | pipeline, outputs, index types |
| `internal/mcp` | MCP server (stdio) |
| `internal/setup` | `stacklit setup` targets, hooks, the embedded skill |
| `skills/stacklit/SKILL.md` | the skill; `internal/setup/skill/SKILL.md` and `plugins/stacklit/skills/stacklit/SKILL.md` must match it |
| `plugins/stacklit`, `.claude-plugin/` | Claude Code plugin and marketplace |
| `server.json`, `npm/` | MCP registry entry, npm wrapper |
| `scripts/` | install.sh, the llms-full.txt generator, sync helpers |
| `docs/ARCHITECTURE.md` | pipeline detail |

## Rules

- Commit subjects: `<type>: <description>` with type one of feat, fix, perf, refactor, docs, test, chore. Keep them short.
- No em dashes or en dashes in code, comments, docs or commit messages. Plain sentences, no marketing language.
- Add a failing test first for behavior changes. Parser tests sit next to the parsers and use `testdata/`.
- Claims about size or speed in docs must come from `bench/` output.
- Do not describe unmerged features as shipped.
- Do not commit generated files (`stacklit.json`, `docs/DEPENDENCIES.md`) unless the change is about them; CI refreshes them.
- Release files (`.goreleaser.yml`, `.github/workflows/release.yml`) and `npm/package.json` are owner-controlled. Ask before editing.
- Never commit secrets or `.env` files.

## Keeping copies in sync

- Skill copies: `scripts/sync-plugin-skill.sh --check` (run without `--check` to fix).
- `llms.txt` is hand written. `site/public/llms-full.txt` is generated: `sh scripts/gen-llms-full.sh`, and `--check` fails if it is stale.

## Using Stacklit on this repo

Read `stacklit.json` before broad searches. `stacklit query <name>` finds a symbol, `stacklit impact <name>` shows what depends on it.
