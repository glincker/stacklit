# Benchmark results

How many tokens does a coding agent read to find the right file, with and without a stacklit map, and how does that compare with other common approaches? This page answers that with a deterministic, offline proxy on six pinned public repos in six languages. It makes no LLM calls. Reproduce it with `make bench` or `go run ./bench -write bench/RESULTS.md` (flags and arms in [README.md](README.md)). An optional LLM-judged variant is described, not implemented, in [OPTIONAL_LLM.md](OPTIONAL_LLM.md).

## Summary

Numbers are from the run recorded in the generated section below (58 tasks, 6 repos, 8 arms). Intervals are 95% bootstrap intervals over tasks.

- **With an oracle grep pattern, grep + read is the strongest baseline, and stacklit does not beat it on accuracy.** Over the 47 localization tasks grep+read put the gold file first on 89% [81-98] and within the first 5 on 100%. Stacklit map+lookup managed 43% [30-57] and 55% [40-70]. The grep pattern is hand-written by someone who knows the answer, which is a large advantage a real agent does not have, but the result stands as measured.
- **Stacklit map+lookup is cheaper per task on average, but the token difference is within noise.** Mean tokens to found over localization tasks was 4.3k [2.7k-6.1k] for stacklit map+lookup versus 5.9k [3.8k-8.7k] for grep+read; the intervals overlap heavily, and stacklit reaches the file on far fewer tasks, so a miss costs the agent follow-up work this metric does not count. No cost-saving claim over grep follows from these data.
- **Compared with dumping the repo, any retrieval arm is far cheaper.** The full-repo dump of code files averages 293k tokens [252k-334k] per task, and 3 of the 6 repos (hono, ripgrep, gson) do not fit a 200k window. A lookup in the 1,000 token range (stacklit map+lookup, query, or the repo-map budget) is about 300 times smaller than the dump, though that comparison only matters if the alternative really is dumping everything.
- **BM25 over the question text is a strong, cheap, no-index-needed baseline.** found@5 was 89% [81-98] at 37 lookup tokens, ahead of stacklit map+lookup (55%) and the approximated repo map (34%) on locating the file.
- **The approximated repo map is an orientation aid, not a retriever, in this setup.** At a 4,000 token budget the gold file appeared somewhere in the map for 94% [85-100] of localization tasks, but ranked first only 9% of the time. At 1,000 tokens it appeared for 74% [62-87]. These numbers describe the approximation in `repomap.go`, not the real tool.
- **`stacklit query` finds the file more often when the budget is smaller, because its output is alphabetical.** found@5 was 66% [53-79] at budget 1000 and 34% [21-47] at 2000, while "gold shown" rose slightly (74% to 77%). The file order the tool prints is by path, so a reader who scans all of the output would do better than this file-order policy suggests. It returns nothing at all for every rack (Ruby) question.
- **Dependents (impact) is stacklit's weakest area today.** Grep passed all 11 tasks under both rules. Stacklit map+lookup passed 55% [27-82] leniently and 45% [18-73] strictly, and 0 of 4 on ripgrep, gson and rack, where the index has no dependency edges for Rust and Java and few for Ruby. Gin, a Go repo, was 3 of 3. Query with a 2000 budget passed 27% [0-55] strictly with 3.5 unexpected modules on average.
- **Generation cost.** Median `stacklit generate` was 73 ms on gin and 0.6 to 3.4 s on flask, hono, ripgrep and gson, with peak memory of 0.5 to 1.7 GB (cold cache). Rack took about 16 s for 103 files, which is unexpectedly slow and not explained here.
- **Stacklit bugs found while building the benchmark** (not fixed here): `generate` hangs on some shell scripts (ripgrep completion files), the Rust parser panics on a random subset of files each run so the index changes between runs, rack generation is slow, `query` returns nothing for Ruby, and Rust and Java have no module dependency edges.


## Methodology

**Repos.** gin (Go, v1.10.0), flask (Python, 3.1.2), hono (TypeScript, v4.6.0), ripgrep (Rust, 14.1.1), gson (Java, 2.11.0) and rack (Ruby, 3.1.8), pinned by full commit SHA in `repos.json`. Each is fetched with `git fetch --depth 1 <sha>` into a temporary directory outside this repository and verified to be at exactly that commit. Shallow clones mean the git-activity part of the index (hot files) is empty; it does not affect the questions asked here.

**Tasks.** 58 tasks in three kinds, in `tasks/*.json`:

- `locate` (35): "where is X implemented?" Gold is the file that defines X.
- `where-to-add` (12), the add-feature kind: "where would I add Z?" Gold is the file (or directory) where the change would start.
- `dependents` (11): "what depends on module Y?" Gold is the set of modules that import Y.

Gold answers were found with `grep` or `rg` on the pinned checkouts, not taken from stacklit's output. The harness re-verifies them on every run (gold files exist and match the task's grep pattern; for dependents tasks an uncapped grep must cover every expected module) and refuses to run if one is wrong. For localization tasks the first matching gold path counts, so a task with several acceptable files is satisfied by any of them.

**Arms.** All arms see the same task. Each produces an ordered list of files and a printed output; the simulated agent opens files in that order.

1. `grep+read`: `grep -rn` with a hand-written regex per task over non-test source files, first 50 output lines visible (each cut to 200 chars). Files visible in that output are ranked by definition-like lines (`func`, `def`, `class`, `fn` and similar containing the pattern), then by match count. This changed from an earlier version of the harness, which ranked using all matches even those past the 50-line cap.
2. `repo-map approx 1000` and `4000`: an in-process approximation of a ranked repo map in the style popularised by aider. Definitions come from per-language regular expressions (not tree-sitter), files are nodes of a graph with edges from referencing to defining files weighted by the square root of the reference count (with the published multipliers for mentioned, long and over-defined identifiers), PageRank with damping 0.85 spreads rank to definitions, words of the question that equal an identifier (case-insensitive) boost it ten times, and the largest ranked prefix that fits the token budget is rendered as definition lines grouped by file. Differences from the real tool: no tree-sitter, no surrounding code context, no tag-less filenames appended. Treat it as an approximation of the technique, not a measurement of the product. A repo map is an orientation aid more than a retriever, so "gold shown" (below) is the fairer column for these arms.
3. `bm25 top-k`: standard BM25 (k1 1.2, b 0.75) over non-test source files, with identifiers split on camelCase and underscores and the path included; the query is the question text minus stop words, with no hand-written terms. The tool prints its top 5 paths and ranks 10.
4. `full dump`: the token cost of every code file of the repo, tests included (docs and data files not counted, so a lower bound). It always contains the answer, so only cost is reported.
5. `stacklit map+lookup`: the `stacklit derive` map is counted in full on every task (in practice it is injected once), then the agent picks modules from `stacklit.json` using the task's hand-written `terms` (name hit 3, purpose hit 2, hit in exports, type definitions or file names 1), counts the entries of the top 3 modules, and opens files from those modules whose name overlaps the terms (else the first non-test file listed). Only the file names the index lists are available: the index keeps about 20 per module.
6. `stacklit query 1000` and `2000`: one `stacklit query "<question>" --budget N` call, question verbatim, no map, no follow-up lookups. Files are opened in the order the tool returns them, which is alphabetical by path, not by relevance, so found@k understates what a smarter reader could do with this output; "gold shown" is the fairer column.

Hand-written inputs cut both ways: grep patterns and stacklit terms are chosen by a person who knows the answer, which favors those two arms, while BM25, repo-map and query use only the question text.

**Metrics** (localization tasks).

- found@1 / found@5: the first gold file is the 1st / within the first 5 files of the arm's ordered list.
- gold shown: a gold file is visible anywhere in the arm's printed output (for stacklit map+lookup, among the file names printed in the top 3 module entries).
- lookup tokens: tokens the arm prints before any file is opened.
- tokens to found: lookup plus the full size of each file opened, in order, until the first gold file is read. A task not found within the first 5 files (`-read-cap`) is charged for all 5 reads, so misses are expensive rather than free.
- 95% CI: percentile bootstrap over tasks (10,000 resamples, fixed seed, so the intervals are reproducible). Intervals treat tasks as independent draws; the six repos are not a random sample of software, so read them as "how much would this number move if the task set were redrawn from similar tasks", not as a population claim.

**Dependents grading.** Lenient: every expected module is reported (short module names are accepted, extra modules ignored). Strict: every expected module is reported by exact name and at most 2 unexpected modules are reported (`-max-extra`). Both rates are reported. Grep and stacklit query infer modules from the files they surface; stacklit map+lookup reads the target module's `depended_by`. The repo-map, BM25 and full-dump arms have no dependents procedure and are marked n/a rather than scored zero.

**Token approximation.** Tokens are `ceil(bytes / 4)`. A second estimator (word and punctuation pieces times 1.3) is reported as a sensitivity check. No tokenizer is bundled, so no figure here is a billed-token measurement.

**Generation time and memory.** Wall-clock time of `stacklit generate --quiet` with the parse cache disabled (`STACKLIT_NO_CACHE=1`, so each run is cold), 5 runs per repo with outputs deleted first. Peak memory is the maximum resident set size the OS reports for the child process. Repo-map and BM25 build times are single in-process runs and their memory is not measured. The real aider was not installed, so it was not run; the repo-map arms are the approximation only.

**ripgrep needs an ignore rule.** `stacklit generate` hangs (over 100 seconds with no output) on some shell completion scripts in ripgrep (`crates/core/flags/complete/*`). The harness therefore writes a `.stacklitrc.json` that ignores shell files in that checkout only. This is a stacklit bug that the benchmark works around, not a property of the repo; separately, the Rust parser panics (nil dereference, index out of range) on a different small set of files on each run, so a plain `generate` on ripgrep drops 0 to about 30 files at random. The harness regenerates with `GOMAXPROCS=1` until a run reports zero parse errors and keeps that index (the "clean-index attempts" column); without this the stacklit arms would not be reproducible on ripgrep. Timed runs are the plain runs and may include dropped files.

## Limitations

Read these before quoting any number.

1. **It is a proxy, not an agent.** Real agents grep several times, read partial files, and make mistakes. No model is involved, so reasoning quality, tool-call overhead and conversation cost are not measured.
2. **The arms are simulations with fixed policies.** Reading files in rank order up to 5, and ranking grep hits by definition-like lines, are assumptions. A smarter or dumber reader changes the tokens-to-found column; `-read-cap` and `-grep-lines` let you vary them.
3. **Hand-written inputs.** The grep pattern and the stacklit terms were chosen knowing the answer. The real MCP `find_module` tool matches module names and purpose only, so real stacklit usage may do worse than the map+lookup arm.
4. **Module granularity limits stacklit.** Java (gson) has one 83-file module for the whole library and Ruby (rack) one 43-file module; the index lists about 20 file names per module, so most files are never named. File-level questions on such repos are where map+lookup is weakest.
5. **Language coverage in the graph.** `stacklit query` returns "no matching symbols" for every rack question, and Rust and Java indexes have no `depended_by` edges, so dependents tasks on ripgrep, gson and rack score zero for stacklit. These are real gaps today, not harness errors.
6. **Small sample.** 58 tasks, 6 repos; the 11 dependents tasks give wide intervals. Differences of one or two tasks are noise. Per-repo cells have 7 to 10 tasks and no intervals.
7. **The repo-map arm is an approximation** (see above) and the full dump excludes non-code files.
8. **No git history.** Shallow clones mean hot-file and activity signals are not exercised.
9. **Timing is machine specific.** Only token counts, success rates and intervals are deterministic. The generated section records the machine and date.
10. **Stacklit's own numbers include its generation cost.** Time and memory are for building the index, a one-time or per-commit cost, not a per-question cost. Generating on rack took about 17 seconds, far more than its size suggests.

## Results

The tables below are generated. They are rewritten by `go run ./bench -write bench/RESULTS.md`.

<!-- BEGIN GENERATED (go run ./bench -write bench/RESULTS.md) -->

Run on 2026-11-08, darwin/arm64, 14 CPUs, go1.26.5, stacklit stacklit version dev (checkout 41b44f1).

Parameters: grep output capped at 50 lines (each cut to 200 chars); an arm may open at most 5 files in rank order; bootstrap 10000 resamples, seed 20260101; dependents strict rule allows at most 2 unexpected modules; stacklit map+lookup inspects the top 3 modules; BM25 prints 5 paths and ranks 10; query budgets [1000 2000]; repo-map budgets [1000 4000].

The real `aider` tool is not installed on the machine that produced these numbers, so the repo-map arms are the in-process approximation only.

### Repos and generation cost

| Repo | Lang | Pinned commit | Tasks | Code files | Lines | Modules | Full dump tokens | Fits in context | stacklit.json tokens | derive map tokens | generate first (ms) | generate median (ms) | peak RSS (MB) | clean-index attempts | repo-map build (ms) | BM25 build (ms) |
|---|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|
| gin | Go | `75ccf94d60` (v1.10.0) | 10 | 101 | 19537 | 8 | 136665 | yes | 2524 | 137 | 683 | 73 | 82 | 1 | 7 | 12 |
| flask | Python | `2c1b30d050` (3.1.2) | 10 | 101 | 18515 | 14 | 143168 | yes | 2681 | 221 | 2401 | 2014 | 1315 | 1 | 11 | 28 |
| hono | TypeScript | `73ff6c0e82` (v4.6.0) | 10 | 360 | 57307 | 79 | 416846 | no | 16126 | 1372 | 606 | 606 | 916 | 1 | 25 | 29 |
| ripgrep | Rust | `4649aa9700` (14.1.1) | 10 | 116 | 51442 | 33 | 423628 | no | 7582 | 368 | 1224 | 3404 | 1731 | 2 | 57 | 87 |
| gson | Java | `828a97be0f` (gson-parent-2.11.0) | 9 | 259 | 53490 | 10 | 456223 | no | 2750 | 187 | 1144 | 880 | 522 | 1 | 27 | 41 |
| rack | Ruby | `0eabeb73b3` (v3.1.8) | 9 | 103 | 19805 | 9 | 162318 | yes | 2162 | 126 | 16089 | 15770 | 1206 | 1 | 163 | 83 |

"Fits in context" compares the full dump with a 200000 token window. Generation time is wall clock for `stacklit generate --quiet` (5 runs per repo, cold parse cache, outputs deleted before each run), peak RSS is the largest resident set of the child process. Clean-index attempts is how many `generate` runs it took to get an index with zero parse errors (the Rust parser panics on a varying set of files from run to run; the harness regenerates with one CPU until a run is clean and keeps that index so the stacklit arms are reproducible). Repo-map and BM25 build times are a single in-process run; their memory is not measured. Tokens are bytes/4.

### Locate tasks

"Where is X implemented?" Lookup tokens are what the arm prints before any file is opened. Tokens to found add the files opened in rank order until the gold file is read; a miss is charged for every file it opened. 58 tasks in total across all kinds.

| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |
|---|---:|---|---|---|---|---|
| grep+read | 35 | 86% [74-97] | 100% [100-100] | 100% [100-100] | 134 [72-219] | 5.1k [3.8k-6.6k] |
| repo-map approx 1000 | 35 | 11% [3-23] | 31% [17-49] | 71% [57-86] | 993 [991-995] | 19k [16k-22k] |
| repo-map approx 4000 | 35 | 11% [3-23] | 31% [17-49] | 91% [80-100] | 3.8k [3.7k-3.9k] | 22k [19k-25k] |
| bm25 top-k | 35 | 66% [49-80] | 94% [86-100] | 94% [86-100] | 37 [30-44] | 8.6k [5.1k-13k] |
| full dump | 35 | n/a | n/a | n/a | 294k [246k-342k] | 294k [246k-342k] |
| stacklit map+lookup | 35 | 40% [23-57] | 54% [37-71] | 66% [49-80] | 796 [617-992] | 3.9k [2.5k-5.6k] |
| stacklit query 1000 | 35 | 11% [3-23] | 74% [60-89] | 80% [66-91] | 784 [657-897] | 18k [12k-25k] |
| stacklit query 2000 | 35 | 9% [0-20] | 34% [20-51] | 80% [66-91] | 1.4k [1.2k-1.7k] | 22k [15k-31k] |

### Add-feature tasks

"Where would I add Z?" Gold is the file (or directory) a change would start in.

| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |
|---|---:|---|---|---|---|---|
| grep+read | 12 | 100% [100-100] | 100% [100-100] | 100% [100-100] | 96 [41-160] | 8.2k [2.1k-18k] |
| repo-map approx 1000 | 12 | 0% [0-0] | 42% [17-67] | 83% [58-100] | 995 [992-997] | 17k [12k-22k] |
| repo-map approx 4000 | 12 | 0% [0-0] | 42% [17-67] | 100% [100-100] | 3.8k [3.7k-4.0k] | 20k [15k-25k] |
| bm25 top-k | 12 | 33% [8-58] | 75% [50-100] | 75% [50-100] | 36 [27-48] | 16k [6.6k-29k] |
| full dump | 12 | n/a | n/a | n/a | 290k [214k-366k] | 290k [214k-366k] |
| stacklit map+lookup | 12 | 50% [25-75] | 58% [33-83] | 67% [42-92] | 827 [587-1.1k] | 5.1k [1.5k-11k] |
| stacklit query 1000 | 12 | 25% [0-50] | 42% [17-67] | 58% [33-83] | 826 [586-989] | 12k [6.0k-19k] |
| stacklit query 2000 | 12 | 17% [0-42] | 33% [8-58] | 67% [42-92] | 1.6k [1.1k-1.9k] | 11k [5.1k-18k] |

### All localization tasks (locate plus add-feature)

| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |
|---|---:|---|---|---|---|---|
| grep+read | 47 | 89% [81-98] | 100% [100-100] | 100% [100-100] | 124 [75-188] | 5.9k [3.8k-8.7k] |
| repo-map approx 1000 | 47 | 9% [2-17] | 34% [21-49] | 74% [62-87] | 994 [992-995] | 18k [16k-21k] |
| repo-map approx 4000 | 47 | 9% [2-17] | 34% [21-49] | 94% [85-100] | 3.8k [3.8k-3.9k] | 21k [19k-24k] |
| bm25 top-k | 47 | 57% [43-72] | 89% [81-98] | 89% [81-98] | 37 [31-43] | 11k [6.7k-15k] |
| full dump | 47 | n/a | n/a | n/a | 293k [252k-334k] | 293k [252k-334k] |
| stacklit map+lookup | 47 | 43% [30-57] | 55% [40-70] | 66% [51-79] | 804 [658-963] | 4.3k [2.7k-6.1k] |
| stacklit query 1000 | 47 | 15% [6-26] | 66% [53-79] | 74% [62-87] | 795 [685-894] | 17k [12k-22k] |
| stacklit query 2000 | 47 | 11% [2-19] | 34% [21-47] | 77% [64-87] | 1.5k [1.2k-1.7k] | 19k [13k-26k] |

### Dependents (impact) tasks

Lenient: every expected module is reported (short module names accepted, extras ignored). Strict: every expected module is reported by exact name and at most 2 unexpected modules are reported. The repo-map, BM25 and full-dump arms have no dependents procedure and are marked n/a.

| Arm | Tasks | lenient pass [95% CI] | strict pass [95% CI] | mean extra modules | lookup tokens [95% CI] |
|---|---:|---|---|---:|---|
| grep+read | 11 | 100% [100-100] | 100% [100-100] | 0.6 | 229 [75-471] |
| repo-map approx 1000 | 11 | n/a | n/a | n/a | n/a |
| repo-map approx 4000 | 11 | n/a | n/a | n/a | n/a |
| bm25 top-k | 11 | n/a | n/a | n/a | n/a |
| full dump | 11 | n/a | n/a | n/a | n/a |
| stacklit map+lookup | 11 | 55% [27-82] | 45% [18-73] | 0.4 | 637 [419-913] |
| stacklit query 1000 | 11 | 18% [0-45] | 18% [0-45] | 2.4 | 889 [710-984] |
| stacklit query 2000 | 11 | 45% [18-73] | 27% [0-55] | 3.5 | 1.6k [1.3k-1.9k] |

### Per repo

Found@5 over localization tasks, as hits/tasks, then mean tokens to found.

| Arm | gin (Go) | flask (Python) | hono (TypeScript) | ripgrep (Rust) | gson (Java) | rack (Ruby) |
|---|---|---|---|---|---|---|
| grep+read | 7/7, 1.7k | 8/8, 6.9k | 8/8, 1.3k | 8/8, 15k | 8/8, 6.0k | 8/8, 4.0k |
| repo-map approx 1000 | 2/7, 7.4k | 4/8, 17k | 1/8, 21k | 2/8, 25k | 2/8, 22k | 5/8, 16k |
| repo-map approx 4000 | 2/7, 10k | 4/8, 20k | 1/8, 24k | 2/8, 28k | 2/8, 25k | 5/8, 18k |
| bm25 top-k | 7/7, 4.5k | 7/8, 10k | 8/8, 508 | 7/8, 22k | 7/8, 18k | 6/8, 6.8k |
| full dump | n/a, 137k | n/a, 143k | n/a, 417k | n/a, 424k | n/a, 456k | n/a, 162k |
| stacklit map+lookup | 4/7, 967 | 6/8, 6.3k | 7/8, 2.3k | 7/8, 13k | 0/8, 604 | 2/8, 2.3k |
| stacklit query 1000 | 7/7, 12k | 7/8, 19k | 6/8, 5.4k | 5/8, 40k | 6/8, 23k | 0/8, 22 |
| stacklit query 2000 | 4/7, 12k | 6/8, 19k | 2/8, 6.7k | 2/8, 53k | 2/8, 24k | 0/8, 22 |

Dependents tasks per repo, strict/lenient passes:

| Arm | gin | flask | hono | ripgrep | gson | rack |
|---|---|---|---|---|---|---|
| grep+read | 3/3, 3/3 | 2/2, 2/2 | 2/2, 2/2 | 2/2, 2/2 | 1/1, 1/1 | 1/1, 1/1 |
| stacklit map+lookup | 3/3, 3/3 | 1/2, 1/2 | 1/2, 2/2 | 0/2, 0/2 | 0/1, 0/1 | 0/1, 0/1 |
| stacklit query 1000 | 1/3, 1/3 | 1/2, 1/2 | 0/2, 0/2 | 0/2, 0/2 | 0/1, 0/1 | 0/1, 0/1 |
| stacklit query 2000 | 3/3, 3/3 | 0/2, 2/2 | 0/2, 0/2 | 0/2, 0/2 | 0/1, 0/1 | 0/1, 0/1 |

### Sensitivity to the token approximation

Mean tokens to found over all localization tasks under bytes/4 and under the second estimator (word and punctuation pieces x 1.3).

| Arm | bytes/4 | pieces x 1.3 |
|---|---:|---:|
| grep+read | 5.9k | 7.3k |
| repo-map approx 1000 | 18k | 23k |
| repo-map approx 4000 | 21k | 26k |
| bm25 top-k | 11k | 12k |
| full dump | 293k | 386k |
| stacklit map+lookup | 4.3k | 5.2k |
| stacklit query 1000 | 17k | 20k |
| stacklit query 2000 | 19k | 24k |

### Per task

Localization cells: rank of the first gold file in the arm's list, `-` when it is not in the first 5 files it was allowed to open. Dependents cells: `S` strict pass, `L` lenient only, `-` fail, `n/a` not applicable. Full dump is omitted (it always contains the answer).

| Task | Kind | grep+read | repo-map approx 1000 | repo-map approx 4000 | bm25 top-k | stacklit map+lookup | stacklit query 1000 | stacklit query 2000 |
|---|---|---|---|---|---|---|---|---|
| gin-01 | locate | 1 | - | - | 1 | - | 5 | - |
| gin-02 | locate | 1 | - | - | 1 | 1 | 1 | 1 |
| gin-03 | locate | 1 | 5 | 5 | 1 | - | 3 | 4 |
| gin-04 | locate | 1 | - | - | 1 | 1 | 1 | 1 |
| gin-05 | locate | 1 | - | - | 1 | - | 4 | - |
| gin-06 | dependents | S | n/a | n/a | n/a | S | - | S |
| gin-07 | dependents | S | n/a | n/a | n/a | S | - | S |
| gin-08 | dependents | S | n/a | n/a | n/a | S | S | S |
| gin-09 | where-to-add | 1 | 2 | 2 | 3 | 1 | 5 | - |
| gin-10 | where-to-add | 1 | - | - | 2 | 1 | 1 | 1 |
| flask-01 | locate | 1 | 2 | 2 | 3 | 2 | 3 | 4 |
| flask-02 | locate | 1 | 1 | 1 | 1 | 1 | 3 | - |
| flask-03 | locate | 1 | 1 | 1 | 1 | - | 2 | 3 |
| flask-04 | locate | 2 | 2 | 2 | 1 | - | 3 | 3 |
| flask-05 | locate | 1 | - | - | 4 | 1 | 4 | 5 |
| flask-06 | locate | 1 | - | - | 1 | 1 | 1 | 3 |
| flask-07 | dependents | S | n/a | n/a | n/a | S | S | L |
| flask-08 | dependents | S | n/a | n/a | n/a | - | - | L |
| flask-09 | where-to-add | 1 | - | - | - | 1 | - | - |
| flask-10 | where-to-add | 1 | - | - | 1 | 1 | 1 | 4 |
| hono-01 | locate | 2 | - | - | 1 | 1 | - | - |
| hono-02 | locate | 2 | - | - | 1 | 1 | 5 | - |
| hono-03 | locate | 2 | - | - | 1 | - | 2 | - |
| hono-04 | locate | 1 | - | - | 1 | 1 | 5 | - |
| hono-05 | locate | 1 | - | - | 1 | 1 | 2 | 2 |
| hono-06 | locate | 1 | - | - | 1 | 1 | 5 | - |
| hono-07 | dependents | S | n/a | n/a | n/a | S | - | - |
| hono-08 | dependents | S | n/a | n/a | n/a | L | - | - |
| hono-09 | where-to-add | 1 | 4 | 4 | 1 | 1 | - | - |
| hono-10 | where-to-add | 1 | - | - | 1 | 1 | 2 | 2 |
| rg-01 | locate | 1 | - | - | 1 | 2 | 5 | - |
| rg-02 | locate | 1 | 5 | 5 | 1 | 2 | 3 | - |
| rg-03 | locate | 1 | - | - | 3 | 2 | 2 | 3 |
| rg-04 | locate | 1 | - | - | 2 | 1 | 1 | 1 |
| rg-05 | locate | 1 | - | - | - | 1 | - | - |
| rg-06 | locate | 1 | - | - | 1 | 3 | 5 | - |
| rg-07 | dependents | S | n/a | n/a | n/a | - | - | - |
| rg-08 | dependents | S | n/a | n/a | n/a | - | - | - |
| rg-09 | where-to-add | 1 | - | - | 5 | - | - | - |
| rg-10 | where-to-add | 1 | 2 | 2 | 1 | 2 | - | - |
| gs-01 | locate | 1 | - | - | 3 | - | - | - |
| gs-02 | locate | 1 | - | - | 2 | - | 5 | - |
| gs-03 | locate | 1 | 1 | 1 | 5 | - | 5 | - |
| gs-04 | locate | 1 | - | - | 2 | - | 4 | 5 |
| gs-05 | locate | 1 | 1 | 1 | 1 | - | 5 | - |
| gs-06 | locate | 1 | - | - | 1 | - | 4 | - |
| gs-07 | dependents | S | n/a | n/a | n/a | - | - | - |
| gs-08 | where-to-add | 1 | - | - | 2 | - | 1 | 1 |
| gs-09 | where-to-add | 1 | - | - | - | - | - | - |
| rk-01 | locate | 1 | 3 | 3 | 4 | 1 | - | - |
| rk-02 | locate | 2 | 4 | 4 | - | - | - | - |
| rk-03 | locate | 1 | - | - | 1 | - | - | - |
| rk-04 | locate | 1 | - | - | 1 | 1 | - | - |
| rk-05 | locate | 1 | - | - | 1 | - | - | - |
| rk-06 | locate | 1 | 4 | 4 | 2 | - | - | - |
| rk-07 | dependents | S | n/a | n/a | n/a | - | - | - |
| rk-08 | where-to-add | 1 | 4 | 4 | - | - | - | - |
| rk-09 | where-to-add | 1 | 3 | 3 | 3 | - | - | - |

<!-- END GENERATED -->
