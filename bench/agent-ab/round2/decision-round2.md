# Round 2 decision report

Generated from summary.json by the rule in PREREGISTRATION-round2.md. Steering variant e becomes the default only if every row below passes.

| id | criterion | observed | rule | result |
|---|---|---|---|---|
| P1 | pooled cost ratio e / control, upper 95% bound | 1.09 [1.04, 1.14] | upper < 1.05 | FAIL |
| P2 | pooled correctness difference e minus control, lower 95% bound | +0.0 points [-1.5, +1.5] | lower > -5 points | pass |
| G1 | trace questions, correctness difference e minus control, lower bound | +0.0 points [+0.0, +0.0] | lower > -10 points | pass |
| G2 | impact questions, cost ratio e / control, upper bound | 1.23 [1.16, 1.31] | upper < 1.15 | FAIL |
| P3 levelrail | cost ratio e / control on levelrail (15978 files) | 0.87 [0.81, 0.93] | point < 1.00 | pass |
| P4 | pooled cost ratio e / default wording a, upper bound | 0.93 [0.90, 0.95] | upper < 1.05 | pass |

Verdict: at least one criterion fails or has no data, so the default stays a and e stays opt-in.

Small repo mode (decides whether it stays on by default):

| id | criterion | observed | rule | result |
|---|---|---|---|---|
| S1 featuredrop | small repo arm: cost ratio vs control, upper bound; correctness difference, lower bound | 1.11 [1.06, 1.16]; +0.0 points [+0.0, +0.0] | upper < 1.15; lower > -10 points | FAIL |
