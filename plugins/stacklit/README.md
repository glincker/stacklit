# Stacklit plugin for Claude Code

Bundles the Stacklit skill, the session and refresh hooks, and a minimal MCP server (`stacklit serve --profile minimal`, stdio).

It needs the `stacklit` binary on PATH (`brew install glincker/tap/stacklit`, `npm install -g stacklit` or `go install github.com/glincker/stacklit/cmd/stacklit@latest`). The hooks do nothing and exit 0 when the binary is missing. Run `stacklit init` once in a repo to create `stacklit.json`.

The crawl guard is not part of this plugin. Enable it yourself with `stacklit setup claude --crawl-guard`.

Install:

```
claude plugin marketplace add glincker/stacklit
claude plugin install stacklit@stacklit
```
