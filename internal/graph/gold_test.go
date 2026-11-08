package graph

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/walker"
)

type goldCase struct {
	q    string
	want []string // any of these files in the top 5 passes
}

// goldCases are questions over this repository; the right file must rank in the top 5.
var goldCases = []goldCase{
	{"parse file imports", []string{"internal/parser/golang.go", "internal/graph/extract_go.go"}},
	{"detect monorepo workspaces", []string{"internal/monorepo/detect.go"}},
	{"install git post-commit hook", []string{"internal/git/hooks.go"}},
	{"compute merkle hash of files", []string{"internal/git/merkle.go", "internal/git/merkle_leaves.go"}},
	{"render mermaid diagram of module dependencies", []string{"internal/renderer/mermaid.go"}},
	{"write a file atomically", []string{"internal/fsutil/atomic.go"}},
	{"cluster files into communities by label propagation", []string{"internal/graph/community.go"}},
	{"shortest path between two symbols", []string{"internal/graph/path.go"}},
	{"blast radius when a symbol changes", []string{"internal/graph/impact.go"}},
	{"print only one symbol's source lines", []string{"internal/graph/slice.go"}},
	{"pagerank score of nodes", []string{"internal/graph/rank.go"}},
	{"watch files and rebuild the graph incrementally", []string{"internal/graph/watch.go"}},
	{"detect frameworks from dependencies", []string{"internal/detect/frameworks.go", "internal/detect/framework_patterns.go"}},
	{"extract environment variables from docker compose", []string{"internal/detect/envvars.go"}},
	{"mcp server tools exposed to agents", []string{"internal/mcp/server.go", "internal/mcp/graph_tools.go"}},
	{"generate an AI summary of the codebase", []string{"internal/summary/ai.go"}},
	{"git hot files and commit activity", []string{"internal/git/activity.go"}},
	{"load configuration from the config file", []string{"internal/config/config.go"}},
	{"ignore patterns used when scanning", []string{"internal/config/scan.go", "internal/walker/walker.go", "internal/walker/scan.go"}},
	{"walk the directory tree skipping ignored files", []string{"internal/walker/walker.go", "internal/walker/scan.go"}},
	{"python import statements", []string{"internal/graph/extract_py.go"}},
	{"javascript import statements", []string{"internal/graph/extract_ecma.go"}},
	{"rust use declarations", []string{"internal/graph/extract_rs.go"}},
	{"ingest an openapi spec", []string{"internal/ingest/api.go"}},
	{"codeowners file ownership", []string{"internal/ingest/owners.go"}},
	{"claude code hooks in settings.json", []string{"internal/setup/claude.go"}},
	{"render the index as markdown for AI context", []string{"internal/renderer/markdown.go"}},
	{"doctor checks the health of the setup", []string{"internal/doctor/doctor.go"}},
	{"tree-sitter parser pool memory budget", []string{"internal/parser/pool.go", "internal/parser/treesitter.go"}},
	{"write the standalone html page", []string{"internal/renderer/html.go", "internal/renderer/html_graph.go"}},
	{"cache of parsed files", []string{"internal/cache/cache.go"}},
	{"resolve calls between symbols by name", []string{"internal/graph/resolve.go"}},
	{"truncate output to a token budget", []string{"internal/graph/view.go"}},
	{"callers and callees of a symbol", []string{"internal/graph/explain.go"}},
	// identifier queries
	{"parseOpenAPIYAML", []string{"internal/ingest/api.go"}},
	{"ParseMemoryBytes", []string{"internal/config/scan.go"}},
	{"importsByModule", []string{"internal/engine/multi.go"}},
	{"parse_openapi_yaml", []string{"internal/ingest/api.go"}},
	{"ingest.parseOpenAPIYAML", []string{"internal/ingest/api.go"}},
	// quoted phrase
	{`"post-commit"`, []string{"internal/git/hooks.go", "internal/setup/git.go"}},
}

var (
	goldOnce sync.Once
	goldG    *SymGraph
)

// repoGraph builds the symbol graph of this repository once for all tests.
func repoGraph(t testing.TB) *SymGraph {
	t.Helper()
	goldOnce.Do(func() {
		root, err := filepath.Abs("../..")
		if err != nil {
			return
		}
		cfg := config.Load(root)
		files, err := walker.Walk(root, cfg.ScanIgnore())
		if err != nil {
			return
		}
		goldG = BuildSymbols(root, files, ExtractOptions{})
	})
	if goldG == nil {
		t.Skip("repository graph unavailable")
	}
	return goldG
}

func rankOf(files []string, want []string) int {
	for i, f := range files {
		for _, w := range want {
			if f == w {
				return i + 1
			}
		}
	}
	return 0
}

func TestGoldRetrieval(t *testing.T) {
	g := repoGraph(t)
	top1, top5 := 0, 0
	for _, c := range goldCases {
		files := g.SearchFiles(c.q, 10)
		r := rankOf(files, c.want)
		if r == 1 {
			top1++
		}
		if r >= 1 && r <= 5 {
			top5++
		} else {
			t.Errorf("%q: want one of %v in top 5, got %v", c.q, c.want, firstN(files, 5))
		}
	}
	n := len(goldCases)
	t.Logf("gold set: %d questions, top-1 %d (%.0f%%), top-5 %d (%.0f%%)", n, top1, 100*float64(top1)/float64(n), top5, 100*float64(top5)/float64(n))
	if n < 25 {
		t.Fatalf("gold set too small: %d", n)
	}
}

func firstN(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

