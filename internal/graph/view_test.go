package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleGraph(t *testing.T) *SymGraph {
	t.Helper()
	srcs := map[string]string{}
	for i := 0; i < 30; i++ {
		srcs[fmt.Sprintf("pkg%d/f.go", i%5)] += ""
	}
	// A chain of packages with doc comments so ranking has text to use.
	srcs["a/a.go"] = "package a\n\n// ParseConfig reads the configuration file.\nfunc ParseConfig() { loadFile(); validate() }\n\nfunc loadFile() {}\n\nfunc validate() { check() }\n\nfunc check() {}\n"
	srcs["b/b.go"] = "package b\n\n// Serve starts the http server.\nfunc Serve() { handle() }\n\nfunc handle() {}\n"
	delete(srcs, "pkg0/f.go")
	delete(srcs, "pkg1/f.go")
	delete(srcs, "pkg2/f.go")
	delete(srcs, "pkg3/f.go")
	delete(srcs, "pkg4/f.go")
	return buildFrom(t, srcs, nil)
}

func TestBudgetIsHard(t *testing.T) {
	g := sampleGraph(t)
	for budget := 1; budget <= 300; budget += 7 {
		for _, js := range []bool{false, true} {
			out := g.Query("parse configuration file validate", budget, js)
			if approxTokens(out) > budget {
				t.Fatalf("query budget %d json=%v produced %d tokens", budget, js, approxTokens(out))
			}
			ex, err := g.Explain("ParseConfig", budget, js)
			if err != nil || approxTokens(ex) > budget {
				t.Fatalf("explain budget %d: %d tokens err=%v", budget, approxTokens(ex), err)
			}
			im, err := g.Impact("check", 4, budget, js)
			if err != nil || approxTokens(im) > budget {
				t.Fatalf("impact budget %d: %d tokens err=%v", budget, approxTokens(im), err)
			}
		}
	}
}

func TestTruncationNotice(t *testing.T) {
	g := sampleGraph(t)
	out := g.Query("parse configuration file validate", 60, false)
	if !strings.Contains(out, "truncated") || !strings.Contains(out, "stacklit query") {
		t.Fatalf("expected truncation notice with command, got:\n%s", out)
	}
	full := g.Query("parse configuration file validate", 4000, false)
	if strings.Contains(full, "truncated") {
		t.Fatalf("unexpected truncation:\n%s", full)
	}
}

func TestQueryRanksDocMatches(t *testing.T) {
	g := sampleGraph(t)
	out := g.Query("where is the http server started", 4000, false)
	first := strings.Index(out, "Serve")
	if first < 0 || strings.Index(out, "b/b.go") < 0 {
		t.Fatalf("doc comment match missing:\n%s", out)
	}
}

func TestByteStable(t *testing.T) {
	var prev []string
	for run := 0; run < 5; run++ {
		g := sampleGraph(t)
		ex, _ := g.Explain("ParseConfig", 800, false)
		im, _ := g.Impact("check", 4, 800, true)
		p, _ := g.ShortestPath("ParseConfig", "check")
		cur := []string{g.Query("config file", 800, false), ex, im, p.Render(false), g.RenderCommunities(true), g.Report(), RenderHubs(g.Hubs(5), true)}
		if run > 0 {
			for i := range cur {
				if cur[i] != prev[i] {
					t.Fatalf("output %d differs between runs:\n%s\n---\n%s", i, prev[i], cur[i])
				}
			}
		}
		prev = cur
	}
}

func TestPathAndImpact(t *testing.T) {
	g := sampleGraph(t)
	p, err := g.ShortestPath("ParseConfig", "check")
	if err != nil || !p.Found || len(p.Hops) != 2 || p.Weakest <= 0 {
		t.Fatalf("path = %+v err=%v", p, err)
	}
	p, _ = g.ShortestPath("ParseConfig", "Serve")
	if p.Found {
		t.Fatalf("unexpected path between disconnected symbols: %+v", p)
	}
	im, _ := g.Impact("check", 4, 2000, false)
	for _, want := range []string{"validate", "ParseConfig", "trust: "} {
		if !strings.Contains(im, want) {
			t.Errorf("impact missing %s:\n%s", want, im)
		}
	}
}

func TestHubsCommunitiesReport(t *testing.T) {
	g := sampleGraph(t)
	hs := g.Hubs(3)
	if len(hs) == 0 || hs[0].In+hs[0].Out < 2 {
		t.Fatalf("hubs = %+v", hs)
	}
	if len(g.Communities) == 0 {
		t.Fatal("no communities")
	}
	r := g.Report()
	for _, s := range []string{"## Key concepts", "## Communities", "## Surprising connections", "## Suggested questions"} {
		if !strings.Contains(r, s) {
			t.Errorf("report missing %q", s)
		}
	}
	if strings.ContainsAny(r, "\u2013\u2014") {
		t.Error("report contains dashes")
	}
}

func TestSlice(t *testing.T) {
	dir := t.TempDir()
	src := "package a\n\nfunc One() {\n\treturn\n}\n\nfunc Two() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	g := Resolve([]*FileSyms{ExtractFile("a.go", []byte(src))}, ResolveOptions{})
	out, err := g.Slice(dir, "One", 0)
	if err != nil || !strings.Contains(out, "3\tfunc One() {") || strings.Contains(out, "Two") {
		t.Fatalf("slice = %q err=%v", out, err)
	}
	small, _ := g.Slice(dir, "One", 12)
	if approxTokens(small) > 12 {
		t.Fatalf("slice over budget: %d", approxTokens(small))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	g := sampleGraph(t)
	p := filepath.Join(t.TempDir(), "g.json")
	if err := g.Save(p); err != nil {
		t.Fatal(err)
	}
	h, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if h.Query("config", 500, false) != g.Query("config", 500, false) {
		t.Fatal("query differs after reload")
	}
}
