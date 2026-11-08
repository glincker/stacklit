package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Hub is a heavily connected symbol (a "god node").
type Hub struct {
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Loc       string  `json:"loc"`
	In        int     `json:"in"`
	Out       int     `json:"out"`
	Rank      float64 `json:"rank"`
	Community string  `json:"community"`
}

// Hubs returns the top n symbols by total degree, ties by PageRank then path.
func (g *SymGraph) Hubs(n int) []Hub {
	if n <= 0 {
		n = 15
	}
	pr := g.PageRank()
	var ids []int
	for i := range g.Nodes {
		if g.Nodes[i].Kind != KindFile && g.degree(i) > 0 {
			ids = append(ids, i)
		}
	}
	sort.SliceStable(ids, func(a, b int) bool {
		da, db := g.degree(ids[a]), g.degree(ids[b])
		if da != db {
			return da > db
		}
		if pr[ids[a]] != pr[ids[b]] {
			return pr[ids[a]] > pr[ids[b]]
		}
		return g.tieLess(ids[a], ids[b])
	})
	if len(ids) > n {
		ids = ids[:n]
	}
	out := make([]Hub, len(ids))
	for k, i := range ids {
		nd := &g.Nodes[i]
		out[k] = Hub{Name: g.Label(i), Kind: nd.Kind, Loc: g.Loc(i), In: len(g.in[i]), Out: len(g.out[i]), Rank: float64(int(pr[i]*100+0.5)) / 100, Community: g.communityLabel2(nd.Community)}
	}
	return out
}

// RenderHubs prints hubs as text or JSON.
func RenderHubs(hs []Hub, asJSON bool) string {
	if asJSON {
		data, _ := json.MarshalIndent(hs, "", " ")
		return string(data) + "\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "hubs (%d)\n", len(hs))
	for _, h := range hs {
		fmt.Fprintf(&b, "  %s %s  %s  in=%d out=%d rank=%.2f  [%s]\n", h.Kind, h.Name, h.Loc, h.In, h.Out, h.Rank, h.Community)
	}
	return b.String()
}

// RenderCommunities prints the community list as text or JSON.
func (g *SymGraph) RenderCommunities(asJSON bool) string {
	if asJSON {
		data, _ := json.MarshalIndent(g.Communities, "", " ")
		return string(data) + "\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "communities (%d)\n", len(g.Communities))
	for _, c := range g.Communities {
		fmt.Fprintf(&b, "  #%d %s  files=%d symbols=%d  top: %s\n", c.ID, c.Label, c.Files, c.Nodes, strings.Join(c.Top, ", "))
	}
	return b.String()
}

// Surprise is a cross-community link that is rare for its pair of communities.
type Surprise struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Kind     string  `json:"kind"`
	FromComm string  `json:"from_community"`
	ToComm   string  `json:"to_community"`
	Score    float64 `json:"score"`
	Conf     string  `json:"conf"`
}

// Surprises ranks cross-community symbol edges, favouring confident edges between
// community pairs that otherwise barely interact.
func (g *SymGraph) Surprises(n int) []Surprise {
	type pair struct{ a, b int }
	cnt := map[pair]int{}
	var idx []int
	for i, e := range g.Edges {
		if e.Kind == EdgeImports || g.Nodes[e.From].Kind == KindFile || g.Nodes[e.To].Kind == KindFile {
			continue
		}
		ca, cb := g.Nodes[e.From].Community, g.Nodes[e.To].Community
		if ca == cb {
			continue
		}
		if ca > cb {
			ca, cb = cb, ca
		}
		cnt[pair{ca, cb}]++
		idx = append(idx, i)
	}
	val := func(i int) float64 {
		e := g.Edges[i]
		ca, cb := g.Nodes[e.From].Community, g.Nodes[e.To].Community
		if ca > cb {
			ca, cb = cb, ca
		}
		return e.Score / float64(cnt[pair{ca, cb}])
	}
	sort.SliceStable(idx, func(a, b int) bool {
		va, vb := val(idx[a]), val(idx[b])
		if va != vb {
			return va > vb
		}
		return idx[a] < idx[b]
	})
	if len(idx) > n {
		idx = idx[:n]
	}
	out := make([]Surprise, len(idx))
	for k, i := range idx {
		e := g.Edges[i]
		out[k] = Surprise{From: g.Label(e.From), To: g.Label(e.To), Kind: e.Kind, FromComm: g.communityLabel2(g.Nodes[e.From].Community), ToComm: g.communityLabel2(g.Nodes[e.To].Community), Score: e.Score, Conf: e.Conf}
	}
	return out
}

// Report renders a GRAPH_REPORT.md style overview. Output is deterministic.
func (g *SymGraph) Report() string {
	var b strings.Builder
	sym, ext, inf := 0, 0, 0
	for i := range g.Nodes {
		if g.Nodes[i].Kind != KindFile {
			sym++
		}
	}
	for _, e := range g.Edges {
		if e.Conf == ConfExtracted {
			ext++
		} else {
			inf++
		}
	}
	b.WriteString("# Graph Report\n\n")
	fmt.Fprintf(&b, "%d files, %d symbols, %d edges (%d extracted, %d inferred), %d communities.\n\n", len(g.Nodes)-sym, sym, len(g.Edges), ext, inf, len(g.Communities))

	hubs := g.Hubs(10)
	b.WriteString("## Key concepts\n\nThe most connected symbols. Change these with care.\n\n")
	for _, h := range hubs {
		fmt.Fprintf(&b, "- `%s` (%s, %s): %d callers/dependents, %d dependencies\n", h.Name, h.Kind, h.Loc, h.In, h.Out)
	}

	b.WriteString("\n## Communities\n\n| # | Label | Files | Symbols | Top symbols |\n|---|---|---|---|---|\n")
	for i, c := range g.Communities {
		if i == 20 {
			fmt.Fprintf(&b, "| | ... %d more | | | |\n", len(g.Communities)-20)
			break
		}
		fmt.Fprintf(&b, "| %d | %s | %d | %d | %s |\n", c.ID, c.Label, c.Files, c.Nodes, strings.Join(c.Top, ", "))
	}

	sur := g.Surprises(8)
	b.WriteString("\n## Surprising connections\n\nLinks between communities that rarely interact.\n\n")
	if len(sur) == 0 {
		b.WriteString("None found.\n")
	}
	for _, s := range sur {
		c := "E"
		if s.Conf == ConfInferred {
			c = "I"
		}
		fmt.Fprintf(&b, "- `%s` %s `%s` (%s to %s, %s%.2f)\n", s.From, s.Kind, s.To, s.FromComm, s.ToComm, c, s.Score)
	}

	b.WriteString("\n## Suggested questions\n\n")
	for i, h := range hubs {
		if i == 3 {
			break
		}
		fmt.Fprintf(&b, "- What breaks if `%s` changes? `stacklit impact %s`\n", h.Name, h.Name)
	}
	for i, s := range sur {
		if i == 3 {
			break
		}
		fmt.Fprintf(&b, "- Why does `%s` reach `%s`? `stacklit path %s %s`\n", s.From, s.To, s.From, s.To)
	}
	for i, c := range g.Communities {
		if i == 3 {
			break
		}
		fmt.Fprintf(&b, "- What does %s do? `stacklit query \"%s\"`\n", c.Label, c.Label)
	}

	var iso []string
	for i := range g.Nodes {
		if g.Nodes[i].Kind == KindFile && g.degree(i) == 0 {
			iso = append(iso, g.Nodes[i].File)
		}
	}
	b.WriteString("\n## Gaps\n\n")
	if inf > ext {
		b.WriteString("- Most edges are inferred from names, treat low scores as hints.\n")
	}
	if len(iso) > 0 {
		sort.Strings(iso)
		if len(iso) > 8 {
			iso = iso[:8]
		}
		fmt.Fprintf(&b, "- Files with no links (dead code or entry scripts?): %s\n", strings.Join(iso, ", "))
	}
	return b.String()
}
