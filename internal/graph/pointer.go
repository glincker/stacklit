package graph

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultPointerTokens caps a hook pointer; STACKLIT_HOOK_TOKENS overrides it.
const DefaultPointerTokens = 120

// Pointer answers "where is this identifier defined and who calls it" in one
// short line, or "" when the graph has no exact definition for it.
func (g *SymGraph) Pointer(ident string) string {
	hits := g.identHits(ident)
	if len(hits) == 0 || hits[0].Score < 3.5 {
		return ""
	}
	id := hits[0].Node
	n := &g.Nodes[id]
	var b strings.Builder
	fmt.Fprintf(&b, "stacklit: %s defined at %s", n.Name, g.Loc(id))
	if n.Sig != "" {
		sig := n.Sig
		if len(sig) > 70 {
			sig = sig[:70] + "..."
		}
		fmt.Fprintf(&b, " (%s)", sig)
	}
	extra := 0
	for _, h := range hits[1:] {
		if h.Score >= 3.5 {
			extra++
		}
	}
	if extra > 0 {
		fmt.Fprintf(&b, ", +%d more definitions", extra)
	}
	type caller struct {
		id int
		s  float64
	}
	var cs []caller
	seen := map[int]bool{}
	for _, ei := range g.in[id] {
		e := g.Edges[ei]
		if e.Kind != EdgeCalls || e.From == id || seen[e.From] || g.Nodes[e.From].Kind == KindFile {
			continue
		}
		seen[e.From] = true
		cs = append(cs, caller{e.From, e.Score})
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].s != cs[j].s {
			return cs[i].s > cs[j].s
		}
		return g.tieLess(cs[i].id, cs[j].id)
	})
	if len(cs) > 0 {
		b.WriteString("; callers: ")
		show := envInt("STACKLIT_HOOK_CALLERS", 3)
		for i, c := range cs {
			if i == show {
				fmt.Fprintf(&b, ", +%d more", len(cs)-show)
				break
			}
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s %s", g.Nodes[c.id].Name, g.Loc(c.id))
		}
	}
	b.WriteString(". Use stacklit explain/slice instead of reading files.")
	budget := envInt("STACKLIT_HOOK_TOKENS", DefaultPointerTokens)
	out := b.String()
	if approxTokens(out) > budget {
		out = clampTokens(out, budget)
	}
	return out
}

var declWords = wordSet("func function def fn class type struct interface enum trait const var let pub impl")

// PatternIdent extracts the identifier a grep pattern is looking for, or ""
// when the pattern is prose, a real regex or too short to be worth a lookup.
func PatternIdent(p string) string {
	p = strings.TrimSpace(p)
	p = strings.NewReplacer(`\b`, "", `\(`, "", `\.`, ".", "^", "", "$", "").Replace(p)
	p = strings.TrimSpace(strings.TrimRight(p, "("))
	if f := strings.Fields(p); len(f) == 2 && declWords[strings.ToLower(f[0])] {
		p = f[1]
	}
	if len(p) < envInt("STACKLIT_HOOK_MIN_IDENT", 3) || !IsIdentifierQuery(p) {
		return ""
	}
	return cleanIdent(p)
}
