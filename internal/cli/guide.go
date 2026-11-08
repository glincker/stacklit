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
index exists: use query to find where things are, grep or impact for exact
names, check the neighbours when the answer is several files, and read code with
slice. The steering key (or STACKLIT_STEERING=d) selects the older strict
no-crawl wording for comparison.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				fmt.Fprint(cmd.OutOrStdout(), setup.GuideJSONFor(setup.SteeringFor(".")))
				return nil
			}
			fmt.Fprint(cmd.OutOrStdout(), setup.GuideTextFor(setup.SteeringFor(".")))
			return nil
		},
	}
	c.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return c
}

func init() { rootCmd.AddCommand(newGuideCmd()) }
