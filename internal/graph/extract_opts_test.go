package graph

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var rels []string
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	return root, rels
}

func TestIncludeTestsOption(t *testing.T) {
	root, files := writeTree(t, map[string]string{
		"a.py":      "def f():\n    return 1\n",
		"test_a.py": "from a import f\n\ndef test_f():\n    f()\n",
		"B.cs":      "class B { void M() {} }\n",
		"BTests.cs": "class BTests { void T() { new B().M(); } }\n",
		"c_spec.rb": "class CSpec\nend\n",
		"c.rb":      "class C\nend\n",
	})
	off := BuildSymbols(root, files, ExtractOptions{})
	on := BuildSymbols(root, files, ExtractOptions{IncludeTests: true})
	has := func(g *SymGraph, id string) bool {
		_, ok := g.byID[id]
		return ok
	}
	for _, id := range []string{"test_a.py", "BTests.cs", "c_spec.rb"} {
		if has(off, id) {
			t.Errorf("%s should be skipped by default", id)
		}
		if !has(on, id) {
			t.Errorf("%s should be present with IncludeTests", id)
		}
	}
	if has(on, "test_a.py") && !hasEdge(on, "imports", "test_a.py", "a.py") {
		t.Errorf("test file should link to the code it imports")
	}
}

func hasEdge(g *SymGraph, kind, from, to string) bool {
	for _, e := range g.Edges {
		if e.Kind == kind && g.Nodes[e.From].ID == from && g.Nodes[e.To].ID == to {
			return true
		}
	}
	return false
}

func TestOversizeFilesFallBack(t *testing.T) {
	py := "class A:\n    def m(self):\n        return 1\n"
	kt := "class A {\n    fun m() = 1\n}\n"
	small := ExtractOptions{MaxParseBytes: 8}
	fs := extractFileWith("a.py", []byte(py), small, nil)
	if fs == nil || len(fs.Defs) != 2 {
		t.Fatalf("python should fall back to the line scanner, got %+v", fs)
	}
	fs = extractFileWith("a.kt", []byte(kt), small, nil)
	if fs == nil || len(fs.Defs) != 0 || fs.Lang != "kotlin" {
		t.Fatalf("kotlin has no scanner, want an empty file entry, got %+v", fs)
	}
	if fs = extractFileWith("a.kt", []byte(kt), ExtractOptions{}, nil); len(fs.Defs) != 2 {
		t.Fatalf("kotlin within limits should parse, got %+v", fs)
	}
}

func TestParseGateSerialisesOversizeRequests(t *testing.T) {
	g := newParseGate(100)
	g.acquire(500) // larger than the budget runs alone
	done := make(chan struct{})
	go func() {
		g.acquire(10)
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("second request must wait for the oversize one")
	default:
	}
	g.release(500)
	<-done
	g.release(10)
	if newParseGate(-1) != nil {
		t.Fatal("negative budget disables the gate")
	}
}

func TestGraphIsByteStable(t *testing.T) {
	var first []byte
	for i := 0; i < 5; i++ {
		var buf bytes.Buffer
		for _, lang := range []string{"python", "csharp", "c", "elixir", "haskell", "typescript"} {
			g := fixtureGraph(t, lang, ExtractOptions{})
			b, err := json.Marshal(g)
			if err != nil {
				t.Fatal(err)
			}
			buf.Write(b)
		}
		if first == nil {
			first = buf.Bytes()
		} else if !bytes.Equal(first, buf.Bytes()) {
			t.Fatalf("run %d differs from run 0", i)
		}
	}
}

func TestReceiverInferenceAndConfidence(t *testing.T) {
	root, files := writeTree(t, map[string]string{
		"svc.py":  "from repo import Repo\n\nclass Svc:\n    def __init__(self, r: Repo):\n        self.repo = Repo()\n\n    def run(self, other: Repo):\n        self.repo.find()\n        other.find()\n        text.find()\n",
		"repo.py": "class Repo:\n    def find(self):\n        return 1\n",
		"misc.py": "class Misc:\n    def find(self):\n        return 2\n",
	})
	g := BuildSymbols(root, files, ExtractOptions{})
	find := func(from, to string) *SymEdge {
		for i, e := range g.Edges {
			if e.Kind == EdgeCalls && g.Nodes[e.From].ID == from && g.Nodes[e.To].ID == to {
				return &g.Edges[i]
			}
		}
		return nil
	}
	typed := find("svc.py::Svc.run", "repo.py::Repo.find")
	if typed == nil || typed.Conf != ConfExtracted || typed.Score < 0.9 {
		t.Fatalf("typed receiver should link to Repo.find as EXTRACTED, got %+v", typed)
	}
	// An untyped receiver can only be guessed, so it must stay INFERRED and low.
	guess := find("svc.py::Svc.run", "misc.py::Misc.find")
	if guess != nil && guess.Conf != ConfInferred {
		t.Fatalf("name-only guess must be INFERRED, got %+v", guess)
	}
}

func TestInheritedMethodResolution(t *testing.T) {
	root, files := writeTree(t, map[string]string{
		"Base.java":  "package p;\npublic class Base {\n    public void greet() {}\n}\n",
		"Child.java": "package p;\npublic class Child extends Base {\n    public void run() { greet(); this.greet(); }\n}\n",
	})
	g := BuildSymbols(root, files, ExtractOptions{})
	if !hasEdge(g, "calls", "Child.java::Child.run", "Base.java::Base.greet") {
		t.Fatalf("inherited call not linked: %v", g.Edges)
	}
}

func TestExternalReceiverTypeDoesNotGuess(t *testing.T) {
	root, files := writeTree(t, map[string]string{
		"A.java": "package p;\npublic class A {\n    public void m(String s) { s.trim(); }\n    public void trim() {}\n}\n",
	})
	g := BuildSymbols(root, files, ExtractOptions{})
	if hasEdge(g, "calls", "A.java::A.m", "A.java::A.trim") {
		t.Fatalf("String receiver must not link to a same-named project method")
	}
}

func TestOptionsFromEnv(t *testing.T) {
	t.Setenv("STACKLIT_GRAPH_INCLUDE_TESTS", "true")
	if !OptionsFromEnv().IncludeTests {
		t.Fatal("env should enable IncludeTests")
	}
	t.Setenv("STACKLIT_GRAPH_MAX_PARSE_BYTES", "123")
	if maxParseBytes(ExtractOptions{}) != 123 {
		t.Fatal("env should set the parse cap")
	}
	t.Setenv("STACKLIT_PARSE_MEMORY_MB", "0")
	if parseBudget(ExtractOptions{}) >= 0 {
		t.Fatal("0 MB means unlimited")
	}
	_ = strings.TrimSpace
}
