package graph

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Hop is one step along a path; Forward is true when the edge points along the path.
type Hop struct {
	From    string  `json:"from"`
	To      string  `json:"to"`
	Kind    string  `json:"kind"`
	Forward bool    `json:"forward"`
	Conf    string  `json:"conf"`
	Score   float64 `json:"score"`
}

// PathResult is the shortest path between two symbols.
type PathResult struct {
	From    string  `json:"from"`
	To      string  `json:"to"`
	Found   bool    `json:"found"`
	Hops    []Hop   `json:"hops,omitempty"`
	Weakest float64 `json:"weakest,omitempty"`
	Note    string  `json:"note,omitempty"`
}

type pqItem struct {
	node int
	dist float64
}
type pq []pqItem

func (p pq) Len() int { return len(p) }
func (p pq) Less(i, j int) bool {
	return p[i].dist < p[j].dist || p[i].dist == p[j].dist && p[i].node < p[j].node
}
func (p pq) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p *pq) Push(x any)   { *p = append(*p, x.(pqItem)) }
func (p *pq) Pop() any {
	o := *p
	n := len(o)
	x := o[n-1]
	*p = o[:n-1]
	return x
}

// ShortestPath runs Dijkstra over both edge directions. Each hop costs 1 plus
// -ln(score), so a short path of confident edges beats a short path of guesses.
func (g *SymGraph) ShortestPath(a, b string) (*PathResult, error) {
	as, bs := g.Find(a), g.Find(b)
	if len(as) == 0 {
		return nil, fmt.Errorf("no symbol matches %q", a)
	}
	if len(bs) == 0 {
		return nil, fmt.Errorf("no symbol matches %q", b)
	}
	src, dst := as[0], bs[0]
	res := &PathResult{From: g.Label(src), To: g.Label(dst)}
	if len(as) > 1 || len(bs) > 1 {
		res.Note = fmt.Sprintf("ambiguous names resolved to the most connected match (%d and %d candidates)", len(as), len(bs))
	}
	dist := make([]float64, len(g.Nodes))
	prev := make([]int, len(g.Nodes))
	prevEdge := make([]int, len(g.Nodes))
	for i := range dist {
		dist[i] = math.Inf(1)
		prev[i] = -1
	}
	dist[src] = 0
	h := &pq{{src, 0}}
	for h.Len() > 0 {
		cur := heap.Pop(h).(pqItem)
		if cur.dist > dist[cur.node] {
			continue
		}
		if cur.node == dst {
			break
		}
		for _, list := range [][]int32{g.out[cur.node], g.in[cur.node]} {
			for _, ei := range list {
				e := g.Edges[ei]
				o := e.To
				if o == cur.node {
					o = e.From
				}
				d := cur.dist + 1 - math.Log(math.Max(e.Score, 0.01))
				if d < dist[o] {
					dist[o], prev[o], prevEdge[o] = d, cur.node, int(ei)
					heap.Push(h, pqItem{o, d})
				}
			}
		}
	}
	if math.IsInf(dist[dst], 1) {
		res.Note = strings.TrimSpace(res.Note + " no path found")
		return res, nil
	}
	res.Found = true
	res.Weakest = 1
	var rev []Hop
	for v := dst; v != src; v = prev[v] {
		e := g.Edges[prevEdge[v]]
		fwd := e.From == prev[v]
		rev = append(rev, Hop{From: g.Label(prev[v]), To: g.Label(v), Kind: e.Kind, Forward: fwd, Conf: e.Conf, Score: e.Score})
		if e.Score < res.Weakest {
			res.Weakest = e.Score
		}
	}
	for i := len(rev) - 1; i >= 0; i-- {
		res.Hops = append(res.Hops, rev[i])
	}
	return res, nil
}

// Render prints the path as text or JSON.
func (p *PathResult) Render(asJSON bool) string {
	if asJSON {
		data, _ := json.MarshalIndent(p, "", " ")
		return string(data) + "\n"
	}
	var b strings.Builder
	if !p.Found {
		fmt.Fprintf(&b, "path %s -> %s: %s\n", p.From, p.To, p.Note)
		return b.String()
	}
	fmt.Fprintf(&b, "path %s -> %s: %d hops, weakest hop %.2f (%s)\n", p.From, p.To, len(p.Hops), p.Weakest, pathTrust(p.Weakest))
	if p.Note != "" {
		b.WriteString(p.Note + "\n")
	}
	for _, h := range p.Hops {
		c := "E"
		if h.Conf == ConfInferred {
			c = "I"
		}
		if h.Forward {
			fmt.Fprintf(&b, "  %s -%s-> %s  %s%.2f\n", h.From, h.Kind, h.To, c, h.Score)
		} else {
			fmt.Fprintf(&b, "  %s <-%s- %s  %s%.2f\n", h.From, h.Kind, h.To, c, h.Score)
		}
	}
	return b.String()
}

// pathTrust turns the weakest hop score into advice on whether to confirm.
func pathTrust(weakest float64) string {
	if weakest >= 0.9 {
		return "sure, no confirming grep needed"
	}
	return "likely, grep to confirm the weak hop"
}
