# Proposed claims from the real-agent A/B

Source: [RESULTS.md](RESULTS.md) and [summary.json](summary.json). Model claude-sonnet-5-5, 3 repos, 30 tasks, 5 trials, 150 pairs per arm, bootstrap 95 percent intervals. Each claim names the cell it comes from. Wording is meant to be copied as is.

## Claims the data supports

1. "In a measured A/B with real Claude Code sessions on three repos, sessions with Stacklit used about a third fewer tool calls than sessions without it (1.77 vs 2.58 per session, ratio 0.68, 95% CI 0.61 to 0.77)." Cell: pooled, stacklit, tool_calls.
2. "They also took about a quarter fewer model turns (2.77 vs 3.58, ratio 0.77, CI 0.71 to 0.84)." Cell: pooled, stacklit, turns.
3. "Grep and Glob calls fell by more than four fifths (2.48 to 0.45 per session, ratio 0.18, CI 0.14 to 0.23)." Cell: pooled, stacklit, grep_glob_calls.
4. "On the largest repo (about 16,000 files) notional cost per session fell 22 percent (ratio 0.78, CI 0.70 to 0.86) and uncached-equivalent tokens fell 18 percent (0.82, CI 0.73 to 0.92)." Cell: levelrail, stacklit, cost_usd and uncached_eq_tokens. One repo, 50 pairs.
5. "For locate and add-feature questions, tool calls fell to about 0.4x (0.43 [0.38, 0.48] and 0.41 [0.35, 0.48]) and cost fell 12 to 22 percent (0.88 [0.79, 0.98] and 0.78 [0.68, 0.90])." Cell: pooled by kind, stacklit.
6. "Stacklit commands were used in every treatment session (150 of 150), and the session hook fired in every one." Cell: stacklit_used_in_treatment.
7. "The models had not memorised these repos: with all tools off they got 0 of 18 tasks right." Cell: memorisation check.

## Claims the data does NOT support (do not make them)

1. That Stacklit lowers cost or tokens in general. Pooled cost ratio is 1.00 [0.93, 1.07] and uncached-equivalent tokens 1.03 [0.97, 1.11]: no saving. On the small repo featuredrop it cost more (1.23 [1.14, 1.33]), and on theauth-go the interval is 0.98 to 1.25.
2. That Stacklit improves answer quality. Correctness was 94.0% control vs 88.0% stacklit (pre-registered grader, difference -6.0 points [-10.7, -1.3]) and 98.0% vs 95.3% after audited grading (difference -3.0 points, interval includes zero). Trace tasks were worse (97.5% to 82.5%).
3. That Stacklit reduces files read. Control sessions barely opened files (0.1 per session), so there was nothing to reduce; the measured drop to 0 is not a meaningful saving.
4. That Stacklit helps "who imports X" (impact) questions. They cost 1.59x [1.47, 1.72] with the plain arm because one exact grep is enough without it.
5. Anything about the strict arm (crawl guard redirect plus steering d) being better. It cost more (1.12 [1.02, 1.24] in dollars, 1.37 [1.18, 1.59] in total tokens) and was less often right (-11.3 points [-17.3, -6.0]; audited -9 points [-14, -3]). The guard fired in only 8 of 150 sessions, so this result is about the stronger rules text, not mainly about redirects.
6. Any latency claim. The `seconds` metric includes clone and `stacklit generate`.
7. Any claim on editing tasks, other models, other repos, harder multi-file reasoning tasks, or membership usage (the dollars are notional list-price figures).
8. That the numbers are final for the strict arm: it was not interleaved with control.

## Round 2 update (650 more sessions, see RESULTS-round2.md)

Supported now, in addition to the above:

- On the 16,000 file repo, Stacklit cut notional cost by 11% (steering a) to 13% (steering e, ratio 0.87 [0.81, 0.93]) in a second set of new tasks, at equal correctness.
- Across four repos, Stacklit sessions made about 24% fewer tool calls (0.76) and 17% fewer turns, and far fewer Grep and Glob calls (0.22 with wording a, 0.53 with the routed wording e).
- Routed wording e costs less than the current wording a (0.93 [0.90, 0.95]) with the same correctness.

Not supported (do not claim):

- Any overall saving. Pooled cost of e against no Stacklit is 1.09 [1.04, 1.14]; wording a is 1.17 [1.11, 1.24].
- Savings on repos under about 3,000 files (1.05 to 1.61 depending on repo and wording), or on "who imports X" questions there.
- That e fixed the round 1 trace correctness loss: control scored 100% on trace in round 2, so that test was at the ceiling.
- That small repo mode is cost neutral: on featuredrop it is 1.11 [1.06, 1.16] against no Stacklit.
- Better answer quality: correctness is equal (97.5% against 97.5%) and tasks were mostly easy.
