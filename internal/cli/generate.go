package cli

import (
	"github.com/glincker/stacklit/internal/engine"
	"github.com/glincker/stacklit/internal/renderer"
	"github.com/spf13/cobra"
)

var generateQuiet, generateGraph bool

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate the stacklit.json codebase index",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := engine.Run(engine.Options{
			Root:  ".",
			Quiet: generateQuiet,
		})
		if err != nil || !generateGraph {
			return err
		}
		g, err := buildSymbolGraph(".")
		if err != nil {
			return err
		}
		if err := g.Save(graphFile); err != nil {
			return err
		}
		return renderer.WriteHTMLWithGraph(res.Index, g, htmlMaxNodes(), res.HTMLPath)
	},
}

func init() {
	generateCmd.Flags().BoolVar(&generateQuiet, "quiet", false, "Suppress progress output")
	generateCmd.Flags().BoolVar(&generateGraph, "graph", false, "Also write stacklit.graph.json (symbol graph)")
}
