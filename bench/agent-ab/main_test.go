package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestRefusesWithoutYesSpend(t *testing.T) {
	c := config{repo: "gin", reposFile: "../repos.json", tasksDir: "../tasks", trials: 3, arms: "control,stacklit"}
	err := run(c)
	if err == nil || !strings.Contains(err.Error(), "--yes-spend") {
		t.Fatalf("must refuse without --yes-spend, got %v", err)
	}
}

func TestDryRunNeverSpends(t *testing.T) {
	c := config{repo: "gin", reposFile: "../repos.json", tasksDir: "../tasks", trials: 3, arms: "control,stacklit", dryRun: true, estLow: 0.1, estMid: 0.2, estHigh: 0.3, maxBudget: 1}
	if err := run(c); err != nil {
		t.Fatal(err)
	}
}

var arms2 = []string{"control", "stacklit"}

func TestPlanSeededAndComplete(t *testing.T) {
	tasks := []task{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	p1, p2, p3 := plan(tasks, 3, 7, arms2), plan(tasks, 3, 7, arms2), plan(tasks, 3, 8, arms2)
	if len(p1) != 3*3*2 {
		t.Fatalf("units: %d", len(p1))
	}
	if !reflect.DeepEqual(p1, p2) {
		t.Fatal("same seed must give the same order")
	}
	if reflect.DeepEqual(p1, p3) {
		t.Fatal("different seeds should differ")
	}
	count := map[string]int{}
	for _, u := range p1 {
		count[u.Arm]++
	}
	if count["control"] != 9 || count["stacklit"] != 9 {
		t.Fatalf("arms unbalanced: %v", count)
	}
}

func TestProjectionAndParse(t *testing.T) {
	c := config{trials: 3, arms: "control,stacklit", estLow: 0.1, estMid: 0.5, estHigh: 1, maxBudget: 2}
	if s := projection(10, c, 60); !strings.Contains(s, "60 paid sessions") || !strings.Contains(s, "$6.00 / $30.00 / $60.00") || !strings.Contains(s, "$120.00") {
		t.Fatalf("%s", s)
	}
	r, err := parseResult([]byte(`{"is_error":false,"result":"gin.go","num_turns":3,"total_cost_usd":0.12,"usage":{"input_tokens":5,"output_tokens":7,"cache_creation_input_tokens":9,"cache_read_input_tokens":11}}`))
	if err != nil || r.Cost != 0.12 || r.Usage.CacheReadTokens != 11 || r.NumTurns != 3 {
		t.Fatalf("%+v %v", r, err)
	}
	if _, err := parseResult([]byte("nope")); err == nil {
		t.Fatal("bad JSON must error")
	}
	if !correct("see tree.go", []string{"tree.go"}) || correct("x", []string{"tree.go"}) {
		t.Fatal("correct()")
	}
}
