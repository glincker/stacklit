# Scale harness

Measures what Stacklit costs (time, memory, output size) on large public repositories. It does not measure answer quality; see `../README.md` for that.

```bash
make scale                       # every repo in repos.json, rewrites the table in bench/SCALE.md
make scale ONLY=django,react     # a subset
go run ./bench/scale -runs 1 -qruns 1 -only tokio   # quick smoke run
```

Flags: `-runs` (repetitions per measurement, default 3, median reported), `-qruns` (repetitions per fixed question, default 5), `-timeout` (cap per process, default 900s, a hit is recorded as DNF), `-workdir`, `-json`, `-write`, `-repos`.

What it does, per repo in `repos.json`:

1. `git clone --depth 1 --branch <ref>` into a temp dir outside the checkout, records the resolved SHA.
2. Builds the current tree once (`go build ./cmd/stacklit`) and uses that binary for everything.
3. Cold `generate` (parse cache emptied before each run), warm `generate` (cache populated), cold `generate --graph`. Wall time from the harness, peak RSS from the child's rusage.
4. Reads `stacklit.json` and `stacklit.graph.json` for sizes, counts and `scan` fields. Token numbers are bytes divided by 4.
5. Runs 20 fixed calls (8 `query`, 6 `slice`, 6 `impact`) 5 times each as separate processes, so startup is included. Slice and impact targets are picked from the graph by degree rank (ties by id), so they are deterministic for a given graph.
6. Deletes the clone and the parse cache (`STACKLIT_CACHE_DIR` points into the temp dir, your real cache is untouched).

Never runs two repos at once. Keep other heavy processes off the machine while it runs. Clones need up to about 4 GB at a time (the Linux kernel is the largest). macOS and Linux only.
