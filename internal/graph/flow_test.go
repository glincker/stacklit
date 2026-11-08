package graph

import (
	"strings"
	"testing"
)

func TestQuestionSuggestsFlow(t *testing.T) {
	yes := []string{
		"wake sleeping app HTTP wake endpoint",
		"trace a request from the CLI through the handler",
		"who calls parseToken",
		"lifecycle of a session",
		"what is the path from login to the store",
	}
	no := []string{"where is the retry limit set", "workflow runner config", "parse the config file", "Ingest openapi spec"}
	for _, q := range yes {
		if !questionSuggestsFlow(q) {
			t.Errorf("%q should suggest a flow", q)
		}
	}
	for _, q := range no {
		if questionSuggestsFlow(q) {
			t.Errorf("%q should not suggest a flow", q)
		}
	}
	t.Setenv("STACKLIT_FLOW_WORDS", "banana")
	if questionSuggestsFlow("trace the flow") || !questionSuggestsFlow("a banana") {
		t.Error("STACKLIT_FLOW_WORDS must replace the list")
	}
}

func TestSpreadsDirs(t *testing.T) {
	spread := []fileScore{{"a/x.go", 1}, {"b/y.go", 0.9}, {"c/z.go", 0.8}, {"d/w.go", 0.7}, {"a/v.go", 0.6}}
	if !spreadsDirs(spread) {
		t.Error("four directories with flat scores must spread")
	}
	if spreadsDirs([]fileScore{{"a/x.go", 1}, {"b/y.go", 0.9}, {"c/z.go", 0.8}, {"d/w.go", 0.2}, {"e/v.go", 0.1}}) {
		t.Error("a clear winner with a scattered tail is a single answer")
	}
	if spreadsDirs([]fileScore{{"a/x.go", 1}, {"a/y.go", 0.9}, {"a/z.go", 0.8}}) {
		t.Error("one directory is not a spread")
	}
	t.Setenv("STACKLIT_QUERY_SPREAD_DIRS", "2")
	if !spreadsDirs([]fileScore{{"a/x.go", 1}, {"b/y.go", 0.9}}) {
		t.Error("env must lower the directory threshold")
	}
}

func nextGraph() *SymGraph {
	g := &SymGraph{Version: SchemaVersion, Nodes: []SymNode{
		{ID: "a/one.go::alpha", Name: "alpha", Kind: KindFunc, File: "a/one.go", Line: 1, End: 2},
		{ID: "b/two.go::beta", Name: "beta", Kind: KindFunc, File: "b/two.go", Line: 1, End: 2},
		{ID: "c/three.go::gamma", Name: "gamma", Kind: KindFunc, File: "c/three.go", Line: 1, End: 2},
		{ID: "d/four.go::delta", Name: "delta", Kind: KindFunc, File: "d/four.go", Line: 1, End: 2},
		{ID: "e/five.go::eps", Name: "eps", Kind: KindFunc, File: "e/five.go", Line: 1, End: 2},
	}}
	g.Index()
	return g
}

func TestBuildViewNextCandidates(t *testing.T) {
	g := nextGraph()
	var items []item
	for i, s := range []float64{0.9, 0.8, 0.7, 0.6, 0.5} {
		items = append(items, item{node: i, score: s})
	}
	sp := viewSpec{title: "t", items: items, ranked: true, order: OrderRelevance, hint: "h", nextFiles: 3}
	v := g.buildView(sp, 2)
	if len(v.Next) != 3 || v.Next[0] != "c/three.go" || v.Next[2] != "e/five.go" {
		t.Fatalf("next files %v", v.Next)
	}
	txt := v.Text()
	if !strings.Contains(txt, "stacklit explain alpha | stacklit path alpha gamma") {
		t.Errorf("missing expand commands:\n%s", txt)
	}
	if strings.Count(txt, "\n") != strings.Count(v.Text(), "\n") {
		t.Error("text not stable")
	}
	// Nothing omitted, nothing to suggest.
	if v := g.buildView(sp, 5); len(v.Next) != 0 || strings.Contains(v.Text(), "also ranked") {
		t.Error("a complete view must not add the line")
	}
	// Off by default in the spec: the view is unchanged.
	sp.nextFiles = 0
	if v := g.buildView(sp, 2); len(v.Next) != 0 || strings.Contains(v.Text(), "also ranked") {
		t.Error("nextFiles 0 must leave the view unchanged")
	}
}

func TestNextLineKeepsBudgetAndSingleAnswersUnchanged(t *testing.T) {
	g := repoGraph(t)
	flow := "trace the call path from the setup command through the hooks to the session digest"
	for _, budget := range []int{300, 600, 900} {
		for _, json := range []bool{false, true} {
			out := g.QueryOrdered(flow, budget, json, OrderRelevance)
			if approxTokens(out) > budget {
				t.Fatalf("budget %d exceeded: %d tokens (json %v)", budget, approxTokens(out), json)
			}
		}
	}
	if out := g.QueryOrdered(flow, 600, false, OrderRelevance); !strings.Contains(out, "also ranked") {
		t.Errorf("flow question did not get the line:\n%s", out)
	}
	t.Setenv("STACKLIT_QUERY_NEXT", "0")
	if out := g.QueryOrdered(flow, 600, false, OrderRelevance); strings.Contains(out, "also ranked") {
		t.Error("STACKLIT_QUERY_NEXT=0 must switch the line off")
	}
}

func TestNextLineRareOnSingleAnswerGoldQuestions(t *testing.T) {
	g := repoGraph(t)
	fired, total := 0, 0
	for _, c := range goldCases {
		total++
		if strings.Contains(g.QueryOrdered(c.q, 800, false, OrderRelevance), "also ranked") {
			fired++
		}
	}
	t.Logf("line fired on %d of %d single-answer gold questions", fired, total)
	if fired*3 > total {
		t.Errorf("line fired on %d of %d single-answer questions, too often", fired, total)
	}
}
