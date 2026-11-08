# Benchmark harness

A reproducible, offline comparison of ways for a coding agent to find the right file in a repo. Eight arms run on the same hand-verified tasks over six pinned public repos. No LLM is called and no API key is needed. The only network access is `git fetch` of the pinned commits. Methodology, limits and results are in [RESULTS.md](RESULTS.md). An optional LLM-judged variant is described (not implemented) in [OPTIONAL_LLM.md](OPTIONAL_LLM.md).

## Run it

```bash
# from the repository root; needs git and Go (version in go.mod)
make bench                                      # same as: go run ./bench -write bench/RESULTS.md
go run ./bench                                  # print results to stdout
go run ./bench -only gin,flask -runs 10         # subset of repos, more timing runs
go run ./bench -workdir /tmp/stk-bench          # keep clones between runs
go run ./bench -json out.json                   # also dump raw per-task data
```

Clones go into a new temporary directory outside this repository (removed on exit unless you pass `-workdir`). The harness builds the stacklit binary from your checkout unless you pass `-stacklit /path/to/binary`. A full run takes a few minutes, most of it generating indexes (rack is the slowest).

Before any arm runs, every task's gold answer is re-checked against the pinned checkout (gold files exist and match the task's grep pattern; a dependents task's expected modules appear in an uncapped grep). A wrong gold answer stops the run.

### Flags

| Flag | Default | Meaning |
|---|---|---|
| `-only` | all | Comma separated repo names |
| `-workdir` | temp dir | Where clones live |
| `-stacklit` | build from checkout | Path to a stacklit binary |
| `-runs` | 5 | Timed `generate` runs per repo (cold parse cache) |
| `-write` | none | Rewrite the generated block of a markdown file |
| `-json` | none | Raw per-task, per-arm results |
| `-grep-lines` | 50 | Grep arm: output lines the agent sees |
| `-read-cap` | 5 | Files any arm may open in rank order before a task counts as missed |
| `-top` | 3 | Stacklit map+lookup: modules inspected per localization task |
| `-query-budgets` | `1000,2000` | `stacklit query --budget` values, one arm each |
| `-repomap-budgets` | `1000,4000` | Repo-map approximation budgets (tokens), one arm each |
| `-bm25-listed`, `-bm25-ranked` | 5, 10 | BM25 paths printed, and files ranked for found@k |
| `-max-extra` | 2 | Dependents strict rule: most unexpected modules allowed |
| `-boot`, `-seed` | 10000, 20260101 | Bootstrap resamples and seed |
| `-context-window` | 200000 | Window used to say whether a full dump fits |

Token counts, success rates and confidence intervals are deterministic: re-running on the same commits with the same flags gives identical numbers. Generation time and peak memory depend on the machine.

## Arms

| Arm | What it simulates | Deterministic |
|---|---|---|
| `grep+read` | `grep -rn` with a hand-written pattern per task, first 50 lines visible, files ranked by definition-like matches then match count, opened in that order | yes |
| `repo-map approx N` | Approximation of a ranked repo map (regex definitions, file graph, PageRank, question words as personalization) cut to an N token budget. Labelled approximation, not the real tool | yes |
| `bm25 top-k` | BM25 over source files, question text as the query, top 5 paths printed | yes |
| `full dump` | Cost of putting every code file in context | yes |
| `stacklit map+lookup` | `stacklit derive` map, then module entries from `stacklit.json` picked with hand-written terms | yes |
| `stacklit query N` | One `stacklit query "<question>" --budget N` call, question verbatim | yes |

Details and the grading rules are in RESULTS.md.

## What is in here

| File | Purpose |
|---|---|
| `repos.json` | The six pinned repos (URL, tag, full commit SHA, file extensions, test-file markers) |
| `tasks/*.json` | Hand-verified tasks per repo |
| `arms.go` | Shared result type, localization and dependents grading |
| `baseline.go`, `bm25.go`, `repomap.go` | Baseline arms |
| `index.go`, `query.go` | Stacklit arms |
| `stats.go` | Seeded bootstrap confidence intervals |
| `cost.go` | Token approximation |
| `main.go`, `report.go` | Orchestration, gold verification, timing, memory, markdown |

## Adding a repo or a task

Add an entry to `repos.json` (a full commit SHA, never a branch) and a `tasks/<name>.json` file. Each task needs:

- `kind`: `locate`, `dependents` or `where-to-add` (the add-feature kind)
- `question`: the text given verbatim to the BM25, repo-map and query arms
- `pattern`: the regex a grep-based agent would search for (`word: true` wraps it in word boundaries)
- `terms`: the words a stacklit map+lookup agent would look for in the index (localization only)
- `target`: the module whose dependents are asked for (dependents only)
- `expect_paths` and `expect_modules`: the answer. Verify it yourself with `grep` or `rg`, never with stacklit's own output. A trailing `/` means "anything under this directory". For localization the first gold path found counts (any of `expect_paths`).

If stacklit hangs or crashes on a file type in a new repo, `stacklit_ignore` in `repos.json` is written to `.stacklitrc.json` in the checkout (this is used for ripgrep, see RESULTS.md).
