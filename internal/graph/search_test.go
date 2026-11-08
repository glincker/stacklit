package graph

import "testing"

func synth() *SymGraph {
	g := &SymGraph{Version: SchemaVersion, Nodes: []SymNode{
		{ID: "a.go", Name: "a.go", Kind: KindFile, File: "a.go"},
		{ID: "a.go::user_accounts", Name: "user_accounts", Kind: KindFunc, File: "a.go", Line: 1, End: 5},
		{ID: "a.go::UserAccountsRepo", Name: "UserAccountsRepo", Kind: KindType, File: "a.go", Line: 7, End: 9},
		{ID: "b.go::load", Name: "load", Kind: KindFunc, File: "b.go", Line: 1, End: 9, Doc: "reads the quarterly report", Body: "tokenize frobnicate checksum"},
		{ID: "c.go::Parse", Name: "Parse", Kind: KindFunc, File: "c.go", Line: 1, End: 9, Doc: "unrelated"},
	}}
	g.Index()
	return g
}

func TestSearchIdentifierSnakeAndSchema(t *testing.T) {
	g := synth()
	for _, q := range []string{"user_accounts", "public.user_accounts", "userAccounts", "user_accounts()"} {
		h := g.Search(q, 3)
		if len(h) == 0 || g.Nodes[h[0].Node].Name != "user_accounts" {
			t.Errorf("%q: want user_accounts first, got %v", q, h)
		}
	}
	// Prefix tier still finds the longer name.
	if h := g.Search("UserAcc", 5); len(h) == 0 {
		t.Error("prefix query found nothing")
	}
}

func TestSearchBodyAndPhrase(t *testing.T) {
	g := synth()
	if h := g.Search("frobnicate checksum", 3); len(h) == 0 || g.Nodes[h[0].Node].Name != "load" {
		t.Errorf("body terms should find load, got %v", h)
	}
	h := g.Search(`"quarterly report"`, 5)
	if len(h) != 1 || g.Nodes[h[0].Node].Name != "load" {
		t.Errorf("phrase should filter to load, got %v", h)
	}
}

func TestIsIdentifierQuery(t *testing.T) {
	for q, want := range map[string]bool{"parseFile": true, "a_b": true, "x.Y": true, "pkg::Name": true, "parse file": false, "": false, "a-b c": false} {
		if got := IsIdentifierQuery(q); got != want {
			t.Errorf("%q: got %v want %v", q, got, want)
		}
	}
}
