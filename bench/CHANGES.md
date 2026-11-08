# Changes since the first benchmark run

This file lists every change made to the benchmark between the first recorded run (stacklit before the symbol graph, BM25 ranking, module splitting and the Rust, Java and Ruby fixes) and the rerun on current master. Grading rules are unchanged: lenient and strict dependents grading, `-max-extra 2`, `-read-cap 5`, `-grep-lines 50`, the same 58 tasks, the same pinned commits, the same hand-written grep patterns and `terms` for localization tasks. Nothing was retuned to help stacklit.

## Task changes (dependents gold answers)

Module splitting renamed and subdivided some modules, so three dependents answers had to be re-derived. Each was re-derived by hand with `grep` on the pinned checkout (not from stacklit output), by listing the non-test source files that import or require the target and mapping each file to the current module that contains it.

| Task | Field | Old | New | Why |
|---|---|---|---|---|
| gs-07 (gson, what depends on `com.google.gson.internal`) | `target` | `gson/src/main/java` | `gson/src/main/java/com/google/gson/internal` | The single 83-file gson module no longer exists; the package is now its own module. |
| gs-07 | `expect_modules` | `extras/src/main/java`, `gson/src/main/java-templates` | `extras/src/main/java`, `gson/src/main/java/com/google/gson`, `gson/src/main/java/com/google/gson/reflect`, `gson/src/main/java/com/google/gson/stream` | Importers found by grep: `extras/.../graph/GraphAdapterBuilder.java`, 11 files directly under `com/google/gson`, `reflect/TypeToken.java`, `stream/JsonReader.java`. The old answer was wrong in two ways: `java-templates/.../GsonBuildConfig.java` only declares `package com.google.gson.internal;` and imports nothing, and importers inside the old single module were hidden because they were inside the target. |
| gs-07 | `pattern` | `com\.google\.gson\.internal` | `^import (static )?com\.google\.gson\.internal` | The old pattern also matched the `package` line and Javadoc. The new one matches import lines only. |
| rg-08 (ripgrep, what depends on `grep_matcher`) | `expect_modules` | `crates/pcre2/src`, `crates/printer/src`, `crates/regex/src`, `crates/searcher/src` | the same four plus `crates/searcher/src/searcher` | `crates/searcher/src/searcher/{core,glue,mod}.rs` use `grep_matcher::` and are now their own module. |
| rk-07 (rack, what depends on `rack/multipart`) | `expect_modules` | `lib` | `lib`, `lib/rack` | The old answer was incomplete: `lib/rack/request.rb` has `require_relative 'multipart'` and `lib/rack/mock_request.rb` calls `Rack::Multipart.build_multipart`, besides the `autoload` in `lib/rack.rb`. Module splitting made `lib/rack` a separate module, which exposes it. |
| rk-07 | `pattern` | `rack/multipart` | `autoload :Multipart\|Rack::Multipart\.\|require_relative 'multipart'` | So an oracle grep can actually find all importers. |

Checked and left unchanged: gin-06, gin-07, gin-08 (modules `root`, `binding`, `render` unchanged), flask-07, flask-08 (importers re-listed, all in `src/flask`, `src/flask/json`, `src/flask/sansio`, which still exist), hono-07, hono-08, rg-07 (`crates/cli/src`, `crates/ignore/src`). The 47 localization tasks use file paths, not module names, so splitting does not affect their gold answers; the harness still verifies on every run that each gold file exists and matches the task's grep pattern. The unused `expect_modules` on localization tasks still say `gson/src/main/java`; they are not read by any arm.

Effect of the corrections: they make gs-07 and rk-07 harder or equal for every arm (more modules must be found), and the corrected gs-07 is now beyond what a 50-line grep window shows, so grep+read no longer passes it (see RESULTS.md).

`terms` for the stacklit map+lookup arm were not retuned for the new module names.

## Harness changes

| Change | Why |
|---|---|
| Removed the `GOMAXPROCS=1` retry loop (`cleanIndex`) and the "clean-index attempts" column. The index every arm uses is now the plain output of `stacklit generate`. | The Rust parser race was fixed on master. A plain `generate` on ripgrep was verified clean (0 files dropped) and byte-identical across repeated cold runs on many CPUs. The report now states, per repo, the files dropped by the parser and whether every timed run wrote an identical `stacklit.json`, so a regression would be visible. |
| Removed `stacklit_ignore` from `repos.json` and the `.stacklitrc.json` it wrote. | The shell-script hang was fixed on master. ripgrep generates in a few seconds with shell completion scripts included. |
| `stacklit query` arms: kept the tool-order arm and added a reader-order arm per budget. | See below. |
| The report header records any `STACKLIT_*` environment variables set for the run. | So the default-config run and the `STACKLIT_MAX_MODULE_FILES=60` run are distinguishable. |

### How `stacklit query` output is ordered

The task asked to re-check the order. Reading `internal/graph/view.go` and `stacklit query --json` output on gin: BM25 and PageRank decide which symbols survive the token budget, but the files are then sorted by path (`sort.Strings(files)`), and symbols inside a file by line number. The JSON has no per-file relevance score, so relevance order cannot be read back from the CLI. The old note was therefore still accurate: output order is alphabetical.

Two arms per budget are reported:

- `stacklit query N`: files opened in the order the tool prints them (path order). This is what a reader gets who opens files top to bottom.
- `stacklit query N + reader order`: same single call, same lookup tokens, but the files are opened in an order chosen from the printed text and the question alone (3 per question word in the path, 2 per word in a symbol name, 1 per word in a signature or doc line; ties keep path order). No gold answer, hand-written term or file contents are used. This is a harness policy standing in for an agent that reads the output; it is not part of stacklit. The `--json` output is used only to parse files and symbols exactly.

Dependents tasks use the tool-order arm only (the module set does not depend on order); the reader-order arm is n/a there.
