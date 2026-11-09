# Session lifecycle

Stacklit gives an agent a few hundred tokens of orientation at the start of a session. It keeps that map true as files change, and it lets you check what it saved.

This page covers the design: what each hook does, what it costs, and what has and has not been measured.

Tracking issue: #55.

> [!NOTE]
> The token figures here measure Stacklit's own answers. They do not show a saving on real tasks. The one real-agent A/B found no overall cost saving (cost ratio 1.00). See [What the first real-agent A/B measured](#what-the-first-real-agent-ab-measured).

## What the Claude Code docs say

These notes come from code.claude.com, read on the build date. Items marked unverified were not tested in a live session.

- Hook events used:
  - `SessionStart`, with matcher values `startup`, `resume`, `clear`, `compact` and `fork`
  - `SubagentStart`, where the matcher is the agent type
  - `PostToolUse`, where the matcher is the tool name
- `additionalContext` in `hookSpecificOutput` works on `SessionStart`, `SubagentStart` and `PostToolUse`. Each injected field is capped at 10,000 characters. Anything beyond that is spilled to a file, and only a 2,000 character preview is shown.
- A matcher made of letters, digits, `_`, `-`, `,` and `|` is an exact list. So `Edit|Write|MultiEdit|NotebookEdit` is an exact tool list.
- `"async": true` runs a command hook in the background, without blocking the loop.
- `CLAUDE_PROJECT_DIR` stays fixed at the session start directory, including inside worktrees. So the hooks find the worktree from the payload `cwd` instead.
- The project-root `CLAUDE.md` and unscoped rules are re-read from disk at start and after compaction. Context that hooks added earlier is summarized along with the conversation. `SessionStart` hooks that match `compact` run after compaction, and their output joins the compacted context.
- Explore and Plan subagents skip `CLAUDE.md`. General-purpose and custom agents load it. A `SubagentStart` hook can add context at the start of the subagent's conversation.
- Hook text should be factual statements. Imperative, system-style wording can set off prompt-injection defenses, so hook payloads are phrased as facts. The rules in `CLAUDE.md` stay imperative, which is the normal place for instructions.
- `PreCompact` stdout becomes extra instructions for the compaction summary. Stacklit does not install it. After compaction, the digest comes back through the two routes above, so nothing needs adding.

Other tools carry a static digest through their rules files. `stacklit setup` already writes those files:

- Cursor: `.cursor/rules/*.mdc` with `alwaysApply: true`
- Codex and others: `AGENTS.md`
- Gemini CLI: `GEMINI.md`

Those conventions come from the existing setup code. They were not checked against a fresh read of each vendor's docs for this change. MCP resources could also expose the digest, but the stacklit server does not publish any today.

## Lifecycle table

Token figures are bytes/4, measured on the three repos in the latency table below (263, 1,008 and 3,679 files). Latency is the wall-clock time of the whole process on an Apple laptop.

| Event | What Stacklit does | Tokens | Failure behavior |
|---|---|---|---|
| `SessionStart` (startup, resume, clear, fork) | Reads `stacklit.json`. A small repo (below `small_repo_files`, see [Small repos](#small-repos)) gets one line saying grep is enough, then the status line. Otherwise, if the project `CLAUDE.md` already holds a digest of the current structure, it injects only the status line. If not, it injects the digest and a tools line, then the status line. Records the structure hash it handed out. | 23 (status only), about 60 (small repo), 290 to 515 (full) | No index: silent, except for one `stacklit init` hint per worktree on `startup`. Bad input or any error: no output, exit 0. |
| `SessionStart` (compact) | Same logic. `CLAUDE.md` comes back from disk by itself, so normally only the status line is added. If the structure moved since `CLAUDE.md` was written, the fresh digest is injected. | 23 to 515 | Same. |
| `SubagentStart` (Explore, Plan) | Injects the digest and tools line, because these agents skip `CLAUDE.md`. Other agent types get nothing, since they already load `CLAUDE.md`. Override the list with `STACKLIT_SUBAGENT_TYPES`. | 264 to 491 | Silent on any error. |
| `PostToolUse` (Edit, Write, MultiEdit, NotebookEdit), async | Runs `stacklit hook posttool`, which calls `stacklit refresh` for the edited file. Never prints, never fails the edit. | 0 | Index missing: does nothing. Lock held: queues the path for the holder. |
| `PreToolUse` (Grep, Glob) | The existing opt-in pointer to a definition and its callers. Unchanged. | up to 120 | Silent without a graph. |
| `UserPromptSubmit`, `Stop`, `PreCompact` | Not used. They would add tokens on every prompt or turn, with no gain over the routes above. | 0 | n/a |

Measured latency:

| Repo size | `hook session` (full digest) | `hook session` (status only) | `refresh`, 1 file | `refresh`, 5 files | `refresh`, nothing changed |
|---|---|---|---|---|---|
| 263 files | about 80 ms | about 70 ms | 88 ms | 84 ms | 46 ms |
| 1,008 files | about 60 ms | about 60 ms | 167 ms | 155 ms | 75 ms |
| 3,679 files | 150 to 160 ms | 120 to 150 ms | 706 ms | 796 ms | 227 ms |

- The `refresh` numbers include the default 30 ms debounce.
- `refresh` meets the 200 ms target up to roughly a thousand files. It misses the target on the 3,679 file repo, where loading and saving the per-file record and re-linking the graph take most of the time.
- The `PostToolUse` hook is async, so this does not block the agent.
- The session hook stays under 150 ms on the first two sizes. On the largest, it sits at the limit because of the git staleness check.

## Where state lives

- `stacklit.json` and `stacklit.graph.json` stay in the worktree root, as before.
- Everything else lives under the user cache dir, in `stacklit/sessions/<hash of the absolute worktree root>/`. Override the location with `STACKLIT_CACHE_DIR`. The directory holds:
  - `refresh-state.json`: one record per file (content hash, parse result, symbols)
  - `refresh.lock`
  - `pending`
  - `injected.json`
- Each linked worktree has its own root, so its own index, lock and state. Two sessions in two worktrees never touch the same files. Two sessions in one worktree share state, and the lock serializes them.

Root resolution works like this. Stacklit uses the nearest ancestor that holds `stacklit.json`. If there is none, it uses the nearest one that holds `.git`, which can be a directory or the file a linked worktree has. Otherwise it uses the current directory. A test checks that this agrees with `git rev-parse --show-toplevel` for a real linked worktree.

A repo with no git still works. The only things lost are branch-switch detection and the staleness line.

## Keeping the map fresh

`stacklit refresh --files a.go,b.go` (or `--stdin` for a hook payload) does this:

1. It takes an advisory lock. The lock is an `O_EXCL` file, and it goes stale after 30 s. Override that with `STACKLIT_LOCK_STALE_SEC`. If another process holds the lock, the paths go into a `pending` queue and the call returns. The holder drains the queue after its pass, so a burst of edits from several sessions becomes one or two runs. A short debounce (`STACKLIT_REFRESH_DEBOUNCE_MS`, default 30) lets a burst gather.
2. It walks the file list, using names and ignore rules only. Nothing is parsed yet. Files that vanished are dropped. Files missing from the record are added. So a rename, or a file created outside the hook, heals on the next call.
3. It hashes each named file and parses only the files whose hash differs from the record.
4. It rebuilds `stacklit.json` with the same function a full `generate` uses. It rebuilds the symbol graph with the same linker, and reuses stored symbols and body words for untouched files. A test checks that the output is byte-identical to a full generate on a fixture, after edits, additions and deletions.
5. It writes both files atomically (a temp file, then a rename), so a concurrent reader never sees half a file.

When the git HEAD differs from the one the record was built at (after a branch switch, rebase or pull), the record is rebuilt from every file through the parse cache.

Files changed behind the hook's back, with no HEAD change (a manual edit in an editor), are picked up by the next `generate`, the post-commit hook or `refresh --files`.

`stacklit.html` and the Mermaid file are not refreshed. `generate` and the git hooks regenerate them.

## Freshness and the prompt cache

The structure hash covers module names, module edges and exported symbol names, and nothing else. Line counts, timestamps, purposes and churn are excluded.

The digest comes from structure only. Modules are ranked by blast radius, then by dependents, then by name. So an ordinary edit leaves the digest byte-identical, and the cached prompt prefix stays valid. The digest changes only when a module, an edge or an exported name does. Tests cover both directions.

- `stacklit digest` prints the current hash.
- `stacklit digest --check [--session ID]` exits 0 when the structure matches what was injected, 1 when it changed, and 2 when nothing was recorded.
- The hash also appears in the digest header (`map <hash>`). The session hook reads it to know whether a `CLAUDE.md` block is current.

Exports are capped per module (`max_exports`). A change to a symbol past the cap does not move the hash.

## Steering text

The `CLAUDE.md` or `AGENTS.md` block opens with a few rules. Pick one of five phrasings with `"steering": "a" | "b" | "c" | "d" | "e"` in `.stacklitrc.json`, or with `STACKLIT_STEERING`.

- **`a`** (default): three imperative lines. Query before grep, slice instead of reading whole files, and check impact before editing shared code.
- **`b`**: the same three rules on one line.
- **`c`**: a one-line rationale and three bullets.
- **`d`**: strict. Not recommended, because it measured worse (see below). It calls Stacklit the source of truth for structure and tells the agent not to crawl. Structure questions go to `stacklit query ... --budget 800`, then `explain`, `impact` or `path`. No recursive listings, no `find`, no `tree`, and no repo-wide grep to learn the structure. Open a file only after the map points to it, and read it with `stacklit slice`. Refresh or regenerate when Stacklit reports a stale index or a missing symbol. Subagents follow the same rules.
- **`e`**: routed. Use Stacklit when you do not know where something is or how things connect. When you know the exact name, use grep or `stacklit impact <name>`. When the answer is several files (a flow or a chain), check the neighbours with `stacklit explain` or `stacklit path` before you answer. One query is a lead, not the answer. The wording does not forbid grep. It was written after the first A/B, and it has not been measured yet. The default stays `a` until a second round says otherwise.

Each variant is byte-stable. The whole block stays under `STACKLIT_DIGEST_TOKENS` (default 500).

`stacklit guide` and the MCP handshake use the `e` wording. Set `STACKLIT_STEERING=d` to select the older strict wording for both. It is kept for comparison.

## What the first real-agent A/B measured

Pull request #77 (`bench/agent-ab/`) ran 450 Claude Code sessions. The setup:

- model: claude-sonnet-5-5
- three repos of about 16,000, 1,300 and 400 files
- 30 tasks, 5 trials each
- notional cost of about 0.03 USD per session

With steering `a` and the session hook:

- **Overall cost**: pooled cost ratio 1.00 [0.93, 1.07]. No overall saving. Tool calls were 0.68x. Grep and Glob calls were 0.18x.
- **Locate and add questions** got cheaper, with cost ratios of 0.88 and 0.78.
- **Repo size**: the 16,000 file repo got cheaper (0.78). The 1,300 and 400 file repos did not (1.10 and 1.23).
- **"Who imports X"** questions cost 1.59x. One exact grep answers them in about 1.2 calls. The Stacklit arm ran `stacklit query` first and grepped anyway.
- **Trace questions**, where the answer is a set of files, were less often right: 97.5% down to 82.5%. In the clearest case, the agent ran one `stacklit query` and answered with the top file. It missed a second file that ranked lower.
- **Correctness overall**: 94.0% down to 88.0% with the pre-registered grader. After three documented grading fixes, 98.0% down to 95.3%.

Steering `d` with `--crawl-guard=redirect` did worse. Its cost ratio was 1.12 [1.02, 1.24], its tokens were 1.37x, and it was right 82.7% of the time (89.3% after the audit). That arm was not interleaved with the control arm. The guard itself fired in only 8 of 150 sessions, so the result is mostly about the strict wording. Both are off by default and not recommended.

The changes in steering `e`, the flow line in `query`, importer lists in `impact`, and small repo mode follow from these results. They are not measured yet. A second round is pre-registered in `bench/agent-ab/`, but it has not run.

## When Stacklit helps

This comes from one A/B, with one model and one task set written by us.

- **Helps**: large repos. The 16,000 file repo cost 22% less per session. It also helps on questions where you do not know where something lives, such as locating code or adding a feature that follows a pattern. The agent used about 0.4x the tool calls.
- **Does not help**: exact-name lookups such as "who imports X", where one grep is already enough. It also does not help small repos (a few hundred files), where the map is overhead. On the 400 file repo, cost went up 23%.
- **Needs care**: questions whose answer is several files. A single query can list one plausible file and look final.

## Small repos

A repo with fewer source files than `small_repo_files` does not need a map. The default is 600. Set it in `.stacklitrc.json`, or with `STACKLIT_SMALL_REPO_FILES`. Set it to 0 to turn the mode off.

For such a repo, the session hook and `setup` write one factual line instead of the digest:

> This repo is small (N source files), so grep and reading files are enough. Stacklit is available if a question needs more

That line is about 60 tokens, compared with about 490 for the digest. The status line follows it. Subagents get the same line.

The count is the number of indexed files across all languages in `stacklit.json`. The line is not a prohibition. `query`, `impact` and the other commands still work.

## Source of truth

Three layers push an agent to answer structure questions from the index, instead of crawling the repo. The first A/B suggests that pushing hard does not pay off (see above). So the shipped wording is the routed one.

1. **MCP handshake.** `stacklit serve` returns an `instructions` string in the initialize result. It is about 100 tokens in both profiles, and it is byte-stable. It says to use `query` when you do not know where something is (and `explain` or `path` to see how things connect). It says to use grep or `impact` when you know the exact name. It says to check the neighbours when the answer is several files, because one query is a lead, not the answer. It also says to read code with `slice`, and to refresh when the index is stale. `STACKLIT_STEERING=d` returns the earlier wording instead. That wording called the index the source of truth and told the agent not to crawl.
2. **Rules block.** Steering variant `e`, set with `steering` or `STACKLIT_STEERING=e`. The default stays `a`.
3. **Crawl guard.** Opt in, and not recommended. See below.

## Crawl guard

> [!WARNING]
> Not recommended. In the real-agent A/B (pull request #77), the strict arm paired `--crawl-guard=redirect` with steering `d`. It cost more than no Stacklit at all: cost ratio 1.12, total tokens 1.37x. It was also right less often, 82.7% against 94.0%. The guard fired in only 8 of 150 sessions. The loss came mostly from the stronger rules text, which sent exact-name questions through `stacklit` first. The guard stays off by default.

`stacklit setup claude --crawl-guard[=nudge|redirect|off]` adds a `PreToolUse` hook (`stacklit hook crawl <mode>`). It matches `Bash|Glob|Read|Agent|Task`.

Mode precedence, from highest to lowest:

1. `STACKLIT_CRAWL_GUARD`
2. `crawl_guard` in `.stacklitrc.json`
3. The mode the hook was installed with

With nothing set, the guard is off.

### What counts as a crawl

- **Bash**: `find` from the root (not `-maxdepth 1`, and not `-delete`), `ls -R`, `tree` (not `-L 1`), `grep -r`, `rg` or `git grep` with no path or only the root, `dir /s`, `Get-ChildItem -Recurse`, and `findstr /s`.
  - Chains are split on `&&`, `;` and `|`.
  - A `cd` into a subdirectory ends the check.
  - `bash -c` and `powershell -Command` bodies are checked.
  - A grep that reads from a pipe is not a crawl.
- **Glob**: `**`, `**/*`, `**/*.ext` or `**/*.{a,b}` with no subdirectory. A named file (`**/main.go`) or a subdirectory is narrow.
- **Read**: the path is a directory.
- **Agent or Task**: a short prompt that asks to explore, survey or map the whole codebase or repo, and names no identifier, path or file.

### What each mode does

- **`nudge`** adds `additionalContext` that names a command. For a grepped identifier, the command is, for example, `stacklit explain parseToken`. It uses the same pointer as the Grep hook when the graph is small enough. Otherwise it suggests `stacklit query "<words>" --budget 800`. It needs an index and a graph, and it is silent without them.
- **`redirect`** returns `permissionDecision: "deny"` with a short reason that names the command. It acts only on a recognised crawl, and only when the index exists and matches the working tree (the same check as `stacklit status`). It does this at most `STACKLIT_CRAWL_REDIRECTS` times in a row per session (default 2). After that, the guard stands down for the rest of the session and only nudges. A non-crawl call resets the run. A stale index or a missing session id falls back to a nudge.
- **Everything else** is allowed with no output. That covers narrow reads and greps, Edit, Write, tests, builds, and any payload that fails to parse. Panics are recovered and treated as allow.

Session state is one small file per session id, under the worktree state dir in the user cache dir (`crawl-<session>.json`). Files are pruned after `STACKLIT_CRAWL_STATE_DAYS`. The staleness verdict is reused for `STACKLIT_CRAWL_FRESH_TTL` seconds, because checking it runs several git commands. Non-crawl calls cost one stat and no git.

Install is idempotent. `--dry-run` previews the change. `--crawl-guard=off` or `--uninstall` removes the hook. Switching modes replaces the entry.

## Measuring

There are two layers, kept apart on purpose.

1. **Local usage log** (off by default). Set `STACKLIT_STATS=1`, or `"stats": true`, and Stacklit appends one JSON line per `query`, `slice`, `explain` and `impact` call. The lines go to `stats.jsonl` under the user cache dir. Each line has the time, the command, the repo base name, `served` (the tokens of the answer, bytes/4), `replaced_est` and the file count. Nothing is sent anywhere. `stacklit stats [--json] [--since 7d|36h|2026-01-31]` reports totals.
   - **Measured**: how many tokens Stacklit's answers put in front of the agent.
   - **Estimated, not measured**: `replaced_est` adds up the sizes of the files an answer named, as if the agent would have read each one in full. Agents do not always read whole files, and they may not have read those files at all. Treat it as an upper bound on one possible alternative. It is never a saving.
   - **Not measured**: whether the agent would have asked the question anyway, what it did instead, the hook injections (their size is in the table above), or the quality of the answers.
2. **Real-agent A/B** in `bench/agent-ab/`. It runs pinned tasks on a pinned repo, with and without Stacklit, through `claude -p --output-format json`. It compares the usage the CLI reports. This is the only layer that measures an actual agent. The README there covers the cost, which is real money. A single smoke test (haiku, Stacklit arm only) confirmed that the harness runs end to end. It reported $0.003, which says nothing about savings.

## Troubleshooting

- **The hook does not fire.** Run `/hooks` in Claude Code, and check the entries for `stacklit hook ...`. Confirm that `stacklit` is on the PATH Claude Code sees. Hooks run with your login environment, which may not match your shell's. Test by hand: `echo '{"source":"startup","cwd":"'$PWD'"}' | stacklit hook session`. Hooks are read at session start, so restart the session after `stacklit setup --session-hook`.
- **Nothing is injected at startup.** This is by design. When `CLAUDE.md` already holds a digest of the current structure, only the status line is added. Check with `stacklit digest`, and compare it with the `map <hash>` in `CLAUDE.md`.
- **The index looks stale.** `stacklit status` lists the files changed since the build. `stacklit refresh --files <paths>` fixes named files. `stacklit generate --graph` rebuilds everything. A stale state record after a branch switch is rebuilt on the next refresh.
- **Concurrent sessions.** They share the worktree's lock. The loser queues its paths and returns. If a crashed process left `refresh.lock` behind, it is reclaimed after 30 s. To reset all state, delete `stacklit/sessions/<hash>/` in the cache dir. Nothing in it is the source of truth.
- **Hand edits do not show up.** The refresh hook only sees the agent's tool edits. Run `stacklit refresh --files ...`, or rely on the post-commit hook.
- **`--no-mcp` used to skip the Claude hooks too.** Hooks now install regardless of that flag.

## What was not verified

- That a live Claude Code session fires `SessionStart` on `compact` and adds the output as documented. Also that `SubagentStart` injection reaches an Explore agent, and that `async` PostToolUse hooks run on every edit. The payload shapes come from the docs, and the handlers are tested against them. No interactive session was driven.
- The exact `cwd` and field names in the `SubagentStart` payload are not in the docs excerpt that was read. The handler reads `agent_type` and `cwd`, and it stays silent if they are missing.
- The crawl guard in a live session. That means confirming that Claude Code shows the `deny` reason to the model, and that `additionalContext` on `PreToolUse` reaches it. Both come from the hooks reference: `permissionDecisionReason` is shown to Claude on deny, and `additionalContext` is added alongside the tool result. The docs do not say what happens to `additionalContext` on a denied call, so the deny path puts everything in the reason. The subagent tool is documented as `Agent`. `Task` is matched too, in case an older version still reports it. The `Glob`, `Read` and `Agent` input field names (`pattern`/`path`, `file_path`, `prompt`) come from the same reference.
- Whether the MCP `instructions` string reaches the model in each client, and whether any of this lowers crawling. That needs the paid A/B run.
- Savings from any of this on real tasks. That needs the paid A/B run.
