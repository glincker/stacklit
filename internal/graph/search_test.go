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

func TestIdentTokens(t *testing.T) {
	got := identTokens("Where is fetchUserRow and load_all_rows defined? See TOML, Where and ab_")
	if len(got) < 2 || got[0] != "fetchUserRow" || got[1] != "load_all_rows" {
		t.Errorf("identTokens = %v", got)
	}
	for _, g := range got {
		if g == "TOML" || g == "Where" {
			t.Errorf("plain word %q taken as identifier", g)
		}
	}
}

func auxGraph() *SymGraph {
	g := &SymGraph{Version: SchemaVersion, Nodes: []SymNode{
		{ID: "src/lib.go::Render", Name: "Render", Kind: KindFunc, File: "src/lib.go", Line: 1, End: 9, Doc: "render output page"},
		{ID: "examples/demo/main.go::Render", Name: "Render", Kind: KindFunc, File: "examples/demo/main.go", Line: 1, End: 9, Doc: "render output page"},
		{ID: "src/other.go::renderHelper", Name: "renderHelper", Kind: KindFunc, File: "src/other.go", Line: 1, End: 9},
	}}
	g.Index()
	return g
}

func TestAuxPathsDemotedUnlessAsked(t *testing.T) {
	g := auxGraph()
	if h := g.Search("render output page", 3); len(h) < 2 || g.Nodes[h[0].Node].File != "src/lib.go" {
		t.Errorf("source file should beat the example: %v", h)
	}
	if h := g.Search("render output page in the examples", 3); len(h) == 0 {
		t.Error("no hits when asking for examples")
	} else if s0, s1 := g.Nodes[h[0].Node].File, g.Nodes[h[1].Node].File; s0 == s1 {
		t.Errorf("same file twice: %v", h)
	}
}

func TestIdentTokenInProseBoostsDefinition(t *testing.T) {
	g := &SymGraph{Version: SchemaVersion, Nodes: []SymNode{
		{ID: "a.go::fetchRow", Name: "fetchRow", Kind: KindFunc, File: "a.go", Line: 1, End: 3},
		{ID: "b.go::Fetcher", Name: "Fetcher", Kind: KindType, File: "b.go", Line: 1, End: 3, Doc: "where rows are fetched, a row at a time, row by row"},
	}}
	g.Index()
	if h := g.Search("where are rows fetched, fetchRow", 2); len(h) == 0 || g.Nodes[h[0].Node].Name != "fetchRow" {
		t.Errorf("exact identifier in prose should rank first: %v", h)
	}
}
