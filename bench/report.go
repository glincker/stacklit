package main

import (
	"fmt"
	"os"
	"sort"
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

func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

type agg struct {
	n                                          int
	baseOK, stkOK                              int
	baseLookup, baseE2E                        int
	stkLookup, stkQuery, stkE2E                int
	baseLookupP, baseE2EP, stkLookupP, stkE2EP int
}

func (a *agg) add(tr TaskResult) {
	b, s := tr.Baseline, tr.Stacklit
	a.n++
	if tr.Task.Kind == "dependents" {
		if b.DependentsOK {
			a.baseOK++
		}
	} else if b.FoundInRead {
		a.baseOK++
	}
	if s.Found {
		a.stkOK++
	}
	a.baseLookup += b.GrepCost.Tokens()
	a.baseE2E += b.GrepCost.Add(b.ReadCost).Tokens()
	a.stkLookup += s.MapCost.Add(s.QueryCost).Tokens()
	a.stkQuery += s.QueryCost.Tokens()
	a.stkE2E += s.MapCost.Add(s.QueryCost).Add(s.ConfirmCost).Tokens()
	a.baseLookupP += b.GrepCost.PieceTokens()
	a.baseE2EP += b.GrepCost.Add(b.ReadCost).PieceTokens()
	a.stkLookupP += s.MapCost.Add(s.QueryCost).PieceTokens()
	a.stkE2EP += s.MapCost.Add(s.QueryCost).Add(s.ConfirmCost).PieceTokens()
}

func mean(total, n int) int {
	if n == 0 {
		return 0
	}
	return (total + n/2) / n
}

func ratio(a, b int) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1fx", float64(a)/float64(b))
}

// RenderMarkdown builds the generated section of RESULTS.md.
func RenderMarkdown(results []RepoResult, env Env, bp BaselineParams) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Run on %s, %s/%s, %d CPUs, %s, stacklit %s.\n\n", env.Date, env.OS, env.Arch, env.CPUs, env.GoVersion, env.Stacklit)
	fmt.Fprintf(&b, "Baseline parameters: grep output capped at %d lines (each cut to %d chars), then the top %d ranked files read in full.\n\n",
		bp.MaxGrepLines, bp.MaxLineChars, bp.ReadTopFiles)

	b.WriteString("### Index size and generation cost\n\n")
	b.WriteString("| Repo | Lang | Pinned commit | Files | Lines | Modules | stacklit.json bytes | stacklit.json tokens | derive map tokens | generate first (ms) | generate median (ms) | peak RSS (MB) |\n")
	b.WriteString("|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, r := range results {
		g := r.Gen
		rss := "n/a"
		if g.MaxRSSMB > 0 {
			rss = fmt.Sprintf("%.0f", g.MaxRSSMB)
		}
		fmt.Fprintf(&b, "| %s | %s | `%s` (%s) | %d | %d | %d | %d | %d | %d | %.0f | %.0f | %s |\n",
			r.Spec.Name, r.Spec.Language, r.Spec.SHA[:10], r.Spec.Ref, g.Files, g.Lines, g.Modules,
			g.IndexCost.Bytes, g.IndexCost.Tokens(), g.MapCost.Tokens(), g.FirstMs, g.MedianMs, rss)
	}
	fmt.Fprintf(&b, "\nTimes are wall clock for `stacklit generate --quiet` (%d runs per repo, outputs deleted before each run). Tokens are bytes/4.\n\n", results[0].Gen.Runs)

	b.WriteString("### Per-task results\n\n")
	b.WriteString("Lookup = tokens to locate the answer. End-to-end = lookup plus the confirming reads. Found = the hand-verified answer was reached.\n\n")
	for _, r := range results {
		fmt.Fprintf(&b, "**%s**\n\n", r.Spec.Name)
		b.WriteString("| Task | Kind | grep tokens | grep+read tokens | grep+read found | map+query tokens | map+query+read tokens | stacklit found |\n")
		b.WriteString("|---|---|---:|---:|---|---:|---:|---|\n")
		for _, tr := range r.Tasks {
			bl, st := tr.Baseline, tr.Stacklit
			found := bl.FoundInRead
			if tr.Task.Kind == "dependents" {
				found = bl.DependentsOK
			}
			fmt.Fprintf(&b, "| %s | %s | %d | %d | %s | %d | %d | %s |\n",
				tr.Task.ID, tr.Task.Kind, bl.GrepCost.Tokens(), bl.GrepCost.Add(bl.ReadCost).Tokens(), yn(found),
				st.MapCost.Add(st.QueryCost).Tokens(), st.MapCost.Add(st.QueryCost).Add(st.ConfirmCost).Tokens(), yn(st.Found))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Aggregate\n\n")
	b.WriteString("Mean tokens per task. \"Query only\" excludes the derive map, which is a standing cost when it is injected into the agent's config file.\n\n")
	b.WriteString("| Scope | Tasks | grep lookup | grep+read | map+query lookup | query only | map+query+read | grep+read found | stacklit found | lookup ratio (grep / stacklit) | end-to-end ratio |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	kinds := []string{"locate", "dependents", "where-to-add"}
	row := func(label string, a *agg) {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d | %d/%d | %d/%d | %s | %s |\n", label, a.n,
			mean(a.baseLookup, a.n), mean(a.baseE2E, a.n), mean(a.stkLookup, a.n), mean(a.stkQuery, a.n), mean(a.stkE2E, a.n),
			a.baseOK, a.n, a.stkOK, a.n, ratio(a.baseLookup, a.stkLookup), ratio(a.baseE2E, a.stkE2E))
	}
	var overall agg
	var overallBy = map[string]*agg{}
	for _, r := range results {
		var per agg
		for _, tr := range r.Tasks {
			per.add(tr)
			overall.add(tr)
			if overallBy[tr.Task.Kind] == nil {
				overallBy[tr.Task.Kind] = &agg{}
			}
			overallBy[tr.Task.Kind].add(tr)
		}
		row(r.Spec.Name, &per)
	}
	for _, k := range kinds {
		if a := overallBy[k]; a != nil {
			row("all repos, "+k, a)
		}
	}
	row("**all repos, all tasks**", &overall)

	b.WriteString("\n### Sensitivity to the token approximation\n\n")
	b.WriteString("Same totals under the second estimator (word and punctuation pieces x 1.3) instead of bytes/4.\n\n")
	b.WriteString("| Estimator | mean grep lookup | mean map+query lookup | lookup ratio | mean grep+read | mean map+query+read | end-to-end ratio |\n|---|---:|---:|---:|---:|---:|---:|\n")
	fmt.Fprintf(&b, "| bytes/4 | %d | %d | %s | %d | %d | %s |\n",
		mean(overall.baseLookup, overall.n), mean(overall.stkLookup, overall.n), ratio(overall.baseLookup, overall.stkLookup),
		mean(overall.baseE2E, overall.n), mean(overall.stkE2E, overall.n), ratio(overall.baseE2E, overall.stkE2E))
	fmt.Fprintf(&b, "| pieces x 1.3 | %d | %d | %s | %d | %d | %s |\n",
		mean(overall.baseLookupP, overall.n), mean(overall.stkLookupP, overall.n), ratio(overall.baseLookupP, overall.stkLookupP),
		mean(overall.baseE2EP, overall.n), mean(overall.stkE2EP, overall.n), ratio(overall.baseE2EP, overall.stkE2EP))

	b.WriteString("\n### Misses\n\n")
	b.WriteString("Every task where either procedure did not reach the verified answer.\n\n")
	var misses []string
	for _, r := range results {
		for _, tr := range r.Tasks {
			bl, st := tr.Baseline, tr.Stacklit
			baseOK := bl.FoundInRead
			if tr.Task.Kind == "dependents" {
				baseOK = bl.DependentsOK
			}
			if baseOK && st.Found {
				continue
			}
			var line string
			if tr.Task.Kind == "dependents" {
				line = fmt.Sprintf("- %s (%s): expected %v. grep path found %v (recall %.2f, precision %.2f). stacklit depended_by %v (recall %.2f, precision %.2f).",
					tr.Task.ID, tr.Task.Question, tr.Task.ExpectModules, bl.Modules, bl.Recall, bl.Precision, st.DependedBy, st.Recall, st.Precision)
			} else {
				line = fmt.Sprintf("- %s (%s): expected %v / %v. grep read %v (found in grep output: %s). stacklit top modules %v.",
					tr.Task.ID, tr.Task.Question, tr.Task.ExpectPaths, tr.Task.ExpectModules, bl.ReadFiles, yn(bl.FoundInGrep), st.TopModules)
			}
			misses = append(misses, line)
		}
	}
	sort.Strings(misses)
	if len(misses) == 0 {
		b.WriteString("None.\n")
	} else {
		b.WriteString(strings.Join(misses, "\n") + "\n")
	}
	return b.String()
}
