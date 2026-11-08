# Benchmark results

How many tokens does a coding agent read to find the right file, with and without a stacklit map, and how does that compare with other common approaches? This page answers that with a deterministic, offline proxy on six pinned public repos in six languages. It makes no LLM calls. It is the rerun on current master (symbol graph for 20 languages, BM25 plus PageRank query ranking, module splitting, Rust, Java and Ruby dependents resolution). Reproduce it with `make bench` or `go run ./bench -write bench/RESULTS.md` (flags and arms in [README.md](README.md)). Everything that changed in the harness and the tasks since the first run is listed in [CHANGES.md](CHANGES.md). Proposed public wording, with the table cell that backs each claim, is in [PROPOSED_CLAIMS.md](PROPOSED_CLAIMS.md).

## Summary

58 tasks (47 localization, 11 dependents), 6 repos, 10 arms. Intervals are 95% bootstrap intervals over tasks (10,000 resamples, seed 20260101). "Reader order" is explained under Arms; it is a harness policy, not a stacklit feature.

- **Stacklit still loses to grep+read on accuracy, and does not beat BM25 on whether the file is found in the first five.** Over the 47 localization tasks grep+read (with an oracle regex written by someone who knows the answer) put the gold file first on 89% [81-98] and in the first 5 on 100%. The best stacklit arm, one `query --budget 1000` call read in reader order, got 79% [66-89] and 94% [85-100]. Paired against grep it is 11 points lower at found@1 [-26, +6] and 6 points lower at found@5 [-15, +0]: the intervals touch 0, so the gap is suggestive, not established. Stacklit map+lookup got 55% [40-70] and 74% [62-87], which is 34 [-49, -17] and 26 [-38, -13] points below grep.
- **Versus BM25 over the question text (no index, 37 lookup tokens): stacklit query in reader order is better at found@1 and level at found@5.** found@1 79% against 57% [43-72], a paired difference of +21 points [+9, +34]. found@5 94% against 89% [81-98], a paired difference of +4 [-4, +13], so no detectable gap. Read in the order the tool prints (alphabetical by path) the same call is worse than BM25: found@1 21% [11-34], found@5 81% [68-91] (paired -36 [-53, -17] and -9 [-21, +4]).
- **The tool's output order is the biggest remaining accuracy problem.** `stacklit query` ranks symbols with BM25 and PageRank, but prints files sorted by path and exposes no relevance score. The gold file is somewhere in the output for 96% [89-100] of localization tasks at budget 1000 and 98% [94-100] at 2000, yet a top-to-bottom reader finds it first only 21% and 13% of the time. A reader that picks files from the printed text and the question alone reaches 79% and 74%.
- **Cost.** Mean tokens to found: grep+read 5.9k [3.8k-8.7k], stacklit map+lookup 5.9k [4.6k-7.3k], query 1000 in reader order 7.2k [5.3k-9.1k], BM25 11k [6.7k-15k], repo-map approximation 18k to 21k, full dump 293k [252k-334k]. All the retrieval arms except the repo map are within noise of each other; no token-saving claim over grep or BM25 follows. All of them are roughly 15 to 50 times cheaper than dumping the code, which only matters if dumping is the alternative (3 of 6 repos do not fit a 200k window).
- **Dependents (impact) improved a lot but is still behind grep.** Stacklit map+lookup passed 73% [45-100] leniently and 64% [36-91] strictly, up from 55% and 45%. Rust now works (ripgrep 2 of 2). Grep+read passed 91% [73-100] under both rules; the one miss (gson) passes with `-grep-lines 200`. Query alone is weak for impact questions: 27% [0-55] lenient and 18% [0-45] strict at budget 1000.
- **Generation.** Cold `generate` takes about 0.9 to 3.7 s on flask, hono, ripgrep, gson and rack, and 0.06 to 0.13 s on gin, with peak memory 82 MB (gin) to 751 MB (flask). Rack went from about 16 s to under 3 s. Every timed run on every repo wrote a byte-identical index and no parser dropped any file, so the Rust race and shell hang workarounds are gone. The timings were taken on a heavily loaded machine (see Limitations), so treat them as upper bounds.
- **The module-file cap helps the map arm a little and nothing else.** With `STACKLIT_MAX_MODULE_FILES=60` (default 30) map+lookup moved from 55% [40-70] to 60% [45-74] found@1 and from 74% to 79% found@5; the query arms are unchanged because they do not read module file lists.

## Before and after

"Before" is the first recorded run, on a stacklit without the symbol graph work (query output was alphabetical and returned nothing for Ruby; Rust and Java had no dependency edges; Java and Ruby were one big module each). It used a harness with a one-CPU retry loop and a shell-file ignore rule for ripgrep, and the old dependents answers for gson, ripgrep and rack (re-derived since, see CHANGES.md), so the dependents rows are not exactly like for like. Localization tasks and arms are identical.

| Arm (47 localization tasks) | found@1 before | found@1 after | found@5 before | found@5 after | tokens to found before | tokens to found after |
|---|---|---|---|---|---|---|
| grep+read | 89% [81-98] | 89% [81-98] | 100% [100-100] | 100% [100-100] | 5.9k [3.8k-8.7k] | 5.9k [3.8k-8.7k] |
| bm25 top-k | 57% [43-72] | 57% [43-72] | 89% [81-98] | 89% [81-98] | 11k [6.7k-15k] | 11k [6.7k-15k] |
| repo-map approx 4000 | 9% [2-17] | 9% [2-17] | 34% [21-49] | 34% [21-49] | 21k [19k-24k] | 21k [19k-24k] |
| stacklit map+lookup | 43% [30-57] | 55% [40-70] | 55% [40-70] | 74% [62-87] | 4.3k [2.7k-6.1k] | 5.9k [4.6k-7.3k] |
| stacklit query 1000 (tool order) | 15% [6-26] | 21% [11-34] | 66% [53-79] | 81% [68-91] | 17k [12k-22k] | 16k [12k-20k] |
| stacklit query 2000 (tool order) | 11% [2-19] | 13% [4-23] | 34% [21-47] | 57% [43-72] | 19k [13k-26k] | 22k [16k-28k] |
| stacklit query 1000 + reader order | not run | 79% [66-89] | not run | 94% [85-100] | not run | 7.2k [5.3k-9.1k] |

| Arm (11 dependents tasks) | lenient before | lenient after | strict before | strict after |
|---|---|---|---|---|
| grep+read | 100% [100-100] | 91% [73-100] | 100% [100-100] | 91% [73-100] |
| stacklit map+lookup | 55% [27-82] | 73% [45-100] | 45% [18-73] | 64% [36-91] |
| stacklit query 1000 | 18% [0-45] | 27% [0-55] | 18% [0-45] | 18% [0-45] |
| stacklit query 2000 | 45% [18-73] | 36% [9-64] | 27% [0-55] | 9% [0-27] |

| Repo | median generate before (ms) | median generate after (ms) | peak RSS before (MB) | peak RSS after (MB) |
|---|---:|---:|---:|---:|
| gin | 73 | 55 | 82 | 82 |
| flask | 2014 | 3664 | 1315 | 751 |
| hono | 606 | 863 | 916 | 534 |
| ripgrep | 3404 | 3153 | 1731 | 572 |
| gson | 880 | 1440 | 522 | 510 |
| rack | 15770 | 2922 | 1206 | 473 |

Generation time "after" was measured with another heavy job running on the same machine (load average about 40 on 14 CPUs), so flask, hono and gson are probably slower here than a quiet machine would show. Treat the "after" times as upper bounds and the rack and ripgrep memory drops as real.

Reading the localization rows: grep, BM25 and the repo map are identical because nothing about them changed; only stacklit's arms moved. The grep+read dependents drop (100% to 91%) comes from the corrected gson answer (CHANGES.md), not from a regression.

## Methodology

**Repos.** gin (Go, v1.10.0), flask (Python, 3.1.2), hono (TypeScript, v4.6.0), ripgrep (Rust, 14.1.1), gson (Java, 2.11.0) and rack (Ruby, 3.1.8), pinned by full commit SHA in `repos.json`. Each is fetched with `git fetch --depth 1 <sha>` into a temporary directory outside this repository and verified to be at exactly that commit. Shallow clones mean the git-activity part of the index (hot files) is empty; it does not affect the questions asked here.

**Tasks.** 58 tasks in three kinds, in `tasks/*.json`:

- `locate` (35): "where is X implemented?" Gold is the file that defines X.
- `where-to-add` (12), the add-feature kind: "where would I add Z?" Gold is the file (or directory) where the change would start.
- `dependents` (11): "what depends on module Y?" Gold is the set of modules that import Y.

Gold answers were found with `grep` or `rg` on the pinned checkouts, not taken from stacklit's output. The harness re-verifies them on every run (gold files exist and match the task's grep pattern; for dependents tasks an uncapped grep must cover every expected module) and refuses to run if one is wrong. For localization tasks the first matching gold path counts, so a task with several acceptable files is satisfied by any of them. For this rerun the three dependents answers that module splitting invalidated were re-derived by hand (CHANGES.md).

**Arms.** All arms see the same task. Each produces an ordered list of files and a printed output; the simulated agent opens files in that order.

1. `grep+read`: `grep -rn` with a hand-written regex per task over non-test source files, first 50 output lines visible (each cut to 200 chars). Files visible in that output are ranked by definition-like lines (`func`, `def`, `class`, `fn` and similar containing the pattern), then by match count.
2. `repo-map approx 1000` and `4000`: an in-process approximation of a ranked repo map in the style popularised by aider (regex definitions, file graph, PageRank with the question's identifiers as personalization, cut to a token budget, rendered as definition lines grouped by file). Differences from the real tool: no tree-sitter, no surrounding code context. It is an approximation of the technique, not a measurement of the product; the real aider was not installed.
3. `bm25 top-k`: standard BM25 (k1 1.2, b 0.75) over non-test source files, with identifiers split on camelCase and underscores and the path included; the query is the question text minus stop words, no hand-written terms. Prints its top 5 paths, ranks 10.
4. `full dump`: the token cost of every code file of the repo, tests included (docs and data files not counted, so a lower bound). It always contains the answer, so only cost is reported.
5. `stacklit map+lookup`: the `stacklit derive` map is counted in full on every task, then the agent picks modules from `stacklit.json` using the task's hand-written `terms` (name hit 3, purpose hit 2, hit in exports, type definitions or file names 1), counts the entries of the top 3 modules, and opens files from those modules whose name overlaps the terms (else the first non-test file listed). Only the file names the index lists are available (30 per module by default).
6. `stacklit query N`: one `stacklit query "<question>" --budget N` call (N is 1000 or 2000), question verbatim, no map, no follow-up lookups, parsed from `--json`. Files are opened in the order the tool prints them. That order is alphabetical by path: the BM25 and PageRank scores decide which symbols fit the budget, but the files are then sorted and no score is printed (`internal/graph/view.go`, checked against the JSON output).
7. `stacklit query N + reader order`: the same call and the same output, but files are opened in an order chosen from the printed text and the question only: 3 points per question word in the path, 2 per word in a symbol name, 1 per word in a signature or doc line, ties in path order. No gold answer, hand-written term or file content is used. It stands in for an agent that reads the output and picks sensibly; it is a harness policy and is not part of stacklit, and a different policy would give a different number.

Hand-written inputs cut both ways: grep patterns and stacklit map+lookup terms are chosen by a person who knows the answer, which favors those two arms, while BM25, repo-map and query use only the question text. The terms were not retuned for the new module names.

**Metrics** (localization tasks).

- found@1 / found@5: the first gold file is the 1st / within the first 5 files of the arm's ordered list.
- gold shown: a gold file is visible anywhere in the arm's printed output (for stacklit map+lookup, among the file names printed in the top 3 module entries).
- lookup tokens: tokens the arm prints before any file is opened.
- tokens to found: lookup plus the full size of each file opened, in order, until the first gold file is read. A task not found within the first 5 files (`-read-cap`) is charged for all 5 reads.
- 95% CI: percentile bootstrap over tasks (10,000 resamples, fixed seed, reproducible). Intervals treat tasks as independent draws; the six repos are not a random sample of software, so read them as sensitivity to the task set, not as a population claim.
- Paired differences: per-task difference between two arms, bootstrapped the same way.

**Dependents grading** (unchanged). Lenient: every expected module is reported (short module names are accepted, extra modules ignored). Strict: every expected module is reported by exact name and at most 2 unexpected modules are reported (`-max-extra`). Grep and stacklit query infer modules from the files they surface, mapped to stacklit's current module boundaries; stacklit map+lookup reads the target module's `depended_by`. Repo-map, BM25, full dump and reader-order arms have no dependents procedure and are marked n/a.

**Token approximation.** Tokens are `ceil(bytes / 4)`. A second estimator (word and punctuation pieces times 1.3) is reported as a sensitivity check. No tokenizer is bundled, so no figure here is a billed-token measurement.

**Generation time and memory.** Wall-clock time of `stacklit generate --quiet` with the parse cache disabled (`STACKLIT_NO_CACHE=1`, so each run is cold), 3 runs per repo with outputs deleted first, on all CPUs. Peak memory is the maximum resident set size the OS reports for the child process. No `.stacklitrc.json` is written and no CPU limit or retry is used; the index the arms read is the plain output of `generate`. The harness also records, per repo, whether any file was dropped by a parser (a verbose run) and whether all timed runs produced a byte-identical `stacklit.json`. Repo-map and BM25 build times are single in-process runs and their memory is not measured.

**Workarounds removed.** The first run needed a `GOMAXPROCS=1` retry loop for a Rust parser race and a shell-file ignore rule for a hang. Both bugs are fixed on master: all six repos, ripgrep included, generated with zero files dropped by a parser and a byte-identical index in every timed run, so both workarounds were deleted.

**The cap experiment.** The whole run was repeated with `STACKLIT_MAX_MODULE_FILES=60` against the default 30, to see how much the per-module file-name listing limit matters (section "Module file cap" below).

## Limitations

Read these before quoting any number.

1. **It is a proxy, not an agent.** Real agents grep several times, read partial files, and make mistakes. No model is involved, so reasoning quality, tool-call overhead and conversation cost are not measured.
2. **The arms are simulations with fixed policies.** Reading files in rank order up to 5, ranking grep hits by definition-like lines, and the reader-order scoring are assumptions. `-read-cap` and `-grep-lines` let you vary the first two. The reader-order arm is the least certain: it was designed after seeing that the tool's order is alphabetical, and a weaker reader would land between the two query arms.
3. **Hand-written inputs.** The grep pattern and the map+lookup terms were chosen knowing the answer. The real MCP `find_module` tool matches module names and purpose only, so real stacklit usage may do worse than the map+lookup arm.
4. **Dependents module names depend on the index.** Gold modules are the directories stacklit now treats as modules, so splitting changes what counts as a module. Gold sets were re-derived by hand from importer files, but the module boundaries come from the tool under test, and the grep arm's inferred modules are mapped through the same boundaries.
5. **The grep line cap decides gs-07.** The corrected gson answer needs importers from several packages; with 50 visible grep lines the window fills before it reaches them and grep fails. With `-grep-lines 200` grep passes it (strict, 1.0 extra module). One task, so it moves the grep dependents rate between 91% and 100%.
6. **Small sample.** 58 tasks, 6 repos; the 11 dependents tasks give very wide intervals. Differences of one or two tasks are noise. Per-repo cells have 7 to 10 tasks and no intervals.
7. **The repo-map arm is an approximation** and the full dump excludes non-code files.
8. **No git history.** Shallow clones mean hot-file and activity signals are not exercised.
9. **Timing is machine specific and was noisy.** Only token counts, success rates and intervals are deterministic. This run's timings were taken while another process kept the 14-core machine at a load average around 40, so they are upper bounds; do not quote them as typical. Peak RSS is less sensitive to load.
10. **Stacklit's numbers exclude generation.** Time and memory are for building the index, a one-time or per-commit cost, not a per-question cost.
11. **Tasks were authored before this rerun.** The 47 localization questions and the oracle patterns date from the first run. They have not been tuned for the new version, but the author also saw the first run's results, so this is not a blind benchmark.

## What this shows and does not show

Shows:

- On these 47 questions, with an oracle grep pattern, grep+read finds the file first more often than any stacklit arm, and stacklit map+lookup is 26 points behind it at found@5 (paired CI excludes 0).
- Stacklit query's output usually contains the file (96% to 98% gold shown) but its path-sorted order hides it from a top-to-bottom reader (found@1 21%).
- Given a sensible reading order, one `query --budget 1000` call beats BM25 at found@1 by about 21 points (paired CI excludes 0) and is level at found@5.
- Rust dependents, Ruby localization and Java module granularity, the gaps of the first run, are largely fixed: query no longer returns nothing for rack (found@5 5 of 8 in tool order, 7 of 8 in reader order), and map+lookup passes both ripgrep dependents tasks.
- The cap of 30 listed files per module costs map+lookup a few points; raising it to 60 gains 5 points at found@1 and found@5, a difference within noise.

Does not show:

- That an LLM agent using stacklit does better or worse than one using grep. No model was run.
- Any token or cost saving over grep or BM25: the intervals overlap.
- That the reader-order result is what users get. The tool prints path order; the reader-order arm is what a good reader could do with the same text.
- Anything about repos outside these six, other languages, or very large repos.
- Anything about query quality for questions that are not "where is X": design, debugging and cross-cutting questions are not in the task set.
- Speed on a quiet machine (see limitation 9).

## Results

The tables below are generated by `go run ./bench -write bench/RESULTS.md` (default configuration). The cap experiment follows them.

<!-- BEGIN GENERATED (go run ./bench -write bench/RESULTS.md) -->

Run on 2026-11-08, darwin/arm64, 14 CPUs, go1.26.5, stacklit stacklit version dev (checkout 4030af4), config: default config.

Parameters: grep output capped at 50 lines (each cut to 200 chars); an arm may open at most 5 files in rank order; bootstrap 10000 resamples, seed 20260101; dependents strict rule allows at most 2 unexpected modules; stacklit map+lookup inspects the top 3 modules; BM25 prints 5 paths and ranks 10; query budgets [1000 2000]; repo-map budgets [1000 4000].

The real `aider` tool is not installed on the machine that produced these numbers, so the repo-map arms are the in-process approximation only.

### Repos and generation cost

| Repo | Lang | Pinned commit | Tasks | Code files | Lines | Modules | Full dump tokens | Fits in context | stacklit.json tokens | derive map tokens | generate first (ms) | generate median (ms) | peak RSS (MB) | files dropped by parser | identical index every run | repo-map build (ms) | BM25 build (ms) |
|---|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|---|---:|---:|
| gin | Go | `75ccf94d60` (v1.10.0) | 10 | 101 | 19537 | 8 | 136665 | yes | 2491 | 137 | 129 | 55 | 82 | 0 | yes | 8 | 14 |
| flask | Python | `2c1b30d050` (3.1.2) | 10 | 101 | 18515 | 14 | 143168 | yes | 2818 | 221 | 3680 | 3664 | 751 | 0 | yes | 16 | 34 |
| hono | TypeScript | `73ff6c0e82` (v4.6.0) | 10 | 356 | 57263 | 77 | 416846 | no | 16022 | 1351 | 863 | 863 | 534 | 0 | yes | 26 | 30 |
| ripgrep | Rust | `4649aa9700` (14.1.1) | 10 | 119 | 52215 | 34 | 423628 | no | 8586 | 477 | 3353 | 3153 | 572 | 0 | yes | 56 | 109 |
| gson | Java | `828a97be0f` (gson-parent-2.11.0) | 9 | 259 | 53490 | 15 | 456223 | no | 5748 | 340 | 1877 | 1440 | 510 | 0 | yes | 38 | 68 |
| rack | Ruby | `0eabeb73b3` (v3.1.8) | 9 | 103 | 19805 | 9 | 162318 | yes | 2391 | 151 | 2706 | 2922 | 473 | 0 | yes | 13 | 34 |

"Fits in context" compares the full dump with a 200000 token window. Generation time is wall clock for `stacklit generate --quiet` (3 runs per repo, cold parse cache, outputs deleted before each run), peak RSS is the largest resident set of the child process. Files dropped by parser is read from one verbose `generate` run; identical index says whether every timed run wrote a byte-identical `stacklit.json`. There is no retry or CPU limit: the index the arms use is the plain output of `generate`. Repo-map and BM25 build times are a single in-process run; their memory is not measured. Tokens are bytes/4.

### Locate tasks

"Where is X implemented?" Lookup tokens are what the arm prints before any file is opened. Tokens to found add the files opened in rank order until the gold file is read; a miss is charged for every file it opened. 58 tasks in total across all kinds.

| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |
|---|---:|---|---|---|---|---|
| grep+read | 35 | 86% [74-97] | 100% [100-100] | 100% [100-100] | 134 [72-219] | 5.1k [3.8k-6.6k] |
| repo-map approx 1000 | 35 | 11% [3-23] | 31% [17-49] | 71% [57-86] | 993 [991-995] | 19k [16k-22k] |
| repo-map approx 4000 | 35 | 11% [3-23] | 31% [17-49] | 91% [80-100] | 3.8k [3.7k-3.9k] | 22k [19k-25k] |
| bm25 top-k | 35 | 66% [49-80] | 94% [86-100] | 94% [86-100] | 37 [30-44] | 8.6k [5.1k-13k] |
| full dump | 35 | n/a | n/a | n/a | 294k [246k-342k] | 294k [246k-342k] |
| stacklit map+lookup | 35 | 51% [34-69] | 74% [60-89] | 89% [77-97] | 1.1k [931-1.3k] | 6.1k [4.6k-7.8k] |
| stacklit query 1000 | 35 | 14% [3-26] | 83% [69-94] | 100% [100-100] | 982 [977-986] | 15k [11k-21k] |
| stacklit query 1000 + reader order | 35 | 86% [74-97] | 100% [100-100] | 100% [100-100] | 982 [977-986] | 6.7k [4.9k-8.9k] |
| stacklit query 2000 | 35 | 9% [0-20] | 57% [40-74] | 100% [100-100] | 1.9k [1.8k-1.9k] | 22k [15k-29k] |
| stacklit query 2000 + reader order | 35 | 80% [66-91] | 97% [91-100] | 100% [100-100] | 1.9k [1.8k-1.9k] | 9.0k [6.6k-12k] |

### Add-feature tasks

"Where would I add Z?" Gold is the file (or directory) a change would start in.

| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |
|---|---:|---|---|---|---|---|
| grep+read | 12 | 100% [100-100] | 100% [100-100] | 100% [100-100] | 96 [41-160] | 8.2k [2.1k-18k] |
| repo-map approx 1000 | 12 | 0% [0-0] | 42% [17-67] | 83% [58-100] | 995 [992-997] | 17k [12k-22k] |
| repo-map approx 4000 | 12 | 0% [0-0] | 42% [17-67] | 100% [100-100] | 3.8k [3.7k-4.0k] | 20k [15k-25k] |
| bm25 top-k | 12 | 33% [8-58] | 75% [50-100] | 75% [50-100] | 36 [27-48] | 16k [6.6k-29k] |
| full dump | 12 | n/a | n/a | n/a | 290k [214k-366k] | 290k [214k-366k] |
| stacklit map+lookup | 12 | 67% [42-92] | 75% [50-100] | 83% [58-100] | 1.0k [794-1.3k] | 5.3k [3.2k-7.5k] |
| stacklit query 1000 | 12 | 42% [17-67] | 75% [50-100] | 83% [58-100] | 985 [978-990] | 16k [7.7k-27k] |
| stacklit query 1000 + reader order | 12 | 58% [33-83] | 75% [50-100] | 83% [58-100] | 985 [978-990] | 8.4k [4.5k-13k] |
| stacklit query 2000 | 12 | 25% [0-50] | 58% [33-83] | 92% [75-100] | 1.9k [1.8k-1.9k] | 22k [9.8k-36k] |
| stacklit query 2000 + reader order | 12 | 58% [33-83] | 75% [50-100] | 92% [75-100] | 1.9k [1.8k-1.9k] | 10k [5.6k-17k] |

### All localization tasks (locate plus add-feature)

| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |
|---|---:|---|---|---|---|---|
| grep+read | 47 | 89% [81-98] | 100% [100-100] | 100% [100-100] | 124 [75-188] | 5.9k [3.8k-8.7k] |
| repo-map approx 1000 | 47 | 9% [2-17] | 34% [21-49] | 74% [62-87] | 994 [992-995] | 18k [16k-21k] |
| repo-map approx 4000 | 47 | 9% [2-17] | 34% [21-49] | 94% [85-100] | 3.8k [3.8k-3.9k] | 21k [19k-24k] |
| bm25 top-k | 47 | 57% [43-72] | 89% [81-98] | 89% [81-98] | 37 [31-43] | 11k [6.7k-15k] |
| full dump | 47 | n/a | n/a | n/a | 293k [252k-334k] | 293k [252k-334k] |
| stacklit map+lookup | 47 | 55% [40-70] | 74% [62-87] | 87% [77-96] | 1.1k [945-1.3k] | 5.9k [4.6k-7.3k] |
| stacklit query 1000 | 47 | 21% [11-34] | 81% [68-91] | 96% [89-100] | 983 [979-986] | 16k [12k-20k] |
| stacklit query 1000 + reader order | 47 | 79% [66-89] | 94% [85-100] | 96% [89-100] | 983 [979-986] | 7.2k [5.3k-9.1k] |
| stacklit query 2000 | 47 | 13% [4-23] | 57% [43-72] | 98% [94-100] | 1.9k [1.8k-1.9k] | 22k [16k-28k] |
| stacklit query 2000 + reader order | 47 | 74% [62-87] | 91% [83-98] | 98% [94-100] | 1.9k [1.8k-1.9k] | 9.4k [7.1k-12k] |

### Paired differences (all localization tasks)

First arm minus second arm on the same tasks, in percentage points, with a 95% bootstrap interval over tasks. An interval that excludes 0 is a difference larger than task-sampling noise.

| First arm | Second arm | found@1 difference | found@5 difference |
|---|---|---|---|
| stacklit query 1000 + reader order | bm25 top-k | +21 [+9, +34] | +4 [-4, +13] |
| stacklit query 1000 + reader order | grep+read | -11 [-26, +6] | -6 [-15, +0] |
| stacklit query 1000 | bm25 top-k | -36 [-53, -17] | -9 [-21, +4] |
| stacklit query 1000 | grep+read | -68 [-81, -55] | -19 [-32, -9] |
| stacklit map+lookup | bm25 top-k | -2 [-21, +17] | -15 [-30, +0] |
| stacklit map+lookup | grep+read | -34 [-49, -17] | -26 [-38, -13] |
| bm25 top-k | grep+read | -32 [-49, -15] | -11 [-19, -2] |

### Dependents (impact) tasks

Lenient: every expected module is reported (short module names accepted, extras ignored). Strict: every expected module is reported by exact name and at most 2 unexpected modules are reported. The repo-map, BM25 and full-dump arms have no dependents procedure and are marked n/a.

| Arm | Tasks | lenient pass [95% CI] | strict pass [95% CI] | mean extra modules | lookup tokens [95% CI] |
|---|---:|---|---|---:|---|
| grep+read | 11 | 91% [73-100] | 91% [73-100] | 0.5 | 241 [85-491] |
| repo-map approx 1000 | 11 | n/a | n/a | n/a | n/a |
| repo-map approx 4000 | 11 | n/a | n/a | n/a | n/a |
| bm25 top-k | 11 | n/a | n/a | n/a | n/a |
| full dump | 11 | n/a | n/a | n/a | n/a |
| stacklit map+lookup | 11 | 73% [45-100] | 64% [36-91] | 0.5 | 679 [452-950] |
| stacklit query 1000 | 11 | 27% [0-55] | 18% [0-45] | 2.3 | 976 [964-986] |
| stacklit query 1000 + reader order | 11 | n/a | n/a | n/a | n/a |
| stacklit query 2000 | 11 | 36% [9-64] | 9% [0-27] | 3.1 | 1.8k [1.7k-2.0k] |
| stacklit query 2000 + reader order | 11 | n/a | n/a | n/a | n/a |

### Per repo

Found@5 over localization tasks, as hits/tasks, then mean tokens to found.

| Arm | gin (Go) | flask (Python) | hono (TypeScript) | ripgrep (Rust) | gson (Java) | rack (Ruby) |
|---|---|---|---|---|---|---|
| grep+read | 7/7, 1.7k | 8/8, 6.9k | 8/8, 1.3k | 8/8, 15k | 8/8, 6.0k | 8/8, 4.0k |
| repo-map approx 1000 | 2/7, 7.4k | 4/8, 17k | 1/8, 21k | 2/8, 25k | 2/8, 22k | 5/8, 16k |
| repo-map approx 4000 | 2/7, 10k | 4/8, 20k | 1/8, 24k | 2/8, 28k | 2/8, 25k | 5/8, 18k |
| bm25 top-k | 7/7, 4.5k | 7/8, 10k | 8/8, 508 | 7/8, 22k | 7/8, 18k | 6/8, 6.8k |
| full dump | n/a, 137k | n/a, 143k | n/a, 417k | n/a, 424k | n/a, 456k | n/a, 162k |
| stacklit map+lookup | 6/7, 2.2k | 5/8, 7.3k | 7/8, 2.4k | 7/8, 12k | 5/8, 5.7k | 5/8, 5.8k |
| stacklit query 1000 | 7/7, 8.4k | 7/8, 16k | 6/8, 6.9k | 6/8, 37k | 7/8, 14k | 5/8, 9.9k |
| stacklit query 1000 + reader order | 7/7, 5.2k | 8/8, 8.5k | 8/8, 1.8k | 7/8, 10k | 7/8, 8.8k | 7/8, 8.2k |
| stacklit query 2000 | 4/7, 11k | 7/8, 20k | 4/8, 9.2k | 5/8, 60k | 5/8, 20k | 2/8, 9.8k |
| stacklit query 2000 + reader order | 7/7, 6.3k | 8/8, 9.4k | 8/8, 2.9k | 7/8, 16k | 7/8, 9.3k | 6/8, 12k |

Dependents tasks per repo, strict/lenient passes:

| Arm | gin | flask | hono | ripgrep | gson | rack |
|---|---|---|---|---|---|---|
| grep+read | 3/3, 3/3 | 2/2, 2/2 | 2/2, 2/2 | 2/2, 2/2 | 0/1, 0/1 | 1/1, 1/1 |
| stacklit map+lookup | 3/3, 3/3 | 1/2, 1/2 | 1/2, 2/2 | 2/2, 2/2 | 0/1, 0/1 | 0/1, 0/1 |
| stacklit query 1000 | 2/3, 2/3 | 0/2, 1/2 | 0/2, 0/2 | 0/2, 0/2 | 0/1, 0/1 | 0/1, 0/1 |
| stacklit query 2000 | 1/3, 2/3 | 0/2, 1/2 | 0/2, 1/2 | 0/2, 0/2 | 0/1, 0/1 | 0/1, 0/1 |

### Sensitivity to the token approximation

Mean tokens to found over all localization tasks under bytes/4 and under the second estimator (word and punctuation pieces x 1.3).

| Arm | bytes/4 | pieces x 1.3 |
|---|---:|---:|
| grep+read | 5.9k | 7.3k |
| repo-map approx 1000 | 18k | 23k |
| repo-map approx 4000 | 21k | 26k |
| bm25 top-k | 11k | 12k |
| full dump | 293k | 386k |
| stacklit map+lookup | 5.9k | 7.3k |
| stacklit query 1000 | 16k | 19k |
| stacklit query 1000 + reader order | 7.2k | 8.6k |
| stacklit query 2000 | 22k | 27k |
| stacklit query 2000 + reader order | 9.4k | 11k |

### Per task

Localization cells: rank of the first gold file in the arm's list, `-` when it is not in the first 5 files it was allowed to open. Dependents cells: `S` strict pass, `L` lenient only, `-` fail, `n/a` not applicable. Full dump is omitted (it always contains the answer).

| Task | Kind | grep+read | repo-map approx 1000 | repo-map approx 4000 | bm25 top-k | stacklit map+lookup | stacklit query 1000 | stacklit query 1000 + reader order | stacklit query 2000 | stacklit query 2000 + reader order |
|---|---|---|---|---|---|---|---|---|---|---|
| gin-01 | locate | 1 | - | - | 1 | 1 | 3 | 1 | - | 1 |
| gin-02 | locate | 1 | - | - | 1 | 1 | 1 | 1 | 1 | 1 |
| gin-03 | locate | 1 | 5 | 5 | 1 | - | 2 | 1 | 2 | 1 |
| gin-04 | locate | 1 | - | - | 1 | 1 | 1 | 1 | 1 | 1 |
| gin-05 | locate | 1 | - | - | 1 | 1 | 5 | 1 | - | 1 |
| gin-06 | dependents | S | n/a | n/a | n/a | S | S | n/a | S | n/a |
| gin-07 | dependents | S | n/a | n/a | n/a | S | - | n/a | - | n/a |
| gin-08 | dependents | S | n/a | n/a | n/a | S | S | n/a | L | n/a |
| gin-09 | where-to-add | 1 | 2 | 2 | 3 | 1 | 3 | 2 | - | 2 |
| gin-10 | where-to-add | 1 | - | - | 2 | 1 | 1 | 2 | 1 | 2 |
| flask-01 | locate | 1 | 2 | 2 | 3 | 1 | 3 | 1 | 3 | 1 |
| flask-02 | locate | 1 | 1 | 1 | 1 | 1 | 3 | 1 | 3 | 1 |
| flask-03 | locate | 1 | 1 | 1 | 1 | - | 2 | 1 | 3 | 1 |
| flask-04 | locate | 2 | 2 | 2 | 1 | - | 2 | 1 | 2 | 1 |
| flask-05 | locate | 1 | - | - | 4 | - | - | 2 | - | 2 |
| flask-06 | locate | 1 | - | - | 1 | 1 | 1 | 1 | 3 | 1 |
| flask-07 | dependents | S | n/a | n/a | n/a | S | L | n/a | L | n/a |
| flask-08 | dependents | S | n/a | n/a | n/a | - | - | n/a | - | n/a |
| flask-09 | where-to-add | 1 | - | - | - | 1 | 4 | 1 | 5 | 1 |
| flask-10 | where-to-add | 1 | - | - | 1 | 1 | 1 | 1 | 2 | 1 |
| hono-01 | locate | 2 | - | - | 1 | 1 | - | 1 | - | 1 |
| hono-02 | locate | 2 | - | - | 1 | 1 | 3 | 1 | 5 | 1 |
| hono-03 | locate | 2 | - | - | 1 | - | 3 | 1 | - | 1 |
| hono-04 | locate | 1 | - | - | 1 | 1 | 5 | 1 | - | 1 |
| hono-05 | locate | 1 | - | - | 1 | 1 | 2 | 1 | 2 | 2 |
| hono-06 | locate | 1 | - | - | 1 | 1 | 2 | 1 | 5 | 1 |
| hono-07 | dependents | S | n/a | n/a | n/a | S | - | n/a | L | n/a |
| hono-08 | dependents | S | n/a | n/a | n/a | L | - | n/a | - | n/a |
| hono-09 | where-to-add | 1 | 4 | 4 | 1 | 1 | - | 1 | - | 1 |
| hono-10 | where-to-add | 1 | - | - | 1 | 1 | 1 | 1 | 1 | 1 |
| rg-01 | locate | 1 | - | - | 1 | 2 | 4 | 1 | 5 | 1 |
| rg-02 | locate | 1 | 5 | 5 | 1 | 2 | 2 | 1 | 3 | 1 |
| rg-03 | locate | 1 | - | - | 3 | 2 | 4 | 1 | - | 1 |
| rg-04 | locate | 1 | - | - | 2 | 1 | 1 | 1 | 1 | 1 |
| rg-05 | locate | 1 | - | - | - | 2 | - | 1 | - | 3 |
| rg-06 | locate | 1 | - | - | 1 | 3 | 2 | 1 | 4 | 1 |
| rg-07 | dependents | S | n/a | n/a | n/a | S | - | n/a | - | n/a |
| rg-08 | dependents | S | n/a | n/a | n/a | S | - | n/a | - | n/a |
| rg-09 | where-to-add | 1 | - | - | 5 | - | - | - | 3 | - |
| rg-10 | where-to-add | 1 | 2 | 2 | 1 | 2 | 3 | 1 | - | 1 |
| gs-01 | locate | 1 | - | - | 3 | - | 3 | 1 | 5 | 1 |
| gs-02 | locate | 1 | - | - | 2 | 3 | 3 | 1 | 5 | 1 |
| gs-03 | locate | 1 | 1 | 1 | 5 | 1 | 4 | 1 | 5 | 1 |
| gs-04 | locate | 1 | - | - | 2 | - | 2 | 2 | 5 | 4 |
| gs-05 | locate | 1 | 1 | 1 | 1 | 1 | 5 | 1 | - | 1 |
| gs-06 | locate | 1 | - | - | 1 | 2 | 4 | 1 | - | 1 |
| gs-07 | dependents | - | n/a | n/a | n/a | - | - | n/a | - | n/a |
| gs-08 | where-to-add | 1 | - | - | 2 | 1 | 1 | 1 | 2 | 1 |
| gs-09 | where-to-add | 1 | - | - | - | - | - | - | - | - |
| rk-01 | locate | 1 | 3 | 3 | 4 | 1 | 4 | 2 | - | 1 |
| rk-02 | locate | 2 | 4 | 4 | - | 2 | 4 | 1 | - | 2 |
| rk-03 | locate | 1 | - | - | 1 | - | - | 1 | - | 1 |
| rk-04 | locate | 1 | - | - | 1 | 1 | 1 | 2 | 4 | 2 |
| rk-05 | locate | 1 | - | - | 1 | - | - | 1 | - | 1 |
| rk-06 | locate | 1 | 4 | 4 | 2 | 1 | - | 3 | - | - |
| rk-07 | dependents | S | n/a | n/a | n/a | - | - | n/a | - | n/a |
| rk-08 | where-to-add | 1 | 4 | 4 | - | - | 1 | - | 1 | - |
| rk-09 | where-to-add | 1 | 3 | 3 | 3 | 1 | 4 | 1 | - | 1 |


<!-- END GENERATED -->

## Module file cap

The same run with `STACKLIT_MAX_MODULE_FILES=60` (default 30). Only arms that read module file lists can change; grep, BM25, repo-map and the query arms are identical, because query reads the symbol graph rather than module file lists. Generation cost is nearly identical (index size grows by under 2%).

| Arm (47 localization tasks) | found@1 cap 30 | found@1 cap 60 | found@5 cap 30 | found@5 cap 60 | tokens to found cap 30 | tokens to found cap 60 |
|---|---|---|---|---|---|---|
| stacklit map+lookup | 55% [40-70] | 60% [45-74] | 74% [62-87] | 79% [66-89] | 5.9k [4.6k-7.3k] | 5.9k [4.7k-7.3k] |
| bm25 top-k | 57% [43-72] | 57% [43-72] | 89% [81-98] | 89% [81-98] | 11k [6.7k-15k] | 11k [6.7k-15k] |
| grep+read | 89% [81-98] | 89% [81-98] | 100% [100-100] | 100% [100-100] | 5.9k [3.8k-8.7k] | 5.9k [3.8k-8.7k] |

Dependents under cap 60: map+lookup 73% [45-100] lenient and 64% [36-91] strict, unchanged. The paired differences versus BM25 and grep for map+lookup are -2 [-21, +17] and -34 [-49, -17] at found@1 with cap 30, and +2 [-17, +21] and -30 [-45, -15] with cap 60.
