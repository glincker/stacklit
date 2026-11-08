# Real-agent A/B harness

Measures what the offline benchmark cannot: what a real Claude Code session does on the same task with and without Stacklit. Results of the first full run are in [RESULTS.md](RESULTS.md); the claims they support (and do not) are in [PROPOSED_CLAIMS.md](PROPOSED_CLAIMS.md); the plan fixed before running is [PREREGISTRATION.md](PREREGISTRATION.md).

It uses real `claude -p` sessions, so it spends plan quota (the dollar figures Claude Code reports are notional). It will not start without `--yes-spend`, keeps an append-only `ledger.jsonl`, stops when the ledger plus one session cap would pass `--cap-usd` (default 60), and stops at once on a rate limit, usage limit or authentication error.

## Files

- `repos.json`: three public repos pinned by full SHA (levelrail, theauth-go, featuredrop). Chosen because the model cannot have memorised them.
- `tasks/<repo>.json`: 10 tasks each (locate, trace, impact, add) with `all_of` and `any_of` expected paths and a rubric.
- `results/<repo>/<arm>/<task>-t<trial>.json`: one raw record per session (usage, tool calls by tool, files read, bytes returned by Read, stacklit use, correctness, call log). `results/memory/` holds the all-tools-off check. Transcripts are not kept in the repo.
- `ledger.jsonl`, `summary.json` (machine readable, for the site), `tables.md` (every table with intervals).

## Arms

- `control`: tools Read, Grep, Glob.
- `stacklit`: `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`; Bash is available but only `Bash(stacklit *)` is allowed.
- `strict`: as `stacklit` plus `--crawl-guard=redirect` and `STACKLIT_STEERING=d`. Needs a stacklit binary built from a branch that has the crawl guard (pass it with `-stacklit`, and make sure a file named `stacklit` is what the hook finds on PATH).

Sessions run with `--output-format stream-json --verbose --no-session-persistence --setting-sources project,local --strict-mcp-config --disable-slash-commands`. The last two flags matter: without them account connectors and skills add about 60,000 tokens to every session. The harness refuses to start if `~/.claude/settings.json` mentions stacklit.

## Run it

```bash
go build -o /tmp/stacklit ./cmd/stacklit
go build -o /tmp/agent-ab ./bench/agent-ab

# plan only, runs nothing
/tmp/agent-ab -repo theauth-go -stacklit /tmp/stacklit -dry-run

# memorisation check: all tools off, first 6 tasks, results under results/memory
/tmp/agent-ab -repo theauth-go -mode memory -limit 6 -results bench/agent-ab/results/memory -stacklit /tmp/stacklit -yes-spend

# a run (resumable: finished sessions are skipped)
/tmp/agent-ab -repo theauth-go -trials 3 -stacklit /tmp/stacklit -yes-spend

# analysis only: writes summary.json and tables.md next to the ledger
/tmp/agent-ab -analyze
```

Set `AGENT_AB_TRANSCRIPTS=<dir>` to keep raw streams outside the repo for hand checks. Each session works in a fresh shallow clone under `-work` (default `/tmp/abreal/work`), deleted afterwards.

## Grading and statistics

A session is correct when every `all_of` path and at least one `any_of` path is named in the answer (boundary-aware match). `correct_audited` in the analysis adds three documented fixes; see RESULTS.md. Differences and ratios of means are paired by repo, task and trial, with seeded bootstrap intervals (10,000 resamples); rates use Wilson intervals.

## Limits

One model, tasks written by us, small n, path-match correctness, notional cost. `seconds` includes clone and setup time. Details in RESULTS.md.
