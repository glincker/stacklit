package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Slice returns only the source lines of one symbol, with line numbers, so an
// agent never has to read the whole file. budget (tokens) is a hard cap.
func (g *SymGraph) Slice(root, sym string, budget int) (string, error) {
	ms := g.Find(sym)
	if len(ms) == 0 {
		return "", fmt.Errorf("no symbol matches %q", sym)
	}
	n := &g.Nodes[ms[0]]
	if n.Kind == KindFile || n.Line == 0 {
		return "", fmt.Errorf("%q is a file, slice needs a symbol", sym)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.File)))
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	end := n.End
	if end < n.Line || end > len(lines) {
		end = n.Line
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", g.Label(ms[0]), locRange(n))
	if len(ms) > 1 {
		fmt.Fprintf(&b, "(%d other matches, pass the full id to pick one)\n", len(ms)-1)
	}
	for i := n.Line; i <= end; i++ {
		fmt.Fprintf(&b, "%d\t%s\n", i, strings.TrimRight(lines[i-1], "\r"))
	}
	out := b.String()
	if budget > 0 && approxTokens(out) > budget {
		notice := fmt.Sprintf("[truncated to %d tokens; raise it with: stacklit slice %s --budget %d]\n", budget, sym, budget*2)
		out = clampTokens(out, budget-approxTokens(notice)-1) + notice
	}
	return out, nil
}
