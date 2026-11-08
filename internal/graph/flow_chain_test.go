package graph

import (
	"strings"
	"testing"
)

func TestFlowQueryReturnsWholeCallChain(t *testing.T) {
	root, files := writeTree(t, map[string]string{
		"go.mod":    "module example.com/m\n\ngo 1.22\n",
		"h/h.go":    "package h\n\nimport \"example.com/m/s\"\n\nfunc Handle() int { return s.Save() }\n",
		"s/s.go":    "package s\n\nimport \"example.com/m/d\"\n\nfunc Save() int { return d.Write() }\n",
		"d/d.go":    "package d\n\nfunc Write() int { return flush() }\n\nfunc flush() int { return 1 }\n",
		"main/m.go": "package main\n\nimport \"example.com/m/h\"\n\nfunc main() { h.Handle() }\n",
	})
	g := BuildSymbols(root, files, ExtractOptions{})
	out := g.Query("trace the Save flow", 2000, false)
	for _, want := range []string{"call chain of", "caller of Save, depth 1", "caller of Save, depth 2", "callee of Save, depth 1", "callee of Save, depth 2", "flush"} {
		if !strings.Contains(out, want) {
			t.Errorf("flow answer lacks %q\n%s", want, out)
		}
	}
	// A plain lookup stays a plain lookup.
	if plain := g.Query("Save", 2000, false); strings.Contains(plain, "call chain of") {
		t.Errorf("non-flow query got a chain\n%s", plain)
	}
	if again := g.Query("trace the Save flow", 2000, false); again != out {
		t.Error("flow answer is not byte stable")
	}
}
