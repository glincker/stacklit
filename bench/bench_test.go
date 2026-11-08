package main

import "testing"

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

func TestPrecisionRecallLoose(t *testing.T) {
	p, r := precisionRecall([]string{"validator", "src"}, []string{"src/validator", "src/middleware/jwt"}, true)
	if r != 0.5 || p != 0.5 {
		t.Errorf("got precision %.2f recall %.2f", p, r)
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
