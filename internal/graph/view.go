package graph

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
)

// Order says how a view lists its files.
type Order string

// Query output orders. Relevance is the default: an agent reads top to bottom,
// so the best file must come first.
const (
	OrderRelevance Order = "relevance"
	OrderPath      Order = "path"
)

// ParseOrder validates a --order value; empty means the environment default.
func ParseOrder(s string) (Order, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return DefaultOrder(), nil
	case "relevance":
		return OrderRelevance, nil
	case "path":
		return OrderPath, nil
	}
	return "", fmt.Errorf("invalid order %q: want relevance or path", s)
}

// DefaultOrder reads STACKLIT_QUERY_ORDER; anything unrecognised means relevance.
func DefaultOrder() Order {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("STACKLIT_QUERY_ORDER")), "path") {
		return OrderPath
	}
	return OrderRelevance
}

// round3 keeps printed scores short and byte-stable.
func round3(x float64) float64 { return math.Round(x*1000) / 1000 }

// Default hard token budgets per command.
const (
	DefaultQueryBudget   = 2000
	DefaultExplainBudget = 1500
	DefaultImpactBudget  = 1500
)

// SymLine is one symbol in a file-grouped view.
type SymLine struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`
	Start int      `json:"start"`
	End   int      `json:"end"`
	Sig   string   `json:"sig,omitempty"`
	Doc   string   `json:"doc,omitempty"`
	Tags  []string `json:"tags,omitempty"`
	Score float64  `json:"score,omitempty"`
}

// FileGroup holds the shown symbols of one file.
type FileGroup struct {
	File    string    `json:"file"`
	Rank    int       `json:"rank,omitempty"`
	Score   float64   `json:"score,omitempty"`
	Symbols []SymLine `json:"symbols"`
}

// View is the compact, file-grouped result shared by query, explain and impact.
type View struct {
	Title   string      `json:"title"`
	Head    []string    `json:"head,omitempty"`
	Files   []FileGroup `json:"files"`
	Total   int         `json:"total"`
	Shown   int         `json:"shown"`
	Omitted int         `json:"omitted,omitempty"`
	Notice  string      `json:"notice,omitempty"`
	ranked  bool
}

// item is a ranked node with relation tags (caller, depth, edge confidence).
type item struct {
	node  int
	tags  []string
	score float64 // relevance; zero for views that do not rank
}

// viewSpec describes how to build a view from ranked items.
type viewSpec struct {
	title string
	head  []string
	items []item
	// edges adds "-> kind target conf" tags between shown symbols (query only).
	edges bool
	// hint is the command prefix reused in the truncation notice.
	hint string
	// ranked marks items as scored and in rank order (query only).
	ranked bool
	order  Order
}

// buildView groups the first k items by file. Ranked views list files by their
// best symbol score (ties by path) unless path order is asked for; all other
// views sort by path. Both are stable between runs.
func (g *SymGraph) buildView(sp viewSpec, k int) *View {
	v := &View{Title: sp.title, Head: sp.head, Files: []FileGroup{}, Total: len(sp.items), Shown: k, ranked: sp.ranked && sp.order != OrderPath}
	sel := map[int]bool{}
	for _, it := range sp.items[:k] {
		sel[it.node] = true
	}
	byFile := map[string][]SymLine{}
	for _, it := range sp.items[:k] {
		n := &g.Nodes[it.node]
		l := SymLine{Name: n.Name, Kind: n.Kind, Start: n.Line, End: n.End, Sig: n.Sig, Doc: n.Doc, Tags: append([]string(nil), it.tags...)}
		if sp.ranked {
			l.Score = round3(it.score)
		}
		if l.End < l.Start {
			l.End = l.Start
		}
		if sp.edges {
			type te struct {
				s string
				w float64
			}
			var es []te
			for _, ei := range g.out[it.node] {
				e := g.Edges[ei]
				if sel[e.To] && e.Kind != EdgeImports {
					es = append(es, te{fmt.Sprintf("%s %s %s", e.Kind, g.Label(e.To), tag(e)), e.Score})
				}
			}
			sort.SliceStable(es, func(a, b int) bool { return es[a].w > es[b].w || es[a].w == es[b].w && es[a].s < es[b].s })
			for i, e := range es {
				if i == 6 {
					break
				}
				l.Tags = append(l.Tags, e.s)
			}
		}
		byFile[n.File] = append(byFile[n.File], l)
	}
	files := make([]string, 0, len(byFile))
	best := map[string]float64{}
	for f, ls := range byFile {
		files = append(files, f)
		for _, l := range ls {
			if l.Score > best[f] {
				best[f] = l.Score
			}
		}
	}
	sort.Strings(files)
	byScore := sp.ranked && sp.order != OrderPath
	rankOf := map[string]int{}
	if sp.ranked {
		// Rank is by relevance even when files print in path order.
		byRel := append([]string(nil), files...)
		sort.SliceStable(byRel, func(a, b int) bool { return best[byRel[a]] > best[byRel[b]] })
		for i, f := range byRel {
			rankOf[f] = i + 1
		}
		if byScore {
			files = byRel
		}
	}
	for _, f := range files {
		ls := byFile[f]
		if byScore {
			sort.SliceStable(ls, func(a, b int) bool {
				if ls[a].Score != ls[b].Score {
					return ls[a].Score > ls[b].Score
				}
				return ls[a].Start < ls[b].Start
			})
		} else {
			sort.SliceStable(ls, func(a, b int) bool { return ls[a].Start < ls[b].Start })
		}
		v.Files = append(v.Files, FileGroup{File: f, Rank: rankOf[f], Score: best[f], Symbols: ls})
	}
	if byScore && len(v.Files) > 0 && len(v.Files[0].Symbols) > 0 {
		top := v.Files[0]
		v.Head = append(append([]string(nil), v.Head...), fmt.Sprintf("best match: %s:%d %s", top.File, top.Symbols[0].Start, top.Symbols[0].Name))
	}
	v.Omitted = v.Total - k
	if v.Omitted > 0 {
		v.Notice = fmt.Sprintf("[truncated: %d of %d omitted, lowest ranked first; raise it with: %s]", v.Omitted, v.Total, sp.hint)
	}
	return v
}

// Text renders the view as indented, file-grouped lines.
func (v *View) Text() string {
	var b strings.Builder
	b.WriteString(v.Title)
	b.WriteByte('\n')
	for _, h := range v.Head {
		b.WriteString(h)
		b.WriteByte('\n')
	}
	for _, f := range v.Files {
		if v.ranked {
			fmt.Fprintf(&b, "%d. ", f.Rank)
		}
		b.WriteString(f.File)
		b.WriteByte('\n')
		for _, s := range f.Symbols {
			rng := fmt.Sprintf("%d", s.Start)
			if s.End > s.Start {
				rng = fmt.Sprintf("%d-%d", s.Start, s.End)
			}
			label := s.Sig
			if label == "" {
				label = s.Kind + " " + s.Name
			}
			line := "  " + rng + " " + label
			if s.Doc != "" {
				line += "  // " + s.Doc
			}
			b.WriteString(line)
			b.WriteByte('\n')
			for _, t := range s.Tags {
				b.WriteString("      " + t + "\n")
			}
		}
	}
	if v.Notice != "" {
		b.WriteString(v.Notice)
		b.WriteByte('\n')
	}
	return b.String()
}

// JSON renders the view as indented JSON.
func (v *View) JSON() string {
	data, _ := json.MarshalIndent(v, "", " ")
	return string(data) + "\n"
}

// Render returns the view as text or JSON.
func (v *View) Render(asJSON bool) string {
	if asJSON {
		return v.JSON()
	}
	return v.Text()
}

// FitView builds the largest prefix of ranked items whose rendering stays within
// budget tokens, then returns it. The result never exceeds budget: lowest-ranked
// items go first, and if even the head does not fit it is cut by bytes.
func (g *SymGraph) FitView(sp viewSpec, budget int, asJSON bool) string {
	if budget <= 0 {
		budget = DefaultQueryBudget
	}
	fits := func(k int) (string, bool) {
		s := g.buildView(sp, k).Render(asJSON)
		return s, approxTokens(s) <= budget
	}
	if s, ok := fits(len(sp.items)); ok {
		return s
	}
	lo, hi := 0, len(sp.items)-1 // largest k in [lo,hi] that fits, lo may not fit
	best, bestOK := "", false
	for lo <= hi {
		mid := (lo + hi) / 2
		if s, ok := fits(mid); ok {
			best, bestOK = s, true
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if bestOK {
		return best
	}
	s := g.buildView(sp, 0).Render(asJSON)
	return clampTokens(s, budget)
}

// ClampTokens is clampTokens for callers outside the package.
func ClampTokens(s string, budget int) string { return clampTokens(s, budget) }

// clampTokens cuts s to at most budget tokens on a line boundary when possible.
func clampTokens(s string, budget int) string {
	max := budget * 4
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexByte(cut, '\n'); i > 0 {
		cut = cut[:i+1]
	}
	return cut
}
