# FAQ

**Does it read my code?**
Yes, locally, to extract structure with tree-sitter. Nothing leaves your machine unless you pass `--summary` (see Privacy).

**Does it help on small repos?**
Less. If the whole repo fits comfortably in a prompt, an agent can just read it. The index pays off once exploring costs more than reading a map, and it saves the most when the agent would otherwise read several large files to confirm a location.

**When is grep better?**
When you already know the exact identifier. A single grep is cheaper than loading a map. The index helps with questions grep cannot answer in one step, such as "what depends on this module" or "where would a new adapter go".

**How accurate are the dependency edges?**
Module-level edges are built from import statements and are good enough to orient an agent, but not exact. In the benchmark, 4 of 7 "what depends on X" lookups were fully correct. Known weak spots: test files counted as dependents, relative imports resolved to the wrong module, and short module names that can be ambiguous. Treat the graph as a guide and confirm with a read.

**What if my language is not supported?**
Files are still counted and language-detected, and the module map, git activity and hints still work. Exports and dependency edges need a parser.

**Does the git hook slow commits down?**
It runs `stacklit generate --quiet` after each commit. On Go repos of this size that is tens of milliseconds; on Python and TypeScript repos in the benchmark it took a few seconds. If that is too slow, run `stacklit generate` manually or in CI instead.

**Can I use it with Copilot?**
Yes. Commit `stacklit.json` and tell Copilot to read it in `.github/copilot-instructions.md`, or paste the output of `stacklit derive` there.

**How is it different from dumping the whole repo into a prompt?**
A dump is the full source. Stacklit is structure only: it is orders of magnitude smaller but cannot answer questions that need code bodies. See [COMPARISON.md](COMPARISON.md) for a factual feature comparison.

## Documentation

- [USAGE.md](USAGE.md): full command reference, MCP tools, configuration, index schema
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): how the pipeline works
- [bench/RESULTS.md](bench/RESULTS.md): benchmark methodology and results
- [ROADMAP.md](ROADMAP.md): what is planned and what is in progress
- [CHANGELOG.md](CHANGELOG.md): release history
- [COMPARISON.md](COMPARISON.md): feature comparison with other context tools
- [SKILL.md](SKILL.md): instructions an agent can follow to use `stacklit.json`
- [examples/](examples/): sample outputs (generated with earlier versions)
- [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md)

## Contributing

```bash
make build   # build the binary
make test    # run all tests
```

See [CONTRIBUTING.md](CONTRIBUTING.md). Questions and ideas are welcome in [Discussions](https://github.com/glincker/stacklit/discussions).

## License

MIT. See [LICENSE](LICENSE).
