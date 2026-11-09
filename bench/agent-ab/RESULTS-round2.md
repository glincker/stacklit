# Round 2 results: does the routed steering (variant e) fix what round 1 found?

Pre-registration: `tasks-round2/PREREGISTRATION-round2.md`. Tracking issue glincker/stacklit#78. Raw records: `results-round2/`, ledger, summary, full tables and the decision report: `round2/` (`ledger.jsonl`, `summary.json`, `tables.md`, `decision-round2.md`). Model claude-sonnet-5-5 in all 650 sessions. Bootstrap seed 20260508, 10,000 resamples, paired by task and trial.

## Verdict

Steering `e` does not become the default. Two of the six pre-registered criteria fail (P1, G2), and the small repo mode criterion S1 fails by 0.01. The default stays `a`. `e` is clearly better than `a` (cost ratio 0.93 [0.90, 0.95]) but still costs more than no stacklit on three of the four repos. No savings claim is supported: the pooled cost ratio of `e` against control is 1.09 [1.04, 1.14], upper bound well above 1.00.

## Method and what ran

| Item | Value |
|---|---|
| Sessions | 650 main (control 200, stacklit 200, stacklit-e 200, stacklit-e-small 50) plus 24 memorisation sessions |
| Repos | levelrail (15,978 files), theauth-go (1,294), featuredrop (412), theauth (2,919, new) |
| Tasks | 40 new tasks (13 locate, 11 trace, 8 impact, 8 add), 5 trials per task and arm |
| Binary | built from master at 533ea24, SHA-256 prefix 692502a851ca in every record |
| Spend (notional) | round 2: 15.77 USD (ledger of 674 rows: 24 memory, 2 smoke, 648 main); round 1: 12.54 USD; grand total 28.31 USD, under the 45 USD stop and the 30 USD round cap |
| Harness failures | none; no rate, usage or auth errors; no session hit the 0.25 USD or 25 turn cap (max 0.071 USD); 0 non-success results; 3 permission denials in total |

Mean notional cost per session: control 0.0223, stacklit 0.0261, stacklit-e 0.0242, stacklit-e-small 0.0199 USD.

## Deviations and notes

- The pre-registration file lives at `tasks-round2/PREREGISTRATION-round2.md`. Approval and a dated amendment (binary from master, `-work` dir) were committed before any paid session.
- Two smoke sessions (featuredrop fd2-01 trial 1, control and stacklit-e) were run into a scratch results folder and then copied into `results-round2/`. They have the same configuration as the main run, so they count as those sessions and were not paid twice. The ledger has 674 rows: 24 memory, 650 main.
- `-analyze` writes next to the ledger, so the round 2 ledger was moved to `round2/ledger.jsonl` to keep round 1's `summary.json` and `tables.md` intact.
- Memorisation check (below) found featuredrop contaminated. The plan says to exclude it from the headline and also report all four. `decide.go` has no exclusion switch, so the exclusion figures below were computed with the same paired bootstrap by a separate script and are labelled sensitivity. The decision rule was not edited.
- Two grading defects were found after the run (see Correctness). They are reported as a separate column and are not used by the decision rule.

## Memorisation check (all tools off, 6 tasks per repo, 1 trial)

| Repo | Correct | Rate | Status |
|---|---|---|---|
| levelrail | 0/6 | 0% | clean |
| theauth-go | 1/6 | 17% | clean (below 20%) |
| featuredrop | 3/6 | 50% | contaminated (excluded from the headline) |
| theauth | 0/6 | 0% | clean |

The featuredrop hits are locate tasks whose answer file name is guessable from the library's own vocabulary (for example `src/rss.ts`, `src/dependencies.ts`). So the featuredrop control numbers are partly guesswork-assisted, which favours control (cheap one grep answers). The main run was done on all four repos as planned.

## Headline table (cost ratio arm / control, 95% interval)

| Scope | n pairs | stacklit (a) | stacklit-e | e / a |
|---|---|---|---|---|
| levelrail | 50 | 0.89 [0.84, 0.96] | 0.87 [0.81, 0.93] | |
| theauth-go | 50 | 1.10 [0.98, 1.24] | 1.05 [0.95, 1.16] | |
| featuredrop (contaminated) | 50 | 1.37 [1.27, 1.47] | 1.21 [1.14, 1.29] | |
| theauth | 50 | 1.61 [1.53, 1.69] | 1.42 [1.36, 1.48] | |
| pooled, all four | 200 | 1.17 [1.11, 1.24] | 1.09 [1.04, 1.14] | 0.93 [0.90, 0.95] |
| pooled, without featuredrop (sensitivity) | 150 | not computed | 1.05 [1.00, 1.11] | 0.94 [0.90, 0.97] |

Small repo mode on featuredrop (`stacklit-e-small` against control): cost 1.11 [1.06, 1.16], correctness 100% in both arms. The agent ran zero stacklit commands in 0/50 sessions (the hook only carries the one line note), so the saving over `stacklit-e` (1.21 to 1.11) comes from removing text and calls, but it still does not reach parity.

## By task kind, stacklit-e against control (cost ratio)

| Kind | pooled (n) | levelrail | theauth-go | featuredrop | theauth |
|---|---|---|---|---|---|
| locate | 0.91 [0.84, 0.99] (65) | 0.81 | 0.79 | 1.02 | 1.34 |
| add | 1.16 [1.08, 1.25] (40) | 0.92 | 1.30 | 1.22 | 1.32 |
| impact | 1.23 [1.16, 1.31] (40) | 0.94 | 1.43 | 1.30 | 1.41 |
| trace | 1.22 [1.12, 1.33] (55) | 0.87 | 1.42 | 1.43 | 1.59 |

Against the current default wording `a`, the e / a cost ratios by kind are add 0.91, impact 0.77, locate 0.98, trace 1.00, so the gain of `e` over `a` is on impact and add questions, as designed. Sensitivity without featuredrop: impact 1.21 [1.12, 1.31], trace 1.16 [1.05, 1.29].

Where it works: levelrail (16,000 files) is below 1.0 for every kind. Where it does not: the three repos of 400 to 3,000 files, on every kind except locate, because control already answers in 1 to 3 greps and each stacklit call adds a few thousand cached tokens of hook text and a tool turn.

## Tool calls, turns and tokens (pooled, 200 pairs)

| Metric | control | stacklit (a) | stacklit-e | e / control |
|---|---|---|---|---|
| tool calls | 2.33 | 1.76 | 1.77 | 0.76 [0.70, 0.83] |
| turns | 3.33 | 2.76 | 2.77 | 0.83 [0.79, 0.89] |
| Grep + Glob calls | 2.27 | 0.50 | 1.20 | 0.53 [0.46, 0.60] |
| uncached-equivalent tokens | 6167 | 7524 | 6875 | 1.11 [1.06, 1.17] |
| cost, USD | 0.0223 | 0.0261 | 0.0242 | 1.09 [1.04, 1.14] |

Sessions with at least one `stacklit` command: `a` 200/200, `e` 76/200 (levelrail 19/50, theauth-go 31/50, theauth 15/50, featuredrop 11/50). Variant `e` does what it was written to do, it tells the agent to skip stacklit for simple lookups, and the agent obeys, but the hook text itself still costs tokens on every session. Fewer tool calls and turns do not translate into lower cost on small repos.

Wall seconds include clone and setup; levelrail stacklit-e 23.5 s against 9.6 s control (the index step), not model latency.

## Correctness

Pre-registered (and audited, identical here) grader, pooled: control 195/200 = 97.5% [94.3, 98.9], stacklit 194/200 = 97.0%, stacklit-e 195/200 = 97.5%. Difference e minus control +0.0 points [-1.5, +1.5]. Trace questions: 100% in both control and e (round 1 had 97.5% to 82.5%, so the harm did not reappear, but control is at the ceiling in round 2, so the test was weak: G1 passes with a zero-width interval, which is true but uninformative).

Hand check: 65 sessions (10%, seeded random, seed 20261108) read against the expected files, plus all 16 failures read. No false pass or false fail in the sample. Limitation seen: the matcher does not penalise listing many files, so long lists on impact questions pass.

The 16 failures:

- 12 are task lr2-02 (levelrail, "where does the platform keep a reserve file on disk"). The question is ambiguous: agents answered with the data path (`<dataDir>/.disk-reserve`) and not the Go file. Failing across all arms (control 5, stacklit 3, stacklit-e 4). A task wording defect.
- 1 is theauth ta-10 (stacklit-e): the answer named `packages/core/src/i18n/locales` without a trailing slash, which the matcher does not accept for a directory entry. A matcher defect.
- 3 are theauth ta-04 (stacklit a): the agent named `audit/erasure.ts` in place of `gdpr-anon.ts`, a real error (the same wrong file three times).

Defect-adjusted (lr2-02 removed, ta-10 accepted), not used by the rule: control 195/195, stacklit 192/195, stacklit-e 195/195, stacklit-e-small 50/50. The conclusions do not change.

## Decision rule (as written, nothing edited)

| Id | Criterion | Observed | Rule | Result |
|---|---|---|---|---|
| P1 | pooled cost ratio e / control, upper bound | 1.09 [1.04, 1.14] | upper < 1.05 | FAIL |
| P2 | pooled correctness difference e minus control, lower bound | +0.0 points [-1.5, +1.5] | lower > -5 | pass |
| P3 | levelrail cost ratio e / control, point | 0.87 [0.81, 0.93] | point < 1.00 | pass |
| P4 | pooled cost ratio e / a, upper bound | 0.93 [0.90, 0.95] | upper < 1.05 | pass |
| G1 | trace correctness e minus control, lower bound | +0.0 points [+0.0, +0.0] | lower > -10 | pass |
| G2 | impact cost ratio e / control, upper bound | 1.23 [1.16, 1.31] | upper < 1.15 | FAIL |
| S1 | featuredrop small repo mode: cost ratio upper bound; correctness lower bound | 1.11 [1.06, 1.16]; +0.0 [+0.0, +0.0] | upper < 1.15; lower > -10 | FAIL (cost bound 1.16 against 1.15) |

Outcome: default stays `a`, `e` stays opt-in. By the plan's S1 text, small repo mode would go to limit 0 (off), a product decision for the owner; the miss is 0.01 and the arm is cheaper than `e` without it (1.11 against 1.21), so it is the better of the two featuredrop treatments. Excluding the contaminated featuredrop repo does not change any outcome (P1 upper 1.11, G2 upper 1.31).

## What helped and what did not

- Helped: variant `e` against `a` (0.93 [0.90, 0.95], all of it on impact 0.77 and add 0.91); the flow line and importer lists removed round 1's trace correctness loss and impact harm on large repos; levelrail is a consistent saving of 13% [7%, 19%] with `e` (89% for `a`); fewer tool calls (0.76) and Grep and Glob calls (0.53 with `e`, 0.22 with `a`).
- Did not help: any repo under about 3,000 files, impact and trace questions there (+20 to +60% cost), and theauth (1,921 source files), where `e` costs 1.42 although 70% of sessions skipped stacklit, so the fixed hook text is itself the cost. The likely cause is the fixed hook text read on every turn (not tested here).
- Not tested: round 2 control reached 100% on trace, so the trace correctness harm of round 1 cannot be confirmed fixed or unfixed; only that no harm appeared on these tasks.

## Limits

One model, one day, one machine; tasks written by us on repos we own; one repo contaminated by memorisation; control at the correctness ceiling; featuredrop and theauth have many short locate answers; notional cost is not membership usage; path-match grading that ignores extra files; by-kind rows rest on 40 to 65 pairs; the fourth repo is below the 5,000 file threshold so P3 rests on levelrail alone.
