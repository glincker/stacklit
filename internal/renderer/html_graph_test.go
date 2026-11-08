package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
)

func TestWriteHTMLWithGraph(t *testing.T) {
	src := "package a\n\nfunc One() { Two() }\n\nfunc Two() {}\n"
	g := graph.Resolve([]*graph.FileSyms{graph.ExtractFile("a.go", []byte(src))}, graph.ResolveOptions{})
	idx := &schema.Index{Modules: map[string]schema.ModuleInfo{}}
	dir := t.TempDir()
	cases := []struct {
		name string
		g    *graph.SymGraph
		want string
	}{
		{"with graph", g, `"n":"One"`},
		{"without graph", nil, `type="application/json">null</script>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, tc.name+".html")
			if err := WriteHTMLWithGraph(idx, tc.g, 100, p); err != nil {
				t.Fatal(err)
			}
			b, _ := os.ReadFile(p)
			s := string(b)
			for _, ph := range []string{"{{STACKLIT_GRAPH}}", "{{SYMBOLS_JS}}", "{{STACKLIT_DATA}}"} {
				if strings.Contains(s, ph) {
					t.Errorf("placeholder %s left in page", ph)
				}
			}
			if !strings.Contains(s, tc.want) {
				t.Errorf("page missing %q", tc.want)
			}
		})
	}
}
