# Round 3 results: does quiet repo mode remove the cost below 5,000 files?

Pre-registration: `PREREGISTRATION-round3.md`. Raw records: `results-round3/`; ledger, summary and tables: `round3/`. Model claude-sonnet-5-5, 300 sessions (control 150, stacklit-quiet 150), 3 repos, 10 tasks each (the round 2 tasks), 5 trials. Notional spend 5.95 USD of the 15 USD cap. No harness failures, rate limits or stopped runs. Bootstrap seed 20260508, 10,000 resamples, paired by task and trial.

## Verdict

Quiet mode does not pass. Q1 fails and Q3 fails on theauth. Q2 passes. No claim of "no regression below 5,000 files" is supported, and the default quiet limit is not shown to be right.

| Id | Criterion | Result | Outcome |
|---|---|---|---|
| Q1 | Pooled cost ratio, upper bound below 1.05 | 1.08 [1.05, 1.11] | fail |
| Q2 | Correctness difference, lower bound above -5 points | 100% in both arms (150/150) | pass |
| Q3 | Per repo cost ratio, upper bound below 1.15 | theauth-go 1.04 [1.00, 1.09], featuredrop 1.07 [1.02, 1.11], theauth 1.13 [1.09, 1.18] | fail (theauth, 1.18) |

## Other metrics, quiet against control (ratio, pooled, 150 pairs)

| Metric | control | stacklit-quiet | ratio [95% CI] |
|---|---|---|---|
| cost (USD) | 0.0191 | 0.0206 | 1.08 [1.05, 1.11] |
| total tokens | 16,925 | 21,710 | 1.28 [1.23, 1.34] |
| uncached-equivalent tokens | 5,252 | 5,974 | 1.14 [1.11, 1.17] |
| turns | 3.19 | 3.09 | 0.97 [0.92, 1.01] |
| tool calls | 2.19 | 2.09 | 0.95 [0.89, 1.02] |

The agent ran zero stacklit commands in 150 of 150 treatment sessions, and the session hook injected nothing. So the extra tokens do not come from stacklit output or from the hook text. They come from what the arm still carries: the one line rules block that `setup` writes, and the Bash tool the arm is given (control has Read, Grep and Glob only). This round cannot tell which of the two causes the cost. That is a hypothesis, not a finding.

## Compared with round 2 (stacklit-e against control, different sessions)

theauth-go 1.05 to 1.04, featuredrop 1.21 to 1.07, theauth 1.42 to 1.13. Quiet mode removed most of the extra cost but not all of it. These are separate runs, so the comparison is indicative only.

## Deviations and limits

- The analyzer did not know the arm name `stacklit-quiet`, so its first pass wrote empty tables. `treatmentArms` in `analyze.go` was extended and the analysis re-run on the same ledger. No data changed.
- The tasks are the round 2 tasks, run again. featuredrop was contaminated in round 2's memorisation check (3 of 6 right with tools off). Without featuredrop the pooled ratio is not computed here.
- Correctness is at ceiling (100% in both arms), so Q2 cannot detect a small loss.
- One model, one machine, tasks written by us on repos we own, notional cost.

## What this means for the default

Leave quiet mode in, since it cut the cost, but do not describe it as free. The next step is a diagnostic arm that separates the two causes: control plus the Bash tool with no stacklit setup, and quiet with no rules block. It needs its own pre-registration and approval.
