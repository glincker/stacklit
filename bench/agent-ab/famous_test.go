package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The model's knowledge cutoff assumed by the study, and the smallest number
// of changed lines that counts as "substantially changed" for an answer file.
const (
	famousCutoff     = "2026-07-01"
	famousMinChanged = 10
)

var famousRepoNames = []string{"django", "react", "tokio", "kafka", "rails"}

func famousRepos(t *testing.T) []repo {
	t.Helper()
	b, err := os.ReadFile("repos-famous.json")
	if err != nil {
		t.Fatal(err)
	}
	var rs []repo
	if err := json.Unmarshal(b, &rs); err != nil {
		t.Fatal(err)
	}
	return rs
}

func allQuestions(t *testing.T, dirs ...string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, d := range dirs {
		files, _ := filepath.Glob(filepath.Join(d, "*.json"))
		for _, f := range files {
			if strings.HasPrefix(filepath.Base(f), "PREREG") {
				continue
			}
			for _, k := range loadTasksFile(t, f) {
				m[normQuestion(k.Question)] = f + ":" + k.ID
			}
		}
	}
	return m
}

func normQuestion(q string) string {
	return strings.Join(strings.Fields(strings.ToLower(q)), " ")
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]`)

func squash(s string) string { return nonAlnum.ReplaceAllString(strings.ToLower(s), "") }

func TestFamousTasksAreNewAndWellFormed(t *testing.T) {
	old := allQuestions(t, "tasks", "tasks-round2")
	seenID, seenQ := map[string]bool{}, map[string]string{}
	kinds := map[string]int{}
	rs := famousRepos(t)
	if len(rs) != len(famousRepoNames) {
		t.Fatalf("%d repos, want %d", len(rs), len(famousRepoNames))
	}
	for i, r := range rs {
		if r.Name != famousRepoNames[i] || len(r.SHA) != 40 || r.URL == "" || r.ShallowSince == "" || r.Est == nil || r.Files == 0 {
			t.Errorf("repo %d (%s) is incomplete: %+v", i, r.Name, r)
		}
		ts := loadTasksFile(t, "tasks-famous/"+r.Name+".json")
		if len(ts) != 10 {
			t.Errorf("%s: %d tasks, want 10", r.Name, len(ts))
		}
		for _, n := range ts {
			if seenID[n.ID] {
				t.Errorf("duplicate id %s", n.ID)
			}
			seenID[n.ID] = true
			kinds[n.Kind]++
			if len(n.AllOf)+len(n.AnyOf) == 0 || n.Question == "" || n.Rubric == "" || len(n.Check) == 0 {
				t.Errorf("%s: incomplete task", n.ID)
			}
			if n.Kind == "impact" && n.Verify == nil {
				t.Errorf("%s: impact task without a grep verification", n.ID)
			}
			text := n.Question + n.Rubric
			if strings.ContainsRune(text, 0x2013) || strings.ContainsRune(text, 0x2014) {
				t.Errorf("%s: dash character", n.ID)
			}
			q := normQuestion(n.Question)
			if where, ok := old[q]; ok {
				t.Errorf("%s reuses a question of rounds 1 or 2 (%s)", n.ID, where)
			}
			if prev, ok := seenQ[q]; ok {
				t.Errorf("%s repeats the question of %s", n.ID, prev)
			}
			seenQ[q] = n.ID
			// A question must not hand over the answer: no answer path in it,
			// and a locate question must not contain an answer file's name.
			for _, p := range append(append([]string{}, n.AllOf...), n.AnyOf...) {
				if strings.Contains(n.Question, p) {
					t.Errorf("%s: the question contains the answer path %s", n.ID, p)
				}
				stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
				if n.Kind == "locate" && !strings.HasSuffix(p, "/") && len(stem) >= 5 && strings.Contains(squash(n.Question), squash(stem)) {
					t.Errorf("%s: a locate question contains the file name %q", n.ID, stem)
				}
			}
		}
	}
	want := map[string][2]int{"locate": {13, 17}, "trace": {12, 16}, "impact": {9, 11}, "add": {9, 11}}
	for k, r := range want {
		if kinds[k] < r[0] || kinds[k] > r[1] {
			t.Errorf("%d %s tasks, want %d to %d", kinds[k], k, r[0], r[1])
		}
	}
}

// changedSince returns the lines changed in path since the cutoff and whether
// a commit since then added it.
func changedSince(dir, path string) (lines int, added bool) {
	cmd := exec.Command("git", "-C", dir, "log", "--since="+famousCutoff, "--no-renames", "--numstat", "--format=%x00%H", "--", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(strings.TrimLeft(l, "\x00"))
		if len(f) == 3 {
			a, _ := strconv.Atoi(f[0])
			d, _ := strconv.Atoi(f[1])
			lines += a + d
		}
	}
	add, err := exec.Command("git", "-C", dir, "log", "--since="+famousCutoff, "--no-renames", "--diff-filter=A", "--format=%H", "--", path).Output()
	return lines, err == nil && strings.TrimSpace(string(add)) != ""
}

// TestFamousAnswersVerifiedByGrep re-checks every answer against shallow
// checkouts at the pinned commits (AGENT_AB_CLONES=<dir with one folder per
// repo>): the file exists, matches its check, impact lists equal the grep
// rule, and at least one answer file was added or substantially changed after
// the cutoff, so memory of older code cannot answer the task.
func TestFamousAnswersVerifiedByGrep(t *testing.T) {
	root := os.Getenv("AGENT_AB_CLONES")
	if root == "" {
		t.Skip("set AGENT_AB_CLONES to a directory of pinned checkouts to verify the answers")
	}
	for _, r := range famousRepos(t) {
		dir := filepath.Join(root, r.Name)
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil || strings.TrimSpace(string(out)) != r.SHA {
			t.Errorf("%s: checkout is not at %s", r.Name, r.SHA)
			continue
		}
		oldest, _ := exec.Command("git", "-C", dir, "log", "--format=%cs").Output()
		dates := strings.Fields(string(oldest))
		if len(dates) == 0 || dates[len(dates)-1] >= famousCutoff {
			t.Errorf("%s: the checkout history does not reach back before %s, so added files cannot be told from old ones", r.Name, famousCutoff)
			continue
		}
		ts := loadTasksFile(t, "tasks-famous/"+r.Name+".json")
		for _, p := range verifyTasks(dir, ts) {
			t.Errorf("%s: %s", r.Name, p)
		}
		fresh := 0
		for _, k := range ts {
			ok := false
			for _, p := range append(append([]string{}, k.AllOf...), k.AnyOf...) {
				if strings.HasSuffix(p, "/") {
					continue
				}
				if lines, added := changedSince(dir, p); added || lines >= famousMinChanged {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("%s %s: no answer file was added or changed by %d lines or more since %s", r.Name, k.ID, famousMinChanged, famousCutoff)
			} else {
				fresh++
			}
		}
		t.Logf("%s: %d of %d tasks have an answer file added or substantially changed since %s", r.Name, fresh, len(ts), famousCutoff)
	}
}

func famousCfg() config {
	c := baseCfg()
	c.repo, c.dryRun = "famous", true
	c.reposFile, c.tasksDir = "repos-famous.json", "tasks-famous"
	c.arms, c.trials, c.seed, c.grader, c.estMid = "control,stacklit,stacklit-e", 3, 20261109, "audited", 0.03
	return c
}

func TestFamousPlanSessionsAndArms(t *testing.T) {
	c := famousCfg()
	total := 0
	for _, n := range famousRepoNames {
		arms := armsFor(c, n)
		if len(arms) != 3 {
			t.Errorf("%s: arms %v", n, arms)
		}
		cc := c
		cc.repo = n
		ts, _, err := load(cc)
		if err != nil {
			t.Fatal(err)
		}
		total += len(plan(ts, c.trials, c.seed, arms))
	}
	if total != 450 {
		t.Errorf("plans %d sessions, want 450 (5 repos x 10 tasks x 3 trials x 3 arms)", total)
	}
	if err := runAll(c); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	c.mode = "memory"
	if err := runAll(c); err != nil {
		t.Fatalf("memory dry run: %v", err)
	}
	c.mode, c.dryRun = "run", false
	if err := runAll(c); err == nil || !strings.Contains(err.Error(), "yes-spend") {
		t.Errorf("a real run must refuse without --yes-spend, got %v", err)
	}
}

func TestFamousPresetFillsOnlyUnsetFlags(t *testing.T) {
	var c config
	applyFamousPreset(&c, map[string]bool{"cap-usd": true})
	c.cap = 7
	if c.arms != "control,stacklit,stacklit-e" || c.model != "claude-sonnet-5-5" || c.trials != 3 || c.maxBudget != 0.25 || !c.excludeMem || !c.famous {
		t.Errorf("preset wrong: %+v", c)
	}
	if !strings.HasSuffix(c.reposFile, "repos-famous.json") || !strings.HasSuffix(c.tasksDir, "tasks-famous") {
		t.Errorf("preset paths wrong: %s %s", c.reposFile, c.tasksDir)
	}
	d := config{mode: "memory", trials: 9}
	applyFamousPreset(&d, map[string]bool{"trials": true})
	if d.trials != 9 || !strings.HasSuffix(d.resultsDir, "memory") {
		t.Errorf("explicit trials must win and memory results go to memory/: %+v", d)
	}
	e := config{includeMem: true}
	applyFamousPreset(&e, nil)
	if e.excludeMem {
		t.Error("-include-memorised must keep memorised tasks")
	}
}

func TestMemorisationRule(t *testing.T) {
	if contaminated(1, 10) || contaminated(2, 10) || !contaminated(3, 10) || !contaminated(0, 0) {
		t.Error("a repo is contaminated above 20 percent, and unchecked repos are not eligible")
	}
}

func TestFamousReportRule(t *testing.T) {
	mk := func(name, arm string, ratio, lo, hi, dLo float64) scope {
		return scope{Name: name, Arm: arm, Base: "control", Compare: []cmp{
			{Metric: "cost_usd", Ratio: ratio, RatioCI: ci{Lo: lo, Hi: hi}},
			{Metric: "correct_audited", Diff: 0, DiffCI: ci{Lo: dLo, Hi: dLo + 0.1}},
		}}
	}
	names := []string{"a", "b", "c", "d", "e"}
	files := map[string]int{"a": 6000, "b": 7000, "c": 800, "d": 5000, "e": 7600}
	mem := map[string]rate{}
	var sc []scope
	for i, n := range names {
		mem[n] = rate{K: 0, N: 10}
		hi := 0.95
		if i >= 3 {
			hi = 1.10
		}
		sc = append(sc, mk(n, "stacklit", 0.9, 0.8, hi, -0.03))
	}
	sc = append(sc, mk("pooled", "stacklit", 0.93, 0.88, 0.98, -0.03))
	s := summary{Memorised: mem, Scopes: sc}
	rep := famousReport(s, names, files, nil)
	if !strings.Contains(rep, "rule R1): 3 of 5") || !strings.Contains(rep, "R2: at least 3 repos and pooled upper bound below 1.00): ALLOWED") {
		t.Errorf("three repos with savings and a pooled interval below 1 allow the general claim:\n%s", rep)
	}
	if !strings.Contains(rep, "large famous repos\" (rule R2 holds and at least 3 of the repos with savings have more than 5000 tracked files; 2 do): not allowed") {
		t.Errorf("only two large repos with savings must not allow the large wording:\n%s", rep)
	}
	// Two repos only: no general claim.
	s.Scopes[2].Compare[0].RatioCI.Hi = 1.2
	if rep := famousReport(s, names, files, nil); strings.Contains(rep, "pooled upper bound below 1.00): ALLOWED") {
		t.Errorf("two repos must not allow the general claim:\n%s", rep)
	}
	// A contaminated repo never counts, even with great numbers.
	s.Scopes[2].Compare[0].RatioCI.Hi = 0.9
	s.Memorised["a"] = rate{K: 5, N: 10}
	if rep := famousReport(s, names, files, nil); !strings.Contains(rep, "rule R1): 2 of 4 eligible") {
		t.Errorf("a contaminated repo must be excluded:\n%s", rep)
	}
	// Harm is reported.
	s.Scopes[4].Compare[0].RatioCI = ci{Lo: 1.1, Hi: 1.3}
	if rep := famousReport(s, names, files, nil); !strings.Contains(rep, "(rule R4, to be reported as harm): e") {
		t.Errorf("harm must be listed:\n%s", rep)
	}
	// No data never allows a claim.
	if rep := famousReport(summary{}, names, files, nil); strings.Contains(rep, "ALLOWED") {
		t.Errorf("no data must allow nothing:\n%s", rep)
	}
}

func TestReuseIndexCopiesGeneratedFilesOnly(t *testing.T) {
	src, gen, dst := t.TempDir(), filepath.Join(t.TempDir(), "gen"), t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(src, "tracked.go"), []byte("package x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.go")
	run("-c", "user.name=t", "-c", "user.email=t@t", "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "commit", "-q", "-m", "x")
	for _, f := range []string{"stacklit.json", "sub/stacklit.graph.json"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(src, f)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(src, f), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveGenerated(src, gen); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(gen, dst); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"stacklit.json", "sub/stacklit.graph.json"} {
		if _, err := os.Stat(filepath.Join(dst, f)); err != nil {
			t.Errorf("%s was not reused: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "tracked.go")); err == nil {
		t.Error("tracked files must not be copied")
	}
}

func TestMemorisedTasksAreDroppedFromPairs(t *testing.T) {
	res, tasks := t.TempDir(), t.TempDir()
	ts := []task{{ID: "x-01", Kind: "locate", AnyOf: []string{"a.go"}}, {ID: "x-02", Kind: "locate", AnyOf: []string{"b.go"}}}
	b, _ := json.Marshal(ts)
	if err := os.WriteFile(filepath.Join(tasks, "x.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	put := func(dir string, r record) {
		if err := writeRecord(filepath.Join(dir, "x", r.Arm, r.Task+"-t1.json"), r); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"x-01", "x-02"} {
		put(res, record{Repo: "x", Task: id, Trial: 1, Arm: "control"})
		put(res, record{Repo: "x", Task: id, Trial: 1, Arm: "stacklit"})
	}
	// The model knew x-01 from memory; x-02 it did not.
	put(filepath.Join(res, "memory"), record{Repo: "x", Task: "x-01", Trial: 1, Arm: "memory", Answer: "a.go"})
	put(filepath.Join(res, "memory"), record{Repo: "x", Task: "x-02", Trial: 1, Arm: "memory", Answer: "nothing"})
	c := config{resultsDir: res, tasksDir: tasks}
	if n := len(pairs(c, "x", "stacklit")); n != 2 {
		t.Fatalf("without exclusion %d pairs, want 2", n)
	}
	c.excludeMem = true
	ps := pairs(c, "x", "stacklit")
	if len(ps) != 1 || ps[0].task.ID != "x-02" {
		t.Fatalf("the memorised task must be dropped: %+v", ps)
	}
	if k, n := memoryRate(res, "x", loadTasks(tasks, "x")); k != 1 || n != 2 {
		t.Fatalf("memory rate %d of %d, want 1 of 2", k, n)
	}
}
