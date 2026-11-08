package merge

import (
	"testing"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
)

func g(file string) *graph.SymGraph {
	x := &graph.SymGraph{Version: "1", Nodes: []graph.SymNode{
		{ID: file, Name: file, Kind: graph.KindFile, File: file, Community: 0},
		{ID: file + "::F", Name: "F", Kind: graph.KindFunc, File: file, Line: 1, Community: 1},
		{ID: "orphan", Name: "orphan", Kind: graph.KindFunc},
	}, Edges: []graph.SymEdge{{From: 0, To: 1, Kind: graph.EdgeImports, Conf: graph.ConfExtracted, Score: 1}},
		Communities: []graph.Community{{ID: 0, Label: "a"}, {ID: 1, Label: "b"}}}
	x.Index()
	return x
}

func TestMergePrefixesAndKeepsSourceFile(t *testing.T) {
	out, err := Merge([]Input{{"app", g("web/x.ts")}, {"lib", g("src/y.ts")}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Nodes) != 6 || len(out.Edges) != 2 {
		t.Fatalf("%d nodes %d edges", len(out.Nodes), len(out.Edges))
	}
	for _, n := range out.Nodes {
		if n.File == "" || n.Repo == "" {
			t.Fatalf("missing file or repo: %+v", n)
		}
	}
	if e := out.Edges[1]; e.From != 3 || e.To != 4 {
		t.Fatalf("edge not reindexed: %+v", e)
	}
	if out.Nodes[4].Community != 3 || out.Communities[3].Label != "lib/b" {
		t.Fatalf("community offset: %+v %+v", out.Nodes[4], out.Communities)
	}
}

func TestMergeLinksByManifest(t *testing.T) {
	multi := &schema.MultiIndex{CrossEdges: []schema.CrossRepoEdge{{From: "app", To: "lib", Weight: 2, Via: []string{"web"}}}}
	out, err := Merge([]Input{{"app", g("web/x.ts")}, {"lib", g("src/y.ts")}}, multi)
	if err != nil {
		t.Fatal(err)
	}
	var repos, link, fileLink int
	for _, n := range out.Nodes {
		if n.Kind == KindRepo {
			repos++
		}
	}
	for _, e := range out.Edges {
		if e.Kind == EdgeDependsOn {
			link++
		}
		if e.Kind == graph.EdgeImports && out.Nodes[e.To].Kind == KindRepo {
			fileLink++
		}
	}
	if repos != 2 || link != 1 || fileLink != 1 {
		t.Fatalf("repos=%d link=%d fileLink=%d", repos, link, fileLink)
	}
}

func TestMergeRejectsDuplicates(t *testing.T) {
	if _, err := Merge([]Input{{"a", g("x")}, {"a", g("y")}}, nil); err == nil {
		t.Fatal("expected duplicate error")
	}
}
