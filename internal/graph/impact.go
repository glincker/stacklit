package graph

import (
	"fmt"
	"sort"
)

// Impact walks dependents of a symbol or file to estimate its blast radius.
// Reverse edges are followed to maxDepth; each node keeps its strongest path score.
func (g *SymGraph) Impact(target string, maxDepth, budget int, asJSON bool) (string, error) {
	ms := g.Find(target)
	if len(ms) == 0 {
		return "", fmt.Errorf("no symbol or file matches %q", target)
	}
	if maxDepth <= 0 {
		maxDepth = 4
	}
	if budget <= 0 {
		budget = DefaultImpactBudget
	}
	root := ms[0]
	seeds := []int{root}
	if g.Nodes[root].Kind == KindFile {
		for i := range g.Nodes {
			if g.Nodes[i].File == g.Nodes[root].File && i != root {
				seeds = append(seeds, i)
			}
		}
	}
	depth := map[int]int{}
	score := map[int]float64{}
	for _, s := range seeds {
		depth[s], score[s] = 0, 1
	}
	frontier := seeds
	for d := 1; d <= maxDepth && len(frontier) > 0; d++ {
		var next []int
		for _, u := range frontier {
			for _, ei := range g.in[u] {
				e := g.Edges[ei]
				v := e.From
				if _, seen := depth[v]; seen {
					continue
				}
				depth[v] = d
				score[v] = score[u] * e.Score
				next = append(next, v)
			}
		}
		sort.Ints(next)
		frontier = next
	}
	var ids []int
	files := map[string]bool{}
	direct := 0
	for v, d := range depth {
		if d == 0 {
			continue
		}
		if g.Nodes[v].Kind != KindFile {
			ids = append(ids, v)
		}
		files[g.Nodes[v].File] = true
		if d == 1 {
			direct++
		}
	}
	pr := g.PageRank()
	sort.SliceStable(ids, func(a, b int) bool {
		x, y := ids[a], ids[b]
		if depth[x] != depth[y] {
			return depth[x] < depth[y]
		}
		if score[x] != score[y] {
			return score[x] > score[y]
		}
		if pr[x] != pr[y] {
			return pr[x] > pr[y]
		}
		return g.tieLess(x, y)
	})
	items := make([]item, len(ids))
	for i, v := range ids {
		items[i] = item{node: v, tags: []string{fmt.Sprintf("depth %d  %s", depth[v], impactConf(score[v]))}}
	}
	head := []string{fmt.Sprintf("%d dependents (%d direct) across %d files, depth<=%d", len(ids), direct, len(files), maxDepth)}
	hint := fmt.Sprintf("stacklit impact %s --budget %d", target, budget*2)
	return g.FitView(viewSpec{title: "impact " + g.Label(root), head: head, items: items, hint: hint}, budget, asJSON), nil
}

func impactConf(s float64) string {
	if s >= 0.9 {
		return fmt.Sprintf("sure %.2f", s)
	}
	return fmt.Sprintf("likely %.2f", s)
}
