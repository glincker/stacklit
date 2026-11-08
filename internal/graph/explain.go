package graph

import (
	"fmt"
	"sort"
)

// relTag names the relationship of other to the subject for one edge.
func relTag(kind string, outgoing bool) string {
	switch kind {
	case EdgeCalls:
		if outgoing {
			return "callee"
		}
		return "caller"
	case EdgeInherits:
		if outgoing {
			return "extends"
		}
		return "extended_by"
	case EdgeImplements:
		if outgoing {
			return "implements"
		}
		return "implemented_by"
	}
	if outgoing {
		return "imports"
	}
	return "imported_by"
}

// Explain describes one symbol: where it is defined, its signature, callers and callees.
func (g *SymGraph) Explain(sym string, budget int, asJSON bool) (string, error) {
	ms := g.Find(sym)
	if len(ms) == 0 {
		return "", fmt.Errorf("no symbol matches %q", sym)
	}
	if budget <= 0 {
		budget = DefaultExplainBudget
	}
	i := ms[0]
	n := &g.Nodes[i]
	head := []string{fmt.Sprintf("%s %s  %s  community: %s", n.Kind, g.Label(i), locRange(n), g.communityLabel2(n.Community))}
	if n.Sig != "" {
		head = append(head, "sig: "+n.Sig)
	}
	if n.Doc != "" {
		head = append(head, "doc: "+n.Doc)
	}
	if len(ms) > 1 {
		var alt []string
		for _, m := range ms[1:] {
			if len(alt) == 4 {
				break
			}
			alt = append(alt, g.Loc(m))
		}
		head = append(head, fmt.Sprintf("%d other matches, e.g. %s (use the full id to pick one)", len(ms)-1, joinComma(alt)))
	}
	type rel struct {
		other int
		tag   string
		score float64
	}
	var rels []rel
	for _, ei := range g.out[i] {
		e := g.Edges[ei]
		rels = append(rels, rel{e.To, relTag(e.Kind, true) + " " + tag(e), e.Score})
	}
	for _, ei := range g.in[i] {
		e := g.Edges[ei]
		rels = append(rels, rel{e.From, relTag(e.Kind, false) + " " + tag(e), e.Score})
	}
	pr := g.PageRank()
	sort.SliceStable(rels, func(a, b int) bool {
		if rels[a].score != rels[b].score {
			return rels[a].score > rels[b].score
		}
		if pr[rels[a].other] != pr[rels[b].other] {
			return pr[rels[a].other] > pr[rels[b].other]
		}
		return g.tieLess(rels[a].other, rels[b].other)
	})
	var items []item
	for _, r := range rels {
		items = append(items, item{node: r.other, tags: []string{r.tag}})
	}
	hint := fmt.Sprintf("stacklit explain %s --budget %d", sym, budget*2)
	return g.FitView(viewSpec{title: "explain " + g.Label(i), head: head, items: dedupeItems(items), hint: hint}, budget, asJSON), nil
}

func dedupeItems(in []item) []item {
	idx := map[int]int{}
	var out []item
	for _, it := range in {
		if j, ok := idx[it.node]; ok {
			out[j].tags = append(out[j].tags, it.tags...)
			continue
		}
		idx[it.node] = len(out)
		out = append(out, it)
	}
	return out
}

func locRange(n *SymNode) string {
	if n.Line == 0 {
		return n.File
	}
	if n.End > n.Line {
		return fmt.Sprintf("%s:%d-%d", n.File, n.Line, n.End)
	}
	return fmt.Sprintf("%s:%d", n.File, n.Line)
}

func (g *SymGraph) communityLabel2(c int) string {
	if c >= 0 && c < len(g.Communities) {
		return g.Communities[c].Label
	}
	return "-"
}

func joinComma(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}
