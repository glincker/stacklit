# Benchmark results

How many tokens does a coding agent read to answer a navigation question, with and without a stacklit map? This page answers that with a deterministic, offline proxy on three pinned public repos. It makes no LLM calls. Reproduce it with `go run ./bench -write bench/RESULTS.md` (see [README.md](README.md)).

## Summary

- **The index is small, the map is smaller, and both grow with module count.** `stacklit.json` was 2,045 to 11,814 tokens on the three repos. The `stacklit derive` map was 122 tokens (6 modules), 164 (9 modules) and 1,122 (72 modules). The "about 250 tokens" figure holds for repos with roughly 10 to 25 modules, not for every repo.
- **When the exact symbol name is known, grep is cheaper to look up.** Mean lookup cost was lower for the grep path than for map plus module lookups (see the aggregate table). A grep for a unique identifier returns a handful of lines.
- **Stacklit wins on end-to-end cost when the baseline has to read files to confirm.** With the baseline reading its top 3 ranked files and the stacklit path reading 1, the mean per task was about 2.4x lower for stacklit across all 30 tasks, ranging from roughly parity on hono to several times lower on gin. This figure depends on the read assumptions below and should not be quoted without them.
- **Stacklit answered fewer questions correctly in this setup.** The grep + read path reached the verified answer on all 30 tasks; the stacklit path on 25. The misses are specific and listed under "Misses": unexported symbols and truncated export lists, and dependency edges that are wrong or incomplete (relative imports resolved to the wrong module, edges that come from test files, short module names that are ambiguous).
- **Generation is fast for Go and slower for Python and TypeScript.** Median `generate` time was tens of milliseconds on gin and seconds on flask and hono, with peak memory of roughly 1 to 2 GB on the latter two. See the table for exact figures on your run. This is a known cost of the tree-sitter parsers and is on the [roadmap](../ROADMAP.md).

## Methodology

**Repos.** Three public repos in three languages, pinned by full commit SHA in `repos.json`: gin (Go, v1.10.0), flask (Python, 3.1.2) and hono (TypeScript, v4.6.0). Each is fetched with `git fetch --depth 1 <sha>` into a temporary directory outside this repository. Because clones are shallow, the git-activity part of the index (hot files) is empty or minimal; it does not affect the questions asked here.

**Tasks.** Ten per repo (30 total): 17 "where is X implemented" (`locate`), 7 "what depends on Y" (`dependents`) and 6 "where would I add Z" (`where-to-add`). The questions and answers are in `tasks/*.json`. Answers were verified by hand with `grep` on the pinned checkouts, not taken from stacklit's own output.

**Token approximation.** Tokens are `ceil(bytes / 4)` of the text an agent would read. This is the common rule of thumb for English and code; real tokenizers differ by model and typically produce more tokens for JSON and punctuation-heavy text. To check how much that matters, the report also computes a second estimate (word and punctuation pieces times 1.3) and prints both. The ratios barely move between the two. No tokenizer is bundled, so no claim here is a billed-token measurement.

**Baseline: grep + read.** A simulated agent runs `grep -rn` for the task's pattern over the repo's source files (extensions per language, test files and vendored code excluded, which favors the baseline). It sees the first 50 output lines, each cut to 200 characters. For `locate` and `where-to-add` tasks it then reads, in full, the 3 files ranked highest by definition-like matches (`func`, `def`, `class`, `type` and similar on the matching line) and then by match count. Cost is the grep output plus the files read. A baseline task is "found" when the verified file is among the 3 files read. For `dependents` tasks the grep output itself is the answer: hit files are mapped to their modules, and the task is "found" when every expected dependent is present.

**Stacklit path.** The simulated agent starts with the `stacklit derive` map (counted in full on every task, even though in practice it is injected once into the agent's config). It then looks up the best-matching modules in `stacklit.json`: modules are scored on the task's hand-written `terms` (name hit 3, purpose hit 2, hit in exports, type definitions or file names 1), the top 3 module entries are counted as read, and a task is "found" when the verified module is among them. For `dependents` tasks the agent reads the one target module's entry and its `depended_by` list is the answer, graded for recall and precision. The "end-to-end" cost adds one confirming file read from the top module (file name overlapping the terms, otherwise the first non-test file listed).

**Generation time and memory.** Wall-clock time of `stacklit generate --quiet`, run 5 times per repo with the output files deleted first. The first run and the median are reported. Peak memory is the maximum resident set size reported by the OS for the child process across the runs (not available on Windows).

## Limitations

Read these before quoting any number.

1. **It is a proxy, not an agent.** Real agents grep several times, read partial files, and make mistakes. No model is involved, so reasoning quality, tool-call overhead and conversation cost are not measured.
2. **The task terms are hand-written.** Choosing `terms` for the stacklit path is a judgment, and a real agent might pick better or worse queries. The stacklit ranking also searches exports and file names, which is more than the shipped MCP `find_module` tool does (it matches module names and purpose only and returns up to 5 matches in arbitrary order). Real MCP usage may do worse than this simulation on locate tasks.
3. **Baseline choices drive the end-to-end ratio.** Reading 3 files and ranking by definition-like lines are assumptions. Reading fewer files makes the baseline cheaper; reading whole large files makes it more expensive. The `-read-files` and `-grep-lines` flags let you vary them.
4. **Module granularity limits stacklit.** gin has only 6 modules and its root package holds 40 files, so "the answer is the root module" is a coarse answer. The harness grades modules, not files, for the stacklit path.
5. **The index truncates by default.** Each module keeps at most 10 exports and about 20 listed file names (see `max_exports` in [USAGE.md](../USAGE.md)). Some symbols are therefore not in the index, which is why two gin locate tasks miss. Raising `max_exports` was not tried here.
6. **Small sample.** 30 tasks on 3 repos in 3 languages. Differences of one or two tasks are noise. Other repos, other languages and larger monorepos may behave differently.
7. **No git history.** Shallow clones mean the hot-files and activity signals are not exercised.
8. **Timing is machine specific.** Only token counts and success rates are deterministic. The generated section records the machine and date of the run that produced it.
9. **Stacklit's own numbers are index-generation numbers.** Time and memory are for building the index, which is a one-time or per-commit cost, not a per-question cost.

## Results

The tables below are generated. They are rewritten by `go run ./bench -write bench/RESULTS.md`.

<!-- BEGIN GENERATED (go run ./bench -write bench/RESULTS.md) -->

Run on 2026-11-08, darwin/arm64, 14 CPUs, go1.26.5, stacklit stacklit version dev (checkout 712df86).

Baseline parameters: grep output capped at 50 lines (each cut to 200 chars), then the top 3 ranked files read in full.

### Index size and generation cost

| Repo | Lang | Pinned commit | Files | Lines | Modules | stacklit.json bytes | stacklit.json tokens | derive map tokens | generate first (ms) | generate median (ms) | peak RSS (MB) |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| gin | Go | `75ccf94d60` (v1.10.0) | 93 | 19161 | 6 | 9638 | 2410 | 122 | 300 | 39 | 18 |
| flask | Python | `2c1b30d050` (3.1.2) | 86 | 17909 | 9 | 8178 | 2045 | 164 | 4432 | 3724 | 1108 |
| hono | TypeScript | `73ff6c0e82` (v4.6.0) | 325 | 55896 | 72 | 47256 | 11814 | 1122 | 2328 | 2328 | 1723 |

Times are wall clock for `stacklit generate --quiet` (5 runs per repo, outputs deleted before each run). Tokens are bytes/4.

### Per-task results

Lookup = tokens to locate the answer. End-to-end = lookup plus the confirming reads. Found = the hand-verified answer was reached.

**gin**

| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |
|---|---|---:|---:|---|---:|---:|---|
| gin-01 | locate | 96 | 11777 | yes | 122 | 122 | no |
| gin-02 | locate | 151 | 1110 | yes | 622 | 1581 | yes |
| gin-03 | locate | 195 | 11446 | yes | 427 | 720 | yes |
| gin-04 | locate | 88 | 2376 | yes | 418 | 1451 | yes |
| gin-05 | locate | 100 | 1401 | yes | 122 | 122 | no |
| gin-06 | dependents | 61 | 61 | yes | 193 | 193 | yes |
| gin-07 | dependents | 93 | 93 | yes | 199 | 199 | yes |
| gin-08 | dependents | 23 | 23 | yes | 427 | 427 | yes |
| gin-09 | where-to-add | 343 | 11331 | yes | 927 | 1221 | yes |
| gin-10 | where-to-add | 204 | 11904 | yes | 418 | 1451 | yes |

**flask**

| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |
|---|---|---:|---:|---|---:|---:|---|
| flask-01 | locate | 215 | 8654 | yes | 497 | 2017 | yes |
| flask-02 | locate | 61 | 9357 | yes | 497 | 9793 | yes |
| flask-03 | locate | 112 | 6302 | yes | 729 | 2130 | yes |
| flask-04 | locate | 107 | 11550 | yes | 729 | 2130 | yes |
| flask-05 | locate | 54 | 25344 | yes | 319 | 9844 | yes |
| flask-06 | locate | 130 | 21757 | yes | 497 | 4367 | yes |
| flask-07 | dependents | 190 | 190 | yes | 396 | 396 | yes |
| flask-08 | dependents | 252 | 252 | yes | 319 | 319 | no |
| flask-09 | where-to-add | 167 | 9463 | yes | 497 | 9793 | yes |
| flask-10 | where-to-add | 220 | 2540 | yes | 396 | 2716 | yes |

**hono**

| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |
|---|---|---:|---:|---|---:|---:|---|
| hono-01 | locate | 198 | 1888 | yes | 1228 | 1248 | yes |
| hono-02 | locate | 190 | 6334 | yes | 1253 | 1274 | yes |
| hono-03 | locate | 909 | 3091 | yes | 1753 | 2453 | yes |
| hono-04 | locate | 51 | 1088 | yes | 1410 | 2447 | yes |
| hono-05 | locate | 161 | 3464 | yes | 1432 | 2404 | yes |
| hono-06 | locate | 36 | 705 | yes | 1688 | 1741 | yes |
| hono-07 | dependents | 57 | 57 | yes | 1228 | 1228 | no |
| hono-08 | dependents | 42 | 42 | yes | 1199 | 1199 | no |
| hono-09 | where-to-add | 92 | 1190 | yes | 1283 | 2381 | yes |
| hono-10 | where-to-add | 15 | 33 | yes | 1354 | 1429 | yes |

### Aggregate

Mean tokens per task. "Query only" excludes the derive map, which is a standing cost when it is injected into the agent's config file. Ratios are grep tokens divided by stacklit tokens: above 1.0x stacklit is cheaper, below 1.0x it costs more.

| Scope | Tasks | grep lookup | grep+read | map+query lookup | query only | map+query+read | grep+read found | stacklit found | lookup ratio (grep / stacklit) | end-to-end ratio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| gin | 10 | 135 | 5152 | 388 | 266 | 749 | 10/10 | 8/10 | 0.3x | 6.9x |
| flask | 10 | 151 | 9541 | 488 | 324 | 4351 | 10/10 | 9/10 | 0.3x | 2.2x |
| hono | 10 | 175 | 1789 | 1383 | 261 | 1780 | 10/10 | 8/10 | 0.1x | 1.0x |
| all repos, locate | 17 | 168 | 7508 | 808 | 319 | 2697 | 17/17 | 15/17 | 0.2x | 2.8x |
| all repos, dependents | 7 | 103 | 103 | 566 | 147 | 566 | 7/7 | 4/7 | 0.2x | 0.2x |
| all repos, where-to-add | 6 | 174 | 6077 | 813 | 344 | 3165 | 6/6 | 6/6 | 0.2x | 1.9x |
| **all repos, all tasks** | 30 | 154 | 5494 | 753 | 284 | 2293 | 30/30 | 25/30 | 0.2x | 2.4x |

### Sensitivity to the token approximation

Same totals under the second estimator (word and punctuation pieces x 1.3) instead of bytes/4.

| Estimator | mean grep lookup | mean map+query lookup | lookup ratio | mean grep+read | mean map+query+read | end-to-end ratio |
|---|---:|---:|---:|---:|---:|---:|
| bytes/4 | 154 | 753 | 0.2x | 5494 | 2293 | 2.4x |
| pieces x 1.3 | 228 | 984 | 0.2x | 6524 | 2732 | 2.4x |

### Misses

Every task where either procedure did not reach the verified answer.

- flask-08 (What depends on the flask.sansio package?): expected [src/flask src/flask/json]. grep path found [src/flask src/flask/json src/flask/sansio] (recall 1.00, precision 0.67). stacklit depended_by [src/flask] (recall 0.50, precision 1.00).
- gin-01 (Where is the radix tree route lookup (getValue) implemented?): expected [tree.go] / [root]. grep read [tree.go gin.go] (found in grep output: yes). stacklit top modules [].
- gin-05 (Where is the panic recovery middleware implemented?): expected [recovery.go] / [root]. grep read [recovery.go] (found in grep output: yes). stacklit top modules [].
- hono-07 (What depends on the trie-router module?): expected [src src/preset src/middleware/combine]. grep path found [src src/middleware/combine src/preset] (recall 1.00, precision 1.00). stacklit depended_by [src src/preset src/router/smart-router] (recall 0.67, precision 0.67).
- hono-08 (What depends on the cookie helper?): expected [src/middleware/jwt src/validator]. grep path found [src/middleware/jwt src/validator] (recall 1.00, precision 1.00). stacklit depended_by [src src/validator] (recall 0.50, precision 0.50).

<!-- END GENERATED -->
