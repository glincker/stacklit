package graph

import (
	"path"
	"sort"
	"strings"
)

// ComputeCommunities clusters files by deterministic weighted label propagation
// and stamps every node with its file's community. No randomness, no LLM.
func (g *SymGraph) ComputeCommunities() {
	var files []int
	fileOf := make([]int, len(g.Nodes)) // node -> file node index
	pos := map[string]int{}
	for i := range g.Nodes {
		if g.Nodes[i].Kind == KindFile {
			pos[g.Nodes[i].File] = i
			files = append(files, i)
		}
	}
	for i := range g.Nodes {
		fileOf[i] = pos[g.Nodes[i].File]
	}
	type pair struct{ a, b int }
	w := map[pair]float64{}
	for _, e := range g.Edges {
		a, b := fileOf[e.From], fileOf[e.To]
		if a == b {
			continue
		}
		if a > b {
			a, b = b, a
		}
		w[pair{a, b}] += e.Score
	}
	type nb struct {
		to int
		w  float64
	}
	adj := map[int][]nb{}
	keys := make([]pair, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].a < keys[j].a || keys[i].a == keys[j].a && keys[i].b < keys[j].b })
	for _, k := range keys {
		adj[k.a] = append(adj[k.a], nb{k.b, w[k]})
		adj[k.b] = append(adj[k.b], nb{k.a, w[k]})
	}

	label := map[int]int{}
	for _, f := range files {
		label[f] = f
	}
	for iter := 0; iter < 30; iter++ {
		changed := false
		for _, f := range files {
			ns := adj[f]
			if len(ns) == 0 {
				continue
			}
			tally := map[int]float64{}
			for _, n := range ns {
				tally[label[n.to]] += n.w
			}
			best, bw := label[f], tally[label[f]]
			cands := make([]int, 0, len(tally))
			for l := range tally {
				cands = append(cands, l)
			}
			sort.Ints(cands)
			for _, l := range cands {
				if tally[l] > bw {
					best, bw = l, tally[l]
				}
			}
			if best != label[f] {
				label[f] = best
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Files with no cross-file edges group by directory instead of one cluster each.
	groups := map[string][]int{}
	for _, f := range files {
		key := ""
		if len(adj[f]) == 0 {
			key = "dir:" + path.Dir(g.Nodes[f].File)
		} else {
			key = "lp:" + itoa(label[f])
		}
		groups[key] = append(groups[key], f)
	}
	gk := make([]string, 0, len(groups))
	for k := range groups {
		gk = append(gk, k)
	}
	sort.Strings(gk)

	type comm struct {
		label string
		files []int
	}
	var comms []comm
	for _, k := range gk {
		comms = append(comms, comm{label: g.communityLabel(groups[k]), files: groups[k]})
	}
	sort.SliceStable(comms, func(i, j int) bool {
		if len(comms[i].files) != len(comms[j].files) {
			return len(comms[i].files) > len(comms[j].files)
		}
		return comms[i].label < comms[j].label
	})
	// Disambiguate repeated labels with the most connected symbol.
	count := map[string]int{}
	for _, c := range comms {
		count[c.label]++
	}
	g.Communities = g.Communities[:0]
	nodeComm := map[int]int{}
	members := make([][]int, len(comms))
	for i := range g.Nodes {
		nodeComm[i] = -1
	}
	deg := make([]int, len(g.Nodes))
	for _, e := range g.Edges {
		deg[e.From]++
		deg[e.To]++
	}
	symByFile := map[int][]int{}
	for i := range g.Nodes {
		if g.Nodes[i].Kind != KindFile {
			symByFile[fileOf[i]] = append(symByFile[fileOf[i]], i)
		}
	}
	for ci, c := range comms {
		var syms []int
		for _, f := range c.files {
			syms = append(syms, symByFile[f]...)
			nodeComm[f] = ci
			for _, s := range symByFile[f] {
				nodeComm[s] = ci
			}
		}
		members[ci] = syms
		sort.SliceStable(syms, func(a, b int) bool {
			if deg[syms[a]] != deg[syms[b]] {
				return deg[syms[a]] > deg[syms[b]]
			}
			return g.tieLess(syms[a], syms[b])
		})
		var top []string
		for _, s := range syms {
			if len(top) == 3 {
				break
			}
			if deg[s] > 0 {
				top = append(top, g.Nodes[s].Name)
			}
		}
		lbl := c.label
		if count[lbl] > 1 && len(top) > 0 {
			lbl += " (" + top[0] + ")"
		}
		g.Communities = append(g.Communities, Community{ID: ci, Label: lbl, Files: len(c.files), Nodes: len(syms), Top: top})
	}
	for i := range g.Nodes {
		g.Nodes[i].Community = nodeComm[i]
	}
}

// communityLabel names a cluster by the directory its files share, else its busiest directory.
func (g *SymGraph) communityLabel(files []int) string {
	dirs := map[string]int{}
	var common []string
	for i, f := range files {
		d := path.Dir(g.Nodes[f].File)
		dirs[d]++
		parts := strings.Split(d, "/")
		if i == 0 {
			common = parts
			continue
		}
		n := 0
		for n < len(common) && n < len(parts) && common[n] == parts[n] {
			n++
		}
		common = common[:n]
	}
	if c := strings.Join(common, "/"); c != "" && c != "." {
		return c
	}
	best, bc := ".", -1
	ds := make([]string, 0, len(dirs))
	for d := range dirs {
		ds = append(ds, d)
	}
	sort.Strings(ds)
	for _, d := range ds {
		if dirs[d] > bc {
			best, bc = d, dirs[d]
		}
	}
	return best
}
