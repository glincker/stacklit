# Round 2 in plain words

Round 1 left a question. Could a different steering wording fix the cost on smaller repos? We wrote the plan down first (tracking issue glincker/stacklit#78), then ran 650 more Claude Code sessions on four repos with 40 new questions. The model was Sonnet again. The full method, every table and the 16 wrong answers are in [RESULTS-round2.md](RESULTS-round2.md).

## What we found

- **Cheaper on the biggest repo, again.** On the repo of about 16,000 files a session cost 11% less with the default wording and 13% less with the routed wording. In round 1 it was 22% less.
- **More expensive on smaller repos.** On the three repos of 400 to 3,000 files, a session cost 5% to 61% more, depending on the repo and the wording.
- **More expensive on average.** Pooled over all four repos, a session cost 17% more with the default wording and 9% more with the routed one. We do not claim any overall saving.
- **Same work pattern as before.** The agent made about a quarter fewer tool calls (0.76 of the no Stacklit count) and about a sixth fewer turns (0.83). Grep and Glob calls fell to 0.22 with the default wording and 0.53 with the routed one. Round 1 had 0.18.
- **Answers equally right.** Right answers were 97.5% without Stacklit, 97.0% with the default wording and 97.5% with the routed one. Most questions were easy, so this test cannot show that Stacklit makes answers better.

## Cost against no Stacklit

Each number is the cost of a session with Stacklit divided by the cost of the same session without it, with a 95% interval. Below 1 is cheaper, above 1 is more expensive. 50 paired sessions per repo, 200 in all.

| Repo | Size | Default wording (a) | Routed wording (e) |
|---|---|---|---|
| levelrail | about 16,000 files | 0.89 [0.84, 0.96] | 0.87 [0.81, 0.93] |
| theauth | about 2,900 files | 1.61 [1.53, 1.69] | 1.42 [1.36, 1.48] |
| theauth-go | about 1,300 files | 1.10 [0.98, 1.24] | 1.05 [0.95, 1.16] |
| featuredrop | about 400 files | 1.37 [1.27, 1.47] | 1.21 [1.14, 1.29] |
| All four repos | | 1.17 [1.11, 1.24] | 1.09 [1.04, 1.14] |

The routed wording tells the agent to skip Stacklit when it already knows the exact name. It costs less than the default wording (0.93 [0.90, 0.95]) but still more than no Stacklit on three of the four repos, so it did not become the default. It stays opt-in.

## Who this is for

Today the evidence supports very large repos (about 16,000 files) and "where is it" questions. It does not support small repos. On those, a plain grep already answers in one to three calls, and each Stacklit call adds text the agent has to read.

## Caveat on featuredrop

We checked whether the model could answer the questions with no tools at all. On featuredrop it got 3 of 6 right, because the answer file names are easy to guess from the library's own words. So the featuredrop numbers favour the side without Stacklit. Without featuredrop the pooled routed wording is 1.05 [1.00, 1.11], still not cheaper. The other three repos scored 0 of 6, 1 of 6 and 0 of 6.

## Limits

One model, one machine, two days, questions we wrote on repos we own. The no Stacklit side already scored 100% on trace questions in round 2, so this round cannot say whether the round 1 loss on those questions is fixed. The grader also does not penalise long lists of files.
