package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/fsutil"
)

// SchemaVersion is the version tag written into stacklit.graph.json.
const SchemaVersion = "1"

// Edge confidence tags. EXTRACTED edges come straight from syntax or a
// unique resolution; INFERRED edges rely on name matching and carry a score.
const (
	ConfExtracted = "EXTRACTED"
	ConfInferred  = "INFERRED"
)

// Symbol node kinds.
const (
	KindFile      = "file"
	KindFunc      = "func"
	KindMethod    = "method"
	KindType      = "type"
	KindClass     = "class"
	KindInterface = "interface"
	KindEnum      = "enum"
	KindTrait     = "trait"
)

// Symbol edge kinds.
const (
	EdgeCalls      = "calls"
	EdgeImports    = "imports"
	EdgeInherits   = "inherits"
	EdgeImplements = "implements"
)

// SymNode is a file or a symbol definition.
type SymNode struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	File string `json:"file"`
	Line int    `json:"line,omitempty"`
	End  int    `json:"end,omitempty"`
	Doc  string `json:"doc,omitempty"`
	Lang string `json:"lang,omitempty"`
	Sig  string `json:"sig,omitempty"`
	Vis  string `json:"vis,omitempty"`
	// Body holds a capped list of distinct body words used only for search.
	Body      string `json:"body,omitempty"`
	Community int    `json:"community"`
	Repo      string `json:"repo,omitempty"`   // set when graphs from several repos are merged
	Source    string `json:"source,omitempty"` // adapter or index that produced the node, empty for the built-in extractor
}

// SymEdge connects two nodes by index into SymGraph.Nodes.
type SymEdge struct {
	From  int     `json:"f"`
	To    int     `json:"t"`
	Kind  string  `json:"k"`
	Conf  string  `json:"c"`
	Score float64 `json:"s"`
	Src   string  `json:"src,omitempty"` // adapter or index that produced the edge
}

// Community is a cluster of files found by label propagation.
type Community struct {
	ID    int      `json:"id"`
	Label string   `json:"label"`
	Files int      `json:"files"`
	Nodes int      `json:"nodes"`
	Top   []string `json:"top,omitempty"`
}

// SymGraph is the symbol-level graph stored in stacklit.graph.json.
type SymGraph struct {
	Version     string      `json:"version"`
	Root        string      `json:"root,omitempty"`
	Nodes       []SymNode   `json:"nodes"`
	Edges       []SymEdge   `json:"edges"`
	Communities []Community `json:"communities,omitempty"`
	// Commit is the HEAD the graph was built at, so status and query can warn
	// when it has gone stale. No timestamp: the file stays reproducible.
	Commit string `json:"commit,omitempty"`

	out, in  [][]int32
	byName   map[string][]int
	byID     map[string]int
	nameKeys []string
	pr       []float64
	sx       *searchIndex
}

// Index builds adjacency lists and name lookups. It is safe to call twice.
func (g *SymGraph) Index() {
	n := len(g.Nodes)
	g.out = make([][]int32, n)
	g.in = make([][]int32, n)
	g.byName = make(map[string][]int, n)
	g.byID = make(map[string]int, n)
	for i := range g.Nodes {
		nd := &g.Nodes[i]
		g.byID[nd.ID] = i
		if nd.Kind != KindFile {
			g.byName[nd.Name] = append(g.byName[nd.Name], i)
		}
	}
	for i, e := range g.Edges {
		g.out[e.From] = append(g.out[e.From], int32(i))
		g.in[e.To] = append(g.in[e.To], int32(i))
	}
	g.pr, g.sx = nil, nil
	g.nameKeys = g.nameKeys[:0]
	for k := range g.byName {
		g.nameKeys = append(g.nameKeys, k)
	}
	sort.Strings(g.nameKeys)
}

// Load reads a graph file and indexes it.
func Load(path string) (*SymGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var g SymGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, e := range g.Edges {
		if e.From < 0 || e.To < 0 || e.From >= len(g.Nodes) || e.To >= len(g.Nodes) {
			return nil, fmt.Errorf("parse %s: edge index out of range", path)
		}
	}
	g.Index()
	return &g, nil
}

// Save writes the graph as compact JSON.
func (g *SymGraph) Save(path string) error {
	data, err := json.Marshal(g)
	if err != nil {
		return err
	}
	// Atomic so a concurrent reader never sees a half-written graph.
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// Label is a short unambiguous display name for a node.
func (g *SymGraph) Label(i int) string {
	n := &g.Nodes[i]
	if n.Kind == KindFile {
		return n.File
	}
	if len(g.byName[n.Name]) > 1 {
		return n.ID
	}
	return n.Name
}

// Loc is "file:line" for a node.
func (g *SymGraph) Loc(i int) string {
	n := &g.Nodes[i]
	if n.Line > 0 {
		return fmt.Sprintf("%s:%d", n.File, n.Line)
	}
	return n.File
}

// Find resolves a user-supplied symbol or file reference to node indexes,
// best match first: exact ID, exact name, case-insensitive name, ID suffix,
// file path, substring.
func (g *SymGraph) Find(q string) []int {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	if i, ok := g.byID[q]; ok {
		return []int{i}
	}
	if m := g.byName[q]; len(m) > 0 {
		return g.rankByDegree(append([]int(nil), m...))
	}
	lq := strings.ToLower(q)
	var res []int
	for _, k := range g.nameKeys {
		if strings.ToLower(k) == lq {
			res = append(res, g.byName[k]...)
		}
	}
	if len(res) > 0 {
		return g.rankByDegree(res)
	}
	for i := range g.Nodes {
		if g.Nodes[i].Kind != KindFile && strings.HasSuffix(g.Nodes[i].ID, "::"+q) {
			res = append(res, i)
		}
	}
	if len(res) > 0 {
		return g.rankByDegree(res)
	}
	for i := range g.Nodes {
		if g.Nodes[i].Kind == KindFile && (g.Nodes[i].File == q || strings.HasSuffix(g.Nodes[i].File, "/"+q)) {
			res = append(res, i)
		}
	}
	if len(res) > 0 {
		return res
	}
	for _, k := range g.nameKeys {
		if strings.Contains(strings.ToLower(k), lq) {
			res = append(res, g.byName[k]...)
		}
	}
	return g.rankByDegree(res)
}

func (g *SymGraph) degree(i int) int { return len(g.out[i]) + len(g.in[i]) }

func (g *SymGraph) rankByDegree(ids []int) []int {
	sort.SliceStable(ids, func(a, b int) bool {
		da, db := g.degree(ids[a]), g.degree(ids[b])
		if da != db {
			return da > db
		}
		return ids[a] < ids[b]
	})
	return ids
}

// NodeRef is the compact node form used in command results.
type NodeRef struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Loc       string `json:"loc"`
	Community int    `json:"community"`
}

// EdgeRef is the compact edge form used in command results.
type EdgeRef struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Kind  string  `json:"kind"`
	Conf  string  `json:"conf"`
	Score float64 `json:"score"`
}

func (g *SymGraph) ref(i int) NodeRef {
	n := &g.Nodes[i]
	return NodeRef{Name: g.Label(i), Kind: n.Kind, Loc: g.Loc(i), Community: n.Community}
}

func (g *SymGraph) edgeRef(e SymEdge) EdgeRef {
	return EdgeRef{From: g.Label(e.From), To: g.Label(e.To), Kind: e.Kind, Conf: e.Conf, Score: e.Score}
}

// tag renders a confidence as E1.00 or I0.55 for compact text output.
func tag(e SymEdge) string {
	c := "E"
	if e.Conf == ConfInferred {
		c = "I"
	}
	return fmt.Sprintf("%s%.2f", c, e.Score)
}

// approxTokens estimates token count at four characters per token.
func approxTokens(s string) int { return (len(s) + 3) / 4 }
