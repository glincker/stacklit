# Proposed public claims

Wording the benchmark data supports, and wording it does not. This file proposes text only; README.md and COMPARISON.md are not edited by the benchmark work. Every claim below names the table in [RESULTS.md](RESULTS.md) and the exact cell with its 95% interval. All figures are a deterministic offline proxy (no LLM), 58 tasks, 6 repos, one run on current master. Quote the caveat sentence with any number: "Measured with a simulated reader on 47 where-is-X tasks across six repos; not an LLM agent."

## Claims the data supports

| # | Proposed wording | Backing cell |
|---|---|---|
| 1 | "For where-is-X questions, one `stacklit query --budget 1000` call prints the file somewhere in its output 96% of the time (interval 89 to 100%)." | "All localization tasks", row `stacklit query 1000`, gold shown 96% [89-100] |
| 2 | "Opening the files in the order `stacklit query` prints them (best match first), the right file is in the first five 96% of the time (89 to 100%) and first 81% of the time (70 to 91%)." Before ranked output the same call gave 49% and 13% (`--order path`). The scoring tweaks that took found@1 from 70% to 81% were chosen after looking at benchmark failures (CHANGES.md): say "on this benchmark". | "All localization tasks", rows `stacklit query 1000` and `stacklit query 1000 path order` |
| 3 | "That is better than BM25 over the question text at finding the file first: 81% against 57%, a paired difference of 23 points (11 to 36)." | "Paired differences", row `stacklit query 1000` vs `bm25 top-k`, found@1 +23 [+11, +36]. Same row at found@5 is +6 [-2, +15]: say "level", not "better", for the first five. |
| 4 | "A single query call costs about 1,000 tokens (983) and a file read to answer costs about 9k on average, against about 290k to put every code file in context." | Lookup tokens 981 [976-986]; tokens to found 9.1k [6.2k-13k]; full dump 293k [252k-334k]. Only valid if dumping the repo is the alternative being compared. |
| 5 | "Stacklit now handles Rust, Java and Ruby: the index-based dependents lookup passes both ripgrep tasks, and `query` returns results for Ruby." | "Dependents tasks per repo": map+lookup ripgrep 2/2, 2/2; "Per repo", `stacklit query 1000` rack 8/8 found@5 (the first run returned nothing for rack). Not claimed for gson or rack dependents (0/1 each). |
| 6 | "Between the first benchmark run and now, `query` read top to bottom went from 15% to 81% found@1 and from 66% to 96% found@5 at budget 1000 (first run versus ranked output), and dependents passed by map+lookup from 55% to 73% (lenient)." Three dependents answers were corrected between the two runs (CHANGES.md), so the dependents comparison is not exactly like for like. | "Before and after" tables. The first-run and current rows grade the file list differently (first run from `--json`, now from printed text, CHANGES.md). Present as "improved"; the found@1 jump is large (15% [6-26] to 81% [70-91]). |
| 7 | "`generate` is deterministic and drops no files on all six benchmark repos, including Rust." | Repos and generation cost: files dropped by parser 0 and identical index every run `yes` on all six rows. |
| 8 | "Cold index generation takes roughly 1 to 4 seconds on repos of 100 to 400 files (and well under a second on gin), with peak memory 82 to 751 MB." Include "measured on a loaded machine; upper bounds". | Repos and generation cost, generate median and peak RSS columns. Rack: 2.9 s now versus 15.8 s before. |
| 9 | "Raising `max_module_files` from 30 to 60 improves the map arm's found@5 by about 5 points (74% to 79%), within noise." Only if the cap is mentioned at all. | "Module file cap" table. The two intervals overlap fully. |

## Claims the data does NOT support

| Do not claim | Why, with the cell |
|---|---|
| "Stacklit is more accurate than grep" or "finds the right file more often than grep". | Grep+read found@1 89% [81-98], found@5 100% [100-100]. Query 1000 at 81% and 96% (paired -9 [-23, +6] and -4 [-11, +0]); map+lookup 55% and 74% (paired -34 [-49, -17] and -26 [-38, -13]). Grep leads. Caveat that favors grep: its regex is hand-written by someone who knows the answer. |
| "Stacklit beats BM25" without qualification. | Only found@1 in reader order (paired +21 [+9, +34]). Found@5 is level (+4 [-4, +13]); in `--order path` BM25 is better. BM25 also needs no index and prints 37 tokens. |
| "Query always returns the best file first." | found@1 is 81% [70-91] at budgets 1000 and 2000; 19% of tasks have the gold file lower. With `--order path` (the old behaviour) it is 13% [4-23] and 11% [2-21]. |
| "Stacklit saves tokens compared with grep or BM25." | Tokens to found: grep 5.9k [3.8k-8.7k], map+lookup 5.9k [4.6k-7.3k], reader-order query 7.2k [5.3k-9.1k], BM25 11k [6.7k-15k]. All intervals overlap. Query costs 9.1k [6.2k-13k] at budget 1000 (20k with `--order path`), above grep. |
| "Stacklit answers what-depends-on-X reliably." | Dependents lenient/strict: map+lookup 73% [45-100] / 64% [36-91]; query 1000 45% [18-73] / 18% [0-45]; grep+read 91% [73-100] / 91%. Eleven tasks; gson and rack dependents fail for stacklit. |
| "Stacklit improves agent success, speed or cost." | No model was run. The benchmark measures files found and tokens read by a simulated reader. |
| Any statement about hot files, git activity or temporal signals. | Shallow clones, no git history. |
| Absolute speed claims ("generates in X ms"). | Timings were taken with another job holding the machine near load 40 on 14 cores. Use only as upper bounds. |
| Comparisons with the real aider repo map. | The real tool was not run; the repo-map arm is an approximation. |
| "Works on large repos." | Largest repo here is hono, 356 files. |

## Suggested short summary (for a README)

"On a 47-question where-is-X benchmark over six repos in six languages, a single `stacklit query` call (about 1,000 tokens) puts the right file in its output 96% of the time. Opening files in the order it prints them finds the right one first 81% of the time, ahead of BM25 over the question text (57%) but behind an oracle grep (89%). Dependents (impact) questions are weaker: 73% leniently with the index. Simulated reader, not an LLM agent; see bench/RESULTS.md for method and limits."

Check before using: every number in that paragraph is a cell in RESULTS.md (claims 1, 2, 3 above, grep row, dependents row).
