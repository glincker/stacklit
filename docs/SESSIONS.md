# Session lifecycle

Stacklit's job in a session is to give an agent a few hundred tokens of orientation at the start, keep that map true as files change, and let you check what it saved. This page is the design, what each hook does, what it costs, and what is and is not measured.

Tracking issue: #55.

## What the Claude Code docs say (read for this design)

Read from code.claude.com on the build date. Items marked unverified were not exercised in a live session.

- Hook events used: `SessionStart` (matcher values `startup`, `resume`, `clear`, `compact`, and `fork`), `SubagentStart` (matcher is the agent type), `PostToolUse` (matcher is the tool name).
- `additionalContext` returned in `hookSpecificOutput` is supported on `SessionStart`, `SubagentStart` and `PostToolUse`. Each injected field is capped at 10,000 characters; the excess is spilled to a file and only a 2,000 character preview is shown.
- Matchers made of letters, digits, `_`, `-`, `,` and `|` are an exact list, so `Edit|Write|MultiEdit|NotebookEdit` is an exact tool list.
- `"async": true` runs a command hook in the background without blocking the loop.
- `CLAUDE_PROJECT_DIR` stays fixed at the session start directory, including inside worktrees, so the hooks resolve the worktree from the payload `cwd` instead.
- Project-root `CLAUDE.md` and unscoped rules are re-read from disk at start and after compaction. Context that hooks added earlier is summarized with the conversation. `SessionStart` hooks that match `compact` run after compaction and their output is added to the compacted context.
- Subagents: Explore and Plan skip `CLAUDE.md`; general-purpose and custom agents load it. A `SubagentStart` hook can add context at the start of the subagent's conversation.
- Hook text should be factual statements. Imperative, system-style wording can trigger prompt-injection defenses, so hook payloads are phrased as facts. The rules in `CLAUDE.md` stay imperative, which is the normal place for instructions.
- `PreCompact` stdout becomes extra instructions for the compaction summary. Stacklit does not install it: after compaction the digest returns by the two routes above, so there is nothing to add.

Other tools carry a static digest through their rules files, which `stacklit setup` already writes: Cursor `.cursor/rules/*.mdc` with `alwaysApply: true`, Codex and others `AGENTS.md`, Gemini CLI `GEMINI.md`. Those conventions come from the existing setup code, not from a fresh fetch of each vendor's docs for this change. MCP resources could also expose the digest, but the stacklit server does not publish any today.

## Lifecycle table

Token figures are bytes/4 measured on the three repos in the table below (263, 1,008 and 3,679 files). Latency is wall clock of the whole process on an Apple laptop.

| Event | What Stacklit does | Tokens | Failure behavior |
|---|---|---|---|
| `SessionStart` (startup, resume, clear, fork) | Reads `stacklit.json`. If the project `CLAUDE.md` already holds a digest of the current structure, injects only one status line. Otherwise injects the digest plus a tools line, then the status line. Records the structure hash it handed out. | 23 (status only), 290 to 515 (full) | No index: silent, except one `stacklit init` hint per worktree on `startup`. Bad input or any error: no output, exit 0. |
| `SessionStart` (compact) | Same logic. `CLAUDE.md` comes back from disk by itself, so normally only the status line is added. If the structure moved since `CLAUDE.md` was written, the fresh digest is injected. | 23 to 515 | Same. |
| `SubagentStart` (Explore, Plan) | Injects the digest and tools line, because these agents skip `CLAUDE.md`. Other agent types get nothing (they already load `CLAUDE.md`). Override the list with `STACKLIT_SUBAGENT_TYPES`. | 264 to 491 | Silent on any error. |
| `PostToolUse` (Edit, Write, MultiEdit, NotebookEdit), async | Runs `stacklit hook posttool`, which calls `stacklit refresh` for the edited file. Never prints, never fails the edit. | 0 | Index missing: does nothing. Lock held: queues the path for the holder. |
| `PreToolUse` (Grep, Glob) | Existing opt-in pointer to a definition and callers. Unchanged. | up to 120 | Silent without a graph. |
| `UserPromptSubmit`, `Stop`, `PreCompact` | Not used. They would add tokens on every prompt or turn with no gain over the routes above. | 0 | n/a |

Measured latency:

| Repo size | `hook session` (full digest) | `hook session` (status only) | `refresh`, 1 file | `refresh`, 5 files | `refresh`, nothing changed |
|---|---|---|---|---|---|
| 263 files | about 80 ms | about 70 ms | 88 ms | 84 ms | 46 ms |
| 1,008 files | about 60 ms | about 60 ms | 167 ms | 155 ms | 75 ms |
| 3,679 files | 150 to 160 ms | 120 to 150 ms | 706 ms | 796 ms | 227 ms |

Refresh includes the default 30 ms debounce. It meets the 200 ms target up to roughly a thousand files and misses it on the 3,679 file repo, where loading and saving the per-file record and re-linking the graph dominate. Because the `PostToolUse` hook is async, this does not block the agent. The session hook stays under 150 ms in the first two sizes and sits at the limit on the largest because of the git staleness check.

## Where state lives

- `stacklit.json` and `stacklit.graph.json` stay in the worktree root, as before.
- Everything else is under the user cache dir, `stacklit/sessions/<hash of the absolute worktree root>/` (override with `STACKLIT_CACHE_DIR`): `refresh-state.json` (one record per file: content hash, parse result, symbols), `refresh.lock`, `pending`, `injected.json`.
- Each linked worktree has its own root, so its own index, lock and state. Two sessions in two worktrees never touch the same files. Two sessions in one worktree share them and are serialized by the lock.

Root resolution is the nearest ancestor holding `stacklit.json`, else the nearest holding `.git` (a directory, or the file a linked worktree has), else the current directory. A test checks that this agrees with `git rev-parse --show-toplevel` for a real linked worktree. A repo with no git works: the only things lost are branch-switch detection and the staleness line.

## Keeping the map fresh

`stacklit refresh --files a.go,b.go` (also `--stdin` for a hook payload):

1. Takes an advisory lock (`O_EXCL` file, stale after 30 s, override `STACKLIT_LOCK_STALE_SEC`). If another process holds it, the paths go to a `pending` queue and the call returns; the holder drains the queue after its pass, so a burst of edits from several sessions becomes one or two runs. A short debounce (`STACKLIT_REFRESH_DEBOUNCE_MS`, default 30) lets a burst gather.
2. Walks the file list (names and ignore rules only, nothing parsed). Files that vanished are dropped, files not in the record are added, so a rename or a file created outside the hook heals on the next call.
3. Hashes each named file and parses only those whose content hash differs from the record.
4. Rebuilds `stacklit.json` with the same function a full `generate` uses, and the symbol graph with the same linker, reusing stored symbols and body words for untouched files. A test asserts the output is byte-identical to a full generate on a fixture after edits, additions and deletions.
5. Writes both files atomically (temp file and rename), so a concurrent reader never sees half a file.

When the git HEAD differs from the one the record was built at (branch switch, rebase, pull), the record is rebuilt from every file through the parse cache. Files changed behind the hook's back with no HEAD change (a manual edit in an editor) are picked up by the next `generate`, the post-commit hook, or `refresh --files`.

Not refreshed: `stacklit.html` and the Mermaid file. They are regenerated by `generate` and the git hooks.

## Freshness and the prompt cache

The structure hash covers module names, module edges and exported symbol names, nothing else. Line counts, timestamps, purposes and churn are excluded. The digest is derived from structure only (modules are ranked by blast radius, then dependents, then name), so an ordinary edit leaves it byte-identical and the cached prompt prefix is not invalidated. It changes only when a module, an edge or an exported name does. Tests cover both directions.

`stacklit digest` prints the current hash. `stacklit digest --check [--session ID]` exits 0 when the structure is the same as what was injected, 1 when it changed, and 2 when nothing was recorded. The hash is also in the digest header (`map <hash>`), which is how the session hook knows a `CLAUDE.md` block is current.

Exports are capped per module (`max_exports`), so a change to a symbol beyond the cap does not move the hash.

## Steering text

The `CLAUDE.md` or `AGENTS.md` block opens with short rules. Three phrasings are available through `"steering": "a" | "b" | "c"` in `.stacklitrc.json` or `STACKLIT_STEERING`:

- `a` (default): three imperative lines, query before grep, slice instead of whole-file reads, impact before editing shared code.
- `b`: the same three rules on one line.
- `c`: a one-line rationale and three bullets.

Each is byte-stable, and the whole block stays under `STACKLIT_DIGEST_TOKENS` (default 500). The variants exist so they can be compared with the agent harness. There is no evidence yet that one wording beats another.

## Measuring

Two layers, kept apart on purpose.

1. Local usage log (off by default). `STACKLIT_STATS=1` or `"stats": true` appends one JSON line per `query`, `slice`, `explain` and `impact` call to `stats.jsonl` under the user cache dir. Fields: time, command, repo base name, `served` (tokens of the answer, bytes/4), `replaced_est` and file count. Nothing is sent anywhere. `stacklit stats [--json] [--since 7d|36h|2026-01-31]` reports totals.
   - Measured: how many tokens Stacklit answers put in front of the agent.
   - Estimated, not measured: `replaced_est` sums the sizes of the files the answer named, as if the agent would otherwise have read each one in full. Agents do not always read whole files, and may not have read those files at all. Treat it as an upper bound on one possible alternative, never as a saving.
   - Not measured: whether the agent would have asked the question anyway, what it actually did instead, hook injections (their size is in the table above), or quality of answers.
2. Real-agent A/B, `bench/agent-ab/`. Runs pinned tasks on a pinned repo with and without Stacklit through `claude -p --output-format json` and compares the usage the CLI reports. It is the only layer that measures an actual agent. See its README for the cost, which is real money. A single one-task smoke test (haiku, stacklit arm only) confirmed the harness runs end to end and reported $0.003, which says nothing about savings.

## Troubleshooting

- Hook does not fire. Run `/hooks` in Claude Code and check the entries for `stacklit hook ...`. Confirm `stacklit` is on the PATH that Claude Code sees (it runs hooks with your login environment, not necessarily your shell's). Test by hand: `echo '{"source":"startup","cwd":"'$PWD'"}' | stacklit hook session`. Hooks are read at session start, so restart the session after `stacklit setup --session-hook`.
- Nothing is injected at startup. By design, when `CLAUDE.md` already holds a digest of the current structure only the status line is added. Check with `stacklit digest` against the `map <hash>` in `CLAUDE.md`.
- The index looks stale. `stacklit status` shows files changed since the build. `stacklit refresh --files <paths>` fixes named files; `stacklit generate --graph` rebuilds everything. A stale state record after a branch switch is rebuilt on the next refresh.
- Concurrent sessions. They share a worktree's lock; the loser queues its paths and returns. If a crashed process left `refresh.lock` behind it is reclaimed after 30 s. Delete `stacklit/sessions/<hash>/` in the cache dir to reset all state; nothing in it is the source of truth.
- Edits by hand do not show up. The refresh hook only sees the agent's tool edits. Run `stacklit refresh --files ...`, or rely on the post-commit hook.
- `--no-mcp` used to skip the Claude hooks as well; hooks now install regardless.

## What was not verified

- That a live Claude Code session fires `SessionStart` on `compact` and adds the output as documented, that `SubagentStart` injection reaches an Explore agent, and that `async` PostToolUse hooks run on every edit. The payload shapes were taken from the docs and the handlers are tested against them, but no interactive session was driven.
- The exact `cwd` and field names in the `SubagentStart` payload are not in the docs excerpt that was read; the handler reads `agent_type` and `cwd` and stays silent if they are absent.
- Savings from any of this on real tasks. That needs the paid A/B run.
