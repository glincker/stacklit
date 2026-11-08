package graph

import (
	"fmt"
	"strings"
	"unicode"
)

var stopWords = wordSet("a an the is are was were be to of in on at for and or how what where which who why does do did it its this that with from by as into about can i we you me my show find list get tell explain used use uses using code function functions method class file files")

// words splits identifiers and prose into lowercase words (camelCase, snake_case, paths).
func words(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if unicode.IsUpper(r) && i > 0 && (unicode.IsLower(rs[i-1]) || unicode.IsDigit(rs[i-1]) ||
				(i+1 < len(rs) && unicode.IsLower(rs[i+1]) && unicode.IsUpper(rs[i-1]))) {
				flush()
			}
			cur = append(cur, r)
		default:
			flush()
		}
	}
	flush()
	return out
}

func queryTerms(q string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range words(q) {
		if len(w) < 2 || stopWords[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}

// Query returns a budgeted, file-grouped subgraph for a natural language question,
// in the order set by STACKLIT_QUERY_ORDER (relevance by default).
func (g *SymGraph) Query(q string, budget int, asJSON bool) string {
	return g.QueryOrdered(q, budget, asJSON, DefaultOrder())
}

// QueryOrdered is Query with an explicit file order.
func (g *SymGraph) QueryOrdered(q string, budget int, asJSON bool, order Order) string {
	if budget <= 0 {
		budget = DefaultQueryBudget
	}
	hint := fmt.Sprintf("stacklit query %q --budget %d", q, budget*2)
	hits := g.Search(q, 60)
	title := fmt.Sprintf("query %q", q)
	if len(hits) == 0 {
		return (&View{Title: title + ": no matching symbols", Files: []FileGroup{}}).Render(asJSON)
	}
	seeds := make([]int, len(hits))
	score := map[int]float64{}
	rel := map[int]float64{}
	maxRel := hits[0].Score
	for i, h := range hits {
		seeds[i] = h.Node
		score[h.Node] = h.Score / maxRel
		rel[h.Node] = h.Score
	}
	pr := g.PageRank()
	// Expand one hop from the strongest seeds; neighbours inherit a damped score.
	expand := seeds
	if len(expand) > 8 {
		expand = expand[:8]
	}
	for _, s := range expand {
		for _, list := range [][]int32{g.out[s], g.in[s]} {
			for _, ei := range list {
				e := g.Edges[ei]
				o := e.To
				if o == s {
					o = e.From
				}
				if g.Nodes[o].Kind == KindFile {
					continue
				}
				v := 0.3*(rel[s]/maxRel)*e.Score + 0.1*pr[o]
				if v > score[o] {
					score[o] = v
				}
			}
		}
	}
	ids := make([]int, 0, len(score))
	for i := range score {
		ids = append(ids, i)
	}
	g.sortByScore(ids, func(i int) float64 { return score[i] })
	if len(ids) > 200 {
		ids = ids[:200]
	}
	items := make([]item, len(ids))
	for i, id := range ids {
		items[i] = item{node: id, score: score[id]}
	}
	head := []string{fmt.Sprintf("%d seeds, %d candidates", len(seeds), len(ids))}
	return g.FitView(viewSpec{title: title, head: head, items: items, edges: true, hint: hint, ranked: true, order: order}, budget, asJSON)
}
