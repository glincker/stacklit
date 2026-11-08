# Pre-registration, famous repos: does the result hold on code the model has heard of?

Written and committed before any famous repos session ran. Nothing here has been run: no paid session has been started, and the harness refuses to start one without `--yes-spend`. Spending needs the owner's approval first (see "Approval"). Tracking issue: glincker/stacklit#83. Rounds 1 and 2 are tracked in #67 and #78.

## Why

Rounds 1 and 2 use repos the model cannot have memorised (small, recent, mostly our own). That is the honest design for the memorisation problem, but it leaves an obvious objection: nobody uses Stacklit on a repo like that. A pilot on gin showed the opposite problem: on a famous repo the model answered from memory in two turns, with or without Stacklit, so the test measured nothing.

This study keeps both properties. It uses five famous, active repos, and writes every task about code that was added or restructured after the model's training cutoff, so memory cannot answer it. A memory check with all tools disabled (below) tests that claim and drops any task or repo where it fails.

## The cutoff

Assumed knowledge cutoff of `claude-sonnet-5-5`: 2026-06-01. Tasks are written about code added or substantially changed on or after 2026-07-01, one month later, to leave a margin. The test `TestFamousAnswersVerifiedByGrep` re-checks this from git history: for every task at least one answer file was added after 2026-07-01 or changed by 10 or more lines since then. The clones used for authoring were fetched with `--shallow-since=2026-05-01`, so the history reaches back before the cutoff (the test fails otherwise).

Result of that check, on the pinned commits: all 50 tasks pass. In 33 of 50 tasks at least one answer file was created after the cutoff; in 18 of 50 every answer file was. The other tasks ask about a post-cutoff feature whose answer includes an older file that the feature touched (for example the template engine file that gained a new option). Those are the tasks where memory of the older file could help, which is why the memory check runs on all 50 tasks and why per-task exclusion is part of the rule.

## Repos

Chosen from the candidates by public facts checked with `gh api` on 2026-11-08, not by looking at how any arm does. All are permissively licensed, were pushed within a day of the check, and have hundreds to thousands of commits after 2026-07-01. Pinned by full SHA in `bench/agent-ab/repos-famous.json` (taken from the head of the default branch when the tasks were written).

| Repo | Languages | SHA | Tracked files | Source files | Stars | Licence | Commits since 2026-07-01 (no merges) | Files added since | Files touched since |
|---|---|---|---|---|---|---|---|---|---|
| django/django | Python | a1f2b45ce5a327f42437bdb620a817839d0bd067 | 7,097 | 2,988 | 91,694 | BSD-3-Clause | 362 | 51 | 930 |
| facebook/react | JavaScript, Rust | d71b537241d26861df496413db2142c7ba192c25 | 7,300 | 4,610 | 252,421 | MIT | 211 | 132 | 857 |
| tokio-rs/tokio | Rust | 99dbe548b7046915cd56e570e31604e1f8fa83cd | 893 | 817 | 33,723 | MIT | 299 | 34 | 456 |
| apache/kafka | Java, Scala | 8f4cc1d80036105ffc79388dc659a9b325996943 | 7,664 | 6,740 | 34,194 | Apache-2.0 | 843 | 318 | 3,173 |
| rails/rails | Ruby | 24259cf2d25c71c3e3320bf0be7ff2e3de93becf | 5,032 | 3,546 | 59,081 | MIT | 1,379 | 85 | 2,214 |

Why each is a fair test:

- django: the largest Python code base of the five by reputation, mature, with a visible 6.1 development line (HTTP QUERY support, multiline template tags, GIS security hardening).
- react: JavaScript monorepo plus a new Rust port of the compiler in the same tree, so one index covers two languages.
- tokio: the small repo of the set (893 files, below the 3,000 to 20,000 preference). Kept on purpose as the counterweight: round 1 found Stacklit costs more on small repos, and a study that only had big repos would be tilted.
- kafka: the biggest source base (6,740 source files, 1.6 million lines of Java), many modules, heavy post-cutoff churn (cluster mirroring, a new uniform assignor, share group dead letter queues, a chunked producer accumulator).
- rails: Ruby, 5,032 files, the highest churn of the set; Active Record's connection layer was restructured after the cutoff.

Not chosen, and why: `kubernetes/kubernetes` (31,746 tracked files at its current head, above the 20,000 preference; the index is cheap there, 23 s and 0.64 GB, but sessions on it would cost the most and it would leave a quarter of the budget for one repo). It is the reserve if one of the five is dropped before the run. `vercel/next.js` and `microsoft/TypeScript` (tens of thousands of fixture and baseline files, so file counts say little about code size). No Go repo is in the set; the tool is strongest on Go, so this errs against it.

## Index cost and feasibility (measured, free)

Each treatment session runs `stacklit generate --graph` on a fresh shallow checkout. Measured with the current branch binary, `GOMEMLIMIT=2GiB`, a 300 s timeout, on a busy machine (load average 12 to 60 from other jobs, so wall times are upper bounds; user CPU seconds are the load-independent number):

| Repo | Wall | User CPU | Peak memory | Within 300 s | Within 2 GB |
|---|---|---|---|---|---|
| tokio | 55 s | 58 s | 2.2 GB | yes | marginal |
| django | 175 s | 184 s | 2.6 GB | yes | no |
| react | 68 s | 80 s | 3.1 GB | yes | no |
| kafka | 261 s | 957 s | 5.5 GB | yes (barely) | no |
| rails | 120 s | 321 s | 1.5 GB (no limit set) | yes | yes |
| kubernetes (reserve) | 23 s | 23 s | 0.64 GB | yes | yes |

`GOMEMLIMIT` did not hold the generator under 2 GB (the memory is not the Go heap). So indexing kafka, react and django needs a machine with about 8 GB free; this is a finding about the tool, reported as is. To keep wall time sane the harness has `-reuse-index` (on in the famous preset): the index of one commit is the same in every session of an arm, so it is generated once per arm and repo and copied into each later checkout (`stacklit.json`, `stacklit.graph.json`, `stacklit.html`, `DEPENDENCIES.md`; nothing in them depends on the path). `stacklit setup` still runs in every session. Without reuse the treatment sessions would spend about 12 hours of index time in total. Reuse changes no token or cost number.

## Tasks

`tasks-famous/<repo>.json`, 10 per repo, 50 in total. Written by reading the post-cutoff code and its commits (not by a model), every one about a feature, file or restructuring dated after 2026-07-01. No question text from rounds 1 or 2 is reused (tested). Kinds and proportions follow rounds 1 and 2:

| Repo | locate | trace | impact | add |
|---|---|---|---|---|
| django | 3 | 3 | 2 | 2 |
| react | 3 | 3 | 2 | 2 |
| tokio | 3 | 3 | 2 | 2 |
| kafka | 3 | 3 | 2 | 2 |
| rails | 3 | 3 | 2 | 2 |
| total | 15 | 15 | 10 | 10 |

(Round 1 had 10, 8, 6, 6 of 30; round 2 had 13, 11, 8, 8 of 40.)

Rules for the tasks, all enforced by `go test ./bench/agent-ab`:

- Each has `all_of` and `any_of` expected paths, a rubric, a `check` regular expression per answer file (the file really is about what the question says) and, for impact tasks, a `verify` grep rule whose result must equal the answer list exactly.
- No answer path appears in the question, and a locate question must not contain the answer file's name, so no task is answerable from a bare name or a file name glob.
- With `AGENT_AB_CLONES=<dir>` (one checkout per repo at its pinned SHA, fetched with `--shallow-since=2026-05-01`) the test re-checks every path, every `check`, every impact grep, the commit, the history depth and the post-cutoff change of the answer files. It passes now, and was shown to fail when an answer list is wrong.
- Impact tasks name the module whose users are asked for. Their grep rule skips comment-only lines and test directories, and the question says so.

## Memorisation check

Before the main run, all 50 tasks with every tool disabled (`--tools ""`), one trial, answer from memory: 50 sessions at about 0.01 USD (0.60 USD notional). Rules, fixed now:

1. A task answered correctly in the memory check is dropped from the analysis for every arm ("memorised task"). The list of dropped tasks is published.
2. A repo is contaminated if more than 20 percent of its 10 memory-checked tasks are correct (3 or more of 10). A contaminated repo gets no claim of any kind and is left out of the pooled result, but its own table is still published.
3. A repo with no memory check results is treated as contaminated.

Round 1 got 0 of 18 on three unknown repos with this check. If famous repos pass at a similar rate the post-cutoff design works; if they do not, that is the main finding and the cost result is secondary.

## Arms

Same three as in round 2, same binary rule: one stacklit binary built from the head of `feat/steering-v2` when the run is approved (SHA-256 prefix written into every record as `stacklit_bin`, commit recorded under "Approval").

| Arm | Setup | Environment |
|---|---|---|
| `control` | nothing added; tools Read, Grep, Glob | none |
| `stacklit` | `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`; Bash allowed only as `Bash(stacklit *)` | `STACKLIT_STEERING=a` (the current default), `STACKLIT_SMALL_REPO_FILES=0` |
| `stacklit-e` | as `stacklit` | `STACKLIT_STEERING=e` (routed wording), `STACKLIT_SMALL_REPO_FILES=0` |

Small repo mode is off in both treatment arms so they differ in one setting. tokio has 817 source files, above the default 600 file limit, so it would not trigger anyway. Session settings as in round 1: `claude -p --output-format stream-json --verbose --no-session-persistence --setting-sources project,local --strict-mcp-config --disable-slash-commands`, fresh shallow clone (depth 1 at the pinned SHA) per session, answer-only prompt suffix, sequential sessions, the harness refuses to start if `~/.claude/settings.json` mentions stacklit. The sessions have no web or search tools. Release notes and changelogs inside the checkout are readable by every arm; that equally helps all arms and is stated as a limit.

## Model, trials, order, seed, caps

- Model `claude-sonnet-5-5` (the id the CLI reports is recorded; any difference is reported).
- Seed 20261109 for the session order within each repo (arms interleaved), bootstrap seed 20260508 with 10,000 resamples, as in rounds 1 and 2.
- Per session: 0.25 USD hard cap (`--max-budget-usd 0.25`), 25 turns. Total notional cap 20 USD across this study's ledger (`bench/agent-ab/famous/ledger.jsonl`), which includes the memory check.
- Stop at once on any rate limit, usage limit or authentication error. Stop when the ledger plus one session cap would pass the cap. Stop after 10 consecutive harness failures (clone, index, setup, parse); a failed session is not saved, so a resume retries it. No interim analysis of correctness or cost. No stopping for efficacy or futility.

### Trials, in two stages

The cost is projected below. Three trials do not safely fit 20 USD at the middle estimate, so the study runs in two stages:

- Stage A: trials 1 and 2, all five repos, three arms: 300 sessions. This stage is the committed minimum.
- Stage B: trial 3, only if the ledger allows it, decided by cost alone and repo by repo in the file order of `repos-famous.json`: run trial 3 of a repo (30 sessions) if the ledger plus 30 times 1.25 times that repo's observed stage A mean session cost stays under 20 USD. No look at correctness or at any ratio is allowed for this decision.

Per-repo pairs are therefore 20 (stage A) or 30 (with stage B): intervals per repo will be wide, so a per-repo "no claim" means "not shown", never "no effect".

## Cost projection (free: dry-run)

From round 1 records: cost is exactly linear in tokens (2 USD per million input, 4 cache creation, 0.10 cache read, 10 output; fit error 1e-16). Control cost per session was 0.0206 (412 files), 0.0243 (1,294) and 0.0344 (15,978), about 0.0087 USD per decade of tracked files. The model: `low = 0.0206 + 0.0087 x (log10(files) - log10(412))`, `mid = 1.5 x low`, `high = 2.5 x low`. The multipliers allow for tasks that need more searching than round 1's (new code, unfamiliar names) and for much larger source files: kafka and react have files of several thousand lines, and one Read of such a file costs 0.05 to 0.10 USD in cache creation alone. Round 1's p95 was about 2 times its mean. Treatment arms are assumed to cost the same as control (round 1 ratios ranged from 0.78 to 1.23).

| Repo | per session low / mid / high (USD) | sessions at 3 trials | mid (USD) | range (USD) |
|---|---|---|---|---|
| django | 0.031 / 0.047 / 0.078 | 90 | 4.23 | 2.83 to 7.06 |
| react | 0.032 / 0.047 / 0.079 | 90 | 4.25 | 2.83 to 7.08 |
| tokio | 0.024 / 0.035 / 0.059 | 90 | 3.18 | 2.12 to 5.29 |
| kafka | 0.032 / 0.048 / 0.079 | 90 | 4.28 | 2.84 to 7.12 |
| rails | 0.030 / 0.045 / 0.075 | 90 | 4.06 | 2.71 to 6.76 |
| 3 trials, 450 sessions | | | 19.99 | 13.33 to 33.31 |
| 2 trials, 300 sessions (stage A) | | | 13.3 | 8.9 to 22.2 |
| memory check, 50 sessions at 0.012 | | | 0.60 | 0.60 |

`agent-ab -repo famous -dry-run` prints exactly these numbers. Recommended: stage A (about 13.9 USD with the memory check at the middle estimate), then stage B if the rule allows it. A fifth trial (750 sessions, 33 USD at mid) would not fit the 20 USD budget. The overall owner cap across all rounds is 60 USD, of which about 12.5 USD was spent on round 1, and round 2 (about 20 USD) is prepared separately.

## Metrics

Per session, as in round 1: input, output, cache creation and cache read tokens; total tokens; uncached-equivalent tokens; notional cost; turns; tool calls by tool; files read; wall seconds; correctness; recall; whether `stacklit` commands were run. Primary: notional cost and correctness. Also reported: tool calls, Grep and Glob calls, tokens. Per-repo tables are always published next to the pooled one, so no repo can be hidden: `bench/agent-ab/famous/tables.md` has a section for every repo and arm.

## Grading

One matcher for all arms, decided before the run (the audited matcher of round 2): every `all_of` path is named in the answer and, if `any_of` is not empty, at least one `any_of` path is named; boundary-aware matching that also accepts a leading `./` and an absolute path inside the checkout; a directory entry ending in `/` matches any file under it. No per-task fixes. 10 percent of transcripts (seeded random, kept outside the repo with `AGENT_AB_TRANSCRIPTS`) are checked by hand. A defect found afterwards is reported as a separate column, applied to every arm, and not used by the decision rule.

## Analysis

Paired by repo, task and trial; mean paired difference and ratio of means with seeded bootstrap intervals; Wilson intervals for rates; by kind and by repo; stacklit use rate; the fraction of sessions in which the hook fired. `agent-ab -repo famous -analyze` writes `summary.json`, `tables.md` and `decision-famous.md` into `bench/agent-ab/famous/`, dropping memorised tasks (`-include-memorised` keeps them for a second table), and pools only repos that passed the memory check (the pool over all repos is kept as `pooled-all`).

## Decision rule (fixed before seeing data)

Implemented in `bench/agent-ab/famous.go`, tested, and printed by the analysis. The unit is the arm against control; it is evaluated for `stacklit` and for `stacklit-e` separately and nothing is selected after the fact.

| Id | Rule |
|---|---|
| R1 | A repo "shows lower cost" iff it passed the memory check, the upper 95% bound of its cost ratio (arm / control) is below 1.00, and the lower 95% bound of its audited correctness difference is above -10 points. The statement allowed is "lower cost on this repo", with the interval. |
| R2 | "Lower cost on famous repos" is allowed iff R1 holds on at least 3 of the 5 repos and the pooled (eligible repos) cost ratio's upper bound is below 1.00. |
| R2L | The words "on large famous repos" additionally need at least 3 of the repos that show lower cost to have more than 5,000 tracked files (django, react, kafka and rails do; tokio does not). |
| R3 | "No loss of correctness" is allowed iff the pooled audited correctness difference's lower bound is above -5 points. |
| R4 | A repo with the lower bound of its cost ratio above 1.00 is reported as harm ("higher cost on this repo"), whatever the other repos show. |

Nothing else may be claimed from this study. In particular: no percentage saving from a point estimate alone, no claim on a repo that failed the memory check, no claim about repos not in the set, no claim that Stacklit "works on famous repos" if R2 does not hold. A comparison of `stacklit-e` against `stacklit` is reported (descriptive, pooled and per repo) but decides nothing here; the default wording is decided by round 2's rule.

## What is published, whatever happens

Everything, in the same format: this document, `repos-famous.json`, the 50 tasks, the memory check results and the list of dropped tasks, every per-session record, `ledger.jsonl`, `summary.json`, `tables.md`, `decision-famous.md`, and a `RESULTS-famous.md` that states the outcome in plain words. If the result is null or negative (no repo shows lower cost, or cost is higher, or correctness is lower), it is published with the same prominence as a positive one, the per-repo harm rows included, and the public claims stay as they are. If the memory check shows the model knew the answers, that is published as the finding. Repos are not swapped, tasks are not edited and sessions are not dropped after data exists, except a documented harness failure. A repo that cannot be indexed (time or memory) is reported as such with the measurement; it is not replaced silently.

## Limits, stated up front

One model; tasks written by us (but about code we do not own); 10 per repo, 2 or 3 trials, so per-repo intervals are wide and the pooled one carries the weight; the cutoff is an assumption; correctness is a path match; notional cost is not membership usage; `seconds` includes clone and setup and is not latency; control has no shell while treatment arms have one limited to `stacklit *`; the answer file of a task may be old even when the feature is new (33 of 50 tasks have a new file among the answers); changelogs and docs in the checkout describe the new features and help every arm equally; no Go repo is in the set; the index of three repos exceeds 2 GB of memory.

## Run (not started)

```bash
go build -o /tmp/stacklit ./cmd/stacklit
go build -o /tmp/agent-ab ./bench/agent-ab

# verify the answers against pinned checkouts (no spend)
AGENT_AB_CLONES=/path/to/checkouts go test ./bench/agent-ab

# plan only (prints sessions per repo, projected cost and range)
/tmp/agent-ab -repo famous -stacklit /tmp/stacklit -dry-run
/tmp/agent-ab -repo famous -mode memory -stacklit /tmp/stacklit -dry-run

# after approval: memory check, stage A, then (rule permitting) stage B, then the analysis
/tmp/agent-ab -repo famous -mode memory -stacklit /tmp/stacklit -yes-spend
/tmp/agent-ab -repo famous -trials 2 -stacklit /tmp/stacklit -yes-spend
/tmp/agent-ab -repo famous -trials 3 -stacklit /tmp/stacklit -yes-spend
/tmp/agent-ab -repo famous -analyze
```

`-repo famous` selects `repos-famous.json`, `tasks-famous/`, the three arms, the model, seed 20261109, 0.25 USD per session, the 20 USD cap, the audited grader, index reuse and memorised-task exclusion; any flag given explicitly wins. Runs are resumable: finished sessions are skipped.

## Approval

Not given. This section is filled in by the owner before the first paid session: date, approved spend cap, and the commit of the stacklit binary to build. Any change to this document after that point is written below as a dated amendment before the affected sessions run, and amendments never change the decision rule after data exists.
