package mcp

import (
	"context"
	"fmt"
	"os"
	"strconv"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glincker/stacklit/internal/graph"
)

// Tool profiles: full exposes every tool, minimal four with tiny schemas.
const (
	ProfileFull    = "full"
	ProfileMinimal = "minimal"
)

// Minimal-profile argument structs keep the tool schemas small on purpose;
// a test caps their combined size.
type minQueryArgs struct {
	Q      string `json:"q" jsonschema:"question or identifier"`
	Budget int    `json:"budget,omitempty"`
}

type minSymArgs struct {
	Symbol string `json:"symbol" jsonschema:"symbol or file"`
	Budget int    `json:"budget,omitempty"`
}

type minMapArgs struct {
	Budget int `json:"budget,omitempty"`
}

// mapBudget is the default token cap for the map tool; STACKLIT_MAP_TOKENS overrides it.
func mapBudget() int {
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_MAP_TOKENS")); err == nil && n > 0 {
		return n
	}
	return 800
}

// staleSuffix is the one-line staleness warning for a graph read from disk.
func staleSuffix() string {
	st, err := os.Stat(graphFileName)
	if err != nil {
		return ""
	}
	g, err := loadGraph()
	if err != nil {
		return ""
	}
	return graph.StaleLine(".", g, st.ModTime())
}

func registerMinimalTools(s *sdkmcp.Server) {
	run := func(h func(g *graph.SymGraph) (string, error)) (*sdkmcp.CallToolResult, any, error) {
		g, err := loadGraph()
		if err != nil {
			return errResult(err)
		}
		out, err := h(g)
		if err != nil {
			return errResult(err)
		}
		return plain(out)
	}
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "query", Description: "Find code for a question or identifier"},
		func(ctx context.Context, req *sdkmcp.CallToolRequest, a minQueryArgs) (*sdkmcp.CallToolResult, any, error) {
			return run(func(g *graph.SymGraph) (string, error) {
				if a.Q == "" {
					return "", fmt.Errorf("q is required")
				}
				return g.Query(a.Q, a.Budget, false) + staleSuffix(), nil
			})
		})
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "slice", Description: "Source of one symbol"},
		func(ctx context.Context, req *sdkmcp.CallToolRequest, a minSymArgs) (*sdkmcp.CallToolResult, any, error) {
			return run(func(g *graph.SymGraph) (string, error) { return g.Slice(".", a.Symbol, a.Budget) })
		})
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "impact", Description: "What depends on a symbol"},
		func(ctx context.Context, req *sdkmcp.CallToolRequest, a minSymArgs) (*sdkmcp.CallToolResult, any, error) {
			return run(func(g *graph.SymGraph) (string, error) { return g.Impact(a.Symbol, 0, a.Budget, false) })
		})
	sdkmcp.AddTool(s, &sdkmcp.Tool{Name: "map", Description: "Repo map: areas and key symbols"},
		func(ctx context.Context, req *sdkmcp.CallToolRequest, a minMapArgs) (*sdkmcp.CallToolResult, any, error) {
			return run(func(g *graph.SymGraph) (string, error) {
				b := a.Budget
				if b <= 0 {
					b = mapBudget()
				}
				return graph.ClampTokens(g.RenderCommunities(false), b), nil
			})
		})
}
