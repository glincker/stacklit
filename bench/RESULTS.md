# Benchmark results

How many tokens does a coding agent read to answer a navigation question, with and without a stacklit map? This page answers that with a deterministic, offline proxy on three pinned public repos. It makes no LLM calls. Reproduce it with `go run ./bench -write bench/RESULTS.md` (see [README.md](README.md)).

## Summary

- **The index is small, the map is smaller, and both grow with module count.** `stacklit.json` was 2,524 to 16,126 tokens on the three repos. The `stacklit derive` map was 137 tokens (8 modules), 221 (14 modules) and 1,372 (79 modules). Module counts rose against the earlier run because JSON, YAML and TOML files are now indexed.
- **When the exact symbol name is known, grep is cheaper to look up.** Mean lookup cost was lower for the grep path than for map plus module lookups (see the aggregate table). A grep for a unique identifier returns a handful of lines.
- **Stacklit wins on end-to-end cost when the baseline has to read files to confirm.** With the baseline reading its top 3 ranked files and the stacklit path reading 1, the mean per task was about 2.3x lower for stacklit across all 30 tasks, ranging from roughly parity on hono to several times lower on gin. This figure depends on the read assumptions below and should not be quoted without them.
- **Stacklit answered fewer questions correctly in this setup.** The grep + read path reached the verified answer on all 30 tasks; the map-and-lookup stacklit path on 27 and the graph query path on 28. The misses are specific and listed under "Misses": unexported symbols and truncated export lists, and dependency edges that are wrong or incomplete (relative imports resolved to the wrong module, edges that come from test files, short module names that are ambiguous).
- **Generation is fast for Go and slower for Python and TypeScript.** Median `generate` time was tens of milliseconds on gin and seconds on flask and hono, with peak memory of about 1.6 GB on flask and 0.9 GB on hono (down from 2.2 GB and 2.4 GB before releasing parse trees and gating concurrent parses). A single 50 KB Python file costs about 350 MB inside the tree-sitter library, so the under 300 MB goal was not reached; see the [roadmap](../ROADMAP.md).
- **The graph query path (`stacklit query`) is a third arm.** One call with the question text and a 2,000 token budget returned the verified file on 28 of 30 tasks at about 1,650 tokens per task, versus about 5,500 for grep plus reads and 2,400 for map plus lookup plus one read. It does worse than grep on tokens when the symbol name is known, and it missed two hono dependents tasks. See the "Graph query arm" section.

## Methodology

**Repos.** Three public repos in three languages, pinned by full commit SHA in `repos.json`: gin (Go, v1.10.0), flask (Python, 3.1.2) and hono (TypeScript, v4.6.0). Each is fetched with `git fetch --depth 1 <sha>` into a temporary directory outside this repository. Because clones are shallow, the git-activity part of the index (hot files) is empty or minimal; it does not affect the questions asked here.

**Tasks.** Ten per repo (30 total): 17 "where is X implemented" (`locate`), 7 "what depends on Y" (`dependents`) and 6 "where would I add Z" (`where-to-add`). The questions and answers are in `tasks/*.json`. Answers were verified by hand with `grep` on the pinned checkouts, not taken from stacklit's own output.

**Token approximation.** Tokens are `ceil(bytes / 4)` of the text an agent would read. This is the common rule of thumb for English and code; real tokenizers differ by model and typically produce more tokens for JSON and punctuation-heavy text. To check how much that matters, the report also computes a second estimate (word and punctuation pieces times 1.3) and prints both. The ratios barely move between the two. No tokenizer is bundled, so no claim here is a billed-token measurement.

**Baseline: grep + read.** A simulated agent runs `grep -rn` for the task's pattern over the repo's source files (extensions per language, test files and vendored code excluded, which favors the baseline). It sees the first 50 output lines, each cut to 200 characters. For `locate` and `where-to-add` tasks it then reads, in full, the 3 files ranked highest by definition-like matches (`func`, `def`, `class`, `type` and similar on the matching line) and then by match count. Cost is the grep output plus the files read. A baseline task is "found" when the verified file is among the 3 files read. For `dependents` tasks the grep output itself is the answer: hit files are mapped to their modules, and the task is "found" when every expected dependent is present.

**Stacklit path.** The simulated agent starts with the `stacklit derive` map (counted in full on every task, even though in practice it is injected once into the agent's config). It then looks up the best-matching modules in `stacklit.json`: modules are scored on the task's hand-written `terms` (name hit 3, purpose hit 2, hit in exports, type definitions or file names 1), the top 3 module entries are counted as read, and a task is "found" when the verified module is among them. For `dependents` tasks the agent reads the one target module's entry and its `depended_by` list is the answer, graded for recall and precision. The "end-to-end" cost adds one confirming file read from the top module (file name overlapping the terms, otherwise the first non-test file listed).

**Graph query arm.** `stacklit graph` is run once per repo, then `stacklit query "<question>" --budget 2000` is called once per task with the question text as written. Cost is the full text output. Found means a returned file matches the verified path (dependents: every expected module has a returned file). Change the budget with `-query-budget`.

**Generation time and memory.** Wall-clock time of `stacklit generate --quiet` with the parse cache disabled (`STACKLIT_NO_CACHE=1`, so each run is cold), run 5 times per repo with the output files deleted first. The first run and the median are reported. Peak memory is the maximum resident set size reported by the OS for the child process across the runs (not available on Windows).

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

Run on 2026-11-08, darwin/arm64, 14 CPUs, go1.26.5, stacklit stacklit version dev (checkout 0c2125d).

Baseline parameters: grep output capped at 50 lines (each cut to 200 chars), then the top 3 ranked files read in full.

### Index size and generation cost

| Repo | Lang | Pinned commit | Files | Lines | Modules | stacklit.json bytes | stacklit.json tokens | derive map tokens | generate first (ms) | generate median (ms) | peak RSS (MB) |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| gin | Go | `75ccf94d60` (v1.10.0) | 101 | 19537 | 8 | 10096 | 2524 | 137 | 295 | 40 | 90 |
| flask | Python | `2c1b30d050` (3.1.2) | 101 | 18515 | 14 | 10722 | 2681 | 221 | 3373 | 2328 | 1574 |
| hono | TypeScript | `73ff6c0e82` (v4.6.0) | 360 | 57307 | 79 | 64501 | 16126 | 1372 | 662 | 606 | 902 |

Times are wall clock for `stacklit generate --quiet` (5 runs per repo, outputs deleted before each run). Tokens are bytes/4.

### Per-task results

Lookup = tokens to locate the answer. End-to-end = lookup plus the confirming reads. Found = the hand-verified answer was reached.

**gin**

| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |
|---|---|---:|---:|---|---:|---:|---|
| gin-01 | locate | 96 | 11777 | yes | 137 | 137 | no |
| gin-02 | locate | 151 | 1110 | yes | 635 | 1595 | yes |
| gin-03 | locate | 195 | 11446 | yes | 442 | 736 | yes |
| gin-04 | locate | 88 | 2376 | yes | 433 | 1466 | yes |
| gin-05 | locate | 100 | 1401 | yes | 137 | 137 | no |
| gin-06 | dependents | 61 | 61 | yes | 209 | 209 | yes |
| gin-07 | dependents | 93 | 93 | yes | 215 | 215 | yes |
| gin-08 | dependents | 23 | 23 | yes | 442 | 442 | yes |
| gin-09 | where-to-add | 343 | 11331 | yes | 940 | 1234 | yes |
| gin-10 | where-to-add | 204 | 11904 | yes | 433 | 1466 | yes |

**flask**

| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |
|---|---|---:|---:|---|---:|---:|---|
| flask-01 | locate | 215 | 8654 | yes | 555 | 2075 | yes |
| flask-02 | locate | 61 | 9357 | yes | 555 | 9851 | yes |
| flask-03 | locate | 112 | 6302 | yes | 846 | 2246 | yes |
| flask-04 | locate | 107 | 11550 | yes | 846 | 2246 | yes |
| flask-05 | locate | 54 | 25344 | yes | 377 | 9901 | yes |
| flask-06 | locate | 130 | 21757 | yes | 555 | 4425 | yes |
| flask-07 | dependents | 190 | 190 | yes | 453 | 453 | yes |
| flask-08 | dependents | 252 | 252 | yes | 377 | 377 | no |
| flask-09 | where-to-add | 167 | 9463 | yes | 555 | 9851 | yes |
| flask-10 | where-to-add | 220 | 2540 | yes | 453 | 2773 | yes |

**hono**

| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |
|---|---|---:|---:|---|---:|---:|---|
| hono-01 | locate | 198 | 1888 | yes | 1487 | 1507 | yes |
| hono-02 | locate | 190 | 6334 | yes | 1510 | 1531 | yes |
| hono-03 | locate | 909 | 3091 | yes | 2038 | 2738 | yes |
| hono-04 | locate | 51 | 1088 | yes | 1859 | 2896 | yes |
| hono-05 | locate | 161 | 3464 | yes | 1862 | 2834 | yes |
| hono-06 | locate | 36 | 705 | yes | 2175 | 2228 | yes |
| hono-07 | dependents | 57 | 57 | yes | 1487 | 1487 | yes |
| hono-08 | dependents | 42 | 42 | yes | 1471 | 1471 | yes |
| hono-09 | where-to-add | 92 | 1190 | yes | 1577 | 2675 | yes |
| hono-10 | where-to-add | 15 | 33 | yes | 1652 | 1727 | yes |

### Aggregate

Mean tokens per task. "Query only" excludes the derive map, which is a standing cost when it is injected into the agent's config file. Ratios are grep tokens divided by stacklit tokens: above 1.0x stacklit is cheaper, below 1.0x it costs more.

| Scope | Tasks | grep lookup | grep+read | map+query lookup | query only | map+query+read | grep+read found | stacklit found | lookup ratio (grep / stacklit) | end-to-end ratio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| gin | 10 | 135 | 5152 | 402 | 265 | 764 | 10/10 | 8/10 | 0.3x | 6.7x |
| flask | 10 | 151 | 9541 | 557 | 336 | 4420 | 10/10 | 9/10 | 0.3x | 2.2x |
| hono | 10 | 175 | 1789 | 1712 | 340 | 2109 | 10/10 | 10/10 | 0.1x | 0.8x |
| all repos, locate | 17 | 168 | 7508 | 968 | 365 | 2856 | 17/17 | 15/17 | 0.2x | 2.6x |
| all repos, dependents | 7 | 103 | 103 | 665 | 151 | 665 | 7/7 | 6/7 | 0.2x | 0.2x |
| all repos, where-to-add | 6 | 174 | 6077 | 935 | 358 | 3288 | 6/6 | 6/6 | 0.2x | 1.8x |
| **all repos, all tasks** | 30 | 154 | 5494 | 890 | 314 | 2431 | 30/30 | 27/30 | 0.2x | 2.3x |

### Sensitivity to the token approximation

Same totals under the second estimator (word and punctuation pieces x 1.3) instead of bytes/4.

| Estimator | mean grep lookup | mean map+query lookup | lookup ratio | mean grep+read | mean map+query+read | end-to-end ratio |
|---|---:|---:|---:|---:|---:|---:|
| bytes/4 | 154 | 890 | 0.2x | 5494 | 2431 | 2.3x |
| pieces x 1.3 | 228 | 1154 | 0.2x | 6524 | 2902 | 2.2x |

### Graph query arm

One `stacklit query "<question>" --budget 2000` call per task, no derive map, no follow-up reads. Tokens are the full text output. Locate and where-to-add tasks count as found when a returned file matches the verified path; dependents tasks when every expected module has a returned file. The question text is used verbatim, not the hand-written terms.

| Scope | Tasks | query tokens (mean) | found | grep+read tokens (mean) | map+query+read tokens (mean) |
|---|---:|---:|---:|---:|---:|
| gin | 10 | 1963 | 10/10 | 5152 | 764 |
| flask | 10 | 1633 | 10/10 | 9541 | 4420 |
| hono | 10 | 1356 | 8/10 | 1789 | 2109 |
| all repos, locate | 17 | 1536 | 17/17 | 7508 | 2856 |
| all repos, dependents | 7 | 1738 | 5/7 | 103 | 665 |
| all repos, where-to-add | 6 | 1872 | 6/6 | 6077 | 3288 |
| **all repos, all tasks** | 30 | 1651 | 28/30 | 5494 | 2431 |

Query misses:

- hono-07 (What depends on the trie-router module?): expected [] / [src src/preset src/middleware/combine], query returned 14 files (1337 tokens).
- hono-08 (What depends on the cookie helper?): expected [] / [src/middleware/jwt src/validator], query returned 9 files (1309 tokens).

### Misses

Every task where either procedure did not reach the verified answer.

- flask-08 (What depends on the flask.sansio package?): expected [src/flask src/flask/json]. grep path found [src/flask src/flask/json src/flask/sansio] (recall 1.00, precision 0.67). stacklit depended_by [src/flask] (recall 0.50, precision 1.00).
- gin-01 (Where is the radix tree route lookup (getValue) implemented?): expected [tree.go] / [root]. grep read [tree.go gin.go] (found in grep output: yes). stacklit top modules [].
- gin-05 (Where is the panic recovery middleware implemented?): expected [recovery.go] / [root]. grep read [recovery.go] (found in grep output: yes). stacklit top modules [].

<!-- END GENERATED -->
