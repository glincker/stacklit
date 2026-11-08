package graph

import (
	"fmt"
	"testing"
	"time"
)

// synthetic builds nFiles Go-like files, each with 10 defs and 10 refs to other
// files, so edges grow linearly with nFiles.
func synthetic(nFiles int) []*FileSyms {
	files := make([]*FileSyms, nFiles)
	for i := range files {
		f := &FileSyms{Path: fmt.Sprintf("p%d/f%d.ts", i/50, i), Lang: "typescript"}
		for d := 0; d < 10; d++ {
			f.Defs = append(f.Defs, Def{Name: fmt.Sprintf("fn_%d_%d", i, d), Qual: fmt.Sprintf("fn_%d_%d", i, d), Kind: KindFunc, Line: d*3 + 1, End: d*3 + 3})
		}
		for r := 0; r < 10; r++ {
			t := (i*7 + r*131 + 1) % nFiles
			f.Imports = append(f.Imports, Import{Spec: relSpec(i, t, nFiles), Names: []string{fmt.Sprintf("fn_%d_%d", t, r)}})
			f.Refs = append(f.Refs, Ref{From: r, Kind: EdgeCalls, Name: fmt.Sprintf("fn_%d_%d", t, r)})
		}
		files[i] = f
	}
	return files
}

func relSpec(from, to, n int) string {
	return fmt.Sprintf("../p%d/f%d", to/50, to)
}

func timeResolve(n int) (time.Duration, *SymGraph) {
	fs := synthetic(n)
	best := time.Hour
	var g *SymGraph
	for i := 0; i < 3; i++ {
		start := time.Now()
		g = Resolve(fs, ResolveOptions{})
		if d := time.Since(start); d < best {
			best = d
		}
	}
	return best, g
}

func TestResolveScalesLinearly(t *testing.T) {
	small, _ := timeResolve(1250)
	big, g := timeResolve(5000) // ~100k edges incl. import edges, 4x the small run
	if len(g.Edges) < 50000 {
		t.Fatalf("synthetic graph has only %d edges", len(g.Edges))
	}
	if big > 20*time.Second {
		t.Fatalf("resolving %d edges took %v", len(g.Edges), big)
	}
	// 4x the input must cost well under 16x the time if resolution is not quadratic.
	if small > 5*time.Millisecond && big > small*10 {
		t.Fatalf("superlinear: 1250 files %v, 5000 files %v", small, big)
	}
	t.Logf("edges=%d small=%v big=%v", len(g.Edges), small, big)
}

func TestQueryOnLargeGraphIsBounded(t *testing.T) {
	_, g := timeResolve(5000)
	start := time.Now()
	out := g.Query("fn 12 3", 500, false)
	if approxTokens(out) > 500 {
		t.Fatal("over budget")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("query took %v", d)
	}
}
