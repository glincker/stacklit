package graph

import "sort"

// HTMLNode is a symbol in the compact form embedded in stacklit.html.
type HTMLNode struct {
	Name string `json:"n"`
	Kind string `json:"k"`
	File int    `json:"f"`
	Line int    `json:"l"`
	End  int    `json:"e"`
	Comm int    `json:"c"`
	Sig  string `json:"s,omitempty"`
	Doc  string `json:"d,omitempty"`
}

// HTMLGraph is the symbol graph reduced for the interactive page. Edges are
// [from, to, kindIndex, score, inferred(0/1)] over indexes into Nodes.
type HTMLGraph struct {
	Files []string     `json:"files"`
	Kinds []string     `json:"kinds"`
	Comms []Community  `json:"comms"`
	Nodes []HTMLNode   `json:"nodes"`
	Edges [][5]float64 `json:"edges"`
	Total int          `json:"total"`
}

var htmlKinds = []string{EdgeCalls, EdgeImports, EdgeInherits, EdgeImplements}

// HTMLData keeps the maxNodes most connected symbols so the page stays light.
func (g *SymGraph) HTMLData(maxNodes int) *HTMLGraph {
	var ids []int
	for i := range g.Nodes {
		if g.Nodes[i].Kind != KindFile {
			ids = append(ids, i)
		}
	}
	total := len(ids)
	if maxNodes > 0 && len(ids) > maxNodes {
		g.sortByScore(ids, func(i int) float64 { return float64(g.degree(i)) })
		ids = ids[:maxNodes]
	}
	sort.Ints(ids)
	h := &HTMLGraph{Kinds: htmlKinds, Comms: g.Communities, Total: total, Files: []string{}, Nodes: []HTMLNode{}, Edges: [][5]float64{}}
	fileIdx := map[string]int{}
	newIdx := map[int]int{}
	for _, i := range ids {
		n := &g.Nodes[i]
		fi, ok := fileIdx[n.File]
		if !ok {
			fi = len(h.Files)
			fileIdx[n.File] = fi
			h.Files = append(h.Files, n.File)
		}
		doc := n.Doc
		if len(doc) > 80 {
			doc = doc[:80]
		}
		newIdx[i] = len(h.Nodes)
		h.Nodes = append(h.Nodes, HTMLNode{Name: n.Name, Kind: n.Kind, File: fi, Line: n.Line, End: n.End, Comm: n.Community, Sig: n.Sig, Doc: doc})
	}
	kindIdx := map[string]float64{}
	for i, k := range htmlKinds {
		kindIdx[k] = float64(i)
	}
	for _, e := range g.Edges {
		a, ok1 := newIdx[e.From]
		b, ok2 := newIdx[e.To]
		if !ok1 || !ok2 {
			continue
		}
		inf := 0.0
		if e.Conf == ConfInferred {
			inf = 1
		}
		h.Edges = append(h.Edges, [5]float64{float64(a), float64(b), kindIdx[e.Kind], e.Score, inf})
	}
	return h
}
