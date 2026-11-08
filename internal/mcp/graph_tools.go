package mcp

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/walker"
)

const graphFileName = "stacklit.graph.json"

var (
	graphMu      sync.Mutex
	cachedGraph  *graph.SymGraph
	cachedGraphT time.Time
	cachedBuilt  time.Time
)

// loadGraph reads stacklit.graph.json when present and fresh, else builds in memory
// and reuses that build for a short while so repeated tool calls stay cheap.
func loadGraph() (*graph.SymGraph, error) {
	graphMu.Lock()
	defer graphMu.Unlock()
	if st, err := os.Stat(graphFileName); err == nil {
		if cachedGraph != nil && !cachedGraphT.IsZero() && !st.ModTime().After(cachedGraphT) {
			return cachedGraph, nil
		}
		g, err := graph.Load(graphFileName)
		if err != nil {
			return nil, err
		}
		cachedGraph, cachedGraphT = g, st.ModTime()
		return g, nil
	}
	if cachedGraph != nil && cachedGraphT.IsZero() && time.Since(cachedBuilt) < 30*time.Second {
		return cachedGraph, nil
	}
	cfg := config.Load(".")
	files, err := walker.Walk(".", cfg.ScanIgnore())
	if err != nil {
		return nil, err
	}
	cachedGraph = graph.BuildSymbols(".", files, graph.ExtractOptions{})
	cachedGraphT, cachedBuilt = time.Time{}, time.Now()
	return cachedGraph, nil
}

type graphArgs struct {
	Query  string `json:"query,omitempty" jsonschema:"question or keywords"`
	Symbol string `json:"symbol,omitempty" jsonschema:"symbol name, Type.Method, or file path"`
	From   string `json:"from,omitempty" jsonschema:"start symbol"`
	To     string `json:"to,omitempty" jsonschema:"end symbol"`
	Budget int    `json:"budget,omitempty" jsonschema:"hard output limit in tokens"`
	Depth  int    `json:"depth,omitempty" jsonschema:"maximum dependency depth for impact"`
	Top    int    `json:"top,omitempty" jsonschema:"number of hubs to return"`
	JSON   bool   `json:"json,omitempty" jsonschema:"return JSON instead of compact text"`
}

func plain(s string) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: s}}}, nil, nil
}

type graphHandler func(g *graph.SymGraph, a graphArgs) (string, error)

func registerGraphTools(s *sdkmcp.Server) {
	add := func(name, desc string, h graphHandler) {
		sdkmcp.AddTool(s, &sdkmcp.Tool{Name: name, Description: desc},
			func(ctx context.Context, req *sdkmcp.CallToolRequest, a graphArgs) (*sdkmcp.CallToolResult, any, error) {
				g, err := loadGraph()
				if err != nil {
					return errResult(err)
				}
				out, err := h(g, a)
				if err != nil {
					return errResult(err)
				}
				return plain(out)
			})
	}
	add("query_graph", "Scoped symbol subgraph for a question, file-grouped, hard token budget (default 2000)",
		func(g *graph.SymGraph, a graphArgs) (string, error) {
			if a.Query == "" {
				return "", fmt.Errorf("query is required")
			}
			return g.Query(a.Query, a.Budget, a.JSON), nil
		})
	add("shortest_path", "Shortest path between two symbols with weakest-hop confidence",
		func(g *graph.SymGraph, a graphArgs) (string, error) {
			p, err := g.ShortestPath(a.From, a.To)
			if err != nil {
				return "", err
			}
			return p.Render(a.JSON), nil
		})
	add("explain_symbol", "Definition, file:line range, signature, callers and callees of a symbol",
		func(g *graph.SymGraph, a graphArgs) (string, error) { return g.Explain(a.Symbol, a.Budget, a.JSON) })
	add("impact", "Blast radius: symbols and files that depend on a symbol or file",
		func(g *graph.SymGraph, a graphArgs) (string, error) {
			return g.Impact(a.Symbol, a.Depth, a.Budget, a.JSON)
		})
	add("get_hubs", "Most connected symbols (god nodes)",
		func(g *graph.SymGraph, a graphArgs) (string, error) {
			return graph.RenderHubs(g.Hubs(a.Top), a.JSON), nil
		})
	add("get_communities", "Deterministic clusters of files with labels and top symbols",
		func(g *graph.SymGraph, a graphArgs) (string, error) { return g.RenderCommunities(a.JSON), nil })
	add("graph_report", "Graph overview: key concepts, surprising cross-community links, suggested questions",
		func(g *graph.SymGraph, a graphArgs) (string, error) { return g.Report(), nil })
	add("slice_symbol", "Return only one symbol's source lines (no whole-file reads)",
		func(g *graph.SymGraph, a graphArgs) (string, error) { return g.Slice(".", a.Symbol, a.Budget) })
}
