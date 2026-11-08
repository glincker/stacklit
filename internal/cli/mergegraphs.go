package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/merge"
	"github.com/glincker/stacklit/internal/schema"
)

func init() { rootCmd.AddCommand(newMergeGraphsCmd()) }

func newMergeGraphsCmd() *cobra.Command {
	var out, multiPath string
	cmd := &cobra.Command{
		Use:   "merge-graphs a.json b.json [name=c.json ...] -o out.json",
		Short: "Merge symbol graphs from several repos into one",
		Long: `Combines stacklit.graph.json files. Node IDs are prefixed with the repo name,
and every node keeps its source_file (file) plus a repo field.

Repo names come from the graph root or its folder; give name=path.json to set one.
--multi stacklit-multi.json (from 'stacklit generate --multi') adds repo nodes and
links repos that depend on each other through manifest package names.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" {
				return &ExitError{Code: ExitUsage, Err: fmt.Errorf("-o is required")}
			}
			var inputs []merge.Input
			for _, a := range args {
				name, file := "", a
				if i := strings.Index(a, "="); i > 0 {
					name, file = a[:i], a[i+1:]
				}
				g, err := graph.Load(file)
				if err != nil {
					return err
				}
				if name == "" {
					name = merge.DefaultName(g, file)
				}
				inputs = append(inputs, merge.Input{Name: name, Graph: g})
			}
			var multi *schema.MultiIndex
			if multiPath != "" {
				data, err := os.ReadFile(multiPath)
				if err != nil {
					return err
				}
				multi = &schema.MultiIndex{}
				if err := json.Unmarshal(data, multi); err != nil {
					return fmt.Errorf("could not parse %s: %w", multiPath, err)
				}
			}
			merged, err := merge.Merge(inputs, multi)
			if err != nil {
				return err
			}
			if err := merged.Save(out); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d repos, %d nodes, %d edges)\n", out, len(inputs), len(merged.Nodes), len(merged.Edges))
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "output", "o", "", "Output graph file")
	cmd.Flags().StringVar(&multiPath, "multi", "", "stacklit-multi.json used to link repos by package names")
	return cmd
}
