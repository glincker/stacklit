# Adapters

`stacklit ingest --adapter "<command>"` lets you feed anything into the graph
that stacklit does not parse itself: PDFs, images, audio, spreadsheets. Stacklit
runs your command, sends it file paths, and stores what it prints. Stacklit makes
no network or model calls of its own; whatever the command does is up to you.

## Contract

Input: the command runs through the shell in the project root. Stdin carries one
slash-separated path per line, relative to the root. Limit the list with
`--adapter-files "*.pdf,*.png"`.

Output: JSON Lines on stdout, one object per line.

```
{"type":"node","id":"rate-limiting","name":"Rate limiting","kind":"concept","summary":"...","source_file":"docs/design.pdf"}
{"type":"edge","from":"rate-limiting","to":"internal/api/limiter.go","kind":"mentioned_in","source_file":"docs/design.pdf"}
```

- `node` needs `id`, `name` and `source_file`. `kind` defaults to `concept`.
- `edge` needs `from` and `to`. Each end is a node id from this run, a file path, or
  (with `--into-graph`) a symbol id from `stacklit.graph.json`.
- Anything else is skipped and counted. Exit non-zero to fail the run.

Everything is stored as `INFERRED` with `source` set to the adapter name
(`--adapter-name`, default the program name). Rerunning the same adapter replaces
its earlier output. Results go to `stacklit.context.json`; add `--into-graph` to
also write them into the symbol graph as `concept` nodes.

## Limits

| Env var | Default | Meaning |
| --- | --- | --- |
| `STACKLIT_ADAPTER_TIMEOUT_SEC` | 60 | kill the command after this long |
| `STACKLIT_ADAPTER_MAX_BYTES` | 4194304 | fail if output is larger |
| `STACKLIT_ADAPTER_MAX_ITEMS` | 5000 | keep at most this many nodes and edges |

## Try it

```
stacklit ingest --adapter "python3 examples/adapters/keyword_concepts.py" --adapter-files "*.md,*.txt,*.pdf"
```
