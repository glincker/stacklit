# AI codebase context tools: comparison

This page compares the main ways to give an AI agent codebase context.

Last updated: 2026-04-10. [Submit corrections](https://github.com/glincker/stacklit/issues).

## The problem

AI coding agents (Claude Code, Cursor, Copilot, Aider) need to understand your codebase before they can help. Without context, they spend thousands of tokens exploring. They read files, grep and glob just to learn your project structure.

Tools solve this in different ways. Some dump everything. Some build knowledge graphs. Some generate compressed maps. The sections below compare them.

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
| **[Stacklit](https://github.com/glincker/stacklit)** | -- | Tree-sitter + module map | JSON + HTML | **137 to 1,351 (compact map on six repos; scales with module count)** | 48 | Yes | CLI (Go) |

*Token cost = tokens consumed to give an agent full project context on a ~10k-line repo.
**Aider's 43k stars are for the entire tool, not just the repo-map feature.

## Approach breakdown

### Full-content dumpers

**Repomix, Gitingest, code2prompt, files-to-prompt**

These tools concatenate all source files into one large prompt. They are simple and work everywhere, but:

- They use 50k to 500k tokens on medium repos.
- They often exceed context windows entirely.
- They have no structural understanding, so the agent still has to parse everything.
- Repomix's `--compress` mode uses tree-sitter to shrink the output. It still works one file at a time, with no cross-file dependency analysis.

Best for: small repos (under 5k lines), one-shot conversations, and pasting into ChatGPT.

### Knowledge graphs (MCP servers)

**Codebase-Memory, Axon**

These tools build a queryable graph of your codebase and serve it over MCP:

- They hold rich structural data: call graphs, blast radius and community detection.
- They need a server process running.
- Each query costs tokens, because of the tool call overhead.
- There is no committable artifact. The knowledge lives in the server.

Best for: large codebases, long interactive sessions, and teams with the infrastructure to run a server.

### Structural index (Stacklit)

Stacklit parses code with tree-sitter, builds a module-level dependency graph, and outputs a compact navigation map.

- **137 to 1,351 tokens** for the compact map, measured on six repos. A full dump is tens of thousands. The full index is 2,391 to 16,022 tokens.
- It is a static artifact. You commit `stacklit.json` to your repo.
- It produces a self-contained HTML visualization.
- `stacklit setup` configures 25 assistants, including Claude Code, Cursor and Codex.
- A git hook keeps the index fresh.
- The basic use needs no running server. The MCP server is optional.

Best for: any repo, any AI tool, and no ongoing maintenance.

### IDE-integrated (proprietary)

**Cursor, Continue.dev, GitHub Copilot, Sourcegraph Cody**

These are built into IDEs, and they are not standalone:

- They use vector embeddings for semantic search.
- They are tuned for their own tool.
- They are not portable across tools.
- They cannot be shared or committed.

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

## Measured Stacklit sizes

These come from the reproducible harness in [bench/](../bench/), run on pinned commits. Tokens are approximated as bytes / 4. We did not re-measure the other tools. A full dump is roughly the size of the source itself.

| Repository | Language | Source lines | Modules | Stacklit JSON | Stacklit compact map |
|-----------|----------|-------------:|--------:|--------------:|---------------------:|
| gin v1.10.0 | Go | 19,537 | 8 | 2,491 tokens | 137 tokens |
| flask 3.1.2 | Python | 18,515 | 14 | 2,818 tokens | 221 tokens |
| hono v4.6.0 | TypeScript | 57,263 | 77 | 16,022 tokens | 1,351 tokens |

Size follows the number of modules, not lines of code. See [bench/RESULTS.md](../bench/RESULTS.md) for methodology, a grep-versus-map cost comparison on 58 navigation tasks, and limitations.

## When to use what

**Use Repomix if:**

- Your repo is small (under 5k lines).
- You paste into ChatGPT or Claude on the web.
- You want the simplest possible tool.

**Use Codebase-Memory if:**

- You need call-graph-level detail.
- You work on a very large codebase (100k+ lines).
- You are comfortable running a background server.

**Use Stacklit if:**

- You want your AI tools to know the shape of your repo from the first token.
- You use several AI tools (Claude Code, Cursor, Aider).
- You want no maintenance. The git hook refreshes the index.
- You pay per token, or you keep hitting context limits.
- You want a visual dependency map you can share.

## Install

```bash
# npm (easiest, downloads the right binary automatically)
npm i -g stacklit

# From source
go install github.com/glincker/stacklit/cmd/stacklit@latest
```

Then set up everything in one command:

```bash
stacklit setup
```

---

This comparison is maintained by the Stacklit team. We try to be accurate and fair. If you spot an error or want to add a tool, [open an issue](https://github.com/glincker/stacklit/issues).
