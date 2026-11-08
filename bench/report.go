package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	beginMarker = "<!-- BEGIN GENERATED (go run ./bench -write bench/RESULTS.md) -->"
	endMarker   = "<!-- END GENERATED -->"
)

// spliceGenerated replaces the block between the markers in path.
func spliceGenerated(path, block string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(raw)
	b := strings.Index(s, beginMarker)
	e := strings.Index(s, endMarker)
	if b < 0 || e < b {
		return fmt.Errorf("%s: generated block markers not found", path)
	}
	out := s[:b] + beginMarker + "\n\n" + block + "\n" + s[e:]
	return os.WriteFile(path, []byte(out), 0o644)
}

func isLocalization(k string) bool { return k == "locate" || k == "where-to-add" }

// pct formats a proportion CI as "62% [45-77]".
func pct(c CI) string {
	return fmt.Sprintf("%.0f%% [%.0f-%.0f]", c.Mean*100, c.Lo*100, c.Hi*100)
}

// num formats a mean CI as "1.2k [0.9k-1.6k]".
func num(c CI) string {
	return fmt.Sprintf("%s [%s-%s]", short(c.Mean), short(c.Lo), short(c.Hi))
}

func short(v float64) string {
	if v >= 10000 {
		return fmt.Sprintf("%.0fk", v/1000)
	}
	if v >= 1000 {
		return fmt.Sprintf("%.1fk", v/1000)
	}
	return fmt.Sprintf("%.0f", v)
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// pick returns the arm results for the tasks that pass the filter.
func pick(results []RepoResult, arm string, keep func(Task) bool) []ArmResult {
	var out []ArmResult
	for _, r := range results {
		for _, tr := range r.Tasks {
			if keep(tr.Task) {
				out = append(out, tr.Arms[arm])
			}
		}
	}
	return out
}

func locTable(b *strings.Builder, results []RepoResult, arms []string, o Opts, keep func(Task) bool) {
	b.WriteString("| Arm | Tasks | found@1 [95% CI] | found@5 [95% CI] | gold shown [95% CI] | lookup tokens [95% CI] | tokens to found [95% CI] |\n|---|---:|---|---|---|---|---|\n")
	for _, arm := range arms {
		rs := pick(results, arm, keep)
		var f1, f5, sh, lk, tf []float64
		for _, r := range rs {
			f1 = append(f1, b2f(r.Found(1)))
			f5 = append(f5, b2f(r.Found(5)))
			sh = append(sh, b2f(r.GoldShown))
			lk = append(lk, float64(r.Lookup.Tokens()))
			tf = append(tf, float64(r.ToFound.Tokens()))
		}
		if len(rs) == 0 {
			continue
		}
		if rs[0].CostOnly {
			fmt.Fprintf(b, "| %s | %d | n/a | n/a | n/a | %s | %s |\n", arm, len(rs), num(bootstrapMean(lk, o.Boot, o.Seed)), num(bootstrapMean(tf, o.Boot, o.Seed)))
			continue
		}
		fmt.Fprintf(b, "| %s | %d | %s | %s | %s | %s | %s |\n", arm, len(rs),
			pct(bootstrapMean(f1, o.Boot, o.Seed)), pct(bootstrapMean(f5, o.Boot, o.Seed)), pct(bootstrapMean(sh, o.Boot, o.Seed)),
			num(bootstrapMean(lk, o.Boot, o.Seed)), num(bootstrapMean(tf, o.Boot, o.Seed)))
	}
	b.WriteString("\n")
}

func depTable(b *strings.Builder, results []RepoResult, arms []string, o Opts) {
	b.WriteString("| Arm | Tasks | lenient pass [95% CI] | strict pass [95% CI] | mean extra modules | lookup tokens [95% CI] |\n|---|---:|---|---|---:|---|\n")
	for _, arm := range arms {
		rs := pick(results, arm, func(t Task) bool { return t.Kind == "dependents" })
		if len(rs) == 0 {
			continue
		}
		if rs[0].NA || rs[0].CostOnly {
			fmt.Fprintf(b, "| %s | %d | n/a | n/a | n/a | n/a |\n", arm, len(rs))
			continue
		}
		var le, st, lk []float64
		var extra float64
		for _, r := range rs {
			le = append(le, b2f(r.Lenient))
			st = append(st, b2f(r.Strict))
			lk = append(lk, float64(r.Lookup.Tokens()))
			extra += float64(r.Extra)
		}
		fmt.Fprintf(b, "| %s | %d | %s | %s | %.1f | %s |\n", arm, len(rs),
			pct(bootstrapMean(le, o.Boot, o.Seed)), pct(bootstrapMean(st, o.Boot, o.Seed)), extra/float64(len(rs)),
			num(bootstrapMean(lk, o.Boot, o.Seed)))
	}
	b.WriteString("\n")
}

// RenderMarkdown builds the generated section of RESULTS.md.
func RenderMarkdown(results []RepoResult, env Env, o Opts) string {
	var b strings.Builder
	arms := armOrder(o)
	fmt.Fprintf(&b, "Run on %s, %s/%s, %d CPUs, %s, stacklit %s, config: %s.\n\n", env.Date, env.OS, env.Arch, env.CPUs, env.GoVersion, env.Stacklit, env.Config)
	fmt.Fprintf(&b, "Parameters: grep output capped at %d lines (each cut to %d chars); an arm may open at most %d files in rank order; bootstrap %d resamples, seed %d; dependents strict rule allows at most %d unexpected modules; stacklit map+lookup inspects the top %d modules; BM25 prints %d paths and ranks %d; query budgets %v; repo-map budgets %v.\n\n",
		o.Baseline.MaxGrepLines, o.Baseline.MaxLineChars, o.ReadCap, o.Boot, o.Seed, o.MaxExtra, o.TopN, o.BM25Listed, o.BM25Ranked, o.QueryBudgets, o.MapBudgets)
	if p, err := exec.LookPath("aider"); err == nil {
		fmt.Fprintf(&b, "A real `aider` was found at %s but this harness does not drive it; the repo-map arms are the in-process approximation.\n\n", p)
	} else {
		b.WriteString("The real `aider` tool is not installed on the machine that produced these numbers, so the repo-map arms are the in-process approximation only.\n\n")
	}

	b.WriteString("### Repos and generation cost\n\n")
	b.WriteString("| Repo | Lang | Pinned commit | Tasks | Code files | Lines | Modules | Full dump tokens | Fits in context | stacklit.json tokens | derive map tokens | generate first (ms) | generate median (ms) | peak RSS (MB) | files dropped by parser | identical index every run | repo-map build (ms) | BM25 build (ms) |\n")
	b.WriteString("|---|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|---:|---:|---|---:|---:|\n")
	for _, r := range results {
		g := r.Gen
		rss := "n/a"
		if g.MaxRSSMB > 0 {
			rss = fmt.Sprintf("%.0f", g.MaxRSSMB)
		}
		fmt.Fprintf(&b, "| %s | %s | `%s` (%s) | %d | %d | %d | %d | %d | %s | %d | %d | %.0f | %.0f | %s | %d | %s | %.0f | %.0f |\n",
			r.Spec.Name, r.Spec.Language, r.Spec.SHA[:10], r.Spec.Ref, len(r.Tasks), g.Files, g.Lines, g.Modules,
			r.DumpCost.Tokens(), yn(r.DumpCost.Tokens() <= o.ContextWindow), g.IndexCost.Tokens(), g.MapCost.Tokens(),
			g.FirstMs, g.MedianMs, rss, g.ParseErrs, yn(g.Identical), r.RepoMapMs, r.BM25Ms)
	}
	fmt.Fprintf(&b, "\n\"Fits in context\" compares the full dump with a %d token window. Generation time is wall clock for `stacklit generate --quiet` (%d runs per repo, cold parse cache, outputs deleted before each run), peak RSS is the largest resident set of the child process. Files dropped by parser is read from one verbose `generate` run; identical index says whether every timed run wrote a byte-identical `stacklit.json`. There is no retry or CPU limit: the index the arms use is the plain output of `generate`. Repo-map and BM25 build times are a single in-process run; their memory is not measured. Tokens are bytes/4.\n\n", o.ContextWindow, results[0].Gen.Runs)

	total := 0
	for _, r := range results {
		total += len(r.Tasks)
	}
	fmt.Fprintf(&b, "### Locate tasks\n\n\"Where is X implemented?\" Lookup tokens are what the arm prints before any file is opened. Tokens to found add the files opened in rank order until the gold file is read; a miss is charged for every file it opened. %d tasks in total across all kinds.\n\n", total)
	locTable(&b, results, arms, o, func(t Task) bool { return t.Kind == "locate" })
	b.WriteString("### Add-feature tasks\n\n\"Where would I add Z?\" Gold is the file (or directory) a change would start in.\n\n")
	locTable(&b, results, arms, o, func(t Task) bool { return t.Kind == "where-to-add" })
	b.WriteString("### All localization tasks (locate plus add-feature)\n\n")
	locTable(&b, results, arms, o, func(t Task) bool { return isLocalization(t.Kind) })

	pairedTable(&b, results, o)

	b.WriteString("### Dependents (impact) tasks\n\n")
	fmt.Fprintf(&b, "Lenient: every expected module is reported (short module names accepted, extras ignored). Strict: every expected module is reported by exact name and at most %d unexpected modules are reported. The repo-map, BM25 and full-dump arms have no dependents procedure and are marked n/a.\n\n", o.MaxExtra)
	depTable(&b, results, arms, o)

	b.WriteString("### Per repo\n\nFound@5 over localization tasks, as hits/tasks, then mean tokens to found.\n\n")
	b.WriteString("| Arm |")
	for _, r := range results {
		fmt.Fprintf(&b, " %s (%s) |", r.Spec.Name, r.Spec.Language)
	}
	b.WriteString("\n|---|")
	for range results {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, arm := range arms {
		fmt.Fprintf(&b, "| %s |", arm)
		for _, r := range results {
			var hit, n, tok int
			costOnly := false
			for _, tr := range r.Tasks {
				if !isLocalization(tr.Task.Kind) {
					continue
				}
				a := tr.Arms[arm]
				n++
				tok += a.ToFound.Tokens()
				costOnly = a.CostOnly
				if a.Found(5) {
					hit++
				}
			}
			if costOnly {
				fmt.Fprintf(&b, " n/a, %s |", short(float64(tok)/float64(n)))
			} else {
				fmt.Fprintf(&b, " %d/%d, %s |", hit, n, short(float64(tok)/float64(n)))
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\nDependents tasks per repo, strict/lenient passes:\n\n| Arm |")
	for _, r := range results {
		fmt.Fprintf(&b, " %s |", r.Spec.Name)
	}
	b.WriteString("\n|---|")
	for range results {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, arm := range []string{armGrep, armStacklit} {
		arms2 := append([]string{arm}, nil...)
		for _, q := range o.QueryBudgets {
			if arm == armStacklit {
				arms2 = append(arms2, armQuery(q))
			}
		}
		for _, an := range arms2 {
			fmt.Fprintf(&b, "| %s |", an)
			for _, r := range results {
				var s, l, n int
				for _, tr := range r.Tasks {
					if tr.Task.Kind != "dependents" {
						continue
					}
					n++
					if tr.Arms[an].Strict {
						s++
					}
					if tr.Arms[an].Lenient {
						l++
					}
				}
				if n == 0 {
					b.WriteString(" - |")
				} else {
					fmt.Fprintf(&b, " %d/%d, %d/%d |", s, n, l, n)
				}
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n### Sensitivity to the token approximation\n\nMean tokens to found over all localization tasks under bytes/4 and under the second estimator (word and punctuation pieces x 1.3).\n\n")
	b.WriteString("| Arm | bytes/4 | pieces x 1.3 |\n|---|---:|---:|\n")
	for _, arm := range arms {
		rs := pick(results, arm, func(t Task) bool { return isLocalization(t.Kind) })
		var a, p float64
		for _, r := range rs {
			a += float64(r.ToFound.Tokens())
			p += float64(r.ToFound.PieceTokens())
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", arm, short(a/float64(len(rs))), short(p/float64(len(rs))))
	}

	b.WriteString("\n### Per task\n\nLocalization cells: rank of the first gold file in the arm's list, `-` when it is not in the first 5 files it was allowed to open. Dependents cells: `S` strict pass, `L` lenient only, `-` fail, `n/a` not applicable. Full dump is omitted (it always contains the answer).\n\n")
	b.WriteString("| Task | Kind |")
	var showArms []string
	for _, a := range arms {
		if a != armDump {
			showArms = append(showArms, a)
			fmt.Fprintf(&b, " %s |", a)
		}
	}
	b.WriteString("\n|---|---|")
	for range showArms {
		b.WriteString("---|")
	}
	b.WriteString("\n")
	for _, r := range results {
		for _, tr := range r.Tasks {
			fmt.Fprintf(&b, "| %s | %s |", tr.Task.ID, tr.Task.Kind)
			for _, an := range showArms {
				a := tr.Arms[an]
				cell := "-"
				switch {
				case a.NA:
					cell = "n/a"
				case tr.Task.Kind == "dependents":
					if a.Strict {
						cell = "S"
					} else if a.Lenient {
						cell = "L"
					}
				case a.Found(o.ReadCap):
					cell = strconv.Itoa(a.FirstGold)
				}
				fmt.Fprintf(&b, " %s |", cell)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// pairedTable reports per-task differences between arm pairs on the same
// localization tasks, in percentage points, with a bootstrap interval over
// tasks. Pairing removes task difficulty, so it is a sharper comparison than
// two overlapping unpaired intervals.
func pairedTable(b *strings.Builder, results []RepoResult, o Opts) {
	if len(o.QueryBudgets) == 0 {
		return
	}
	q := o.QueryBudgets[0]
	pairs := [][2]string{
		{armQueryRD(q), armBM25}, {armQueryRD(q), armGrep},
		{armQuery(q), armBM25}, {armQuery(q), armGrep}, {armQuery(q), armQueryPath(q)},
		{armStacklit, armBM25}, {armStacklit, armGrep},
		{armBM25, armGrep},
	}
	b.WriteString("### Paired differences (all localization tasks)\n\nFirst arm minus second arm on the same tasks, in percentage points, with a 95% bootstrap interval over tasks. An interval that excludes 0 is a difference larger than task-sampling noise.\n\n")
	b.WriteString("| First arm | Second arm | found@1 difference | found@5 difference |\n|---|---|---|---|\n")
	for _, p := range pairs {
		x := pick(results, p[0], func(t Task) bool { return isLocalization(t.Kind) })
		y := pick(results, p[1], func(t Task) bool { return isLocalization(t.Kind) })
		if len(x) == 0 || len(x) != len(y) {
			continue
		}
		d1 := make([]float64, len(x))
		d5 := make([]float64, len(x))
		for i := range x {
			d1[i] = (b2f(x[i].Found(1)) - b2f(y[i].Found(1))) * 100
			d5[i] = (b2f(x[i].Found(5)) - b2f(y[i].Found(5))) * 100
		}
		c1 := bootstrapMean(d1, o.Boot, o.Seed)
		c5 := bootstrapMean(d5, o.Boot, o.Seed)
		fmt.Fprintf(b, "| %s | %s | %+.0f [%+.0f, %+.0f] | %+.0f [%+.0f, %+.0f] |\n", p[0], p[1], c1.Mean, c1.Lo, c1.Hi, c5.Mean, c5.Lo, c5.Hi)
	}
	b.WriteString("\n")
}
