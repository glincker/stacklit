# Pre-registration, round 3: does quiet repo mode remove the cost on repos below 5,000 files?

Written and committed before any round 3 session ran. Spend approval is recorded under "Approval". Branch: `feat/gate-and-trust`.

## Why

Round 2 (`RESULTS-round2.md`) found Stacklit cost more than no Stacklit on the three repos below 5,000 files: stacklit-e against control was 1.05 on theauth-go, 1.21 on featuredrop and 1.42 on theauth. It only paid off on levelrail (15,978 files, 0.87). Quiet repo mode (default `quiet_repo_files` 5000) makes the session and subagent hooks inject nothing below that size. This round asks one question: with quiet mode on, does a session on those repos cost the same as one without Stacklit?

## What this round cannot answer

Where the break-even sits between 2,919 and 15,978 files. No repo we own or can use here has between 3,000 and 15,000 source files (see the repo notes in `tasks-round2/PREREGISTRATION-round2.md`), so the 5000 default stays an unmeasured guess above 2,919 files. Famous public repos would fill the gap but need tasks on code newer than the model's knowledge (issue #83) and a separate approval.

## Arms

| Arm | Setup | Environment |
|---|---|---|
| `control` | nothing added; tools Read, Grep, Glob | none |
| `stacklit-quiet` | `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`; Bash only as `Bash(stacklit *)` | `STACKLIT_STEERING=e`, `STACKLIT_QUIET_REPO_FILES=5000`, `STACKLIT_SMALL_REPO_FILES=600` |

The binary is built from the head of `feat/gate-and-trust`; its SHA-256 prefix is in every session record. Session settings are those of round 2.

## Repos and tasks

theauth-go (1,294 files), featuredrop (412) and theauth (2,919), at the round 2 SHAs in `repos-round2.json`. levelrail is left out because it is above the quiet limit, so the arm would equal `stacklit-e`, already measured. The tasks are the round 2 tasks (`tasks-round2/`, 10 per repo, 30 in all). They are not new to the model's harness but were not used to tune quiet mode, which does not touch task content. Round 2 found featuredrop contaminated (3 of 6 right with tools off), so every result is reported on all three repos and without featuredrop.

## Size, cost and caps

2 arms x 30 tasks x 5 trials = 300 sessions. Projection at 0.025 USD per session (round 2 mean 0.022 to 0.026): 7.50 USD notional. Caps: 0.25 USD per session, 25 turns, `--cap-usd 15` for the ledger. Stop at once on a rate limit, usage limit or authentication error; stop after 10 consecutive harness failures. No interim analysis and no stopping for efficacy. If the run stops early, the decision rule is not evaluated. Order is shuffled with seed 20261109. Bootstrap seed 20260508, 10,000 resamples, paired by task and trial. No new memorisation check: the round 2 check covers these repos and tasks.

## Grading

The round 2 matcher, unchanged. 10 percent of transcripts are checked by hand. A defect found afterwards is reported as a separate column and is not used by the decision rule.

## Decision rule (fixed before seeing data)

Quiet mode counts as "no regression below 5,000 files" if and only if all hold, pooled over the three repos (150 pairs):

| Id | Criterion |
|---|---|
| Q1 | Cost ratio `stacklit-quiet` / `control`, upper 95% bound below 1.05 |
| Q2 | Correctness difference (`stacklit-quiet` minus `control`), lower 95% bound above -5 points |
| Q3 | On each repo, cost ratio upper 95% bound below 1.15 |

If any fails, the default quiet limit is wrong for that size and the result says which repo. If all pass, the supported claim is only: "on three repos of 412 to 2,919 files, sessions with quiet-mode Stacklit cost no more than sessions without it". It is not a savings claim, and it says nothing about repos between 3,000 and 15,000 files.

## Limits

One model, one machine, tasks written by us on repos we own, path-match correctness, notional cost, small n per cell, round 2 tasks reused. The setup still writes a one line note into CLAUDE.md in quiet repos, so this tests quiet mode as shipped and not a fully silent install.

## Run

```bash
go build -o /tmp/stacklit ./cmd/stacklit && go build -o /tmp/agent-ab ./bench/agent-ab
R3="-repos bench/agent-ab/repos-round2.json -tasks bench/agent-ab/tasks-round2 -results bench/agent-ab/results-round3 -ledger bench/agent-ab/round3/ledger.jsonl -stacklit /tmp/stacklit -model claude-sonnet-5-5 -seed 20261109 -max-budget-usd 0.25 -cap-usd 15"
/tmp/agent-ab -repo theauth-go,featuredrop,theauth $R3 -arms control,stacklit-quiet -trials 5 -dry-run
/tmp/agent-ab -repo theauth-go,featuredrop,theauth $R3 -arms control,stacklit-quiet -trials 5 -yes-spend
/tmp/agent-ab -analyze $R3
```

## Approval

The owner replied "ok" on 2026-11-08 to a proposed cap of 15 USD notional for a re-measure of the gate. This round spends at most that. The round 3 run starts only after this file and the harness change are committed.
