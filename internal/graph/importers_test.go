package graph

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func importFixture(t *testing.T) (*SymGraph, string) {
	t.Helper()
	root, files := writeTree(t, map[string]string{
		"go.mod":                  "module example.com/m\n\ngo 1.22\n",
		"internal/store/store.go": "package store\n\nfunc Open() int { return 1 }\n",
		"internal/store/more.go":  "package store\n\nfunc Close() int { return 2 }\n",
		"internal/store/sub/s.go": "package sub\n\nfunc S() int { return 3 }\n",
		"cmd/app/main.go":         "package main\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/m/internal/store\"\n)\n\nfunc main() { fmt.Println(store.Open()) }\n",
		"api/api.go":              "package api\n\nimport \"example.com/m/internal/store\"\n\nfunc A() int { return store.Close() }\n",
		"api/other.go":            "package api\n\nimport \"example.com/m/internal/store/sub\"\n\nfunc B() int { return sub.S() }\n",
		"web/lib/core.ts":         "export function core(): number { return 1; }\n",
		"web/lib/index.ts":        "export * from './core';\n",
		"web/a.ts":                "import { core } from './lib/core';\nexport const a = core();\n",
		"web/b.ts":                "import {\n  core,\n} from \"./lib/core.js\";\nexport const b = core();\n",
		"web/c.ts":                "import { x } from './lib';\nexport const c = x;\n",
		"web/d.ts":                "// from './lib/core' is only a comment\nexport const d = 1;\n",
	})
	return BuildSymbols(root, files, ExtractOptions{}), root
}

func importerNames(out string) []string {
	var r []string
	for _, l := range strings.Split(strings.TrimSpace(out), "\n")[1:] {
		if !strings.HasPrefix(l, "[") && !strings.HasPrefix(l, "also") {
			r = append(r, l)
		}
	}
	return r
}

func TestImpactForPackageListsExactImporters(t *testing.T) {
	g, root := importFixture(t)
	out, err := g.ImpactFor(root, "internal/store", 0, 500, false, false)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(importerNames(out), " ")
	if got != "api/api.go:3 cmd/app/main.go:6" {
		t.Fatalf("importers %q\n%s", got, out)
	}
	if !strings.HasPrefix(out, "impact internal/store: 2 importers of package internal/store\n") {
		t.Errorf("header: %q", out)
	}
	// The subpackage is a separate package: its importer is not listed above.
	sub, _ := g.ImpactFor(root, "internal/store/sub", 0, 500, false, false)
	if got := strings.Join(importerNames(sub), " "); got != "api/other.go:3" {
		t.Errorf("subpackage importers %q", got)
	}
}

func TestImpactForFileAndBareName(t *testing.T) {
	g, root := importFixture(t)
	for _, target := range []string{"web/lib/core.ts", "lib/core.ts", "core.ts"} {
		out, err := g.ImpactFor(root, target, 0, 500, false, false)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		got := strings.Join(importerNames(out), " ")
		if got != "web/a.ts:1 web/b.ts:3 web/lib/index.ts:1" {
			t.Errorf("%s importers %q\n%s", target, got, out)
		}
	}
	// The index file is imported as its directory; the comment in d.ts is not an import.
	out, _ := g.ImpactFor(root, "web/lib/index.ts", 0, 500, false, false)
	if got := strings.Join(importerNames(out), " "); got != "web/c.ts:1" {
		t.Errorf("index importers %q\n%s", got, out)
	}
}

func TestImpactForScanFindsEdgesTheGraphMissed(t *testing.T) {
	g, root := importFixture(t)
	var kept []SymEdge
	for _, e := range g.Edges {
		if !(e.Kind == EdgeImports && g.Nodes[e.From].File == "web/b.ts") {
			kept = append(kept, e)
		}
	}
	g.Edges = kept
	g.Index()
	out, _ := g.ImpactFor(root, "web/lib/core.ts", 0, 500, false, false)
	if !strings.Contains(out, "web/b.ts:3") {
		t.Errorf("scan must restore the missed importer:\n%s", out)
	}
	t.Setenv("STACKLIT_IMPACT_SCAN", "0")
	out, _ = g.ImpactFor(root, "web/lib/core.ts", 0, 500, false, false)
	if strings.Contains(out, "web/b.ts") {
		t.Errorf("STACKLIT_IMPACT_SCAN=0 must use the graph only:\n%s", out)
	}
}

func TestImpactForSymbolKeepsCallView(t *testing.T) {
	g, root := importFixture(t)
	out, err := g.ImpactFor(root, "Open", 0, 500, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "importers of") || !strings.Contains(out, "dependents") {
		t.Errorf("an exact symbol name keeps the call-level view:\n%s", out)
	}
	call, err := g.ImpactFor(root, "internal/store", 0, 500, false, true)
	if err == nil && strings.Contains(call, "importers of") {
		t.Errorf("--calls must force the call-level view:\n%s", call)
	}
	if _, err := g.ImpactFor(root, "nothing/here", 0, 500, false, false); err == nil {
		t.Error("unknown target must fail")
	}
}

func TestImpactForBudgetAndJSON(t *testing.T) {
	g, root := importFixture(t)
	for _, budget := range []int{10, 20, 40, 60, 500} {
		out, _ := g.ImpactFor(root, "web/lib/core.ts", 0, budget, false, false)
		if approxTokens(out) > budget {
			t.Errorf("budget %d exceeded: %d tokens\n%s", budget, approxTokens(out), out)
		}
	}
	files := map[string]string{"lib/core.ts": "export const core = 1;\n"}
	for i := 0; i < 40; i++ {
		files[fmt.Sprintf("use/u%02d.ts", i)] = "import { core } from '../lib/core';\nexport const v = core;\n"
	}
	root2, rels := writeTree(t, files)
	g2 := BuildSymbols(root2, rels, ExtractOptions{})
	out, _ := g2.ImpactFor(root2, "lib/core.ts", 0, 120, false, false)
	if !strings.Contains(out, "[truncated:") || approxTokens(out) > 120 || !strings.Contains(out, "use/u00.ts:1") {
		t.Errorf("a cut list must say so, keep the first files and fit the budget:\n%s", out)
	}
	js, err := g.ImpactFor(root, "internal/store", 0, 500, true, false)
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Total     int        `json:"total"`
		Importers []Importer `json:"importers"`
	}
	if json.Unmarshal([]byte(js), &v) != nil || v.Total != 2 || len(v.Importers) != 2 || v.Importers[0].Line != 3 {
		t.Errorf("bad json: %s", js)
	}
	// Without a root the lines are simply absent.
	out, _ = g.ImpactFor("", "internal/store", 0, 500, false, false)
	if strings.Contains(out, ".go:") {
		t.Errorf("no root, no line numbers:\n%s", out)
	}
}

func TestImpactForOnThisRepoMatchesGrep(t *testing.T) {
	g := repoGraphFresh(t)
	out, err := g.ImpactFor("../..", "internal/fsutil", 0, 1500, false, false)
	if err != nil {
		t.Fatal(err)
	}
	list := importerNames(out)
	if len(list) < 3 {
		t.Fatalf("expected several importers of internal/fsutil, got %v", list)
	}
	for _, l := range list {
		if !strings.Contains(l, ".go:") {
			t.Errorf("missing file:line in %q", l)
		}
	}
	t.Logf("internal/fsutil: %d importers, %d tokens", len(list), approxTokens(out))
}
