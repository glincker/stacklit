// Package export renders the module graph and the symbol graph into formats
// other tools read: GraphML, Cypher, JSON, DOT, Mermaid, an Obsidian vault
// and a token-budgeted markdown wiki. Output is deterministic.
package export

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
)

// Formats lists every supported format name.
var Formats = []string{"graphml", "cypher", "json", "dot", "mermaid", "obsidian", "markdown-wiki"}

// Scope picks which graph a single-file format renders.
const (
	ScopeModules = "modules"
	ScopeSymbols = "symbols"
)

// Input is everything a renderer may use. Graph may be nil.
type Input struct {
	Index *schema.Index
	Graph *graph.SymGraph
}

// Options tunes rendering.
type Options struct {
	Scope      string // ScopeModules or ScopeSymbols; empty picks symbols when a graph is present
	PageTokens int    // markdown-wiki per page budget in tokens; 0 reads STACKLIT_EXPORT_PAGE_TOKENS
	MaxNodes   int    // dot and mermaid node cap; 0 reads STACKLIT_EXPORT_MAX_NODES
}

// File is one rendered output file with a slash-separated relative name.
type File struct {
	Name    string
	Content string
}

// Node is the shared node shape for all graph formats.
type Node struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	File  string `json:"source_file,omitempty"`
	Line  int    `json:"line,omitempty"`
	Group string `json:"group,omitempty"`
}

// Edge is the shared edge shape for all graph formats.
type Edge struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Kind  string  `json:"kind"`
	Conf  string  `json:"confidence,omitempty"`
	Score float64 `json:"score,omitempty"`
}

// Model is a flattened graph.
type Model struct {
	Scope string `json:"scope"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func (o Options) pageTokens() int {
	if o.PageTokens > 0 {
		return o.PageTokens
	}
	return envInt("STACKLIT_EXPORT_PAGE_TOKENS", 2000)
}

func (o Options) maxNodes() int {
	if o.MaxNodes > 0 {
		return o.MaxNodes
	}
	return envInt("STACKLIT_EXPORT_MAX_NODES", 300)
}

// moduleOf maps a file to the nearest module directory in idx, or "root".
func moduleOf(idx *schema.Index, file string) string {
	if idx == nil {
		return ""
	}
	dir := path.Dir(file)
	for dir != "." && dir != "/" && dir != "" {
		if _, ok := idx.Modules[dir]; ok {
			return dir
		}
		dir = path.Dir(dir)
	}
	if _, ok := idx.Modules["root"]; ok {
		return "root"
	}
	return ""
}

func sortedModules(idx *schema.Index) []string {
	names := make([]string, 0, len(idx.Modules))
	for n := range idx.Modules {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// moduleModel builds the module dependency graph from the index.
func moduleModel(idx *schema.Index) *Model {
	m := &Model{Scope: ScopeModules}
	for _, name := range sortedModules(idx) {
		mi := idx.Modules[name]
		m.Nodes = append(m.Nodes, Node{ID: name, Label: name, Kind: "module", File: name, Group: mi.Language})
	}
	seen := map[[2]string]bool{}
	var edges [][2]string
	add := func(from, to string) {
		k := [2]string{from, to}
		if from == to || seen[k] {
			return
		}
		if _, ok := idx.Modules[from]; !ok {
			return
		}
		if _, ok := idx.Modules[to]; !ok {
			return
		}
		seen[k] = true
		edges = append(edges, k)
	}
	for _, e := range idx.Dependencies.Edges {
		add(e[0], e[1])
	}
	for _, name := range sortedModules(idx) {
		for _, d := range idx.Modules[name].DependsOn {
			add(name, d)
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}
		return edges[i][1] < edges[j][1]
	})
	for _, e := range edges {
		m.Edges = append(m.Edges, Edge{From: e[0], To: e[1], Kind: "depends_on", Conf: graph.ConfExtracted, Score: 1})
	}
	return m
}

// symbolModel flattens the symbol graph. Order follows the graph so it stays stable.
func symbolModel(g *graph.SymGraph, idx *schema.Index) *Model {
	m := &Model{Scope: ScopeSymbols}
	ids := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		ids[i] = n.ID
		grp := moduleOf(idx, n.File)
		m.Nodes = append(m.Nodes, Node{ID: n.ID, Label: n.Name, Kind: n.Kind, File: n.File, Line: n.Line, Group: grp})
	}
	for _, e := range g.Edges {
		m.Edges = append(m.Edges, Edge{From: ids[e.From], To: ids[e.To], Kind: e.Kind, Conf: e.Conf, Score: e.Score})
	}
	return m
}

func (in Input) model(o Options) (*Model, error) {
	scope := o.Scope
	if scope == "" {
		scope = ScopeModules
		if in.Graph != nil {
			scope = ScopeSymbols
		}
	}
	switch scope {
	case ScopeModules:
		if in.Index == nil {
			return nil, fmt.Errorf("module scope needs stacklit.json")
		}
		return moduleModel(in.Index), nil
	case ScopeSymbols:
		if in.Graph == nil {
			return nil, fmt.Errorf("symbol scope needs the symbol graph (run 'stacklit graph')")
		}
		return symbolModel(in.Graph, in.Index), nil
	}
	return nil, fmt.Errorf("unknown scope %q (modules, symbols)", scope)
}

// capped keeps the nodes with the highest degree when m exceeds max.
func (m *Model) capped(max int) *Model {
	if len(m.Nodes) <= max {
		return m
	}
	deg := map[string]int{}
	for _, e := range m.Edges {
		deg[e.From]++
		deg[e.To]++
	}
	order := make([]int, len(m.Nodes))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		da, db := deg[m.Nodes[order[a]].ID], deg[m.Nodes[order[b]].ID]
		if da != db {
			return da > db
		}
		return order[a] < order[b]
	})
	keep := map[string]bool{}
	for _, i := range order[:max] {
		keep[m.Nodes[i].ID] = true
	}
	out := &Model{Scope: m.Scope}
	for _, n := range m.Nodes {
		if keep[n.ID] {
			out.Nodes = append(out.Nodes, n)
		}
	}
	for _, e := range m.Edges {
		if keep[e.From] && keep[e.To] {
			out.Edges = append(out.Edges, e)
		}
	}
	return out
}

func approxTokens(s string) int { return (len(s) + 3) / 4 }

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
