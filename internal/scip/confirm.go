package scip

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/graph"
)

// SrcTag is written into the src field of edges this package upgrades.
const SrcTag = "scip"

// Stats reports what Upgrade changed.
type Stats struct {
	Upgraded        int // INFERRED edges now EXACT
	AlreadyExact    int // EXTRACTED edges the index also confirms
	StillInferred   int // INFERRED edges the index did not confirm
	UnmatchedFiles  int // indexed documents with no node in the graph
	DocumentsParsed int
}

type span struct{ node, lo, hi int }

// Upgrade marks INFERRED edges EXACT when the SCIP data contains a reference
// from the edge's source to a symbol defined at the edge's target. It never
// adds or removes edges.
func Upgrade(g *graph.SymGraph, idx *Index) Stats {
	st := Stats{DocumentsParsed: len(idx.Documents)}
	g.Index()

	fileNode := map[string]int{}
	spans := map[string][]span{}
	for i, n := range g.Nodes {
		f := filepath.ToSlash(n.File)
		if n.Kind == graph.KindFile {
			fileNode[f] = i
			continue
		}
		hi := n.End
		if hi < n.Line {
			hi = n.Line
		}
		spans[f] = append(spans[f], span{i, n.Line, hi})
	}
	resolvePath := pathResolver(fileNode)

	// owner finds the innermost symbol covering a 1-based line, or the file node.
	owner := func(file string, line int) int {
		best, bestSize := -1, 0
		for _, s := range spans[file] {
			if line >= s.lo && line <= s.hi {
				if size := s.hi - s.lo; best < 0 || size < bestSize {
					best, bestSize = s.node, size
				}
			}
		}
		if best >= 0 {
			return best
		}
		if f, ok := fileNode[file]; ok {
			return f
		}
		return -1
	}

	// Pass 1: where is each symbol defined.
	defs := map[string]int{}
	for _, d := range idx.Documents {
		file, ok := resolvePath(d.Path)
		if !ok {
			st.UnmatchedFiles++
			continue
		}
		for _, o := range d.Occurrences {
			if o.Roles&RoleDefinition == 0 || o.Symbol == "" || o.Line < 0 {
				continue
			}
			key := symKey(o, file)
			if n := owner(file, o.Line+1); n >= 0 {
				if _, seen := defs[key]; !seen {
					defs[key] = n
				}
			}
		}
	}

	// Pass 2: every reference yields a confirmed (from, to) pair, at symbol
	// level and at file level.
	type pair struct{ a, b int }
	confirmed := map[pair]bool{}
	for _, d := range idx.Documents {
		file, ok := resolvePath(d.Path)
		if !ok {
			continue
		}
		for _, o := range d.Occurrences {
			if o.Roles&RoleDefinition != 0 || o.Symbol == "" || o.Line < 0 {
				continue
			}
			to, ok := defs[symKey(o, file)]
			if !ok {
				continue
			}
			from := owner(file, o.Line+1)
			if from < 0 || from == to {
				continue
			}
			confirmed[pair{from, to}] = true
			ff, tf := fileNode[filepath.ToSlash(g.Nodes[from].File)], fileNode[filepath.ToSlash(g.Nodes[to].File)]
			if ff != tf {
				confirmed[pair{ff, tf}] = true
			}
		}
	}

	for i := range g.Edges {
		e := &g.Edges[i]
		ok := confirmed[pair{e.From, e.To}]
		switch {
		case e.Conf == graph.ConfInferred && ok:
			e.Conf, e.Score, e.Src = graph.ConfExact, 1, SrcTag
			st.Upgraded++
		case e.Conf == graph.ConfInferred:
			st.StillInferred++
		case e.Conf == graph.ConfExtracted && ok:
			st.AlreadyExact++
		}
	}
	return st
}

// symKey scopes local symbols to their document, since "local 3" repeats per file.
func symKey(o Occurrence, file string) string {
	if o.IsLocal {
		return file + "\x00" + o.Symbol
	}
	return o.Symbol
}

// pathResolver maps an index document path to a graph file path: exact first,
// then a unique suffix match for indexes made from a parent directory.
func pathResolver(files map[string]int) func(string) (string, bool) {
	var names []string
	for f := range files {
		names = append(names, f)
	}
	sort.Strings(names)
	return func(p string) (string, bool) {
		p = filepath.ToSlash(p)
		if _, ok := files[p]; ok {
			return p, true
		}
		var hit string
		n := 0
		for _, f := range names {
			if strings.HasSuffix(p, "/"+f) || strings.HasSuffix(f, "/"+p) {
				hit = f
				n++
			}
		}
		return hit, n == 1
	}
}
