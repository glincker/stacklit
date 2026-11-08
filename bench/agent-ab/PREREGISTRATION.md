# Pre-registration: real-agent A/B, Stacklit vs nothing

Written and committed before any paid session ran. Tasks, repos, arms, metrics and the analysis plan below are fixed; nothing is tuned to results. Tracking issue: glincker/stacklit#67.

## Question

Does giving a Claude Code session Stacklit's generated map (session hook plus the `stacklit` command) change how much it explores, how many tokens and notional dollars it uses, and whether it names the right files, on code the model cannot have memorised?

## Repos (pinned by full SHA, all public, in `repos.json`)

| Repo | Language | SHA | Why |
|---|---|---|---|
| glincker/levelrail | Go and TypeScript, about 16,000 tracked files | 8fbb2789404ba4b0d445187e511bdda67e871060 | large, multi-layer |
| glincker/theauth-go | Go, about 1,300 files | d4b3841044aef0a3c21c490431b1bd8a4f507b96 | medium |
| glincker/featuredrop | TypeScript, about 400 files | 9d0fc5f6d59bbd0b89156c7deab485df56c5cf7a | small (glinui was the fallback, not needed) |

All three were pushed in Oct to Nov 2026, so they are recent and not famous. A famous repo (gin) was piloted earlier and discarded: the model answered from memory.

## Tasks

Ten per repo in `tasks/<repo>.json`, written by reading the code (not by a model), each answer checked by grep and by file existence at the pinned SHA. Kinds: locate, trace (answer is a set of paths), impact (who imports X), add (where to add a feature following the pattern). Each task has `all_of` (every path must be named) and `any_of` (at least one must be named), plus a rubric.

Grading is deterministic: a session is correct when every `all_of` path is named in the final answer (boundary-aware match, so `audit/events.go` does not match inside `internal/audit/events.go`) and, if `any_of` is non-empty, at least one `any_of` path is named. Also recorded: recall, the fraction of `all_of` paths named. 10 percent of transcripts are hand-checked for grading errors.

## Memorisation check (stage a)

Six tasks per repo are run with every tool disabled (`--tools ""`), one trial, so the model must answer from memory. If correctness is above 20 percent on a repo, its tasks are called contaminated and excluded from the headline.

## Arms

1. control: nothing added. Tools: Read, Grep, Glob.
2. stacklit: `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`, tools as control plus `Bash(stacklit *)`.
3. stacklit-strict (only if `origin/feat/source-of-truth` has landed `--crawl-guard` and steering variant d): built into a separate binary from a temp worktree. Run only after arms 1 and 2 are finished.

Model: sonnet alias, the exact id is read from the CLI output and reported. Sessions run sequentially, seed-shuffled and interleaved, with `--no-session-persistence --setting-sources project,local --max-turns 25 --max-budget-usd 1.5`. Every session works in a fresh clone in a temp dir, deleted afterwards.

## Stages and caps

(a) memorisation check; (b) pilot on theauth-go, 5 tasks x 1 trial x 2 arms, to check cost, parsing and grading and fix harness bugs, then project total cost; (c) full run, 10 tasks x 3 trials x 2 arms per repo, reduced to fewer trials or tasks if the projection exceeds the cap; (d) optional third arm. Total notional spend must stay under 60 USD, tracked in `ledger.jsonl`. The run stops at once on any rate limit, usage limit or authentication error.

## Metrics

Per session: input, output, cache creation and cache read tokens; total tokens (all four summed); uncached-equivalent tokens (input + output + cache creation + 0.1 x cache read); notional cost; turns; tool calls by tool; files read and bytes returned by Read; wall seconds; correctness; recall; whether `stacklit` commands were run.

## Analysis

Paired by task and trial. For each metric: mean per arm, mean paired difference and ratio of means, with 95 percent seeded bootstrap intervals (10,000 resamples, resampling task-trial pairs), per repo and pooled. Correctness: rate per arm with Wilson intervals, plus a paired bootstrap of the difference. Fraction of stacklit-arm sessions in which Stacklit commands were used. Breakdown by task kind. No session is dropped after seeing results, except documented harness failures (such sessions are re-run only if the cap allows, and listed). Results are reported whichever way they go.

## Known limits (stated up front)

One model, tasks written by us, small n, correctness by path match, notional cost is not membership usage, day-to-day variation in the service.

## Amendment 1 (written after the 3-trial checkpoint, before any further session)

Facts at this point: the 3-trial run for arms control and stacklit on all three repos finished (180 sessions, 4.9 USD notional of the 60 USD cap, no harness failures). The pilot and run showed sessions cost about 0.03 USD, far below the planning figure. Outcomes of the checkpoint are committed in git history (tables.md at that commit) and have been seen.

Decisions, taken on cost and on grading defects, not to improve either arm:

1. Trials 4 and 5 are added for control and stacklit on every task (same seed, same harness). The headline uses all 5 trials. The 3-trial table stays in git history.
2. Third arm `strict` (stacklit-strict) is run for 5 trials per task, paired with the existing control sessions. It is NOT interleaved with control (control ran earlier), a limit stated in RESULTS.md. Interface from the branch owner: `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook --crawl-guard=redirect`, steering variant d via STACKLIT_STEERING=d, `Bash(stacklit *)` allowed, binary built from origin/feat/source-of-truth at fdc0f40.
3. Transcript spot check found three grading defects, all independent of arm. They are reported as a second metric, `correct_audited`, next to the pre-registered `correct` (never replacing it): (a) the matcher rejected answers written as `./path` or as an absolute path inside the checkout, (b) fd-07 asks where to add a new adapter and the model correctly names a new file under src/adapters/, which the pinned any-of list (existing files) did not allow, (c) fd-09: src/analytics.ts (PostHogAdapter) also forwards events to PostHog and was missing from the pinned answer.
