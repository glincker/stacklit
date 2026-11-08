# Benchmark harness

A reproducible, offline comparison of two ways for a coding agent to answer navigation questions about a repo:

1. **grep + read** (baseline): search the source tree, then read the best-ranked files.
2. **stacklit map**: read the `stacklit derive` map, then look up the relevant modules in `stacklit.json`.

No LLM is called and no API key is needed. The only network access is `git fetch` of three pinned public commits. Results and methodology are in [RESULTS.md](RESULTS.md).

## Run it

```bash
# from the repository root; needs git and Go (version in go.mod)
go run ./bench                                  # print results to stdout
go run ./bench -write bench/RESULTS.md          # refresh the generated block of RESULTS.md
go run ./bench -only gin -runs 10               # one repo, more timing runs
go run ./bench -workdir /tmp/stk-bench          # keep clones between runs
go run ./bench -json out.json                   # also dump raw per-task data
```

Clones go into a new temporary directory outside this repository (removed on exit unless you pass `-workdir`). The harness builds the stacklit binary from your checkout unless you pass `-stacklit /path/to/binary`.

Token counts and success rates are deterministic: re-running on the same commits gives identical numbers. Generation time and peak memory depend on the machine.

## What is in here

| File | Purpose |
|---|---|
| `repos.json` | The three pinned repos (URL, tag, full commit SHA, file extensions, test-file markers) |
| `tasks/*.json` | Ten tasks per repo with hand-verified answers |
| `baseline.go` | The simulated grep + read agent |
| `index.go` | The simulated stacklit agent (module ranking, get_module style lookups) |
| `cost.go` | Token approximation |
| `main.go`, `report.go` | Orchestration, timing, memory, markdown output |

## Adding a repo or a task

Add an entry to `repos.json` (use a full commit SHA, never a branch) and a `tasks/<name>.json` file. Each task needs:

- `kind`: `locate`, `dependents` or `where-to-add`
- `pattern`: the regex a grep-based agent would search for (`word: true` wraps it in word boundaries)
- `terms`: the words a stacklit-based agent would look for in the index (locate and where-to-add)
- `target`: the module whose dependents are asked for (dependents only)
- `expect_paths` and `expect_modules`: the correct answer. Verify it yourself with `grep`, not with stacklit's own output. A trailing `/` means "anything under this directory".

## Optional: LLM-judged variant (not implemented here)

The deterministic proxy measures how many tokens an agent must read, not whether a real model reasons well from them. To measure that, run each task through a real agent twice:

1. Arm A: the agent has only its normal file tools.
2. Arm B: the agent additionally has `stacklit.json` (or the MCP server from `stacklit serve`).

Give both arms the same question text from `tasks/*.json` and the same token and turn limits. Log input and output tokens from the provider's usage report. Have a judge model, or a human, compare each final answer with `expect_paths` and `expect_modules`. Report mean tokens, success rate and wall time per arm, with at least 5 repetitions per task because agent runs are noisy. That variant needs API keys and costs money, so it is not part of this harness or of CI.
