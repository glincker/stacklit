# Optional: LLM-judged end-to-end variant

Not implemented, and not part of CI. No API calls are made anywhere in this repository's benchmark. This page describes how someone with an API key could extend the deterministic harness so the claims can be checked against a real agent.

## Why it exists

The deterministic harness in this folder measures how many tokens an arm shows or has the agent open before the gold file is reached. It does not measure whether a model reasons well from that text, how many tool calls it makes, or how much a whole conversation costs. A real-agent run answers those questions. The two kinds of result should be reported side by side and never merged.

## Design

Reuse `repos.json` (pinned commits) and `tasks/*.json` (questions and gold answers) unchanged, so the judged run asks exactly the questions the proxy asks.

Arms, each in a fresh checkout and a fresh conversation:

1. Plain: the agent has file read, directory listing and grep tools only.
2. Map: plain plus the `stacklit derive` output injected in its instructions file and `stacklit.json` in the repo.
3. MCP: plain plus the `stacklit serve` MCP tools.
4. Optional extra arms: a repo-map style tool, a BM25 retrieval tool, a full-repo dump in the prompt (only for repos that fit the window).

Controls that must be identical across arms:

- Same model, version string, temperature and system prompt.
- Same question text, same turn limit and same total token limit.
- Tool output size limits identical, so no arm is silently truncated.
- Tasks in a randomized order fixed by a published seed; arms interleaved, not run in blocks, so provider load drift hits all arms equally.

Repetitions: at least 5 per task and arm, because agent runs are noisy. Report the number of failed or timed out runs instead of dropping them.

## Measurements

From the provider's usage report, never from local estimates: input tokens, output tokens, cached input tokens (reported separately), tool call count, wall time, and cost at the published price on the run date.

Correctness: the final answer must name the gold file or files from `expect_paths` (localization) or the expected modules (dependents). Score it two ways and publish both:

- Mechanical: normalized path match of the answer against the gold, the same rule the proxy uses.
- Judged: a second model, different from the agent model and blind to the arm, receives the question, the gold and the answer and returns correct, partly correct or wrong with one sentence of reasoning. Spot check at least 10 percent of judgments by hand and publish the disagreement rate.

## Statistics

Use the same seeded bootstrap as `stats.go`, resampling tasks (and repetitions within a task) with replacement, and report 95 percent intervals for success rate, input tokens, total cost and tool calls. Compare arms with paired differences per task, not by eyeballing separate intervals.

## What to publish

The raw transcripts or at least the per-run usage records, the model id, the prompts, the seed, the date, and the exact commit of this repository. A claim that cannot be re-run by someone else with their own key is not a benchmark result.

## Cost and safety notes

Expect many thousands of model calls for 50+ tasks, 3 arms and 5 repetitions. Set a hard spend cap before starting. Run agents with read-only tools in a sandbox with no network, and do not let them write into the checkout, because a changed checkout invalidates the pinned commit.
