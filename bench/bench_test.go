package main

import (
	"strings"
	"testing"
)

func TestCostTokens(t *testing.T) {
	cases := map[int]int{0: 0, 1: 1, 4: 1, 5: 2, 400: 100}
	for bytes, want := range cases {
		if got := (Cost{Bytes: bytes}).Tokens(); got != want {
			t.Errorf("Tokens(%d) = %d, want %d", bytes, got, want)
		}
	}
}

func TestPathMatches(t *testing.T) {
	if !pathMatches("render/json.go", "render/") || pathMatches("renderer/x.go", "render/") {
		t.Error("directory prefix matching is wrong")
	}
	if !pathMatches("tree.go", "tree.go") || pathMatches("a/tree.go", "tree.go") {
		t.Error("exact matching is wrong")
	}
}

func TestGradeDependents(t *testing.T) {
	task := Task{ExpectModules: []string{"src/validator", "src/middleware/jwt"}}
	// Short names pass the lenient rule only.
	a := ArmResult{Modules: []string{"validator", "jwt"}}
	gradeDependents(&a, task, 2)
	if !a.Lenient || a.Strict {
		t.Errorf("short names: lenient=%v strict=%v", a.Lenient, a.Strict)
	}
	// Exact names with too many extras fail the strict rule.
	a = ArmResult{Modules: []string{"src/validator", "src/middleware/jwt", "x", "y", "z"}}
	gradeDependents(&a, task, 2)
	if !a.Lenient || a.Strict || a.Extra != 3 {
		t.Errorf("extras: lenient=%v strict=%v extra=%d", a.Lenient, a.Strict, a.Extra)
	}
	a = ArmResult{Modules: []string{"src/validator", "src/middleware/jwt", "x"}}
	gradeDependents(&a, task, 2)
	if !a.Strict {
		t.Error("exact names with one extra should pass strict")
	}
	// A missing module fails both.
	a = ArmResult{Modules: []string{"src/validator"}}
	gradeDependents(&a, task, 2)
	if a.Lenient || a.Strict {
		t.Error("missing module must fail both rules")
	}
}

func TestFinishLocalization(t *testing.T) {
	sizes := RepoSizes{"a.go": {Bytes: 400}, "b.go": {Bytes: 800}, "c.go": {Bytes: 1200}}
	task := Task{ExpectPaths: []string{"b.go"}}
	a := ArmResult{Lookup: Cost{Bytes: 40}, Ranked: []string{"a.go", "b.go", "c.go"}}
	finishLocalization(&a, task, sizes, 5)
	if a.FirstGold != 2 || !a.Found(5) || a.Found(1) {
		t.Errorf("rank %d", a.FirstGold)
	}
	if got := a.ToFound.Tokens(); got != 10+100+200 {
		t.Errorf("to found %d", got)
	}
	miss := ArmResult{Lookup: Cost{Bytes: 40}, Ranked: []string{"a.go", "c.go"}}
	finishLocalization(&miss, task, sizes, 5)
	if miss.FirstGold != 0 || miss.ToFound.Tokens() != 10+100+300 {
		t.Errorf("miss %d %d", miss.FirstGold, miss.ToFound.Tokens())
	}
}

func TestBM25PrefersIdentifierFile(t *testing.T) {
	files := []SourceFile{
		{Rel: "tree.go", Lines: []string{"func getValue(path string) {}", "radix tree lookup"}},
		{Rel: "other.go", Lines: []string{"func other() {}"}},
	}
	got := BuildBM25(files).Rank(queryTokens("Where is the radix tree route lookup (getValue) implemented?"), 5)
	if len(got) == 0 || got[0] != "tree.go" {
		t.Errorf("got %v", got)
	}
}

func TestRepoMapDeterministicAndBudgeted(t *testing.T) {
	files := []SourceFile{
		{Rel: "a.go", Lines: []string{"package a", "func Alpha() {}", "func helperThing() { Beta() }"}},
		{Rel: "b.go", Lines: []string{"package a", "func Beta() {}", "func Gamma() { Alpha(); Beta() }"}},
	}
	rm := BuildRepoMap(files)
	t1, f1 := rm.Render("where is Alpha", 100)
	t2, f2 := rm.Render("where is Alpha", 100)
	if t1 != t2 || len(f1) != len(f2) || len(f1) == 0 {
		t.Fatal("render is not deterministic or empty")
	}
	if CostOf(t1).Tokens() > 100 {
		t.Error("map exceeds budget")
	}
	small, _ := rm.Render("where is Alpha", 5)
	if CostOf(small).Tokens() > 5 {
		t.Error("small map exceeds budget")
	}
}

func TestBootstrapDeterministic(t *testing.T) {
	v := []float64{1, 0, 1, 1, 0, 1, 1, 1}
	a, b := bootstrapMean(v, 2000, 7), bootstrapMean(v, 2000, 7)
	if a != b || a.Lo > a.Mean || a.Hi < a.Mean {
		t.Errorf("%+v %+v", a, b)
	}
}

func TestModuleOfFile(t *testing.T) {
	mods := []string{"root", "render", "internal/json"}
	if got := moduleOfFile("render/json.go", mods); got != "render" {
		t.Errorf("got %s", got)
	}
	if got := moduleOfFile("tree.go", mods); got != "root" {
		t.Errorf("got %s", got)
	}
}

func TestRankModulesDeterministic(t *testing.T) {
	iv := &IndexView{Modules: map[string]map[string]any{
		"a": {"purpose": "alpha", "exports": []any{"Foo"}},
		"b": {"purpose": "beta", "exports": []any{"Foo"}},
	}}
	got := iv.RankModules([]string{"foo"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v", got)
	}
}

func TestParseQueryText(t *testing.T) {
	ranked := "query \"q\"\n3 seeds, 9 candidates\nbest match: b/x.go:4 Foo\n1. b/x.go\n  4-9 func Foo()  // does foo\n      calls Bar E1.00\n2. a/y.go\n  7 const Z\n[truncated: 2 of 9 omitted, lowest ranked first; raise it with: x]\n[stale: 1 source files changed]\n"
	plain := "query \"q\"\n3 seeds, 9 candidates\na/y.go\n  7 const Z\nb/x.go\n  4-9 func Foo()\n"
	for name, tc := range map[string]struct {
		in   string
		want []string
	}{"ranked": {ranked, []string{"b/x.go", "a/y.go"}}, "path": {plain, []string{"a/y.go", "b/x.go"}}} {
		doc := parseQueryText([]byte(tc.in))
		var got []string
		for _, f := range doc.Files {
			got = append(got, f.File)
		}
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: files %v, want %v", name, got, tc.want)
		}
	}
	if d := parseQueryText([]byte(ranked)); len(d.Files[0].Symbols) != 1 || d.Files[0].Symbols[0].Doc != "does foo" {
		t.Errorf("symbols not parsed: %+v", d.Files[0].Symbols)
	}
}
