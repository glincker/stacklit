package export

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
)

func fixture() Input {
	idx := &schema.Index{
		Project: schema.Project{Name: "demo"},
		Modules: map[string]schema.ModuleInfo{
			"src/api": {Purpose: "HTTP layer", Language: "Go", Files: 1, Lines: 10, DependsOn: []string{"src/db"}, Exports: []string{"Serve"}},
			"src/db":  {Purpose: "Storage", Language: "Go", Files: 1, Lines: 20, DependedBy: []string{"src/api"}},
		},
		Dependencies: schema.Dependencies{Edges: [][2]string{{"src/api", "src/db"}}},
	}
	g := &graph.SymGraph{Version: "1", Nodes: []graph.SymNode{
		{ID: "src/api/a.go", Name: "a.go", Kind: graph.KindFile, File: "src/api/a.go"},
		{ID: "src/api/a.go::Serve", Name: "Serve", Kind: graph.KindFunc, File: "src/api/a.go", Line: 3},
		{ID: "src/db/d.go::Open", Name: "Open", Kind: graph.KindFunc, File: "src/db/d.go", Line: 5},
	}, Edges: []graph.SymEdge{{From: 1, To: 2, Kind: graph.EdgeCalls, Conf: graph.ConfInferred, Score: 0.6}}}
	g.Index()
	return Input{Index: idx, Graph: g}
}

func one(t *testing.T, format string, in Input, o Options) string {
	t.Helper()
	fs, err := Render(format, in, o)
	if err != nil || len(fs) != 1 {
		t.Fatalf("%s: %v %d", format, err, len(fs))
	}
	return fs[0].Content
}

func TestGraphMLWellFormed(t *testing.T) {
	out := one(t, "graphml", fixture(), Options{})
	if err := xml.Unmarshal([]byte(out), new(struct{ XMLName xml.Name })); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `source_file`) || !strings.Contains(out, `src/db/d.go`) {
		t.Fatalf("missing source_file: %s", out)
	}
}

func TestModuleScopeAndJSON(t *testing.T) {
	out := one(t, "json", fixture(), Options{Scope: ScopeModules})
	var m Model
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Nodes) != 2 || len(m.Edges) != 1 || m.Edges[0].Kind != "depends_on" {
		t.Fatalf("bad model %+v", m)
	}
}

func TestCypherDotMermaid(t *testing.T) {
	in := fixture()
	if c := one(t, "cypher", in, Options{}); !strings.Contains(c, "CREATE (:Func {id:'src/api/a.go::Serve'") || !strings.Contains(c, "-[:CALLS {confidence:'INFERRED'") {
		t.Fatalf("cypher: %s", c)
	}
	if d := one(t, "dot", in, Options{}); !strings.Contains(d, "style=dashed") {
		t.Fatalf("dot: %s", d)
	}
	if m := one(t, "mermaid", in, Options{}); !strings.Contains(m, "-.->") {
		t.Fatalf("mermaid: %s", m)
	}
}

func TestCapKeepsHighDegree(t *testing.T) {
	m := &Model{Nodes: []Node{{ID: "a"}, {ID: "b"}, {ID: "c"}}, Edges: []Edge{{From: "b", To: "c"}}}
	c := m.capped(2)
	if len(c.Nodes) != 2 || c.Nodes[0].ID != "b" || len(c.Edges) != 1 {
		t.Fatalf("%+v", c)
	}
}

func TestObsidianNotes(t *testing.T) {
	fs, err := Render("obsidian", fixture(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, f := range fs {
		names[f.Name] = f.Content
	}
	api := names["src__api.md"]
	if !strings.Contains(api, "[[src__db|src/db]]") || !strings.Contains(api, "### Serve") || !strings.Contains(api, "[[src__db#Open|Open]]") {
		t.Fatalf("api note: %s", api)
	}
	if _, ok := names["Index.md"]; !ok {
		t.Fatal("no index")
	}
}

func TestWikiDeterministicAndBudgeted(t *testing.T) {
	a, _ := Render("markdown-wiki", fixture(), Options{PageTokens: 40})
	b, _ := Render("markdown-wiki", fixture(), Options{PageTokens: 40})
	if len(a) != 3 || a[0].Name != "index.md" {
		t.Fatalf("files %v", a)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("not deterministic")
		}
		if approxTokens(a[i].Content) > 80 {
			t.Fatalf("%s over budget: %d", a[i].Name, approxTokens(a[i].Content))
		}
	}
}

func TestWriteTo(t *testing.T) {
	dir := t.TempDir()
	fs, _ := Render("markdown-wiki", fixture(), Options{})
	if _, err := WriteTo(filepath.Join(dir, "wiki"), fs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wiki", "modules", "src__api.md")); err != nil {
		t.Fatal(err)
	}
	single, _ := Render("dot", fixture(), Options{})
	out := filepath.Join(dir, "g.dot")
	if _, err := WriteTo(out, single); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownFormat(t *testing.T) {
	if _, err := Render("nope", fixture(), Options{}); err == nil {
		t.Fatal("expected error")
	}
}
