package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

// orderGraph has symbols spread over files whose paths sort opposite to the
// scores the tests assign, so any path-order leak shows up.
func orderGraph() *SymGraph {
	g := &SymGraph{Version: SchemaVersion, Nodes: []SymNode{
		{ID: "a.go", Name: "a.go", Kind: KindFile, File: "a.go"},
		{ID: "a.go::low", Name: "low", Kind: KindFunc, File: "a.go", Line: 3, End: 4},
		{ID: "b.go::mid", Name: "mid", Kind: KindFunc, File: "b.go", Line: 9, End: 10},
		{ID: "b.go::midEarly", Name: "midEarly", Kind: KindFunc, File: "b.go", Line: 2, End: 3},
		{ID: "c.go::top", Name: "top", Kind: KindFunc, File: "c.go", Line: 5, End: 6},
		{ID: "d.go::tie", Name: "tie", Kind: KindFunc, File: "d.go", Line: 1, End: 2},
	}}
	g.Index()
	return g
}

func specFor(scores map[string]float64, order Order) viewSpec {
	g := orderGraph()
	var items []item
	for i, n := range g.Nodes {
		if s, ok := scores[n.Name]; ok {
			items = append(items, item{node: i, score: s})
		}
	}
	return viewSpec{title: "t", items: items, ranked: true, order: order, hint: "h"}
}

func TestBuildViewOrder(t *testing.T) {
	g := orderGraph()
	tests := []struct {
		name   string
		scores map[string]float64
		order  Order
		files  []string
		first  string // first symbol of the first file
	}{
		{"relevance by best symbol", map[string]float64{"low": 0.2, "mid": 0.6, "midEarly": 0.6, "top": 0.9}, OrderRelevance, []string{"c.go", "b.go", "a.go"}, "top"},
		{"file score is its best symbol", map[string]float64{"low": 0.9, "mid": 0.1, "top": 0.5}, OrderRelevance, []string{"a.go", "c.go", "b.go"}, "low"},
		{"ties by path", map[string]float64{"low": 0.5, "top": 0.5, "tie": 0.5}, OrderRelevance, []string{"a.go", "c.go", "d.go"}, "low"},
		{"symbols by score then line", map[string]float64{"mid": 0.4, "midEarly": 0.4}, OrderRelevance, []string{"b.go"}, "midEarly"},
		{"path order", map[string]float64{"low": 0.2, "top": 0.9, "tie": 0.5}, OrderPath, []string{"a.go", "c.go", "d.go"}, "low"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := g.buildView(specFor(tc.scores, tc.order), len(tc.scores))
			var got []string
			for _, f := range v.Files {
				got = append(got, f.File)
			}
			if strings.Join(got, ",") != strings.Join(tc.files, ",") {
				t.Fatalf("files %v, want %v", got, tc.files)
			}
			if s := v.Files[0].Symbols[0].Name; s != tc.first {
				t.Errorf("first symbol %q, want %q", s, tc.first)
			}
		})
	}
}

func TestBuildViewRankFields(t *testing.T) {
	g := orderGraph()
	for _, order := range []Order{OrderRelevance, OrderPath} {
		v := g.buildView(specFor(map[string]float64{"low": 0.2, "top": 0.9, "tie": 0.5}, order), 3)
		ranks := map[string]int{}
		for _, f := range v.Files {
			ranks[f.File] = f.Rank
			if f.Score <= 0 {
				t.Errorf("%s: missing score", f.File)
			}
		}
		if ranks["c.go"] != 1 || ranks["d.go"] != 2 || ranks["a.go"] != 3 {
			t.Errorf("order %s: ranks %v, want c=1 d=2 a=3", order, ranks)
		}
	}
}

func TestQueryRelevanceOrderEndToEnd(t *testing.T) {
	g := synth()
	out := g.QueryOrdered("quarterly report load", 800, false, OrderRelevance)
	lines := strings.Split(out, "\n")
	if !strings.HasPrefix(lines[3], "1. b.go") && !strings.HasPrefix(lines[4], "1. b.go") {
		t.Errorf("best file not first:\n%s", out)
	}
	if !strings.Contains(out, "best match: b.go:1 load") {
		t.Errorf("missing best match header:\n%s", out)
	}
	path := g.QueryOrdered("quarterly report load", 800, false, OrderPath)
	if strings.Contains(path, "best match") || strings.Contains(path, "1. ") {
		t.Errorf("path order must keep the old format:\n%s", path)
	}
}

func TestQueryJSONRankAndScore(t *testing.T) {
	g := synth()
	var v struct {
		Files []struct {
			File    string  `json:"file"`
			Rank    int     `json:"rank"`
			Score   float64 `json:"score"`
			Symbols []struct {
				Score float64 `json:"score"`
			} `json:"symbols"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(g.QueryOrdered("quarterly report load parse", 2000, true, OrderRelevance)), &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Files) == 0 {
		t.Fatal("no files")
	}
	for i, f := range v.Files {
		if f.Rank != i+1 || f.Score <= 0 || f.Symbols[0].Score <= 0 {
			t.Errorf("file %d %+v: want rank %d and positive scores", i, f, i+1)
		}
		if i > 0 && f.Score > v.Files[i-1].Score {
			t.Errorf("scores not descending at %d", i)
		}
	}
}

func TestQueryBudgetKeepsBestFiles(t *testing.T) {
	g := repoGraph(t)
	q := "write a file atomically"
	full := g.QueryOrdered(q, 4000, true, OrderRelevance)
	small := g.QueryOrdered(q, 300, true, OrderRelevance)
	if approxTokens(small) > 300 {
		t.Fatalf("budget exceeded: %d tokens", approxTokens(small))
	}
	var f, s View
	if json.Unmarshal([]byte(full), &f) != nil || json.Unmarshal([]byte(small), &s) != nil {
		t.Fatal("bad json")
	}
	if len(s.Files) == 0 || len(s.Files) >= len(f.Files) {
		t.Fatalf("expected a truncated subset: %d of %d files", len(s.Files), len(f.Files))
	}
	if s.Files[0].File != f.Files[0].File {
		t.Errorf("truncation dropped the best file: %s vs %s", s.Files[0].File, f.Files[0].File)
	}
	if s.Notice == "" {
		t.Error("missing truncation notice")
	}
}

func TestQueryOrderDeterministic(t *testing.T) {
	for _, order := range []Order{OrderRelevance, OrderPath} {
		for _, js := range []bool{false, true} {
			var first string
			for i := 0; i < 5; i++ {
				g := repoGraphFresh(t)
				out := g.QueryOrdered("detect frameworks from dependencies", 1000, js, order)
				if i == 0 {
					first = out
				} else if out != first {
					t.Fatalf("order %s json %v: run %d differs", order, js, i)
				}
			}
		}
	}
}

func TestParseOrder(t *testing.T) {
	t.Setenv("STACKLIT_QUERY_ORDER", "path")
	if o, err := ParseOrder(""); err != nil || o != OrderPath {
		t.Errorf("env default: %v %v", o, err)
	}
	if o, err := ParseOrder("relevance"); err != nil || o != OrderRelevance {
		t.Errorf("explicit flag must beat env: %v %v", o, err)
	}
	if _, err := ParseOrder("bogus"); err == nil {
		t.Error("want error for bogus order")
	}
}
