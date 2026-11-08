package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/glincker/stacklit/internal/export"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/renderer"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var format, source, output, graphPath, scope string
	var budget int

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the index in a human-readable format",
		Long: `Renders stacklit.json in a different format for sharing.

Currently supports markdown, which produces a readable project overview
suitable for pasting into PR descriptions, GitHub issues, or chat.

Examples:
  stacklit export                          Print markdown to stdout
  stacklit export --format md              Same as above
  stacklit export -o stacklit.md           Write markdown to a file
  stacklit export --source ./other.json    Read from a non-default index

Graph formats (graphml, cypher, json, dot, mermaid, obsidian, markdown-wiki)
print to stdout, or write to -o (a file for one-file formats, a directory
for obsidian and markdown-wiki). Nothing is written into the repo unless -o
points there. --scope modules|symbols picks the graph; the default is symbols
when stacklit.graph.json exists, otherwise modules.
  stacklit export --format graphml -o /tmp/g.graphml
  stacklit export --format obsidian -o ~/vault/myproj`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(source)
			if err != nil {
				return fmt.Errorf("could not read %s: %w (run 'stacklit generate' first, or 'stacklit init')", source, err)
			}
			var idx schema.Index
			if err := json.Unmarshal(data, &idx); err != nil {
				return fmt.Errorf("could not parse %s: %w", source, err)
			}

			switch format {
			case "md", "markdown":
				if output == "" {
					fmt.Print(renderer.RenderMarkdown(&idx))
					return nil
				}
				if err := renderer.WriteMarkdown(&idx, output); err != nil {
					return fmt.Errorf("could not write %s: %w", output, err)
				}
				fmt.Fprintf(cmd.OutOrStderr(), "Wrote %s\n", output)
				return nil
			default:
				return runGraphExport(cmd, format, idx, graphPath, scope, budget, output)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "md", "Output format: md, graphml, cypher, json, dot, mermaid, obsidian, markdown-wiki")
	cmd.Flags().StringVar(&source, "source", "stacklit.json", "Path to the index JSON")
	cmd.Flags().StringVar(&graphPath, "graph", graphFile, "Symbol graph file used by graph formats")
	cmd.Flags().StringVar(&scope, "scope", "", "Graph scope for single-file formats: modules or symbols")
	cmd.Flags().IntVar(&budget, "budget", 0, "Token budget per markdown-wiki page (env STACKLIT_EXPORT_PAGE_TOKENS)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write to file instead of stdout")
	return cmd
}

// runGraphExport renders the graph formats from the new export package.
func runGraphExport(cmd *cobra.Command, format string, idx schema.Index, graphPath, scope string, budget int, output string) error {
	known := false
	for _, f := range export.Formats {
		known = known || f == format
	}
	if !known {
		return fmt.Errorf("unknown format %q (supported: md, markdown, %s)", format, strings.Join(export.Formats, ", "))
	}
	in := export.Input{Index: &idx}
	if scope != export.ScopeModules {
		if g, err := graph.Load(graphPath); err == nil {
			in.Graph = g
		} else if scope == export.ScopeSymbols {
			g, berr := buildSymbolGraph(".")
			if berr != nil {
				return berr
			}
			in.Graph = g
		}
	}
	files, err := export.Render(format, in, export.Options{Scope: scope, PageTokens: budget})
	if err != nil {
		return err
	}
	if output == "" {
		return export.WriteStdout(cmd.OutOrStdout(), files)
	}
	written, err := export.WriteTo(output, files)
	if err != nil {
		return fmt.Errorf("could not write %s: %w", output, err)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %d file(s) under %s\n", len(written), output)
	return nil
}
