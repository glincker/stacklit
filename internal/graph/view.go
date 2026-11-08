package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

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
}

// FileGroup holds the shown symbols of one file.
type FileGroup struct {
	File    string    `json:"file"`
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
}

// item is a ranked node with relation tags (caller, depth, edge confidence).
type item struct {
	node int
	tags []string
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
}

// buildView groups the first k items by file, in rank order of first appearance
// sorted by file path so output is stable between runs.
func (g *SymGraph) buildView(sp viewSpec, k int) *View {
	v := &View{Title: sp.title, Head: sp.head, Total: len(sp.items), Shown: k}
	sel := map[int]bool{}
	for _, it := range sp.items[:k] {
		sel[it.node] = true
	}
	byFile := map[string][]SymLine{}
	for _, it := range sp.items[:k] {
		n := &g.Nodes[it.node]
		l := SymLine{Name: n.Name, Kind: n.Kind, Start: n.Line, End: n.End, Sig: n.Sig, Doc: n.Doc, Tags: append([]string(nil), it.tags...)}
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
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	for _, f := range files {
		ls := byFile[f]
		sort.SliceStable(ls, func(a, b int) bool { return ls[a].Start < ls[b].Start })
		v.Files = append(v.Files, FileGroup{File: f, Symbols: ls})
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
