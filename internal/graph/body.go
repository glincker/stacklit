package graph

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// codeNoise lists language keywords that carry no retrieval signal and would
// otherwise eat the capped body-term budget.
var codeNoise = wordSet("func function def fn pub let var const return if else for while switch case break continue nil null none true false err error self this new struct interface class import package public private static void int string bool byte range defer go chan map type impl use mod match async await try catch throw throws extends implements final")

func envFloat(key string, def float64) float64 {
	if f, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil && f > 0 {
		return f
	}
	return def
}

// bodyTermCap is how many distinct body words each symbol keeps;
// STACKLIT_BODY_TERMS overrides it (it must be positive).
func bodyTermCap() int { return envInt("STACKLIT_BODY_TERMS", 48) }

// BodyCache keeps computed body words per file, keyed by "line:name" within the
// file, so an incremental rebuild reads only the files that changed.
type BodyCache map[string]map[string]string

func bodyKey(n *SymNode) string { return strconv.Itoa(n.Line) + ":" + n.Name }

// attachBodies stores a capped, deduplicated word list from each symbol's body
// so search can match what code does, not only what it is called.
func (g *SymGraph) attachBodies(root string) { g.AttachBodiesCached(root, nil) }

// AttachBodiesCached is attachBodies that reuses and fills c. Callers drop a
// file's entry from c when the file changes. A nil c disables caching.
func (g *SymGraph) AttachBodiesCached(root string, c BodyCache) {
	limit := bodyTermCap()
	byFile := map[string][]int{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind != KindFile && n.Line > 0 {
			byFile[n.File] = append(byFile[n.File], i)
		}
	}
	for file, ids := range byFile {
		if hit, ok := c[file]; ok {
			for _, id := range ids {
				g.Nodes[id].Body = hit[bodyKey(&g.Nodes[id])]
			}
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		var fresh map[string]string
		if c != nil {
			fresh = make(map[string]string, len(ids))
		}
		for _, id := range ids {
			n := &g.Nodes[id]
			if n.Line < 1 || n.Line > len(lines) {
				continue
			}
			n.Body = bodyTerms(n, lines, limit)
			if fresh != nil && n.Body != "" {
				fresh[bodyKey(n)] = n.Body
			}
		}
		if c != nil {
			c[file] = fresh
		}
	}
}

func bodyTerms(n *SymNode, lines []string, limit int) string {
	end := n.End
	if end < n.Line || end > len(lines) {
		end = n.Line
	}
	own := map[string]bool{}
	for _, w := range words(n.Name) {
		own[w] = true
	}
	seen := map[string]bool{}
	var terms []string
scan:
	for _, ln := range lines[n.Line-1 : end] {
		for _, w := range words(ln) {
			if len(w) < 3 || len(w) > 24 || codeNoise[w] || own[w] || seen[w] {
				continue
			}
			seen[w] = true
			terms = append(terms, w)
			if len(terms) >= limit {
				break scan
			}
		}
	}
	return strings.Join(terms, " ")
}
