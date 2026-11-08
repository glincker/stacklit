# AI Codebase Context Tools - Comparison

> How do you give AI agents codebase context? Here's every approach compared.
>
> Last updated: 2026-04-10 | [Submit corrections](https://github.com/glincker/stacklit/issues)

## The Problem

AI coding agents (Claude Code, Cursor, Copilot, Aider) need to understand your codebase before they can help. Without context, they waste thousands of tokens exploring  - reading files, grepping, globbing  - just to figure out your project structure.

Different tools solve this differently. Some dump everything. Some build knowledge graphs. Some generate compressed maps. Here's how they compare.

## Quick Comparison

| Tool | Stars | Approach | Output | Token Cost* | Languages | Dependencies | Standalone |
|------|-------|----------|--------|-------------|-----------|-------------|-----------|
| [Repomix](https://github.com/yamadashy/repomix) | 23k | Full file dump | XML/MD/JSON | 50k-500k | Any | No | CLI (Node) |
| [Gitingest](https://github.com/coderamp-labs/gitingest) | 14k | Full file dump | Text | 50k-500k | Any | No | Web + CLI |
| [code2prompt](https://github.com/mufeedvh/code2prompt) | 7k | Full dump + templates | Text | 50k-500k | Any | No | CLI (Rust) |
| [files-to-prompt](https://github.com/simonw/files-to-prompt) | 2.6k | Concat files | XML | 50k-500k | Any | No | CLI (Python) |
| [Aider repo-map](https://github.com/Aider-AI/aider) | 43k** | Tree-sitter + PageRank | Text | ~1k | 40+ | Yes | Locked to Aider |
| [Codebase-Memory](https://github.com/DeusData/codebase-memory-mcp) | 1.4k | Knowledge graph | SQLite | 2k-10k/session | 66 | Yes (call graph) | MCP server (C) |
| [Axon](https://github.com/harshkedia177/axon) | 648 | Graph + community detection | Neo4j/KuzuDB | 2k-10k/session | 3 | Yes (blast radius) | MCP server (Python) |
| **[Stacklit](https://github.com/glincker/stacklit)** | -- | Tree-sitter + module map | JSON + HTML | **120 to 1,100 (compact map; scales with module count)** | 11 | Yes | CLI (Go) |

*Token cost = tokens consumed to give an agent full project context on a ~10k-line repo.
**Aider's 43k stars are for the entire tool, not just the repo-map feature.

## Approach Breakdown

### Full-Content Dumpers
**Repomix, Gitingest, code2prompt, files-to-prompt**

Concatenate all source files into one big prompt. Simple, works everywhere, but:
- Burns 50k-500k tokens on medium repos
- Often exceeds context windows entirely
- No structural intelligence  - agent still has to parse everything
- Repomix's `--compress` mode uses tree-sitter to reduce output, but remains per-file (no cross-file dependency analysis)

Best for: Small repos (<5k lines), one-shot conversations, pasting into ChatGPT.

### Knowledge Graphs (MCP Servers)
**Codebase-Memory, Axon**

Build a queryable graph of your codebase, served over MCP:
- Rich structural data (call graphs, blast radius, community detection)
- Requires running a server process
- Each query costs tokens (tool call overhead)
- No committable artifact  - the knowledge lives in the server

Best for: Large codebases, long interactive sessions, teams with infra capacity.

### Structural Index (Stacklit)

Parses code with tree-sitter, builds a module-level dependency graph, outputs a compact navigation map:
- **137 to 1,372 tokens** for the compact map on three measured repos (vs tens of thousands for a full dump); the full index is 2,500 to 16,000
- Static artifact  - commit `stacklit.json` to your repo
- Self-contained HTML visualization
- Auto-configures Claude Code, Cursor, Aider via `stacklit setup`
- Git hook keeps the index fresh
- No running server needed for basic use (MCP server optional)

Best for: Any repo, any AI tool, zero ongoing maintenance.

### IDE-Integrated (Proprietary)
**Cursor, Continue.dev, GitHub Copilot, Sourcegraph Cody**

Built into IDEs, not standalone:
- Vector embeddings for semantic search
- Optimized for their specific tool
- Not portable across tools
- Can't be shared or committed

## Feature Matrix

| Feature | Repomix | Aider | CB Memory | Axon | Stacklit |
|---------|---------|-------|-----------|------|----------|
| Zero config | Yes | Yes | Yes | No | Yes |
| Tree-sitter parsing | Compress mode | Yes | Yes | Yes | Yes |
| Dependency graph | No | Yes | Yes (call graph) | Yes | Yes |
| Committable artifact | No* | No | No | No | **Yes** |
| Visual output | No | No | No | Web UI (server) | **HTML (static)** |
| MCP server | Yes | No | Yes | Yes | Yes |
| Monorepo support | No | No | No | No | **Yes** |
| Git activity tracking | No | No | Partial | Yes | Yes |
| Compact map output | No | No | No | No | **Yes** |
| Auto-configure agents | No | No | No | No | **Yes** |
| Single binary, no deps | No (Node) | No (Python) | Yes (C) | No (Python) | Yes (Go) |

*Repomix output is too large to commit meaningfully.

## Measured Stacklit Sizes

Measured with the reproducible harness in [bench/](bench/) on pinned commits (tokens approximated as bytes / 4). Figures for other tools are not re-measured here; a full dump is roughly the size of the source itself.

| Repository | Language | Source lines | Modules | Stacklit JSON | Stacklit compact map |
|-----------|----------|-------------:|--------:|--------------:|---------------------:|
| gin v1.10.0 | Go | 19,537 | 8 | 2,524 tokens | 137 tokens |
| flask 3.1.2 | Python | 18,515 | 14 | 2,681 tokens | 221 tokens |
| hono v4.6.0 | TypeScript | 57,307 | 79 | 16,126 tokens | 1,372 tokens |
| ripgrep 14.1.1 | Rust | 51,442 | 33 | 7,582 tokens | 368 tokens |
| gson 2.11.0 | Java | 53,490 | 10 | 2,750 tokens | 187 tokens |
| rack 3.1.8 | Ruby | 19,805 | 9 | 2,162 tokens | 126 tokens |

Size follows the number of modules, not lines of code. See [bench/RESULTS.md](bench/RESULTS.md) for methodology, a comparison of grep, BM25, a repo-map approximation, a full dump and stacklit on 58 navigation tasks over six repos, and limitations. In that comparison the full code of the repos is 137,000 to 456,000 tokens.

## When to Use What

**Use Repomix if:**
- Your repo is small (<5k lines)
- You're pasting into ChatGPT/Claude web
- You want the simplest possible tool

**Use Codebase-Memory if:**
- You need call-graph-level detail
- You're working on a very large codebase (100k+ lines)
- You're comfortable running a background server

**Use Stacklit if:**
- You want your AI tools to understand your repo from token zero
- You use multiple AI tools (Claude Code, Cursor, Aider)
- You want zero maintenance (git hook auto-refreshes)
- Token efficiency matters (pay-per-token or hitting context limits)
- You want a visual dependency map you can share

## Install

```bash
# npm (easiest - downloads the right binary automatically)
npm i -g stacklit

# From source
go install github.com/glincker/stacklit/cmd/stacklit@latest
```

```bash
# One command to set up everything
stacklit setup
```

---

*This comparison is maintained by the Stacklit team. We aim to be accurate and fair. If you spot an error or want to add a tool, [open an issue](https://github.com/glincker/stacklit/issues).*
