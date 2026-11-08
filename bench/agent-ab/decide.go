package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Decision thresholds, fixed in tasks-round2/PREREGISTRATION-round2.md before
// any round 2 session ran. They are constants here so the report cannot drift
// from the document.
const (
	maxPooledCostUpper   = 1.05 // P1: pooled cost ratio, upper bound of the 95% interval
	minPooledCorrectDiff = -0.05
	maxLargeRepoRatio    = 1.00 // P3: point estimate on every repo above largeRepoFiles
	largeRepoFiles       = 5000
	minTraceCorrectDiff  = -0.10 // G1
	maxImpactCostUpper   = 1.15  // G2
	maxVsDefaultUpper    = 1.05  // P4: not worse than the default wording
	maxSmallCostUpper    = 1.15  // S1, the small repo arm on repos below the limit
	minSmallCorrectDiff  = -0.10
)

// repoFileCounts reads the tracked file count of each repo from the repos file.
func repoFileCounts(file string) map[string]int {
	out := map[string]int{}
	b, err := os.ReadFile(file)
	if err != nil {
		return out
	}
	var rs []repo
	if json.Unmarshal(b, &rs) == nil {
		for _, r := range rs {
			out[r.Name] = r.Files
		}
	}
	return out
}

func findScope(s summary, name, arm, base string) *scope {
	for i := range s.Scopes {
		sc := &s.Scopes[i]
		b := sc.Base
		if b == "" {
			b = "control"
		}
		if sc.Name == name && sc.Arm == arm && b == base {
			return sc
		}
	}
	return nil
}

func findCmp(cs []cmp, metric string) *cmp {
	for i := range cs {
		if cs[i].Metric == metric {
			return &cs[i]
		}
	}
	return nil
}

type verdictRow struct {
	id, what, observed, rule string
	pass                     bool
}

func (r verdictRow) line() string {
	res := "FAIL"
	if r.pass {
		res = "pass"
	}
	return fmt.Sprintf("| %s | %s | %s | %s | %s |\n", r.id, r.what, r.observed, r.rule, res)
}

func missing(id, what, rule string) verdictRow {
	return verdictRow{id, what, "no data", rule, false}
}

// decisionReport evaluates the pre-registered rule for making steering
// variant e the default. It only reads the analysis summary.
func decisionReport(s summary, files map[string]int) string {
	var rows []verdictRow
	if sc := findScope(s, "pooled", "stacklit-e", "control"); sc != nil {
		if c := findCmp(sc.Compare, "cost_usd"); c != nil {
			rows = append(rows, verdictRow{"P1", "pooled cost ratio e / control, upper 95% bound",
				fmt.Sprintf("%.2f [%.2f, %.2f]", c.Ratio, c.RatioCI.Lo, c.RatioCI.Hi), fmt.Sprintf("upper < %.2f", maxPooledCostUpper), c.RatioCI.Hi < maxPooledCostUpper})
		} else {
			rows = append(rows, missing("P1", "pooled cost ratio", "upper bound"))
		}
		if c := findCmp(sc.Compare, "correct_audited"); c != nil {
			rows = append(rows, verdictRow{"P2", "pooled correctness difference e minus control, lower 95% bound",
				fmt.Sprintf("%+.1f points [%+.1f, %+.1f]", 100*c.Diff, 100*c.DiffCI.Lo, 100*c.DiffCI.Hi), fmt.Sprintf("lower > %+.0f points", 100*minPooledCorrectDiff), c.DiffCI.Lo > minPooledCorrectDiff})
		} else {
			rows = append(rows, missing("P2", "pooled correctness", "lower bound"))
		}
		if kc := findCmp(sc.ByKind["trace"], "correct_audited"); kc != nil {
			rows = append(rows, verdictRow{"G1", "trace questions, correctness difference e minus control, lower bound",
				fmt.Sprintf("%+.1f points [%+.1f, %+.1f]", 100*kc.Diff, 100*kc.DiffCI.Lo, 100*kc.DiffCI.Hi), fmt.Sprintf("lower > %+.0f points", 100*minTraceCorrectDiff), kc.DiffCI.Lo > minTraceCorrectDiff})
		} else {
			rows = append(rows, missing("G1", "trace correctness", "lower bound"))
		}
		if kc := findCmp(sc.ByKind["impact"], "cost_usd"); kc != nil {
			rows = append(rows, verdictRow{"G2", "impact questions, cost ratio e / control, upper bound",
				fmt.Sprintf("%.2f [%.2f, %.2f]", kc.Ratio, kc.RatioCI.Lo, kc.RatioCI.Hi), fmt.Sprintf("upper < %.2f", maxImpactCostUpper), kc.RatioCI.Hi < maxImpactCostUpper})
		} else {
			rows = append(rows, missing("G2", "impact cost", "upper bound"))
		}
	} else {
		rows = append(rows, missing("P1", "pooled cost ratio", "upper bound"), missing("P2", "pooled correctness", "lower bound"),
			missing("G1", "trace correctness", "lower bound"), missing("G2", "impact cost", "upper bound"))
	}
	large := 0
	for name, n := range files {
		if n <= largeRepoFiles {
			continue
		}
		large++
		sc := findScope(s, name, "stacklit-e", "control")
		var c *cmp
		if sc != nil {
			c = findCmp(sc.Compare, "cost_usd")
		}
		id := "P3 " + name
		if c == nil {
			rows = append(rows, missing(id, fmt.Sprintf("cost ratio e / control on %s (%d files)", name, n), "point estimate"))
			continue
		}
		rows = append(rows, verdictRow{id, fmt.Sprintf("cost ratio e / control on %s (%d files)", name, n),
			fmt.Sprintf("%.2f [%.2f, %.2f]", c.Ratio, c.RatioCI.Lo, c.RatioCI.Hi), fmt.Sprintf("point < %.2f", maxLargeRepoRatio), c.Ratio < maxLargeRepoRatio})
	}
	if large == 0 {
		rows = append(rows, missing("P3", "no repo above the size threshold in the repos file", "point estimate"))
	}
	if sc := findScope(s, "pooled", "stacklit-e", "stacklit"); sc != nil {
		if c := findCmp(sc.Compare, "cost_usd"); c != nil {
			rows = append(rows, verdictRow{"P4", "pooled cost ratio e / default wording a, upper bound",
				fmt.Sprintf("%.2f [%.2f, %.2f]", c.Ratio, c.RatioCI.Lo, c.RatioCI.Hi), fmt.Sprintf("upper < %.2f", maxVsDefaultUpper), c.RatioCI.Hi < maxVsDefaultUpper})
		} else {
			rows = append(rows, missing("P4", "e versus a", "upper bound"))
		}
	} else {
		rows = append(rows, missing("P4", "e versus a", "upper bound"))
	}
	allPass := true
	var b strings.Builder
	b.WriteString("# Round 2 decision report\n\nGenerated from summary.json by the rule in PREREGISTRATION-round2.md. Steering variant e becomes the default only if every row below passes.\n\n")
	b.WriteString("| id | criterion | observed | rule | result |\n|---|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(r.line())
		allPass = allPass && r.pass
	}
	if allPass {
		b.WriteString("\nVerdict: every criterion passes, so e becomes the default steering.\n")
	} else {
		b.WriteString("\nVerdict: at least one criterion fails or has no data, so the default stays a and e stays opt-in.\n")
	}
	var sm []verdictRow
	for name := range files {
		sc := findScope(s, name, "stacklit-e-small", "control")
		if sc == nil {
			continue
		}
		cc, kc := findCmp(sc.Compare, "cost_usd"), findCmp(sc.Compare, "correct_audited")
		if cc == nil || kc == nil {
			continue
		}
		sm = append(sm, verdictRow{"S1 " + name, "small repo arm: cost ratio vs control, upper bound; correctness difference, lower bound",
			fmt.Sprintf("%.2f [%.2f, %.2f]; %+.1f points [%+.1f, %+.1f]", cc.Ratio, cc.RatioCI.Lo, cc.RatioCI.Hi, 100*kc.Diff, 100*kc.DiffCI.Lo, 100*kc.DiffCI.Hi),
			fmt.Sprintf("upper < %.2f; lower > %+.0f points", maxSmallCostUpper, 100*minSmallCorrectDiff), cc.RatioCI.Hi < maxSmallCostUpper && kc.DiffCI.Lo > minSmallCorrectDiff})
	}
	if len(sm) > 0 {
		b.WriteString("\nSmall repo mode (decides whether it stays on by default):\n\n| id | criterion | observed | rule | result |\n|---|---|---|---|---|\n")
		for _, r := range sm {
			b.WriteString(r.line())
		}
	}
	return b.String()
}
