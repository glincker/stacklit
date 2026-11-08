package ingest

import (
	"strings"

	"github.com/glincker/stacklit/internal/graph"
)

// KindConcept is the symbol graph node kind for adapter concepts.
const KindConcept = "concept"

// MergeIntoGraph adds adapter concepts and links to g as INFERRED nodes and
// edges. Existing nodes from the same adapter are replaced. It returns how many
// links were dropped because an endpoint did not resolve.
func MergeIntoGraph(g *graph.SymGraph, r *AdapterResult) (dropped int) {
	prefix := "concept:" + r.Name + ":"
	g.Index()
	// Drop earlier nodes and edges from this adapter, then re-index.
	keep := make([]int, len(g.Nodes))
	var nodes []graph.SymNode
	for i, n := range g.Nodes {
		if n.Source == r.Name && (n.Kind == KindConcept || n.Kind == graph.KindFile) {
			keep[i] = -1
			continue
		}
		keep[i] = len(nodes)
		nodes = append(nodes, n)
	}
	var edges []graph.SymEdge
	for _, e := range g.Edges {
		if keep[e.From] < 0 || keep[e.To] < 0 {
			continue
		}
		e.From, e.To = keep[e.From], keep[e.To]
		edges = append(edges, e)
	}
	g.Nodes, g.Edges = nodes, edges
	g.Index()

	comm := -1
	for _, n := range g.Nodes {
		if n.Community > comm {
			comm = n.Community
		}
	}
	comm++
	byID := map[string]int{}
	for _, c := range r.Concepts {
		g.Nodes = append(g.Nodes, graph.SymNode{
			ID: prefix + c.ID, Name: c.Name, Kind: KindConcept, File: c.SourceFile, Doc: c.Summary,
			Community: comm, Source: r.Name,
		})
		byID[c.ID] = len(g.Nodes) - 1
	}
	if len(r.Concepts) > 0 {
		g.Communities = append(g.Communities, graph.Community{ID: comm, Label: "concepts:" + r.Name, Nodes: len(r.Concepts)})
	}
	g.Index()

	srcFiles := map[string]bool{}
	for _, c := range r.Concepts {
		srcFiles[c.SourceFile] = true
	}
	resolve := func(ref string) (int, bool) {
		if i, ok := byID[ref]; ok {
			return i, true
		}
		// Exact node id or file path only; fuzzy matches would invent links.
		for _, j := range g.Find(ref) {
			if g.Nodes[j].ID == ref || (g.Nodes[j].Kind == graph.KindFile && g.Nodes[j].File == ref) {
				return j, true
			}
		}
		// A non-code source file (PDF, image) has no node yet: add a file node so
		// the concept can point at it.
		if srcFiles[ref] {
			g.Nodes = append(g.Nodes, graph.SymNode{ID: ref, Name: ref, Kind: graph.KindFile, File: ref, Community: comm, Source: r.Name})
			g.Index()
			return len(g.Nodes) - 1, true
		}
		return 0, false
	}
	for _, l := range r.Links {
		f, ok1 := resolve(strings.TrimSpace(l.From))
		t, ok2 := resolve(strings.TrimSpace(l.To))
		if !ok1 || !ok2 {
			dropped++
			continue
		}
		g.Edges = append(g.Edges, graph.SymEdge{From: f, To: t, Kind: l.Kind, Conf: graph.ConfInferred, Score: 0.5, Src: r.Name})
	}
	g.Index()
	return dropped
}
