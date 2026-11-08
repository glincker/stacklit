package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Impact walks dependents of a symbol or file to estimate its blast radius.
// Reverse edges are followed to maxDepth; each node keeps its strongest path score.
func (g *SymGraph) Impact(target string, maxDepth, budget int, asJSON bool) (string, error) {
	return g.impactAt("", target, maxDepth, budget, asJSON)
}

// impactAt is Impact plus call-site lines for direct dependents when root is set.
func (g *SymGraph) impactAt(root, target string, maxDepth, budget int, asJSON bool) (string, error) {
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
	top := ms[0]
	seeds := []int{top}
	if g.Nodes[top].Kind == KindFile {
		for i := range g.Nodes {
			if g.Nodes[i].File == g.Nodes[top].File && i != top {
				seeds = append(seeds, i)
			}
		}
	}
	depth := map[int]int{}
	parent := map[int]int{}
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
				parent[v] = u
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
		tag := fmt.Sprintf("depth %d  %s", depth[v], impactConf(score[v]))
		if root != "" && depth[v] == 1 {
			if sites := g.callSites(root, v, parent[v]); sites != "" {
				tag += "  at " + sites
			}
		}
		items[i] = item{node: v, tags: []string{tag}}
	}
	head := []string{fmt.Sprintf("%d dependents (%d direct) across %d files, depth<=%d", len(ids), direct, len(files), maxDepth)}
	hint := fmt.Sprintf("stacklit impact %s --budget %d", target, budget*2)
	return g.FitView(viewSpec{title: "impact " + g.Label(top), head: head, items: items, hint: hint}, budget, asJSON), nil
}

func impactConf(s float64) string {
	if s >= 0.9 {
		return fmt.Sprintf("sure %.2f", s)
	}
	return fmt.Sprintf("likely %.2f", s)
}

// maxCallSites caps the lines listed per caller so one busy function stays short.
const maxCallSites = 3

// callSites returns "file:l1,l2" for the lines inside the caller's body that
// use the callee's name, skipping comment lines. Empty when the source is
// unreadable or no line matches.
func (g *SymGraph) callSites(root string, caller, callee int) string {
	cn, tn := &g.Nodes[caller], &g.Nodes[callee]
	if cn.Kind == KindFile || tn.Kind == KindFile || cn.Line <= 0 {
		return ""
	}
	name := tn.Name
	if i := strings.LastIndexAny(name, ".:"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cn.File)))
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	end := cn.End
	if end < cn.Line {
		end = cn.Line
	}
	var lines []string
	for n, l := range strings.Split(string(data), "\n") {
		ln := n + 1
		if ln < cn.Line || ln > end || !re.MatchString(l) {
			continue
		}
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "*") {
			continue
		}
		if ln == cn.Line && end > cn.Line {
			continue // the declaration line, not a call
		}
		lines = append(lines, strconv.Itoa(ln))
		if len(lines) == maxCallSites {
			break
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return cn.File + ":" + strings.Join(lines, ",")
}
