package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/ingest"
	"github.com/glincker/stacklit/internal/scip"
)

// extIngest holds the flags for adapter and SCIP ingestion.
type extIngest struct {
	adapter, name, files, scipPath string
	intoGraph                      bool
	graphPath, outPath             string
	stdout                         bool
}

func (e *extIngest) active() bool { return e.adapter != "" || e.scipPath != "" }

func (e *extIngest) bind(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringVar(&e.adapter, "adapter", "", "Shell command that reads file paths on stdin and prints JSONL nodes and edges")
	f.StringVar(&e.name, "adapter-name", "", "Source label stored on adapter output (default: the command's program name)")
	f.StringVar(&e.files, "adapter-files", "", "Comma-separated globs choosing which files the adapter receives (default: all non-hidden files)")
	f.BoolVar(&e.intoGraph, "into-graph", false, "Also merge adapter concepts into the symbol graph file")
	f.StringVar(&e.scipPath, "scip", "", "SCIP index (.scip) used to upgrade INFERRED graph edges to EXACT")
	f.StringVar(&e.graphPath, "graph", graphFile, "Symbol graph file for --scip and --into-graph")
}

// run handles --adapter and --scip. The built-in connectors are skipped.
func (e *extIngest) run(cmd *cobra.Command) error {
	out := cmd.OutOrStdout()
	if e.adapter != "" {
		var globs []string
		if e.files != "" {
			globs = strings.Split(e.files, ",")
		}
		files := ingest.FilterFiles(ingest.ListFiles("."), globs)
		res, err := ingest.RunAdapter(context.Background(), ingest.AdapterOptions{Root: ".", Command: e.adapter, Name: e.name, Files: files})
		if err != nil {
			return err
		}
		ctx := &ingest.Context{Version: "1"}
		if data, err := os.ReadFile(e.outPath); err == nil {
			if jerr := json.Unmarshal(data, ctx); jerr != nil {
				return fmt.Errorf("could not parse existing %s: %w", e.outPath, jerr)
			}
		}
		ingest.ApplyAdapter(ctx, res)
		b, err := json.MarshalIndent(ctx, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		if e.stdout {
			_, err = out.Write(b)
			return err
		}
		if err := os.WriteFile(e.outPath, b, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(out, "wrote %s (adapter %s: %d concepts, %d links, %d lines skipped)\n", e.outPath, res.Name, len(res.Concepts), len(res.Links), res.Skipped)
		if e.intoGraph {
			g, err := graph.Load(e.graphPath)
			if err != nil {
				return fmt.Errorf("--into-graph needs %s: %w", e.graphPath, err)
			}
			dropped := ingest.MergeIntoGraph(g, res)
			if err := g.Save(e.graphPath); err != nil {
				return err
			}
			fmt.Fprintf(out, "merged into %s (%d links dropped, endpoint not in graph)\n", e.graphPath, dropped)
		}
	}
	if e.scipPath != "" {
		g, err := graph.Load(e.graphPath)
		if err != nil {
			return fmt.Errorf("--scip needs %s (run 'stacklit graph'): %w", e.graphPath, err)
		}
		idx, err := scip.Load(e.scipPath)
		if err != nil {
			return err
		}
		st := scip.Upgrade(g, idx)
		if err := g.Save(e.graphPath); err != nil {
			return err
		}
		fmt.Fprintf(out, "scip: %d documents, %d edges upgraded to EXACT, %d confirmed already, %d still INFERRED, %d documents not in graph\n",
			st.DocumentsParsed, st.Upgraded, st.AlreadyExact, st.StillInferred, st.UnmatchedFiles)
	}
	return nil
}
