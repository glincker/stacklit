# Real-agent A/B results

Date: 2026-11-08. Model: `claude-sonnet-5-5` (the id the CLI reports for the `sonnet` alias). Pre-registration: [PREREGISTRATION.md](PREREGISTRATION.md), committed before the first paid session, with one amendment (written into it) made after the 3-trial checkpoint and before the extension. Machine-readable numbers: [summary.json](summary.json). Every table for every repo is in [tables.md](tables.md); raw per-session JSON is in `results/`; the cost ledger is [ledger.jsonl](ledger.jsonl).

## Headline

Stacklit did not make these sessions cheaper, and on this task set it made the answers slightly less often right. It did cut tool calls and turns by about a quarter to a third, and it nearly removed Grep and Glob use. Sessions here were already cheap (about 0.026 USD, 3.6 turns), because the questions can be answered with one or two greps on code the model has not seen.

Pooled over the three repos, 150 paired sessions per arm (10 tasks x 5 trials x 3 repos), stacklit arm vs control, 95 percent bootstrap intervals:

| Metric | Control | Stacklit | Ratio stacklit / control [95% CI] |
|---|---|---|---|
| Notional cost, USD | 0.0264 | 0.0263 | 1.00 [0.93, 1.07] |
| Total tokens (all four kinds) | 22,967 | 23,100 | 1.01 [0.92, 1.10] |
| Uncached-equivalent tokens | 7,266 | 7,518 | 1.03 [0.97, 1.11] |
| Turns | 3.58 | 2.77 | 0.77 [0.71, 0.84] |
| Tool calls | 2.58 | 1.77 | 0.68 [0.61, 0.77] |
| Grep + Glob calls | 2.48 | 0.45 | 0.18 [0.14, 0.23] |
| Files opened with Read | 0.10 | 0.00 | not meaningful (control mean is 0.1) |
| Correct, pre-registered grader | 94.0% | 88.0% | difference -6.0 points [-10.7, -1.3] |
| Correct, audited grader | 98.0% | 95.3% | difference -3.0 points [-7, +1] |

Strict arm (crawl guard in redirect mode plus steering variant d, 150 sessions, binary from `origin/feat/source-of-truth` at fdc0f40) vs control:

| Metric | Control | Strict | Ratio strict / control [95% CI] |
|---|---|---|---|
| Notional cost, USD | 0.0264 | 0.0296 | 1.12 [1.02, 1.24] |
| Total tokens | 22,967 | 31,395 | 1.37 [1.18, 1.59] |
| Uncached-equivalent tokens | 7,266 | 8,818 | 1.21 [1.09, 1.35] |
| Turns | 3.58 | 3.23 | 0.90 [0.80, 1.03] |
| Tool calls | 2.58 | 2.23 | 0.87 [0.72, 1.04] |
| Grep + Glob calls | 2.48 | 0.18 | 0.07 [0.04, 0.12] |
| Correct, pre-registered grader | 94.0% | 82.7% | difference -11.3 points [-17.3, -6.0] |
| Correct, audited grader | 98.0% | 89.3% | difference -9 points [-14, -3] |

The strict arm did worse than the plain stacklit arm on cost, tokens and correctness. The crawl guard itself fired in only 8 of 150 sessions (17 redirects, all on `grep -r` style Bash calls), so most of the difference comes from the stronger rules text and the agent routing its search through `stacklit` commands instead of Grep.

## Where Stacklit helped and where it did not

Pooled stacklit arm vs control by task kind (per-repo tables with intervals are in [tables.md](tables.md)):

| Kind (n pairs) | Cost ratio [95% CI] | Tool call ratio [95% CI] | Correct, control / stacklit |
|---|---|---|---|
| add, where to put a new thing (30) | 0.78 [0.68, 0.90] | 0.41 [0.35, 0.48] | 83% / 83% (audited 100% / 100%) |
| locate (50) | 0.88 [0.79, 0.98] | 0.43 [0.38, 0.48] | 94% / 94% (audited 96% / 100%) |
| trace, a set of paths (40) | 1.05 [0.96, 1.17] | 0.99 [0.84, 1.18] | 97.5% / 82.5% |
| impact, who imports X (30) | 1.59 [1.47, 1.72] | 1.89 [1.70, 2.09] | 100% / 90% (audited 100% / 100%) |

- Helped: add-feature placement and locate questions. Fewer tool calls (about 0.4x) and 12 to 22 percent lower cost.
- No effect on cost: trace questions.
- Hurt: impact questions. A single exact-string Grep answers "who imports X" in one call (1.2 calls on average). The stacklit arm runs a `stacklit query` first and then usually greps anyway, so it takes about twice the calls and 1.6x the cost. In the strict arm, which pushes the agent away from grep, impact questions cost 2.5x and were right only 77 percent of the time.
- Correctness losses concentrate in trace tasks (97.5% to 82.5%, difference -15 points [-27.5, -5]). The clearest case is lr-10 (three files for waking a sleeping app): in both treatment arms the agent ran one `stacklit query`, answered at once with `internal/api/app_sleep.go` instead of `internal/api/app_sleep_wake.go`, and was wrong in 5 of 5 trials in each arm; the control arm grepped and was right in 5 of 5.
- By repo, stacklit vs control cost ratio: levelrail 0.78 [0.70, 0.86] (the largest repo, where it helped), theauth-go 1.10 [0.98, 1.25], featuredrop 1.23 [1.14, 1.33] (the smallest repo, where the injected map is mostly overhead).

## Memorisation check

Six tasks per repo, all tools disabled (`--tools ""`), one trial each, 18 sessions. Correct: 0 of 18 (0 of 6 on every repo). Several answers were plausible layout guesses and none matched, so no repo was excluded. This check asks for exact paths, so it shows the model does not know the layout, not that it cannot guess a file name.

## Method

- Repos, pinned by full SHA: glincker/levelrail (Go and TypeScript, about 16,000 files), glincker/theauth-go (Go, about 1,300 files), glincker/featuredrop (TypeScript, about 400 files). All public, pushed Oct to Nov 2026.
- Tasks: 10 per repo in `tasks/`, written by reading the code, answers verified by grep and file existence. Kinds: locate, trace, impact, add. Grading: every `all_of` path and at least one `any_of` path named in the answer, boundary-aware match.
- Arms: control (Read, Grep, Glob only); stacklit (`stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`, plus a Bash tool restricted to `Bash(stacklit *)`); strict (as stacklit, with `--crawl-guard=redirect` and `STACKLIT_STEERING=d`).
- Each session: `claude -p` with `--output-format stream-json --verbose --no-session-persistence --setting-sources project,local --strict-mcp-config --disable-slash-commands --tools ... --max-turns 25 --max-budget-usd 1.5 --model sonnet`, in a fresh shallow clone in a temp dir, sequential, seed-shuffled. `--strict-mcp-config --disable-slash-commands` cut the fixed context of an empty session from about 64,000 to about 5,400 tokens (account connectors and skills were otherwise loaded). I added them after a first trivial test, before any measured session.
- Trials: 5 per task and arm. The pre-registration asked for 3; trials 4 and 5 were added in Amendment 1 because the observed cost (0.03 USD per session) left most of the cap unused.
- Statistics: paired by repo, task and trial; seeded bootstrap (seed 20260508, 10,000 resamples of pairs) for the mean difference and ratio of means; Wilson intervals for rates.
- Sessions: 150 control, 150 stacklit, 150 strict, 18 memory check. The 10 pilot sessions are the first part of the full run (same task, trial and arm), not duplicated. Total notional spend from the ledger: 12.54 USD of the 60 USD cap. No rate limit, usage limit or authentication error occurred, and no session failed for harness reasons.
- Spot check: 45 of 450 sessions (10 percent, seeded random) were compared by hand, expected paths against the answer. Nothing new beyond the three defects below; the known fd-07 defect appeared twice. The false answers I saw (lr-10, fd-05, tg-05) were genuinely wrong or incomplete.

## Grading defects found

The `correct` column is the pre-registered grader. `correct_audited` adds three fixes, applied to both arms identically and decided from the spot check and the failure list: (a) the matcher rejected answers written as `./path` or as an absolute path inside the checkout (this hit the treatment arms more, since they often answer from shell `grep` output), (b) fd-07 asks where to add a new adapter, and the model correctly names a new file in `src/adapters/`, which the pinned any-of list (existing files) did not allow, (c) fd-09: `src/analytics.ts` (PostHogAdapter) also forwards events to PostHog and was missing from the pinned answer. After the fixes the gap between control and stacklit shrinks from -6.0 to -3.0 points, and its interval includes zero. For the strict arm the gap stays clear (-9 points).

## Pooled tables

### pooled: control vs stacklit (150 pairs)

| metric | n | control | stacklit | diff (arm minus control) [95% CI] | ratio (arm / control) [95% CI] |
|---|---|---|---|---|---|
| cost_usd | 150 | 0.0264 | 0.0263 | -0.0001 [-0.0020, 0.0016] | 1.00 [0.93, 1.07] |
| total_tokens | 150 | 22967 | 23100 | 133 [-1875, 2044] | 1.01 [0.92, 1.10] |
| uncached_eq_tokens | 150 | 7266 | 7518 | 251 [-269, 734] | 1.03 [0.97, 1.11] |
| turns | 150 | 3.58 | 2.77 | -0.81 [-1.09, -0.54] | 0.77 [0.71, 0.84] |
| tool_calls | 150 | 2.58 | 1.77 | -0.81 [-1.09, -0.54] | 0.68 [0.61, 0.77] |
| read_calls | 150 | 0.10 | 0.00 | -0.10 [-0.15, -0.05] | 0.00 [0.00, 0.00] |
| grep_glob_calls | 150 | 2.48 | 0.45 | -2.03 [-2.31, -1.76] | 0.18 [0.14, 0.23] |
| files_read | 150 | 0.10 | 0.00 | -0.10 [-0.15, -0.05] | 0.00 [0.00, 0.00] |
| read_kb | 150 | 0.18 | 0.00 | -0.18 [-0.28, -0.09] | 0.00 [0.00, 0.00] |
| seconds | 150 | 8.92 | 19.09 | 10.17 [7.88, 12.62] | 2.14 [1.88, 2.42] |
| correct | 150 | 0.940 | 0.880 | -0.060 [-0.107, -0.013] | 0.94 [0.89, 0.99] |
| correct_audited | 150 | 0.98 | 0.95 | -0.03 [-0.07, 0.01] | 0.97 [0.93, 1.01] |
| recall | 95 | 0.995 | 0.940 | -0.054 [-0.096, -0.021] | 0.95 [0.90, 0.98] |

Correctness after audit fixes: control 147/150 = 98.0% [94.3, 99.3]; stacklit 143/150 = 95.3% [90.7, 97.7].

Correctness (pre-registered grader): control 141/150 = 94.0% [89.0, 96.8]; stacklit 132/150 = 88.0% [81.8, 92.3]. Stacklit commands used in 150/150 = 100.0% [97.5, 100.0] of treatment sessions; session hook fired in 150/150 = 100.0% [97.5, 100.0].

By task kind (pooled):

| kind | metric | n | control | stacklit | diff [95% CI] | ratio [95% CI] |
|---|---|---|---|---|---|---|
| add | cost_usd | 30 | 0.0350 | 0.0273 | -0.0076 [-0.0127, -0.0030] | 0.78 [0.68, 0.90] |
| add | uncached_eq_tokens | 30 | 9683 | 7993 | -1690 [-3067, -394] | 0.83 [0.72, 0.95] |
| add | tool_calls | 30 | 4.37 | 1.80 | -2.57 [-3.23, -2.00] | 0.41 [0.35, 0.48] |
| add | files_read | 30 | 0.33 | 0.00 | -0.33 [-0.50, -0.17] | 0.00 [0.00, 0.00] |
| add | correct | 30 | 0.833 | 0.833 | 0.000 [0.000, 0.000] | 1.00 [1.00, 1.00] |
| add | correct_audited | 30 | 1.00 | 1.00 | 0.00 [0.00, 0.00] | 1.00 [1.00, 1.00] |
| impact | cost_usd | 30 | 0.0174 | 0.0277 | 0.0103 [0.0089, 0.0116] | 1.59 [1.47, 1.72] |
| impact | uncached_eq_tokens | 30 | 4720 | 7691 | 2971 [2621, 3301] | 1.63 [1.52, 1.75] |
| impact | tool_calls | 30 | 1.17 | 2.20 | 1.03 [0.90, 1.17] | 1.89 [1.70, 2.09] |
| impact | files_read | 30 | 0.00 | 0.00 | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] |
| impact | correct | 30 | 1.000 | 0.900 | -0.100 [-0.233, 0.000] | 0.90 [0.77, 1.00] |
| impact | correct_audited | 30 | 1.00 | 1.00 | 0.00 [0.00, 0.00] | 1.00 [1.00, 1.00] |
| locate | cost_usd | 50 | 0.0266 | 0.0235 | -0.0031 [-0.0061, -0.0004] | 0.88 [0.79, 0.98] |
| locate | uncached_eq_tokens | 50 | 7419 | 6740 | -680 [-1570, 81] | 0.91 [0.81, 1.01] |
| locate | tool_calls | 50 | 2.62 | 1.12 | -1.50 [-1.74, -1.26] | 0.43 [0.38, 0.48] |
| locate | files_read | 50 | 0.02 | 0.00 | -0.02 [-0.06, 0.00] | 0.00 [0.00, 0.00] |
| locate | correct | 50 | 0.940 | 0.940 | 0.000 [-0.080, 0.080] | 1.00 [0.92, 1.09] |
| locate | correct_audited | 50 | 0.96 | 1.00 | 0.04 [0.00, 0.10] | 1.04 [1.00, 1.11] |
| trace | cost_usd | 40 | 0.0265 | 0.0278 | 0.0014 [-0.0013, 0.0039] | 1.05 [0.96, 1.17] |
| trace | uncached_eq_tokens | 40 | 7173 | 8004 | 831 [157, 1467] | 1.12 [1.02, 1.22] |
| trace | tool_calls | 40 | 2.25 | 2.23 | -0.02 [-0.40, 0.35] | 0.99 [0.84, 1.18] |
| trace | files_read | 40 | 0.10 | 0.00 | -0.10 [-0.20, -0.03] | 0.00 [0.00, 0.00] |
| trace | correct | 40 | 0.975 | 0.825 | -0.150 [-0.275, -0.050] | 0.85 [0.72, 0.95] |
| trace | correct_audited | 40 | 0.97 | 0.82 | -0.15 [-0.28, -0.05] | 0.85 [0.72, 0.95] |

### pooled: control vs strict (150 pairs)

| metric | n | control | strict | diff (arm minus control) [95% CI] | ratio (arm / control) [95% CI] |
|---|---|---|---|---|---|
| cost_usd | 150 | 0.0264 | 0.0296 | 0.0032 [0.0005, 0.0059] | 1.12 [1.02, 1.24] |
| total_tokens | 150 | 22967 | 31395 | 8429 [4276, 12900] | 1.37 [1.18, 1.59] |
| uncached_eq_tokens | 150 | 7266 | 8818 | 1551 [715, 2384] | 1.21 [1.09, 1.35] |
| turns | 150 | 3.58 | 3.23 | -0.35 [-0.75, 0.09] | 0.90 [0.80, 1.03] |
| tool_calls | 150 | 2.58 | 2.23 | -0.35 [-0.76, 0.09] | 0.87 [0.72, 1.04] |
| read_calls | 150 | 0.10 | 0.01 | -0.09 [-0.13, -0.05] | 0.13 [0.00, 0.33] |
| grep_glob_calls | 150 | 2.48 | 0.18 | -2.30 [-2.57, -2.03] | 0.07 [0.04, 0.12] |
| files_read | 150 | 0.10 | 0.01 | -0.09 [-0.13, -0.05] | 0.13 [0.00, 0.33] |
| read_kb | 150 | 0.18 | 0.02 | -0.15 [-0.25, -0.07] | 0.14 [0.00, 0.36] |
| seconds | 150 | 8.92 | 12.87 | 3.95 [2.63, 5.29] | 1.44 [1.29, 1.61] |
| correct | 150 | 0.940 | 0.827 | -0.113 [-0.173, -0.060] | 0.88 [0.82, 0.94] |
| correct_audited | 150 | 0.98 | 0.89 | -0.09 [-0.14, -0.03] | 0.91 [0.86, 0.97] |
| recall | 95 | 0.995 | 0.940 | -0.055 [-0.087, -0.028] | 0.94 [0.91, 0.97] |

Correctness after audit fixes: control 147/150 = 98.0% [94.3, 99.3]; strict 134/150 = 89.3% [83.4, 93.3].

Correctness (pre-registered grader): control 141/150 = 94.0% [89.0, 96.8]; strict 124/150 = 82.7% [75.8, 87.9]. Stacklit commands used in 150/150 = 100.0% [97.5, 100.0] of treatment sessions; session hook fired in 150/150 = 100.0% [97.5, 100.0].

By task kind (pooled):

| kind | metric | n | control | strict | diff [95% CI] | ratio [95% CI] |
|---|---|---|---|---|---|---|
| add | cost_usd | 30 | 0.0350 | 0.0288 | -0.0062 [-0.0117, -0.0003] | 0.82 [0.70, 0.99] |
| add | uncached_eq_tokens | 30 | 9683 | 8698 | -985 [-2722, 872] | 0.90 [0.75, 1.10] |
| add | tool_calls | 30 | 4.37 | 2.43 | -1.93 [-2.87, -0.77] | 0.56 [0.40, 0.82] |
| add | files_read | 30 | 0.33 | 0.07 | -0.27 [-0.43, -0.13] | 0.20 [0.00, 0.50] |
| add | correct | 30 | 0.833 | 0.833 | 0.000 [0.000, 0.000] | 1.00 [1.00, 1.00] |
| add | correct_audited | 30 | 1.00 | 1.00 | 0.00 [0.00, 0.00] | 1.00 [1.00, 1.00] |
| impact | cost_usd | 30 | 0.0174 | 0.0435 | 0.0261 [0.0220, 0.0305] | 2.50 [2.24, 2.78] |
| impact | uncached_eq_tokens | 30 | 4720 | 13452 | 8732 [7317, 10361] | 2.85 [2.53, 3.20] |
| impact | tool_calls | 30 | 1.17 | 4.47 | 3.30 [2.60, 4.07] | 3.83 [3.08, 4.71] |
| impact | files_read | 30 | 0.00 | 0.00 | 0.00 [0.00, 0.00] | 0.00 [0.00, 0.00] |
| impact | correct | 30 | 1.000 | 0.767 | -0.233 [-0.400, -0.100] | 0.77 [0.60, 0.90] |
| impact | correct_audited | 30 | 1.00 | 0.77 | -0.23 [-0.40, -0.10] | 0.77 [0.60, 0.90] |
| locate | cost_usd | 50 | 0.0266 | 0.0239 | -0.0027 [-0.0059, 0.0001] | 0.90 [0.80, 1.00] |
| locate | uncached_eq_tokens | 50 | 7419 | 6904 | -516 [-1442, 340] | 0.93 [0.82, 1.05] |
| locate | tool_calls | 50 | 2.62 | 1.26 | -1.36 [-1.68, -1.04] | 0.48 [0.39, 0.59] |
| locate | files_read | 50 | 0.02 | 0.00 | -0.02 [-0.06, 0.00] | 0.00 [0.00, 0.00] |
| locate | correct | 50 | 0.940 | 0.900 | -0.040 [-0.140, 0.060] | 0.96 [0.85, 1.07] |
| locate | correct_audited | 50 | 0.96 | 1.00 | 0.04 [0.00, 0.10] | 1.04 [1.00, 1.11] |
| trace | cost_usd | 40 | 0.0265 | 0.0270 | 0.0006 [-0.0021, 0.0031] | 1.02 [0.93, 1.13] |
| trace | uncached_eq_tokens | 40 | 7173 | 7824 | 651 [-129, 1338] | 1.09 [0.98, 1.20] |
| trace | tool_calls | 40 | 2.25 | 1.62 | -0.62 [-1.00, -0.28] | 0.72 [0.60, 0.86] |
| trace | files_read | 40 | 0.10 | 0.00 | -0.10 [-0.20, -0.03] | 0.00 [0.00, 0.00] |
| trace | correct | 40 | 0.975 | 0.775 | -0.200 [-0.325, -0.075] | 0.79 [0.67, 0.92] |
| trace | correct_audited | 40 | 0.97 | 0.78 | -0.20 [-0.33, -0.07] | 0.79 [0.67, 0.92] |

## Limitations

- One model (Sonnet), one day of runs, one machine. Service load can differ across the day; the strict arm ran after control and stacklit and is not interleaved with control, so time-of-day or cache effects are not balanced for that comparison.
- Notional cost is the CLI's list-price figure, not membership usage. Cache reads dominate tokens, so cost and the uncached-equivalent view are both given.
- Tasks are written by us, with a known file answer, from code we know. Ten per repo, three repos, so n is small and the by-kind rows rest on 30 to 50 pairs. Trials of one task are not independent draws of tasks; the intervals understate uncertainty about other tasks.
- These tasks can be answered with one or two exact-string greps, so a map has little room to save exploration. Opening files was almost absent in both arms (0.1 Read calls per control session), so the claim about files read cannot be tested here. Tasks that need reading and reasoning across files, or editing, are not covered.
- Correctness is a path match, not an understanding check. The audited column is a post-hoc view.
- Control has no Bash tool; treatment arms have one limited to `stacklit *`. That is the existing harness design. A control with a shell would grep more flexibly.
- `seconds` includes the clone and, for treatment arms, `stacklit generate` and setup (about 30 seconds on levelrail). It is not model time and must not be quoted as latency.
- The repos ship their own agent guide files, present in both arms, and the user-level CLAUDE.md was loaded in all arms (a few thousand tokens).
- The harness looked for the crawl guard state file in the wrong directory, so redirect counts for the strict arm were taken from the transcripts afterwards (8 sessions, 17 redirects). The harness was fixed after the run.
- Transcripts are not committed. A call log (tool and key argument) per session is in each result JSON.
