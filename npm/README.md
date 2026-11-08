# Stacklit

**A committed map of your codebase, built for AI coding agents.**

One command generates a JSON index that any AI agent can read. No server, no setup. Fully open source (MIT), no paid tier, no gated features.

[![CI](https://github.com/glincker/stacklit/actions/workflows/ci.yml/badge.svg)](https://github.com/glincker/stacklit/actions)
[![npm](https://img.shields.io/npm/v/stacklit)](https://www.npmjs.com/package/stacklit)
[![downloads](https://img.shields.io/npm/dm/stacklit)](https://www.npmjs.com/package/stacklit)
[![License](https://img.shields.io/badge/license-MIT-green)](https://opensource.org/licenses/MIT)
[![platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](https://github.com/glincker/stacklit/releases)
[![node](https://img.shields.io/badge/node-%3E%3D16-brightgreen)](https://nodejs.org)

## Quick start

```bash
npx stacklit init
```

![Stacklit demo](https://raw.githubusercontent.com/glincker/stacklit/master/demo.gif)

**Size:** on three pinned public repos the full `stacklit.json` was 2,045 to 11,814 tokens and the compact map from `stacklit derive` was 122 to 1,122 tokens (bytes/4 estimate). Both grow with module count. Method and limitations: [bench/RESULTS.md](https://github.com/glincker/stacklit/blob/master/bench/RESULTS.md).

**Privacy:** runs locally, no network requests. The only opt-in exception is `stacklit init --summary`, which pipes an index snapshot to a command you configure.

## What you get

| File | What it does | Committed? |
|------|-------------|------------|
| `stacklit.json` | Machine-readable codebase index | Yes |
| `DEPENDENCIES.md` | Mermaid dependency diagram (renders on GitHub) | Yes |
| `stacklit.html` | Interactive visual map with 4 views | No (gitignored) |

## Visual map

![Stacklit visual map](https://raw.githubusercontent.com/glincker/stacklit/master/stacklit-og.png)

## MCP server

```bash
stacklit serve
```

Add to Claude Desktop or Cursor MCP config:

```json
{
  "mcpServers": {
    "stacklit": {
      "command": "stacklit",
      "args": ["serve"]
    }
  }
}
```

Seven tools: `get_overview`, `get_module`, `find_module`, `list_modules`, `get_dependencies`, `get_hot_files`, `get_hints`.

## CLI

```
stacklit init                    # scan, generate all outputs, open HTML
stacklit generate                # regenerate from current source
stacklit view                    # regenerate HTML and open in browser
stacklit diff                    # check if index is stale
stacklit serve                   # start MCP server
```

## Language support

Go (standard library AST), TypeScript, JavaScript, Python, Rust, Java, C#, Ruby, PHP, Kotlin, Swift, C and C++ (tree-sitter). Other source files are counted but not parsed.

For a factual feature comparison with other context tools see [COMPARISON.md](https://github.com/glincker/stacklit/blob/master/COMPARISON.md).

## Links

- [GitHub](https://github.com/glincker/stacklit)
- [Full documentation](https://github.com/glincker/stacklit#readme)
- [Releases](https://github.com/glincker/stacklit/releases)

## License

MIT
