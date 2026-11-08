package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func baseCfg() config {
	return config{repo: "theauth-go", reposFile: "repos.json", tasksDir: "tasks", trials: 3, arms: "control,stacklit", maxBudget: 1.5, cap: 60, estMid: 0.3}
}

func TestRefusesWithoutYesSpend(t *testing.T) {
	err := run(baseCfg())
	if err == nil || !strings.Contains(err.Error(), "--yes-spend") {
		t.Fatalf("must refuse without --yes-spend, got %v", err)
	}
}

func TestDryRunNeverSpends(t *testing.T) {
	c := baseCfg()
	c.dryRun = true
	if err := run(c); err != nil {
		t.Fatal(err)
	}
}

func TestTaskFilesValid(t *testing.T) {
	for _, name := range []string{"levelrail", "theauth-go", "featuredrop"} {
		c := baseCfg()
		c.repo = name
		tasks, r, err := load(c)
		if err != nil || len(tasks) != 10 || len(r.SHA) != 40 {
			t.Fatalf("%s: %d tasks, sha %q, %v", name, len(tasks), r.SHA, err)
		}
		for _, k := range tasks {
			if len(k.AllOf)+len(k.AnyOf) == 0 || k.Question == "" || k.Rubric == "" {
				t.Fatalf("%s %s incomplete", name, k.ID)
			}
		}
	}
}

func TestPlanSeededAndComplete(t *testing.T) {
	arms := []string{"control", "stacklit"}
	tasks := []task{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	p1, p2, p3 := plan(tasks, 3, 7, arms), plan(tasks, 3, 7, arms), plan(tasks, 3, 8, arms)
	if len(p1) != 18 || !reflect.DeepEqual(p1, p2) || reflect.DeepEqual(p1, p3) {
		t.Fatal("plan must be complete, seeded and seed dependent")
	}
}

func TestGrade(t *testing.T) {
	tk := task{AllOf: []string{"audit/events.go", "internal/audit/events.go"}}
	if ok, rc := grade("internal/audit/events.go", tk); ok || rc != 0.5 {
		t.Fatalf("substring inside a longer path must not match: %v %v", ok, rc)
	}
	if ok, _ := grade("internal/audit/events.go, audit/events.go", tk); !ok {
		t.Fatal("both paths named")
	}
	any := task{AnyOf: []string{"src/a.ts", "src/b.ts"}}
	if ok, _ := grade("`src/b.ts`", any); !ok {
		t.Fatal("any-of")
	}
	if ok, _ := grade("src/c.ts", any); ok {
		t.Fatal("none named")
	}
}

func TestParseStream(t *testing.T) {
	s := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"model":"m","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"/x/a.go"}},{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"stacklit query x"}}]}}
{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"t1","content":"hello"}]}}
{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","unifiedWindows":{"seven_day":{"utilization":0.5}}}}
{"type":"result","subtype":"success","is_error":false,"result":"a.go","num_turns":3,"total_cost_usd":0.12,"usage":{"input_tokens":5,"output_tokens":7,"cache_creation_input_tokens":9,"cache_read_input_tokens":11},"modelUsage":{"claude-sonnet-5-5":{}},"permission_denials":[]}
`
	p, err := parseStream([]byte(s))
	if err != nil || p.cost != 0.12 || p.use.CacheReadTokens != 11 || p.turns != 3 || p.readBytes != 5 || p.stacklit != 1 || p.calls["Read"] != 1 || p.util != 0.5 || p.model != "claude-sonnet-5-5" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := parseStream([]byte("nope")); err == nil {
		t.Fatal("no result must error")
	}
	lim, _ := parseStream([]byte(`{"type":"result","is_error":true,"result":"Claude usage limit reached"}`))
	if lim.stop == "" {
		t.Fatal("usage limit must stop the run")
	}
}

func loadTasksFile(t *testing.T, path string) []task {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ts []task
	if err := json.Unmarshal(b, &ts); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return ts
}

// answerSet is the file set a task expects, for the overlap test.
func answerSet(t task) map[string]bool {
	m := map[string]bool{}
	for _, p := range append(append([]string{}, t.AllOf...), t.AnyOf...) {
		m[p] = true
	}
	return m
}

func TestRound2TasksAreNewAndWellFormed(t *testing.T) {
	kinds := map[string]int{}
	seenID := map[string]bool{}
	for _, repo := range []string{"levelrail", "theauth-go", "featuredrop", "theauth"} {
		ts := loadTasksFile(t, "tasks-round2/"+repo+".json")
		if len(ts) != 10 {
			t.Errorf("%s: %d tasks, want 10", repo, len(ts))
		}
		var old []task
		if repo != "theauth" {
			old = loadTasksFile(t, "tasks/"+repo+".json")
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
			if strings.ContainsRune(n.Question+n.Rubric, 0x2013) || strings.ContainsRune(n.Question+n.Rubric, 0x2014) {
				t.Errorf("%s: dash character", n.ID)
			}
			for _, o := range old {
				if o.Question == n.Question {
					t.Errorf("%s reuses the question of %s", n.ID, o.ID)
				}
				a, b := answerSet(n), answerSet(o)
				shared := 0
				for p := range a {
					if b[p] {
						shared++
					}
				}
				if shared > 0 && float64(shared)/float64(len(a)+len(b)-shared) >= 0.5 {
					t.Errorf("%s shares most of its answer files with round 1 task %s", n.ID, o.ID)
				}
			}
		}
	}
	// Same kinds as round 1 in about the same proportions (locate, trace, impact, add).
	want := map[string][2]int{"locate": {11, 15}, "trace": {9, 13}, "impact": {7, 9}, "add": {7, 9}}
	for k, r := range want {
		if kinds[k] < r[0] || kinds[k] > r[1] {
			t.Errorf("%d %s tasks, want %d to %d", kinds[k], k, r[0], r[1])
		}
	}
}

// TestRound2AnswersVerifiedByGrep checks every answer against the pinned
// checkouts. Set AGENT_AB_CLONES to a directory holding one checkout per repo
// at its pinned commit; without it the test is skipped.
func TestRound2AnswersVerifiedByGrep(t *testing.T) {
	root := os.Getenv("AGENT_AB_CLONES")
	if root == "" {
		t.Skip("set AGENT_AB_CLONES to a directory of pinned checkouts to verify the answers")
	}
	b, err := os.ReadFile("repos-round2.json")
	if err != nil {
		t.Fatal(err)
	}
	var repos []repo
	if err := json.Unmarshal(b, &repos); err != nil {
		t.Fatal(err)
	}
	for _, r := range repos {
		dir := filepath.Join(root, r.Name)
		out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
		if err != nil || strings.TrimSpace(string(out)) != r.SHA {
			t.Errorf("%s: checkout is not at %s", r.Name, r.SHA)
			continue
		}
		for _, p := range verifyTasks(dir, loadTasksFile(t, "tasks-round2/"+r.Name+".json")) {
			t.Errorf("%s: %s", r.Name, p)
		}
	}
}

func round2Cfg() config {
	c := baseCfg()
	c.repo, c.reposFile, c.tasksDir = "all", "repos-round2.json", "tasks-round2"
	c.arms, c.trials, c.seed, c.grader = "control,stacklit,stacklit-e,stacklit-e-small", 5, 20261108, "audited"
	c.smallRepos, c.smallFiles, c.dryRun, c.estMid = "featuredrop", 600, true, 0.03
	return c
}

func TestRound2PlanSessionsAndArms(t *testing.T) {
	c := round2Cfg()
	total := 0
	for _, n := range []string{"levelrail", "theauth-go", "featuredrop", "theauth"} {
		arms := armsFor(c, n)
		want := 3
		if n == "featuredrop" {
			want = 4
		}
		if len(arms) != want {
			t.Errorf("%s: arms %v", n, arms)
		}
		cc := c
		cc.repo = n
		ts, _, err := load(cc)
		if err != nil {
			t.Fatal(err)
		}
		total += len(plan(ts, 5, c.seed, arms))
	}
	if total != 650 {
		t.Errorf("round 2 plans %d sessions, want 650 (4 repos x 10 tasks x 5 trials x 3 arms + 50 small repo sessions)", total)
	}
	if err := runAll(c); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	c.dryRun = false
	if err := runAll(c); err == nil || !strings.Contains(err.Error(), "yes-spend") {
		t.Errorf("a real run must refuse without --yes-spend, got %v", err)
	}
}

func TestArmEnvDiffersInOneThingEach(t *testing.T) {
	c := round2Cfg()
	has := func(arm, kv string) bool {
		for _, e := range armEnv(c, arm) {
			if e == kv {
				return true
			}
		}
		return false
	}
	if len(armEnv(c, "control")) != 0 {
		t.Error("control must not set stacklit variables")
	}
	for _, a := range []string{"stacklit", "stacklit-e"} {
		if !has(a, "STACKLIT_SMALL_REPO_FILES=0") {
			t.Errorf("%s must switch small repo mode off", a)
		}
	}
	if !has("stacklit", "STACKLIT_STEERING=a") || !has("stacklit-e", "STACKLIT_STEERING=e") || !has("stacklit-e-small", "STACKLIT_STEERING=e") {
		t.Error("steering variants wrong")
	}
	if !has("stacklit-e-small", "STACKLIT_SMALL_REPO_FILES=600") || !has("strict", "STACKLIT_STEERING=d") {
		t.Error("small repo or strict env wrong")
	}
}

func TestGradeWithAuditedAcceptsRelativeAndAbsolutePaths(t *testing.T) {
	tk := task{AllOf: []string{"internal/a/b.go"}, AnyOf: []string{"c.go"}}
	for _, ans := range []string{"internal/a/b.go, c.go", "./internal/a/b.go, ./c.go", "/tmp/x/co-123/internal/a/b.go, c.go"} {
		if ok, _ := gradeWith("audited", ans, tk); !ok {
			t.Errorf("audited rejected %q", ans)
		}
	}
	if ok, _ := gradeWith("audited", "xinternal/a/b.go, c.go", tk); ok {
		t.Error("a path inside a longer name must not match")
	}
	if ok, _ := gradeWith("prereg", "./internal/a/b.go, c.go", tk); ok {
		t.Error("the round 1 matcher keeps rejecting ./path")
	}
	if ok, _ := gradeWith("audited", "internal/a/b.go", tk); ok {
		t.Error("the any-of path is still required")
	}
}

func TestDecisionReportRule(t *testing.T) {
	mk := func(name, base string, costHi, costRatio, corrDiff, corrLo, traceLo, impactHi float64) scope {
		return scope{Name: name, Arm: "stacklit-e", Base: base,
			Compare: []cmp{
				{Metric: "cost_usd", Ratio: costRatio, RatioCI: ci{Lo: costRatio - 0.05, Hi: costHi}},
				{Metric: "correct_audited", Diff: corrDiff, DiffCI: ci{Lo: corrLo, Hi: corrLo + 0.1}},
			},
			ByKind: map[string][]cmp{
				"trace":  {{Metric: "correct_audited", DiffCI: ci{Lo: traceLo}}},
				"impact": {{Metric: "cost_usd", RatioCI: ci{Hi: impactHi}}},
			}}
	}
	good := summary{Scopes: []scope{
		mk("pooled", "control", 1.03, 0.97, -0.01, -0.03, -0.05, 1.10),
		mk("pooled", "stacklit", 1.02, 0.96, 0, 0, 0, 0),
		mk("levelrail", "control", 0.9, 0.85, 0, 0, 0, 0),
	}}
	files := map[string]int{"levelrail": 15978, "theauth-go": 1294}
	rep := decisionReport(good, files)
	if !strings.Contains(rep, "every criterion passes") {
		t.Errorf("all criteria hold but the verdict is negative:\n%s", rep)
	}
	for name, mut := range map[string]func(*summary){
		"P1": func(s *summary) { s.Scopes[0].Compare[0].RatioCI.Hi = 1.06 },
		"P2": func(s *summary) { s.Scopes[0].Compare[1].DiffCI.Lo = -0.06 },
		"G1": func(s *summary) { s.Scopes[0].ByKind["trace"][0].DiffCI.Lo = -0.11 },
		"G2": func(s *summary) { s.Scopes[0].ByKind["impact"][0].RatioCI.Hi = 1.16 },
		"P3": func(s *summary) { s.Scopes[2].Compare[0].Ratio = 1.01 },
		"P4": func(s *summary) { s.Scopes[1].Compare[0].RatioCI.Hi = 1.06 },
	} {
		cp := good
		cp.Scopes = append([]scope(nil), good.Scopes...)
		for i := range cp.Scopes {
			cp.Scopes[i].Compare = append([]cmp(nil), good.Scopes[i].Compare...)
			bk := map[string][]cmp{}
			for k, v := range good.Scopes[i].ByKind {
				bk[k] = append([]cmp(nil), v...)
			}
			cp.Scopes[i].ByKind = bk
		}
		mut(&cp)
		if r := decisionReport(cp, files); !strings.Contains(r, "default stays a") || !strings.Contains(r, "FAIL") {
			t.Errorf("%s: a failed criterion must block the change:\n%s", name, r)
		}
	}
	if r := decisionReport(summary{}, files); !strings.Contains(r, "default stays a") {
		t.Error("no data must keep the default")
	}
}
