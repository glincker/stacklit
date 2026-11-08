# Contributing

Thanks for helping. Stacklit is MIT licensed, fully open source, and has no paid tier or gated features; contributions land in the one codebase everyone uses.

## Setup

You need Go (the version in `go.mod`) and git. Node is only needed for the `npm/` wrapper.

```bash
git clone https://github.com/glincker/stacklit
cd stacklit
make build        # builds ./stacklit
make test         # go test ./...
```

Before opening a PR, run what CI runs:

```bash
go vet ./...
go test ./... -race -count=1
```

Try your change on a real repo:

```bash
go build -o /tmp/stacklit ./cmd/stacklit
cd /path/to/some/repo && /tmp/stacklit generate && /tmp/stacklit derive
```

## Where things live

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). In short: `internal/parser` reads files, `internal/graph` builds modules and edges, `internal/engine` ties the pipeline together, `internal/renderer` writes outputs, `internal/cli` holds the commands.

## Common contributions

**Fix or improve a language extractor.** Parser tests live next to the parsers (`internal/parser/*_test.go`) and use fixtures in `testdata/`. Add a failing test first with a small source snippet, then fix the extractor in `internal/parser/ts_<language>.go`.

**Add a language.** Add `ts_<language>.go` with an extractor, register it in `tsLangMap` in `internal/parser/treesitter.go`, add the file extension to `sourceExtensions` in `internal/walker/walker.go` if it is missing, add a fixture and a test, and update the language table in the README.

**Improve dependency resolution.** The logic is in `internal/graph/graph.go` (`resolveImport`). Include a minimal repro of the wrong edge in the test. The [benchmark tasks](bench/tasks) show the kinds of cases that currently fail.

**Add a benchmark task or repo.** See [bench/README.md](bench/README.md). Verify the expected answer by hand with grep, never with stacklit's own output.

**Docs.** Fix anything that is wrong or unclear. Claims about size or speed must come from `bench/` output, with the command that produced them.

## Pull requests

- One logical change per PR, with tests for behavior changes.
- Commit messages: `<type>: <description>` where type is one of `feat`, `fix`, `perf`, `refactor`, `docs`, `test`, `chore`. Keep them short.
- Keep generated files (`stacklit.json`, `DEPENDENCIES.md`) out of your PR unless the change is about them; CI refreshes them on the default branch.
- Do not describe unmerged or planned features as shipped in docs.
- Writing style in docs and comments: plain sentences, no marketing language, and no em dashes or en dashes.

## Reporting problems

Use the issue templates. For bugs, the most useful report is the command, the output, the language of the repo, and, if you can share it, a minimal repo or snippet. For security issues, see [SECURITY.md](SECURITY.md) and do not open a public issue.

## License

By contributing you agree that your contributions are licensed under the MIT License.
