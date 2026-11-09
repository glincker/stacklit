<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/brand/stacklit-logo-dark.svg">
    <img src="docs/brand/stacklit-logo-light.svg" alt="Stacklit" width="360">
  </picture>
</p>

<p align="center"><b>A committed map of your codebase for AI coding agents. 137 tokens for a 19,500 line Go repo.</b></p>

<p align="center">
  <a href="https://github.com/glincker/stacklit/actions/workflows/ci.yml"><img src="https://github.com/glincker/stacklit/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/glincker/stacklit/releases"><img src="https://badgen.net/github/release/glincker/stacklit" alt="Release"></a>
  <a href="https://www.npmjs.com/package/stacklit"><img src="https://badgen.net/npm/v/stacklit" alt="npm version"></a>
  <a href="https://www.npmjs.com/package/stacklit"><img src="https://badgen.net/npm/dm/stacklit" alt="npm downloads"></a>
  <a href="https://pkg.go.dev/github.com/glincker/stacklit"><img src="https://pkg.go.dev/badge/github.com/glincker/stacklit.svg" alt="Go reference"></a>
  <a href="LICENSE"><img src="https://badgen.net/badge/license/MIT/green" alt="MIT license"></a>
</p>

<p align="center">
  <img src="docs/assets/demo.gif" alt="stacklit init in a terminal" width="720">
</p>

Stacklit maps your codebase once and keeps the map fresh. It parses your repo with tree-sitter and writes `stacklit.json`: modules, exports, dependency edges and hints. Commit it, and your coding agent reads a few hundred tokens instead of crawling your files, then uses a few commands (`query`, `slice`, `impact`) to go straight to the right file.
Free and MIT, no paid tier. One Go binary, local and deterministic, no LLM, and the CLI makes no network calls. (The map page can load language icons from thesvg.org; `STACKLIT_MAP_ICONS=off` removes that.)

## Install

```bash
npx stacklit init                                   # scan, write the index, open the map
stacklit setup                                      # wire up the assistants it detects
git add stacklit.json && git commit -m "add stacklit index"
```

Also: `brew install glincker/tap/stacklit`, `npm install -g stacklit`, `go install github.com/glincker/stacklit/cmd/stacklit@latest`, or a binary from [Releases](https://github.com/glincker/stacklit/releases).

## See it

<p align="center">
  <a href="docs/assets/map-overview.png"><img src="docs/assets/map-overview.png" alt="Visual map of a repo" width="32%"></a>
  <a href="docs/assets/agent-prompts.png"><img src="docs/assets/agent-prompts.png" alt="Copy-ready prompts for an agent" width="32%"></a>
  <a href="docs/assets/flow-steps.png"><img src="docs/assets/flow-steps.png" alt="Flow steps through a codebase" width="32%"></a>
</p>

## Output

`stacklit derive` prints a compact map for a prompt (gin v1.10.0, 137 tokens, module list trimmed here):

```
gin | go | 8 modules | 19,537 lines | Gin, GitHub Actions, net/http
test: go test ./... | env: PORT

modules:
  binding             binding | depends: bytesconv, json
  internal/bytesconv  bytesconv | depended-by: binding, render, root
  render              render | depends: bytesconv, json
  root                root package | depends: binding, bytesconv, json, render
do-not-touch: .github/
```

`stacklit query` returns a token-budgeted slice of the symbol graph (output trimmed here):

```
$ stacklit query "how is the symbol graph built" --budget 600
internal/cli/graphcmds.go
  50-63 func buildSymbolGraph(root string, includeTests ...bool) (*graph.SymGraph, error)
      calls BuildSymbols E0.95
internal/graph/resolve.go
  955-961 func BuildSymbols(root string, files []string, eo ExtractOptions) *SymGraph
```

## What you get

| | |
|---|---|
| Index | `stacklit.json`, a Mermaid diagram (`DEPENDENCIES.md` by default, set with `output.mermaid`) and an interactive `stacklit.html` map |
| Languages | 48 languages and formats indexed; symbol graph for 20 ([list](docs/LANGUAGES.md)) |
| Queries | `query`, `explain`, `impact`, `path`, `hubs`, `slice`, each with a hard `--budget` |
| MCP | `stacklit serve`: 15 tools, or `--profile minimal` for 4 (about 230 tokens of schema) |
| Hooks | git post-commit refresh; optional Claude Code hook that points a grepped identifier to its definition |
| CI | `export` (graphml, cypher, dot, mermaid and more), `check` architecture rules, `status` freshness, `merge-graphs`, a [GitHub Action](https://github.com/glincker/stacklit-action) |
| Deterministic | same repo, same bytes. A committed index only changes when the code does |
| Privacy | no telemetry, no upload. The map loads language logos from thesvg.org unless `STACKLIT_MAP_ICONS=off` ([details](docs/PRIVACY.md)) |

## Assistants

<p align="center">
  <a href="https://claude.com/claude-code"><img src="https://thesvg.org/icons/claude-code/default.svg" alt="Claude Code" height="32"></a>&nbsp;
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://thesvg.org/icons/cursor/dark.svg"><img src="https://thesvg.org/icons/cursor/light.svg" alt="Cursor" height="32"></picture>&nbsp;
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://thesvg.org/icons/codex/dark.svg"><img src="https://thesvg.org/icons/codex/light.svg" alt="Codex" height="32"></picture>&nbsp;
  <img src="https://thesvg.org/icons/gemini/default.svg" alt="Gemini CLI" height="32">&nbsp;
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://thesvg.org/icons/github-copilot/dark.svg"><img src="https://thesvg.org/icons/github-copilot/light.svg" alt="GitHub Copilot" height="32"></picture>&nbsp;
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://thesvg.org/icons/windsurf/dark.svg"><img src="https://thesvg.org/icons/windsurf/light.svg" alt="Windsurf" height="32"></picture>&nbsp;
  <picture><source media="(prefers-color-scheme: dark)" srcset="https://thesvg.org/icons/zed/dark.svg"><img src="https://thesvg.org/icons/zed/light.svg" alt="Zed" height="32"></picture>&nbsp;
  <img src="https://thesvg.org/icons/amp/default.svg" alt="Amp" height="32">
</p>

`stacklit setup` sets up 25 assistants, including Aider, Cline, Continue, OpenCode, Roo Code and Kiro, plus a generic `AGENTS.md` (`stacklit setup --list` shows all 26 targets). Anything else can read `stacklit.json` directly.

## Languages

Stacklit indexes 48 languages and formats ([full table](docs/LANGUAGES.md)). Symbol graph and call edges (`query`, `slice`, `impact`) cover 20 of them.

**Symbol graph and call edges (20)**

<table>
  <tr><td align="center" valign="top"><picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/go/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/go/light.svg" alt="Go" width="28" height="28"></picture><br><sub>Go</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/python/default.svg" alt="Python" width="28" height="28"><br><sub>Python</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/typescript/default.svg" alt="TypeScript / JS" width="28" height="28"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/javascript/default.svg" alt="TypeScript / JS" width="28" height="28"><br><sub>TypeScript / JS</sub></td><td align="center" valign="top"><picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/rust/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/rust/light.svg" alt="Rust" width="28" height="28"></picture><br><sub>Rust</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/java/default.svg" alt="Java" width="28" height="28"><br><sub>Java</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/csharp/default.svg" alt="C#" width="28" height="28"><br><sub>C#</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/kotlin/default.svg" alt="Kotlin" width="28" height="28"><br><sub>Kotlin</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/swift/default.svg" alt="Swift" width="28" height="28"><br><sub>Swift</sub></td><td align="center" valign="top"><picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/php/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/php/light.svg" alt="PHP" width="28" height="28"></picture><br><sub>PHP</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/ruby/default.svg" alt="Ruby" width="28" height="28"><br><sub>Ruby</sub></td></tr>
  <tr><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/c/default.svg" alt="C" width="28" height="28"><br><sub>C</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/c-plusplus/default.svg" alt="C++" width="28" height="28"><br><sub>C++</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/scala/default.svg" alt="Scala" width="28" height="28"><br><sub>Scala</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/dart/default.svg" alt="Dart" width="28" height="28"><br><sub>Dart</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/elixir/default.svg" alt="Elixir" width="28" height="28"><br><sub>Elixir</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/lua/default.svg" alt="Lua" width="28" height="28"><br><sub>Lua</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/zig/default.svg" alt="Zig" width="28" height="28"><br><sub>Zig</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/haskell/default.svg" alt="Haskell" width="28" height="28"><br><sub>Haskell</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/ocaml/default.svg" alt="OCaml" width="28" height="28"><br><sub>OCaml</sub></td><td align="center" valign="top"><b>S</b><br><sub>Solidity</sub></td></tr>
</table>

**Index only (modules, exports and imports, no call edges)**

<table>
  <tr><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/shell/default.svg" alt="Shell" width="28" height="28"><br><sub>Shell</sub></td><td align="center" valign="top"><b>S</b><br><sub>SQL</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/godot/default.svg" alt="GDScript" width="28" height="28"><br><sub>GDScript</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/vue/default.svg" alt="Vue / Svelte" width="28" height="28"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/svelte/default.svg" alt="Vue / Svelte" width="28" height="28"><br><sub>Vue / Svelte</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/terraform/default.svg" alt="Terraform / HCL" width="28" height="28"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/hcl/default.svg" alt="Terraform / HCL" width="28" height="28"><br><sub>Terraform / HCL</sub></td><td align="center" valign="top"><b>P</b><br><sub>Protobuf</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/clojure/default.svg" alt="Clojure" width="28" height="28"><br><sub>Clojure</sub></td><td align="center" valign="top"><picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/r/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/r/light.svg" alt="R" width="28" height="28"></picture><br><sub>R</sub></td></tr>
  <tr><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/perl/default.svg" alt="Perl" width="28" height="28"><br><sub>Perl</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/julia/default.svg" alt="Julia" width="28" height="28"><br><sub>Julia</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/objectivec/default.svg" alt="Objective-C" width="28" height="28"><br><sub>Objective-C</sub></td><td align="center" valign="top"><b>G</b><br><sub>Groovy</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/nim/default.svg" alt="Nim" width="28" height="28"><br><sub>Nim</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/erlang/default.svg" alt="Erlang" width="28" height="28"><br><sub>Erlang</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/fsharp/default.svg" alt="F#" width="28" height="28"><br><sub>F#</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/fortran/default.svg" alt="Fortran" width="28" height="28"><br><sub>Fortran</sub></td></tr>
  <tr><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/elm/default.svg" alt="Elm" width="28" height="28"><br><sub>Elm</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/powershell/default.svg" alt="PowerShell" width="28" height="28"><br><sub>PowerShell</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/gleam/default.svg" alt="Gleam" width="28" height="28"><br><sub>Gleam</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/cmake/default.svg" alt="CMake" width="28" height="28"><br><sub>CMake</sub></td><td align="center" valign="top"><b>M</b><br><sub>Makefile</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/docker/default.svg" alt="Dockerfile" width="28" height="28"><br><sub>Dockerfile</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/yaml/default.svg" alt="YAML / TOML / JSON" width="28" height="28"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/toml/default.svg" alt="YAML / TOML / JSON" width="28" height="28"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/json/default.svg" alt="YAML / TOML / JSON" width="28" height="28"><br><sub>YAML / TOML / JSON</sub></td><td align="center" valign="top"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/graphql/default.svg" alt="GraphQL" width="28" height="28"><br><sub>GraphQL</sub></td></tr>
</table>

Anything else gets line counts and language detection.

## Benchmark

Two tests, with different answers. The first, with no model in the loop: six pinned repos in six languages, 58 tasks, tokens as bytes / 4. The derived map is 137 to 1,351 tokens. On 47 where-is-X questions one `stacklit query` with a 1,000 token budget put the right file first on 81% (95% interval 70 to 91) and in the top five on 96%. Grep with a hand written pattern did better, 89% first and 100% in the top five. The 81% was tuned after looking at failures, so treat it as an upper bound. Method and limits: [bench/RESULTS.md](bench/RESULTS.md).

The second is a real agent test, run twice. Round 1: 450 Claude Code sessions (Sonnet, 30 questions, three public repos, plan written before the first run). Stacklit changed how the agent works but **did not lower cost overall**: pooled cost ratio 1.00 [0.93, 1.07], tokens 1.01 [0.92, 1.10]. It used about a third fewer tool calls (0.68 [0.61, 0.77]), a quarter fewer turns (0.77) and over four fifths fewer grep and glob calls (0.18). Answers were right slightly less often (94.0% to 88.0% on the pre-registered grader, 98.0% to 95.3% audited). Cost ratio by repo (below 1 is cheaper, 50 paired sessions each; full tables, method, limits and the claims we do not make are in [bench/agent-ab/RESULTS.md](bench/agent-ab/RESULTS.md)):

| Repo | Size | Cost ratio [95% CI] |
|---|---|---|
| levelrail | about 16,000 files | 0.78 [0.70, 0.86] |
| theauth-go | about 1,300 files | 1.10 [0.98, 1.25] |
| featuredrop | about 400 files | 1.23 [1.14, 1.33] |

**Round 2** (650 more sessions, 40 new questions, four repos, [bench/agent-ab/RESULTS-round2.md](bench/agent-ab/RESULTS-round2.md)) pointed the same way. The 16,000 file repo was cheaper again (0.89 [0.84, 0.96] with the default wording, 0.87 [0.81, 0.93] with the routed wording). The smaller repos cost more: theauth-go 1.10 and 1.05, featuredrop 1.37 and 1.21, theauth (about 2,900 files) 1.61 and 1.42. Pooled over all four repos a session cost more, not less: 1.17 [1.11, 1.24] and 1.09 [1.04, 1.14]. Tool calls were 0.76 of the no Stacklit count and Grep and Glob calls 0.22 and 0.53. Right answers were equal (97.5%, 97.0%, 97.5%). The model had partly memorised featuredrop, which favours the side without Stacklit. So today Stacklit is for very large repos and "where is it" questions, not for small repos.

**When it helps.** Locate and add-feature questions ("where is it", "where does a new thing go") were 12 to 22 percent cheaper, mostly on the large repo. Trace questions cost the same and were right less often. "Who imports X" cost 1.59x [1.47, 1.72] because one exact grep already answers it. Small repos cost more, since the map is overhead there. The strict setup (crawl guard plus strict rules) measured worse and is off by default.

## Docs

[Usage](docs/USAGE.md) | [Languages](docs/LANGUAGES.md) | [FAQ](docs/FAQ.md) | [Privacy](docs/PRIVACY.md) | [Architecture](docs/ARCHITECTURE.md) | [Comparison](docs/COMPARISON.md) | [Roadmap](docs/ROADMAP.md) | [Changelog](CHANGELOG.md) | [Agent skill](skills/stacklit/SKILL.md) | [Brand kit](docs/brand/README.md)

Contributing: [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md). Security reports: [.github/SECURITY.md](.github/SECURITY.md). Questions: [Discussions](https://github.com/glincker/stacklit/discussions).

MIT. See [LICENSE](LICENSE).
