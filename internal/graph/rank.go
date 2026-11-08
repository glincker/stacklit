package graph

import "sort"

// PageRank scores nodes by how much weighted traffic reaches them along edges.
// Iteration order is fixed by node index, so results are identical between runs.
func (g *SymGraph) PageRank() []float64 {
	if g.pr != nil && len(g.pr) == len(g.Nodes) {
		return g.pr
	}
	n := len(g.Nodes)
	pr := make([]float64, n)
	if n == 0 {
		return pr
	}
	outW := make([]float64, n)
	for _, e := range g.Edges {
		outW[e.From] += e.Score
	}
	const damping, iters = 0.85, 30
	for i := range pr {
		pr[i] = 1 / float64(n)
	}
	next := make([]float64, n)
	for it := 0; it < iters; it++ {
		dangling := 0.0
		for i := range next {
			next[i] = 0
			if outW[i] == 0 {
				dangling += pr[i]
			}
		}
		for _, e := range g.Edges {
			next[e.To] += damping * pr[e.From] * e.Score / outW[e.From]
		}
		base := (1-damping)/float64(n) + damping*dangling/float64(n)
		for i := range next {
			next[i] += base
		}
		pr, next = next, pr
	}
	max := 0.0
	for _, v := range pr {
		if v > max {
			max = v
		}
	}
	if max > 0 {
		for i := range pr {
			pr[i] /= max
		}
	}
	g.pr = pr
	return pr
}

// less orders node indexes by score descending, then file, line and ID so ties are stable.
func (g *SymGraph) tieLess(a, b int) bool {
	na, nb := &g.Nodes[a], &g.Nodes[b]
	if na.File != nb.File {
		return na.File < nb.File
	}
	if na.Line != nb.Line {
		return na.Line < nb.Line
	}
	return na.ID < nb.ID
}

func (g *SymGraph) sortByScore(ids []int, score func(int) float64) {
	sort.SliceStable(ids, func(i, j int) bool {
		si, sj := score(ids[i]), score(ids[j])
		if si != sj {
			return si > sj
		}
		return g.tieLess(ids[i], ids[j])
	})
}
