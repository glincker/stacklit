package mcp

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func connect(t *testing.T, profile string) *sdkmcp.ClientSession {
	t.Helper()
	srv, err := NewServer(profile)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ct, st := sdkmcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	sess, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func toolSchemaTokens(t *testing.T, sess *sdkmcp.ClientSession) (names []string, tokens int) {
	t.Helper()
	res, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res.Tools)
	if err != nil {
		t.Fatal(err)
	}
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}
	return names, (len(b) + 3) / 4
}

func TestMinimalProfileIsSmall(t *testing.T) {
	names, tokens := toolSchemaTokens(t, connect(t, ProfileMinimal))
	if got := strings.Join(sortedCopy(names), ","); got != "impact,map,query,slice" {
		t.Fatalf("minimal tools = %s", got)
	}
	limit := 500
	if v := os.Getenv("STACKLIT_MINIMAL_SCHEMA_TOKENS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		limit = n
	}
	t.Logf("minimal profile tool schemas: %d tokens (4 tools)", tokens)
	if tokens > limit {
		t.Fatalf("minimal schemas are %d tokens, limit %d", tokens, limit)
	}
}

func TestFullProfileKeepsAllTools(t *testing.T) {
	names, tokens := toolSchemaTokens(t, connect(t, ProfileFull))
	t.Logf("full profile tool schemas: %d tokens (%d tools)", tokens, len(names))
	if len(names) != 15 {
		t.Fatalf("full profile has %d tools, want 15: %v", len(names), names)
	}
}

func TestMinimalToolsWork(t *testing.T) {
	t.Chdir(t.TempDir())
	src := "package a\n\n// Load reads data.\nfunc Load() { parse() }\n\nfunc parse() {}\n"
	if err := os.WriteFile("a.go", []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cachedGraph = nil
	sess := connect(t, ProfileMinimal)
	for tool, c := range map[string]struct {
		args map[string]any
		want string
	}{
		"query":  {map[string]any{"q": "Load"}, "Load"},
		"slice":  {map[string]any{"symbol": "parse"}, "func parse()"},
		"impact": {map[string]any{"symbol": "parse"}, "Load"},
		"map":    {map[string]any{}, "communities"},
	} {
		res, err := sess.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: tool, Arguments: c.args})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", tool, err, res)
		}
		if txt := res.Content[0].(*sdkmcp.TextContent).Text; !strings.Contains(txt, c.want) {
			t.Errorf("%s: %q not in %q", tool, c.want, txt)
		}
	}
}

func TestUnknownProfile(t *testing.T) {
	if _, err := NewServer("huge"); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestInstructionsPresentAndSmall(t *testing.T) {
	limit := 120
	if v := os.Getenv("STACKLIT_INSTRUCTIONS_TOKENS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			t.Fatal(err)
		}
		limit = n
	}
	for _, p := range []string{ProfileFull, ProfileMinimal} {
		sess := connect(t, p)
		got := sess.InitializeResult().Instructions
		if got == "" {
			t.Fatalf("%s: no instructions in initialize result", p)
		}
		if got != Instructions(p) {
			t.Fatalf("%s: instructions not byte stable", p)
		}
		if tok := (len(got) + 3) / 4; tok > limit {
			t.Fatalf("%s: instructions %d tokens > %d", p, tok, limit)
		} else {
			t.Logf("%s instructions: %d tokens", p, tok)
		}
		for _, w := range []string{"source of truth", "slice", "refresh"} {
			if !strings.Contains(got, w) {
				t.Fatalf("%s: missing %q", p, w)
			}
		}
	}
}
