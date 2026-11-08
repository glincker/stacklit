# Real-agent A/B harness

Measures what the earlier offline benchmark cannot: how many tokens and dollars a real Claude Code session uses on the same task with and without Stacklit. It calls the paid API, so it will not start without `--yes-spend`.

## What it does

For every pinned task (from `bench/tasks/<repo>.json`), trial and arm it:

1. Clones the pinned repo (`bench/repos.json`, full commit SHA) once, then makes a clean local clone per session in a temp dir that is deleted afterwards.
2. Control arm: nothing added. Treatment arm: `stacklit generate --graph`, then `stacklit setup claude --session-hook --no-mcp --no-git-hook`, and `Bash(stacklit *)` allowed.
3. Runs `claude -p "<question>" --output-format json --no-session-persistence --setting-sources project,local --max-turns N --max-budget-usd X` in the checkout. `--setting-sources` leaves your user settings out so personal hooks cannot leak into either arm. The harness also refuses to start if `~/.claude/settings.json` mentions stacklit.
4. Reads the reported usage (`input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`), `num_turns` and `total_cost_usd`, and checks whether the answer names an expected path (a heuristic that can miss a correct answer worded differently, equally in both arms).

Sessions are shuffled with `--seed`, so arms interleave and the order repeats. Use 3 or more trials: single runs are noisy. Raw results go to `--out` (default `agent-ab-results.json`); a per-arm table is printed.

## Run it

```bash
go build -o /tmp/stacklit ./cmd/stacklit

# 1. see the plan and the projected cost, runs nothing
go run ./bench/agent-ab -repo gin -stacklit /tmp/stacklit -dry-run

# 2. smoke test: one task, one arm, cheap model
go run ./bench/agent-ab -repo gin -stacklit /tmp/stacklit -arms stacklit \
  -limit 1 -trials 1 -model haiku -max-turns 5 -max-budget-usd 0.25 -yes-spend

# 3. the full run, after you have approved the spend
go run ./bench/agent-ab -repo gin -stacklit /tmp/stacklit -trials 3 -yes-spend
```

Flags worth knowing: `-arms` (limit to one arm), `-limit` (first N tasks), `-model`, `-max-turns`, `-max-budget-usd` (hard per-session cap), `-seed`, and `-est-low/-est-mid/-est-high` for the projection.

## Cost

The number of paid sessions is tasks x trials x arms. With the shipped task files (10 per repo) and 3 trials:

| Scope | Sessions |
|---|---|
| one repo | 10 x 3 x 2 = 60 |
| all three repos | 180 |

The dollar figures printed by `-dry-run` use per-session assumptions (default $0.15, $0.40, $1.00 for low, middle, high), not measurements: one repo is therefore $9, $24 and $60, and all three repos $27, $72 and $180. The only measured figure is a smoke test on a tiny task with a small model, $0.003, which is not representative of a real task on a default model. Run step 2 with your real model on one real task first, then pass the observed cost as `-est-mid` to get a better projection. `-max-budget-usd` bounds each session; the worst case is sessions x that cap.

## Limits

- Tasks are navigation questions with a known file answer, the case Stacklit targets. They say little about editing tasks.
- The correctness check is substring matching on the answer.
- It does not confirm that a hook fired inside the `-p` session; check the transcript of a smoke run if that matters.
- Cache effects: a session that reads a warm prompt cache costs less. Interleaving the arms spreads this across both.
