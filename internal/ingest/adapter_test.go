package ingest

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/glincker/stacklit/internal/graph"
)

func skipWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell adapters are tested on unix")
	}
}

const echoAdapter = `while read f; do printf '{"type":"node","id":"c:%s","name":"Topic","source_file":"%s"}\n' "$f" "$f"; printf '{"type":"edge","from":"c:%s","to":"%s","kind":"mentions"}\n' "$f" "$f"; done; echo not json; echo '{"type":"node","id":"x"}'`

func TestRunAdapterContract(t *testing.T) {
	skipWindows(t)
	r, err := RunAdapter(context.Background(), AdapterOptions{Root: t.TempDir(), Command: echoAdapter, Name: "echo", Files: []string{"a.pdf", "b.png"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Concepts) != 2 || len(r.Links) != 2 || r.Skipped != 2 {
		t.Fatalf("%+v", r)
	}
	c := r.Concepts[0]
	if c.Conf != ConfInferred || c.Source != "echo" || c.SourceFile != "a.pdf" {
		t.Fatalf("%+v", c)
	}
}

func TestRunAdapterLimits(t *testing.T) {
	skipWindows(t)
	root := t.TempDir()
	if _, err := RunAdapter(context.Background(), AdapterOptions{Root: root, Command: "sleep 5", Timeout: 150 * time.Millisecond}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
	if _, err := RunAdapter(context.Background(), AdapterOptions{Root: root, Command: "yes aaaaaaaaaaaaaaaa", MaxBytes: 1000}); err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("size: %v", err)
	}
	// A trailing command stops sh from exec-ing, so the child outlives sh like it does under dash.
	if _, err := RunAdapter(context.Background(), AdapterOptions{Root: root, Command: "yes aaaaaaaaaaaaaaaa; true", MaxBytes: 1000}); err == nil || !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("size with forked child: %v", err)
	}
	r, err := RunAdapter(context.Background(), AdapterOptions{Root: root, Command: echoAdapter, Files: []string{"a", "b", "c"}, MaxItems: 2})
	if err != nil || len(r.Concepts)+len(r.Links) != 2 {
		t.Fatalf("items: %v %+v", err, r)
	}
	if _, err := RunAdapter(context.Background(), AdapterOptions{Root: root, Command: "echo boom >&2; exit 3"}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("exit: %v", err)
	}
}

func TestApplyAdapterReplacesSameSource(t *testing.T) {
	ctx := &Context{}
	ApplyAdapter(ctx, &AdapterResult{Name: "x", Concepts: []Concept{{ID: "1", Source: "x"}, {ID: "2", Source: "x"}}})
	ApplyAdapter(ctx, &AdapterResult{Name: "y", Concepts: []Concept{{ID: "9", Source: "y"}}})
	ApplyAdapter(ctx, &AdapterResult{Name: "x", Concepts: []Concept{{ID: "3", Source: "x"}}})
	if len(ctx.Concepts) != 2 || ctx.Concepts[0].ID != "3" || ctx.Concepts[1].ID != "9" {
		t.Fatalf("%+v", ctx.Concepts)
	}
}

func TestFilterFiles(t *testing.T) {
	got := FilterFiles([]string{"docs/a.pdf", "b.png", "c.go"}, []string{"*.pdf", "b.*"})
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestMergeIntoGraph(t *testing.T) {
	g := &graph.SymGraph{Version: "1", Nodes: []graph.SymNode{
		{ID: "a.go", Name: "a.go", Kind: graph.KindFile, File: "a.go"},
	}}
	g.Index()
	r := &AdapterResult{Name: "ad", Concepts: []Concept{{ID: "k", Name: "Idea", SourceFile: "paper.pdf", Source: "ad"}},
		Links: []ConceptLink{{From: "k", To: "a.go", Kind: "mentions"}, {From: "k", To: "paper.pdf", Kind: "in"}, {From: "k", To: "missing"}}}
	if d := MergeIntoGraph(g, r); d != 1 {
		t.Fatalf("dropped %d", d)
	}
	if len(g.Nodes) != 3 || len(g.Edges) != 2 || g.Edges[0].Conf != graph.ConfInferred || g.Edges[0].Src != "ad" || g.Nodes[1].File != "paper.pdf" {
		t.Fatalf("%+v %+v", g.Nodes, g.Edges)
	}
	MergeIntoGraph(g, r) // idempotent
	if len(g.Nodes) != 3 || len(g.Edges) != 2 {
		t.Fatalf("not idempotent: %d %d", len(g.Nodes), len(g.Edges))
	}
}
