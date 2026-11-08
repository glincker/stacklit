package main

import (
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
