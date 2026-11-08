package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// metric names a per-session number the analysis compares between arms.
type metric struct {
	Name string
	Desc string
	Fn   func(r record, t task) (float64, bool)
}

func sessionTokens(r record) float64 {
	u := r.Usage
	return float64(u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens)
}

func uncachedEq(r record) float64 {
	u := r.Usage
	return float64(u.InputTokens+u.OutputTokens+u.CacheCreationTokens) + 0.1*float64(u.CacheReadTokens)
}

var metrics = []metric{
	{"cost_usd", "notional cost, USD", func(r record, _ task) (float64, bool) { return r.CostUSD, true }},
	{"total_tokens", "input + output + cache creation + cache read", func(r record, _ task) (float64, bool) { return sessionTokens(r), true }},
	{"uncached_eq_tokens", "input + output + cache creation + 0.1 x cache read", func(r record, _ task) (float64, bool) { return uncachedEq(r), true }},
	{"turns", "model turns", func(r record, _ task) (float64, bool) { return float64(r.Turns), true }},
	{"tool_calls", "all tool calls", func(r record, _ task) (float64, bool) { return float64(sum(r.ToolCalls)), true }},
	{"read_calls", "Read calls", func(r record, _ task) (float64, bool) { return float64(r.ToolCalls["Read"]), true }},
	{"grep_glob_calls", "Grep + Glob calls", func(r record, _ task) (float64, bool) {
		return float64(r.ToolCalls["Grep"] + r.ToolCalls["Glob"]), true
	}},
	{"files_read", "distinct files opened with Read", func(r record, _ task) (float64, bool) { return float64(r.FilesRead), true }},
	{"read_kb", "KB returned by Read", func(r record, _ task) (float64, bool) { return float64(r.ReadBytes) / 1024, true }},
	{"seconds", "wall seconds (includes clone and stacklit setup)", func(r record, _ task) (float64, bool) { return r.Seconds, true }},
	{"correct", "answer names the expected paths (0 or 1)", func(r record, _ task) (float64, bool) {
		if r.Correct {
			return 1, true
		}
		return 0, true
	}},
	{"correct_audited", "correct after the three documented grading fixes (see RESULTS.md)", func(r record, t task) (float64, bool) {
		if gradeAudited(r.Answer, t) {
			return 1, true
		}
		return 0, true
	}},
	{"recall", "fraction of all_of paths named (tasks with all_of only)", func(r record, t task) (float64, bool) {
		if len(t.AllOf) == 0 {
			return 0, false
		}
		_, rc := grade(r.Answer, t)
		return rc, true
	}},
}

type pair struct {
	repo  string
	task  task
	trial int
	a, b  record // a = control, b = treatment
}

type ci struct {
	Lo float64 `json:"lo"`
	Hi float64 `json:"hi"`
}

type cmp struct {
	Metric      string  `json:"metric"`
	N           int     `json:"n_pairs"`
	MeanControl float64 `json:"mean_control"`
	MeanArm     float64 `json:"mean_arm"`
	Diff        float64 `json:"mean_diff"`
	DiffCI      ci      `json:"diff_ci95"`
	Ratio       float64 `json:"ratio"`
	RatioCI     ci      `json:"ratio_ci95"`
}

type rate struct {
	K    int     `json:"k"`
	N    int     `json:"n"`
	Rate float64 `json:"rate"`
	CI   ci      `json:"wilson95"`
}

type scope struct {
	Name           string           `json:"name"`
	Arm            string           `json:"arm"`
	Pairs          int              `json:"n_pairs"`
	Compare        []cmp            `json:"comparisons"`
	Correct        map[string]rate  `json:"correctness"`
	CorrectAudited map[string]rate  `json:"correctness_audited"`
	StacklitUsed   rate             `json:"stacklit_used_in_treatment"`
	HookFired      rate             `json:"hook_fired_in_treatment"`
	ByKind         map[string][]cmp `json:"by_kind,omitempty"`
}

type summary struct {
	Date      string            `json:"date"`
	Model     string            `json:"model"`
	Seed      int64             `json:"bootstrap_seed"`
	Resamples int               `json:"bootstrap_resamples"`
	LedgerUSD float64           `json:"ledger_total_notional_usd"`
	Sessions  map[string]int    `json:"sessions_per_arm"`
	Memorised map[string]rate   `json:"memorisation_check_correct"`
	Scopes    []scope           `json:"scopes"`
	Metrics   map[string]string `json:"metric_definitions"`
}

func loadRecords(resultsDir, repoName, arm string) []record {
	files, _ := filepath.Glob(filepath.Join(resultsDir, repoName, arm, "*.json"))
	sort.Strings(files)
	var out []record
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var r record
		if json.Unmarshal(b, &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

func loadTasks(dir, repoName string) map[string]task {
	b, err := os.ReadFile(filepath.Join(dir, repoName+".json"))
	m := map[string]task{}
	if err != nil {
		return m
	}
	var ts []task
	_ = json.Unmarshal(b, &ts)
	for _, t := range ts {
		m[t.ID] = t
	}
	return m
}

func pairs(c config, repoName, arm string) []pair {
	tasks := loadTasks(c.tasksDir, repoName)
	ctl := map[string]record{}
	for _, r := range loadRecords(c.resultsDir, repoName, "control") {
		ctl[fmt.Sprintf("%s|%d", r.Task, r.Trial)] = r
	}
	var out []pair
	for _, r := range loadRecords(c.resultsDir, repoName, arm) {
		k := fmt.Sprintf("%s|%d", r.Task, r.Trial)
		if a, ok := ctl[k]; ok {
			out = append(out, pair{repo: repoName, task: tasks[r.Task], trial: r.Trial, a: a, b: r})
		}
	}
	return out
}

func compare(ps []pair, m metric, rng *rand.Rand, resamples int) (cmp, bool) {
	var a, b []float64
	for _, p := range ps {
		x, ok1 := m.Fn(p.a, p.task)
		y, ok2 := m.Fn(p.b, p.task)
		if ok1 && ok2 {
			a, b = append(a, x), append(b, y)
		}
	}
	n := len(a)
	if n == 0 {
		return cmp{Metric: m.Name}, false
	}
	mean := func(v []float64) float64 {
		s := 0.0
		for _, x := range v {
			s += x
		}
		return s / float64(len(v))
	}
	res := cmp{Metric: m.Name, N: n, MeanControl: mean(a), MeanArm: mean(b)}
	res.Diff = res.MeanArm - res.MeanControl
	if res.MeanControl != 0 {
		res.Ratio = res.MeanArm / res.MeanControl
	}
	diffs, ratios := make([]float64, 0, resamples), make([]float64, 0, resamples)
	for i := 0; i < resamples; i++ {
		var sa, sb float64
		for j := 0; j < n; j++ {
			k := rng.Intn(n)
			sa += a[k]
			sb += b[k]
		}
		diffs = append(diffs, (sb-sa)/float64(n))
		if sa != 0 {
			ratios = append(ratios, sb/sa)
		}
	}
	res.DiffCI = pct(diffs)
	res.RatioCI = pct(ratios)
	return res, true
}

func pct(v []float64) ci {
	if len(v) == 0 {
		return ci{}
	}
	sort.Float64s(v)
	return ci{Lo: v[int(0.025*float64(len(v)))], Hi: v[int(0.975*float64(len(v)))-1]}
}

// wilson returns the 95 percent Wilson score interval.
func wilson(k, n int) rate {
	if n == 0 {
		return rate{}
	}
	const z = 1.96
	p := float64(k) / float64(n)
	d := 1 + z*z/float64(n)
	centre := (p + z*z/(2*float64(n))) / d
	half := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n))) / d
	return rate{K: k, N: n, Rate: p, CI: ci{Lo: math.Max(0, centre-half), Hi: math.Min(1, centre+half)}}
}

func buildScope(name, arm string, ps []pair, rng *rand.Rand, resamples int) scope {
	s := scope{Name: name, Arm: arm, Pairs: len(ps), Correct: map[string]rate{}, ByKind: map[string][]cmp{}}
	for _, m := range metrics {
		if c, ok := compare(ps, m, rng, resamples); ok {
			s.Compare = append(s.Compare, c)
		}
	}
	var ka, kb, used, hook int
	for _, p := range ps {
		if p.a.Correct {
			ka++
		}
		if p.b.Correct {
			kb++
		}
		if p.b.StacklitCalls > 0 {
			used++
		}
		if p.b.HookEvents > 0 {
			hook++
		}
	}
	s.Correct["control"], s.Correct[arm] = wilson(ka, len(ps)), wilson(kb, len(ps))
	var aa, ab int
	for _, p := range ps {
		if gradeAudited(p.a.Answer, p.task) {
			aa++
		}
		if gradeAudited(p.b.Answer, p.task) {
			ab++
		}
	}
	s.CorrectAudited = map[string]rate{"control": wilson(aa, len(ps)), arm: wilson(ab, len(ps))}
	s.StacklitUsed, s.HookFired = wilson(used, len(ps)), wilson(hook, len(ps))
	byKind := map[string][]pair{}
	for _, p := range ps {
		byKind[p.task.Kind] = append(byKind[p.task.Kind], p)
	}
	for kind, kp := range byKind {
		for _, m := range metrics {
			switch m.Name {
			case "cost_usd", "uncached_eq_tokens", "tool_calls", "files_read", "correct", "correct_audited":
				if c, ok := compare(kp, m, rng, resamples); ok {
					s.ByKind[kind] = append(s.ByKind[kind], c)
				}
			}
		}
	}
	return s
}

func analyze(c config, repoNames []string) error {
	const seed, resamples = 20260508, 10000
	sum := summary{Date: time.Now().UTC().Format("2006-01-02"), Seed: seed, Resamples: resamples,
		Sessions: map[string]int{}, Memorised: map[string]rate{}, Metrics: map[string]string{}}
	sum.LedgerUSD, _ = ledgerTotal(c.ledger)
	for _, m := range metrics {
		sum.Metrics[m.Name] = m.Desc
	}
	var arms []string
	for _, a := range []string{"stacklit", "strict"} {
		for _, rn := range repoNames {
			if len(loadRecords(c.resultsDir, rn, a)) > 0 {
				arms = append(arms, a)
				break
			}
		}
	}
	for _, rn := range repoNames {
		for _, a := range []string{"control", "stacklit", "strict"} {
			recs := loadRecords(c.resultsDir, rn, a)
			sum.Sessions[a] += len(recs)
			for _, r := range recs {
				if r.Model != "" {
					sum.Model = r.Model
				}
			}
		}
		mem := loadRecords(filepath.Join(c.resultsDir, "memory"), rn, "memory")
		k := 0
		for _, r := range mem {
			if r.Correct {
				k++
			}
		}
		sum.Memorised[rn] = wilson(k, len(mem))
	}
	sum.Sessions["memory"] = 18
	for _, a := range arms {
		rng := rand.New(rand.NewSource(seed))
		var all []pair
		for _, rn := range repoNames {
			ps := pairs(c, rn, a)
			all = append(all, ps...)
			if len(ps) > 0 {
				sum.Scopes = append(sum.Scopes, buildScope(rn, a, ps, rng, resamples))
			}
		}
		sum.Scopes = append(sum.Scopes, buildScope("pooled", a, all, rng, resamples))
	}
	b, _ := json.MarshalIndent(sum, "", "  ")
	if err := os.WriteFile(filepath.Join(filepath.Dir(c.ledger), "summary.json"), b, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(c.ledger), "tables.md"), []byte(tables(sum)), 0o644)
}

func f(v float64, name string) string {
	switch name {
	case "cost_usd":
		return fmt.Sprintf("%.4f", v)
	case "correct", "recall":
		return fmt.Sprintf("%.3f", v)
	case "total_tokens", "uncached_eq_tokens":
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func cmpRow(c cmp) string {
	g := func(v float64) string { return f(v, c.Metric) }
	return fmt.Sprintf("| %s | %d | %s | %s | %s [%s, %s] | %.2f [%.2f, %.2f] |\n", c.Metric, c.N,
		g(c.MeanControl), g(c.MeanArm), g(c.Diff), g(c.DiffCI.Lo), g(c.DiffCI.Hi), c.Ratio, c.RatioCI.Lo, c.RatioCI.Hi)
}

func pc(r rate) string {
	return fmt.Sprintf("%d/%d = %.1f%% [%.1f, %.1f]", r.K, r.N, 100*r.Rate, 100*r.CI.Lo, 100*r.CI.Hi)
}

func tables(s summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Model: %s. Bootstrap: %d resamples, seed %d, paired by task and trial. Ledger total: %.2f USD notional.\n\n", s.Model, s.Resamples, s.Seed, s.LedgerUSD)
	for _, sc := range s.Scopes {
		fmt.Fprintf(&b, "### %s: control vs %s (%d pairs)\n\n| metric | n | control | %s | diff (arm minus control) [95%% CI] | ratio (arm / control) [95%% CI] |\n|---|---|---|---|---|---|\n", sc.Name, sc.Arm, sc.Pairs, sc.Arm)
		for _, c := range sc.Compare {
			b.WriteString(cmpRow(c))
		}
		fmt.Fprintf(&b, "\nCorrectness after audit fixes: control %s; %s %s.\n", pc(sc.CorrectAudited["control"]), sc.Arm, pc(sc.CorrectAudited[sc.Arm]))
		fmt.Fprintf(&b, "\nCorrectness (pre-registered grader): control %s; %s %s. Stacklit commands used in %s of treatment sessions; session hook fired in %s.\n\n",
			pc(sc.Correct["control"]), sc.Arm, pc(sc.Correct[sc.Arm]), pc(sc.StacklitUsed), pc(sc.HookFired))
		kinds := make([]string, 0, len(sc.ByKind))
		for k := range sc.ByKind {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		if len(kinds) > 0 {
			fmt.Fprintf(&b, "By task kind (%s):\n\n| kind | metric | n | control | %s | diff [95%% CI] | ratio [95%% CI] |\n|---|---|---|---|---|---|---|\n", sc.Name, sc.Arm)
			for _, k := range kinds {
				for _, c := range sc.ByKind[k] {
					row := cmpRow(c)
					b.WriteString("| " + k + " " + row)
				}
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// auditedRe also accepts a leading "./" and an absolute checkout prefix,
// which the pre-registered matcher rejected (a harness bug found in the
// transcript spot check).
func auditedRe(p string) *regexp.Regexp {
	return regexp.MustCompile(`(^|[^A-Za-z0-9_./-]|\./|co-[0-9]+/)` + regexp.QuoteMeta(p))
}

// auditedTask applies the two task defects found in the spot check: fd-07
// also accepts a new file under src/adapters/ (the model names the file to
// create), and fd-09 also accepts src/analytics.ts (PostHogAdapter forwards
// events there; the pinned answer missed it).
func auditedTask(t task) task {
	switch t.ID {
	case "fd-07":
		t.AnyOf = append(append([]string{}, t.AnyOf...), "src/adapters/")
	case "fd-09":
		t.AnyOf = append(append([]string{}, t.AnyOf...), "src/analytics.ts")
	}
	return t
}

func gradeAudited(answer string, t task) bool {
	t = auditedTask(t)
	for _, p := range t.AllOf {
		if !auditedRe(p).MatchString(answer) {
			return false
		}
	}
	if len(t.AnyOf) == 0 {
		return true
	}
	for _, p := range t.AnyOf {
		if auditedRe(p).MatchString(answer) {
			return true
		}
	}
	return false
}
