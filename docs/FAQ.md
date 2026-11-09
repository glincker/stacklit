# FAQ

Short answers on what Stacklit reads, when it helps, how accurate its dependency edges are, and how it fits with git and AI assistants.

## Does it read my code?

Yes, locally. It uses tree-sitter to extract structure. Nothing leaves your machine unless you pass `--summary`. See [Privacy](PRIVACY.md).

## Does it help on small repos?

Less. If the whole repo fits in a prompt, an agent can just read it.

The index pays off once exploring costs more than reading a map. That happens when an agent would otherwise open several large files just to confirm where something lives.

## When is grep better?

When you already know the exact identifier. One grep is cheaper than loading a map.

The index helps with questions grep cannot answer in one step, such as "what depends on this module" or "where would a new adapter go".

## How accurate are the dependency edges?

Module-level edges come from import statements. They are good enough to orient an agent, but they are not exact.

In the benchmark, 4 of 7 "what depends on X" lookups were fully correct. Known weak spots:

- Test files counted as dependents.
- Relative imports resolved to the wrong module.
- Short module names that can be ambiguous.

Treat the graph as a guide, and confirm with a read.

## What if my language is not supported?

Stacklit still counts the files and detects their language. The module map, git activity and hints still work. Exports and dependency edges need a parser.

## Does the git hook slow commits down?

The hook runs `stacklit generate --quiet` after each commit.

- On Go repos of this size, it takes tens of milliseconds.
- On Python and TypeScript repos in the benchmark, it took a few seconds.

If that is too slow, run `stacklit generate` by hand, or run it in CI instead.

## Can I keep the Mermaid file out of the repo root?

Yes. Set `"output": {"mermaid": "docs/DEPENDENCIES.md"}` in `.stacklitrc.json`. The git hook stages the configured path.

## Can I use it with Copilot?

Yes. Commit `stacklit.json`, then tell Copilot to read it in `.github/copilot-instructions.md`. You can also paste the output of `stacklit derive` there.

## How is it different from dumping the whole repo into a prompt?

A dump is the full source. Stacklit is structure only. It is orders of magnitude smaller, but it cannot answer questions that need code bodies.

For a factual feature comparison, see [COMPARISON.md](COMPARISON.md).
