package cli

import (
	"fmt"

	"github.com/glincker/stacklit/internal/setup"
	"github.com/spf13/cobra"
)

func newGuideCmd() *cobra.Command {
	var asJSON bool
	c := &cobra.Command{
		Use:   "guide",
		Short: "Print the agent usage guide (paste it into any AI tool)",
		Long: `Prints the rules and command list an AI agent should follow when a Stacklit
index exists: answer structure questions from the index, read code with slice,
and do not crawl the repo. It is the same text the rules block uses.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				fmt.Fprint(cmd.OutOrStdout(), setup.GuideJSON())
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), setup.GuideText())
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return c
}

func init() { rootCmd.AddCommand(newGuideCmd()) }
