---
name: stacklit
description: Navigate this repo with its stacklit codebase map (stacklit.json) before grepping or globbing. Use at session start, when locating code, planning a change, or checking blast radius. Invoke as /stacklit.
---

# stacklit

`stacklit.json` at the repo root is a pre-computed map of modules, exports, dependency edges, hot files and hints. Reading it costs a few hundred tokens and replaces most exploratory grep and glob calls.

## Workflow

1. Check the map exists: `ls stacklit.json`. If missing, run `stacklit generate`.
2. Locate the area: find the module whose `purpose` matches the task. Read only its `file_list`.
3. Check blast radius before editing: `depended_by` of the module, and `dependencies.most_depended`.
4. Follow `hints.add_feature` for new code and `hints.test_command` to verify.
5. After structural changes (new, moved or deleted modules), run `stacklit generate`.

## MCP tools (when the stacklit server is connected)

- `get_overview`: project summary, call first
- `find_module("auth")`: search modules by name or purpose
- `get_module("src/auth")`: files, exports, deps for one module
- `get_dependencies("src/auth")`: imports and importers
- `list_modules`: every module, one line each
- `get_hot_files`: most changed files in 90 days
- `get_hints`: conventions, test command, env vars

## Index sections

- `modules`: purpose, files, exports, `depends_on`, `depended_by`, activity
- `dependencies.edges` and `most_depended`: widest blast radius
- `git.hot_files`: churn in the last 90 days
- `hints`: add-feature location, test command, env vars, do-not-touch

If `stacklit.context.json` exists (from `stacklit ingest`), it also lists ADRs, RFCs, OpenAPI operations, SQL tables and code owners. Read it when a task touches design decisions, an API contract or a schema.

## Rules

- Prefer the map over repo-wide grep. Grep only inside the module the map points to.
- Respect `do-not-touch` entries.
- Do not paste the whole index into the conversation. Quote the module you need.
- The map can lag the code. If a path does not exist, run `stacklit diff`, then `stacklit generate`.
