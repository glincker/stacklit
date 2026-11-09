# Pre-registration, round 2: does the routed steering fix what round 1 found?

Written and committed before any round 2 session ran. Nothing here has been run: no paid session has been started, and the harness refuses to start one without `--yes-spend`. Spending needs the owner's approval first (see "Approval"). Tracking issue: glincker/stacklit#78. Round 1 is pull request #77 (`bench/agent-ab/`, tracking issue #67).

## Why a second round

Round 1 (450 sessions, claude-sonnet-5-5, three repos) found no overall saving: pooled cost ratio 1.00 [0.93, 1.07], tool calls 0.68, Grep and Glob calls 0.18, correctness 94.0% to 88.0% (98.0% to 95.3% audited). It helped on the 16,000 file repo (cost 0.78) and on locate and add questions (0.88 and 0.78). It hurt on small repos (featuredrop 1.23), on "who imports X" questions (1.59, one grep is enough) and on questions whose answer is several files (trace correctness 97.5% to 82.5%, where the agent took one `stacklit query` result as the answer). The findings are summarised in `docs/SESSIONS.md`.

Since then the product changed in five ways, each aimed at a category, none at a task (branch `feat/steering-v2`):

1. Steering variant `e` (routed wording) and matching MCP and `stacklit guide` text.
2. `stacklit query` adds one line naming the next ranked files, with the commands to expand, for flow-like questions.
3. `stacklit impact` on a file or package lists importers as `path:line`, checked against the sources for script languages.
4. Small repo mode: below 600 source files the hook and `setup` carry a one line note instead of the 490 token digest.
5. Docs.

This round asks whether the combination, mainly variant `e`, removes the harm without losing the gains.

## Arms

All treatment arms use one stacklit binary built from the head of `feat/steering-v2` when the run is approved (its SHA-256 prefix is written into every session record as `stacklit_bin`, and the commit is recorded under "Approval"). They differ in one setting each, so the comparison between them is not confounded with the other product changes.

| Arm | Setup | Environment |
|---|---|---|
| `control` | nothing added; tools Read, Grep, Glob | none |
| `stacklit` | `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`; Bash allowed only as `Bash(stacklit *)` | `STACKLIT_STEERING=a` (the current default), `STACKLIT_SMALL_REPO_FILES=0` |
| `stacklit-e` | as `stacklit` | `STACKLIT_STEERING=e` (routed wording), `STACKLIT_SMALL_REPO_FILES=0` |
| `stacklit-e-small` | as `stacklit-e` | `STACKLIT_STEERING=e`, `STACKLIT_SMALL_REPO_FILES=600` (the default), only on repos below that limit |

`stacklit-e-small` runs only on featuredrop (412 tracked files, 283 indexed). The other three repos are above the limit, so for them the arm would be identical to `stacklit-e`. The strict arm of round 1 (crawl guard, steering `d`) is not repeated: it measured worse and is documented as not recommended.

Same session settings as round 1: `claude -p --output-format stream-json --verbose --no-session-persistence --setting-sources project,local --strict-mcp-config --disable-slash-commands`, fresh shallow clone per session, answer-only prompt suffix, sequential sessions. The harness refuses to start if `~/.claude/settings.json` mentions stacklit.

## Repos

Pinned by full SHA in `bench/agent-ab/repos-round2.json`. The first three are the round 1 repos at the same commits.

| Repo | Language | Tracked files | SHA |
|---|---|---|---|
| glincker/levelrail | Go and TypeScript | 15,978 | 8fbb2789404ba4b0d445187e511bdda67e871060 |
| glincker/theauth-go | Go | 1,294 | d4b3841044aef0a3c21c490431b1bd8a4f507b96 |
| glincker/featuredrop | TypeScript | 412 | 9d0fc5f6d59bbd0b89156c7deab485df56c5cf7a |
| glincker/theauth | TypeScript monorepo | 2,919 (1,921 source files) | 8c5de6256af91c90a970153fb8278185d6d92429 |

The fourth repo is new. Public glincker repos of 3,000 to 10,000 files do not exist: `thesvg` has 14,656 tracked files but only 431 are code (the rest are icons), `glinui` has 1,008 and `theauth` has 2,919, the closest to the range and a real code base. It was created in March 2026, last pushed 2026-11-01, has 22 stars, and was renamed from an earlier name, so it is neither famous nor a fork of a known project. It is not above the 5,000 file threshold of the decision rule, so only levelrail counts there. Whether the model has memorised it is not known and could not be tested without spending (see "Memorisation check").

## Tasks

`tasks-round2/<repo>.json`, 10 per repo, 40 in total. They are new: no question text is reused from round 1, and a test (`TestRound2TasksAreNewAndWellFormed`) fails if a round 2 task shares half or more of its answer files with a round 1 task of the same repo. Kinds and proportions match round 1 (locate, trace, impact, add):

| Repo | locate | trace | impact | add |
|---|---|---|---|---|
| levelrail | 3 | 3 | 2 | 2 |
| theauth-go | 4 | 2 | 2 | 2 |
| featuredrop | 3 | 3 | 2 | 2 |
| theauth | 3 | 3 | 2 | 2 |
| total | 13 | 11 | 8 | 8 |

Round 1 had 10, 8, 6 and 6 of 30. Authoring: each task was written by reading the code at the pinned commit. Every answer file carries a `check` (a regular expression that must match its content, so the file really is about what the question says), every impact task carries a `verify` (an independent grep rule whose result must equal the answer list exactly), and `TestRound2AnswersVerifiedByGrep` re-checks all of it against checkouts at the pinned SHAs (`AGENT_AB_CLONES=<dir> go test ./bench/agent-ab`). It passes now; it was also shown to fail when an answer list is wrong. The tasks were written and fixed before any session of the new arms ran, and before looking at how variant `e` performs. Task wording does not name the answer files, except that impact questions name the module whose importers are asked for. The `any_of` and `all_of` lists were chosen so that a correct answer needs no later patching.

## Memorisation check

Before the main run: the six first tasks of each repo with every tool disabled (`--tools ""`, `-mode memory -limit 6`), one trial, so 24 sessions at about 0.01 USD. A repo whose correctness exceeds 20 percent is called contaminated and excluded from the headline (the decision rule is then computed on the remaining repos and, separately, on all four). Round 1 got 0 of 18 on its three repos with other tasks; the new tasks on those repos and all of `theauth` are untested until this check runs.

## Metrics

Per session, as in round 1: input, output, cache creation and cache read tokens; total tokens; uncached-equivalent tokens (input + output + cache creation + 0.1 x cache read); notional cost; turns; tool calls by tool; files read; wall seconds; correctness; recall; whether `stacklit` commands were run. Primary: notional cost and correctness. Also reported: tool calls, Grep and Glob calls, tokens.

## Grading

One matcher, decided before the run, for every arm: every `all_of` path is named in the answer and, if `any_of` is not empty, at least one `any_of` path is named, with boundary-aware matching that also accepts a leading `./` and an absolute path inside the checkout (these are the grader fixes round 1 had to add after the fact). A directory entry ending in `/` matches any file under it. There are no per-task fixes. 10 percent of transcripts (seeded random) are checked by hand for grading errors. A defect found afterwards is reported as a separate column, applied to every arm, and is not used by the decision rule.

## Trials, model, order, seed

5 trials per task and arm. Model `claude-sonnet-5-5` (`-model claude-sonnet-5-5`; the id the CLI reports is recorded and any difference is reported). Session order is shuffled with seed 20261108 within each repo, so arms are interleaved. Bootstrap seed 20260508, 10,000 resamples of task-trial pairs, as in round 1.

## Size, cost and caps

| Item | Sessions |
|---|---|
| control, stacklit, stacklit-e: 4 repos x 10 tasks x 5 trials x 3 arms | 600 |
| stacklit-e-small: featuredrop only, 10 x 5 | 50 |
| Main run | 650 |
| Memorisation check | 24 |

Projection at 0.03 USD per session (round 1 observed a mean of 0.0274, p99 0.062, maximum 0.080): 19.50 USD for the main run and about 0.30 USD for the memorisation check. These are notional list-price dollars, not membership usage.

Caps: 0.25 USD per session (`--max-budget-usd 0.25`, three times the round 1 maximum), 25 turns, and a total notional cap of 30 USD across the ledger (`--cap-usd 30`, about 1.5 times the projection).

## Stop rules

- Stop at once on any rate limit, usage limit or authentication error (the harness does this).
- Stop when the ledger plus one session cap would pass 30 USD.
- Stop after 10 consecutive harness failures (clone, setup, parse); fix and resume. Failed sessions are not saved, so a resume retries them.
- No interim analysis of correctness or cost. No stopping for efficacy or futility. If the run stops before every session is done, the decision rule is not evaluated and the default stays `a`.

## Decision rule (fixed before seeing data)

Steering `e` becomes the default if and only if all of these hold. They are implemented in `bench/agent-ab/decide.go` and printed by `agent-ab -analyze` as `decision-round2.md`. Pooled means all four repos, 400 pairs of `stacklit-e` against `control`.

| Id | Criterion |
|---|---|
| P1 | Pooled cost ratio (`stacklit-e` / `control`), upper 95% bound below 1.05 |
| P2 | Pooled correctness difference (`stacklit-e` minus `control`), lower 95% bound above -5 points |
| P3 | Cost ratio point estimate below 1.00 on every repo with more than 5,000 tracked files (levelrail) |
| P4 | Pooled cost ratio `stacklit-e` / `stacklit`, upper 95% bound below 1.05 (not worse than the current default wording) |
| G1 | Trace questions: correctness difference `stacklit-e` minus `control`, lower 95% bound above -10 points (round 1: -15 [-27.5, -5]) |
| G2 | Impact questions: cost ratio `stacklit-e` / `control`, upper 95% bound below 1.15 (round 1: 1.59 [1.47, 1.72]) |

If any criterion fails, or has no data, the default stays `a` and `e` stays opt-in; the result is published either way.

Small repo mode is already on by default at 600 files (it is a different kind of change: it removes text, it adds none). It stays on if, on featuredrop, `stacklit-e-small` against `control` has a cost ratio with upper bound below 1.15 (round 1: 1.23 [1.14, 1.33]) and a correctness difference with lower bound above -10 points (criterion S1). Otherwise the default limit goes to 0 (off).

No claim of savings may be made from this round unless the pooled cost ratio's upper bound is below 1.00; P1 only decides the default wording.

## Limits, stated up front

One model, one day of runs, one machine; tasks written by us on repos we own; 40 tasks, 5 trials, so by-kind rows rest on 40 to 100 pairs; correctness is a path match; the product changes were designed from round 1 failures, so improvement on the categories round 1 found is expected and the new tasks test generalisation to new questions, not to new kinds; `impact` now answers import questions directly, which makes the impact tasks easy for the treatment arms by design; notional cost is not membership usage; `seconds` includes clone and setup and is not latency; control has no shell while treatment arms have one limited to `stacklit *`.

## Run (not started)

```bash
go build -o /tmp/stacklit ./cmd/stacklit
go build -o /tmp/agent-ab ./bench/agent-ab
R2="-repos bench/agent-ab/repos-round2.json -tasks bench/agent-ab/tasks-round2 -results bench/agent-ab/results-round2 -ledger bench/agent-ab/ledger-round2.jsonl -stacklit /tmp/stacklit -model claude-sonnet-5-5 -seed 20261108 -max-budget-usd 0.25 -cap-usd 30"

# verify the answers against pinned checkouts (no spend)
AGENT_AB_CLONES=/path/to/checkouts go test ./bench/agent-ab

# plan only (prints sessions per repo and the projected cost)
/tmp/agent-ab -repo all $R2 -arms control,stacklit,stacklit-e,stacklit-e-small -trials 5 -dry-run

# after approval: memorisation check, then the run, then the analysis
/tmp/agent-ab -repo all $R2 -mode memory -limit 6 -results bench/agent-ab/results-round2/memory -yes-spend
/tmp/agent-ab -repo all $R2 -arms control,stacklit,stacklit-e,stacklit-e-small -trials 5 -yes-spend
/tmp/agent-ab -analyze $R2
```

## Approval

Given. On 2026-11-08 the owner approved running round 2 on their Claude membership ("spin agents do it"), after being told that round 2 is about 650 sessions and about 19.5 USD notional with a 30 USD cap, plus a 24 session memorisation check of about 0.3 USD. The overall owner cap across all rounds is 60 USD notional and about 12.54 USD was spent in round 1 (`ledger.jsonl`). The runner stops if the grand total of both rounds would pass 45 USD, to leave room for a later round on famous repos. Per session hard cap 0.25 USD and 25 turns. Sessions run sequentially.

Binary: built from the head of `master` at commit 533ea24 (the `feat/steering-v2` changes were merged there by pull request #81), not from the `feat/steering-v2` branch named under "Arms". The product code is the same; the binary hash is recorded in every session record.

Amendment 2026-11-08 (before any paid session): the run commands above use `-work` for the clone directory (outside the repo) and the pre-registered `-grader audited` default, which is the matcher described under "Grading". No change to arms, tasks, trials, seed, or the decision rule.
