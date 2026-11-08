package cli

import (
	"github.com/glincker/stacklit/internal/engine"
	"github.com/spf13/cobra"
)

var generateQuiet, generateGraph bool

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate the stacklit.json codebase index",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := engine.Run(engine.Options{
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
		return g.Save(graphFile)
	},
}

func init() {
	generateCmd.Flags().BoolVar(&generateQuiet, "quiet", false, "Suppress progress output")
	generateCmd.Flags().BoolVar(&generateGraph, "graph", false, "Also write stacklit.graph.json (symbol graph)")
}
