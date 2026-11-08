package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/walker"
)

// graphFile is the default optional symbol graph, kept apart so stacklit.json stays small.
const graphFile = "stacklit.graph.json"

type graphFlags struct {
	file  string
	root  string
	json  bool
	tests bool
	// loaded is the mtime of the graph file read by load, zero when built in memory.
	loaded time.Time
}

func (f *graphFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.file, "graph", graphFile, "Path to the symbol graph file")
	c.Flags().StringVar(&f.root, "root", ".", "Project root")
	c.Flags().BoolVar(&f.json, "json", false, "Output JSON")
	c.Flags().BoolVar(&f.tests, "include-tests", false, "Include test files when building the graph (or set STACKLIT_GRAPH_INCLUDE_TESTS=1)")
}

// graphExtractOptions combines environment settings with the project's parse memory budget.
func graphExtractOptions(cfg *config.Config) graph.ExtractOptions {
	eo := graph.OptionsFromEnv()
	if b := cfg.ParseMemoryBytes(); b > 0 {
		eo.MemoryBudget = b
	} else {
		eo.MemoryBudget = -1
	}
	return eo
}

// buildSymbolGraph scans root with the project's ignore rules and links symbols.
func buildSymbolGraph(root string, includeTests ...bool) (*graph.SymGraph, error) {
	cfg := config.Load(root)
	files, err := walker.Walk(root, cfg.ScanIgnore())
	if err != nil {
		return nil, err
	}
	eo := graphExtractOptions(cfg)
	for _, b := range includeTests {
		eo.IncludeTests = eo.IncludeTests || b
	}
	g := graph.BuildSymbols(root, files, eo)
	g.Stamp(root)
	return g, nil
}

// load reads the graph file, or builds one in memory when it does not exist.
func (f *graphFlags) load(cmd *cobra.Command) (*graph.SymGraph, error) {
	p := f.file
	if !filepath.IsAbs(p) && f.root != "." {
		if _, err := os.Stat(p); err != nil {
			p = filepath.Join(f.root, p)
		}
	}
	if g, err := graph.Load(p); err == nil {
		if st, serr := os.Stat(p); serr == nil {
			f.loaded = st.ModTime()
		}
		return g, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s not found, building in memory (run 'stacklit graph' to save it)\n", f.file)
	return buildSymbolGraph(f.root, f.tests)
}

// htmlMaxNodes caps symbols embedded in stacklit.html; STACKLIT_HTML_MAX_NODES overrides it.
func htmlMaxNodes() int {
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_HTML_MAX_NODES")); err == nil && n > 0 {
		return n
	}
	return 5000
}

func newGraphCmd() *cobra.Command {
	var f graphFlags
	c := &cobra.Command{
		Use:   "graph",
		Short: "Build the symbol graph (stacklit.graph.json)",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := buildSymbolGraph(f.root, f.tests)
			if err != nil {
				return err
			}
			out := f.file
			if !filepath.IsAbs(out) {
				out = filepath.Join(f.root, out)
			}
			if err := g.Save(out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s: %d nodes, %d edges, %d communities\n", f.file, len(g.Nodes), len(g.Edges), len(g.Communities))
			return nil
		},
	}
	f.bind(c)
	return c
}

func newQueryCmd() *cobra.Command {
	var f graphFlags
	var budget int
	c := &cobra.Command{
		Use:   "query <question>",
		Short: "Scoped, token-budgeted subgraph for a question",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.Query(joinArgs(args), budget, f.json))
			if !f.json && !f.loaded.IsZero() {
				fmt.Fprint(cmd.OutOrStdout(), graph.StaleLine(f.root, g, f.loaded))
			}
			return nil
		},
	}
	f.bind(c)
	c.Flags().IntVar(&budget, "budget", graph.DefaultQueryBudget, "Hard output limit in tokens")
	return c
}

func joinArgs(a []string) string {
	s := ""
	for i, x := range a {
		if i > 0 {
			s += " "
		}
		s += x
	}
	return s
}

func newPathCmd() *cobra.Command {
	var f graphFlags
	c := &cobra.Command{
		Use:   "path <from> <to>",
		Short: "Shortest path between two symbols, with weakest-hop confidence",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			p, err := g.ShortestPath(args[0], args[1])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), p.Render(f.json))
			return nil
		},
	}
	f.bind(c)
	return c
}

func newExplainCmd() *cobra.Command {
	var f graphFlags
	var budget int
	c := &cobra.Command{
		Use:   "explain <symbol>",
		Short: "Definition, location, callers and callees of a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			out, err := g.Explain(args[0], budget, f.json)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	f.bind(c)
	c.Flags().IntVar(&budget, "budget", graph.DefaultExplainBudget, "Hard output limit in tokens")
	return c
}

func newImpactCmd() *cobra.Command {
	var f graphFlags
	var budget, depth int
	c := &cobra.Command{
		Use:   "impact <symbol|file>",
		Short: "Blast radius: everything that depends on a symbol or file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			out, err := g.Impact(args[0], depth, budget, f.json)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	f.bind(c)
	c.Flags().IntVar(&budget, "budget", graph.DefaultImpactBudget, "Hard output limit in tokens")
	c.Flags().IntVar(&depth, "depth", 4, "Maximum dependency depth")
	return c
}

func newHubsCmd() *cobra.Command {
	var f graphFlags
	var n int
	c := &cobra.Command{
		Use:   "hubs",
		Short: "Most connected symbols (god nodes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), graph.RenderHubs(g.Hubs(n), f.json))
			return nil
		},
	}
	f.bind(c)
	c.Flags().IntVarP(&n, "top", "n", 15, "Number of hubs")
	return c
}

func newCommunitiesCmd() *cobra.Command {
	var f graphFlags
	c := &cobra.Command{
		Use:   "communities",
		Short: "Deterministic clusters of files with generated labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), g.RenderCommunities(f.json))
			return nil
		},
	}
	f.bind(c)
	return c
}

func newReportCmd() *cobra.Command {
	var f graphFlags
	var out string
	c := &cobra.Command{
		Use:   "report",
		Short: "GRAPH_REPORT.md: key concepts, surprising links, suggested questions",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			r := g.Report()
			if out == "" {
				fmt.Fprint(cmd.OutOrStdout(), r)
				return nil
			}
			return os.WriteFile(out, []byte(r), 0o644)
		},
	}
	f.bind(c)
	c.Flags().StringVarP(&out, "output", "o", "", "Write to a file (for example GRAPH_REPORT.md)")
	return c
}

func newSliceCmd() *cobra.Command {
	var f graphFlags
	var budget int
	c := &cobra.Command{
		Use:   "slice <symbol>",
		Short: "Print only one symbol's source, with line numbers",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.load(cmd)
			if err != nil {
				return err
			}
			out, err := g.Slice(f.root, args[0], budget)
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	f.bind(c)
	c.Flags().IntVar(&budget, "budget", 0, "Hard output limit in tokens (0 = whole symbol)")
	return c
}

func newWatchCmd() *cobra.Command {
	var f graphFlags
	c := &cobra.Command{
		Use:   "watch",
		Short: "Rebuild the symbol graph incrementally when files change",
		Long: `Watches source directories (not ignored ones) and re-parses only changed files.
Falls back to polling above STACKLIT_WATCH_MAX directories (default 4096) so huge
trees never exhaust inotify. STACKLIT_WATCH_POLL_MS sets the polling interval.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			cfg := config.Load(f.root)
			return graph.Watch(ctx, graph.WatchOptions{
				Root: f.root, Out: f.file, Ignore: cfg.ScanIgnore(),
				Extract: func() graph.ExtractOptions {
					eo := graphExtractOptions(cfg)
					eo.IncludeTests = eo.IncludeTests || f.tests
					return eo
				}(),
				Log: func(format string, a ...any) { fmt.Fprintf(cmd.ErrOrStderr(), "[stacklit] "+format+"\n", a...) },
			})
		},
	}
	f.bind(c)
	c.Flags().Lookup("json").Hidden = true
	return c
}

func addGraphCommands() {
	rootCmd.AddCommand(newGraphCmd(), newQueryCmd(), newPathCmd(), newExplainCmd(), newImpactCmd(),
		newHubsCmd(), newCommunitiesCmd(), newReportCmd(), newSliceCmd(), newWatchCmd())
}
