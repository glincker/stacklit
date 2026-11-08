package graph

import (
	"fmt"
	"math"
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

// relevance scores a node against query terms using its name, signature,
// doc comment and path. Terms matching many nodes are diluted.
func (g *SymGraph) relevance(terms []string) map[int]float64 {
	type feat struct {
		name, nameL, sig, doc, file string
		nw                          map[string]bool
	}
	hits := make([]int, len(terms))
	score := map[int]float64{}
	contrib := func(f *feat, t string) float64 {
		s := 0.0
		switch {
		case f.nameL == t:
			s = 8
		case f.nw[t]:
			s = 4
		case strings.Contains(f.nameL, t):
			s = 2
		}
		if strings.Contains(f.sig, t) {
			s += 1
		}
		if strings.Contains(f.doc, t) {
			s += 1.5
		}
		if strings.Contains(f.file, t) {
			s += 1
		}
		return s
	}
	feats := make([]feat, len(g.Nodes))
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind == KindFile {
			continue
		}
		f := feat{nameL: strings.ToLower(n.Name), sig: strings.ToLower(n.Sig), doc: strings.ToLower(n.Doc), file: strings.ToLower(n.File), nw: map[string]bool{}}
		for _, w := range words(n.Name) {
			f.nw[w] = true
		}
		feats[i] = f
		for ti, t := range terms {
			if contrib(&f, t) > 0 {
				hits[ti]++
			}
		}
	}
	n := float64(len(g.Nodes))
	for i := range g.Nodes {
		if g.Nodes[i].Kind == KindFile {
			continue
		}
		total, matched := 0.0, 0
		for ti, t := range terms {
			c := contrib(&feats[i], t)
			if c == 0 {
				continue
			}
			matched++
			total += c * (1 + math.Log(1+n/float64(1+hits[ti])))
		}
		if matched > 0 {
			// Reward covering more of the question's terms.
			score[i] = total * (1 + 0.5*float64(matched-1))
		}
	}
	return score
}

// Query returns a budgeted, file-grouped subgraph for a natural language question.
func (g *SymGraph) Query(q string, budget int, asJSON bool) string {
	if budget <= 0 {
		budget = DefaultQueryBudget
	}
	hint := fmt.Sprintf("stacklit query %q --budget %d", q, budget*2)
	terms := queryTerms(q)
	rel := g.relevance(terms)
	pr := g.PageRank()
	var seeds []int
	maxRel := 0.0
	for i, s := range rel {
		seeds = append(seeds, i)
		if s > maxRel {
			maxRel = s
		}
	}
	title := fmt.Sprintf("query %q", q)
	if len(seeds) == 0 {
		return (&View{Title: title + ": no matching symbols", Files: []FileGroup{}}).Render(asJSON)
	}
	g.sortByScore(seeds, func(i int) float64 { return rel[i]/maxRel + 0.25*pr[i] })
	if len(seeds) > 60 {
		seeds = seeds[:60]
	}
	score := map[int]float64{}
	for _, s := range seeds {
		score[s] = rel[s]/maxRel + 0.25*pr[s]
	}
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
		items[i] = item{node: id}
	}
	head := []string{fmt.Sprintf("%d seeds, %d candidates", len(seeds), len(ids))}
	return g.FitView(viewSpec{title: title, head: head, items: items, edges: true, hint: hint}, budget, asJSON)
}
