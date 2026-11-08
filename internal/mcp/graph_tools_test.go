package mcp

import (
	"context"
	"os"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGraphToolsRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())
	src := "package a\n\n// Load reads data.\nfunc Load() { parse() }\n\nfunc parse() {}\n"
	if err := os.WriteFile("a.go", []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cachedGraph = nil
	ctx := context.Background()
	srv := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "t", Version: "0"}, nil)
	registerGraphTools(srv)
	ct, st := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "c", Version: "0"}, nil)
	sess, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		tool string
		args map[string]any
		want string
	}{
		{"query_graph", map[string]any{"query": "load data", "budget": 300}, "Load"},
		{"explain_symbol", map[string]any{"symbol": "Load"}, "callee"},
		{"impact", map[string]any{"symbol": "parse"}, "Load"},
		{"shortest_path", map[string]any{"from": "Load", "to": "parse"}, "1 hops"},
		{"slice_symbol", map[string]any{"symbol": "parse"}, "func parse()"},
		{"get_hubs", map[string]any{}, "hubs"},
		{"get_communities", map[string]any{}, "communities"},
		{"graph_report", map[string]any{}, "Graph Report"},
	}
	for _, c := range cases {
		res, err := sess.CallTool(ctx, &sdkmcp.CallToolParams{Name: c.tool, Arguments: c.args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v res=%+v", c.tool, err, res)
		}
		txt := res.Content[0].(*sdkmcp.TextContent).Text
		if !strings.Contains(txt, c.want) {
			t.Errorf("%s: %q not in %q", c.tool, c.want, txt)
		}
	}
}
