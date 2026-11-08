package cli

import (
	"fmt"

	"github.com/glincker/stacklit/internal/setup"
	"github.com/spf13/cobra"
)

// newHookCmd is the target of the hooks that `setup --pretool-hook` and
// `--session-hook` install. Hidden: it is plumbing, not a user command.
func newHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "hook <pretool|session|subagent|posttool|crawl> [mode]",
		Short:     "Claude Code hook handler (used by setup)",
		Hidden:    true,
		Args:      cobra.RangeArgs(1, 2),
		ValidArgs: []string{"pretool", "session", "subagent", "posttool", "crawl"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var out string
			switch args[0] {
			case "pretool":
				out = setup.PreToolHook(cmd.InOrStdin())
			case "session":
				out = setup.SessionHook(cmd.InOrStdin())
			case "subagent":
				out = setup.SubagentHook(cmd.InOrStdin())
			case "crawl":
				installed := ""
				if len(args) > 1 {
					installed = args[1]
				}
				out = setup.CrawlGuardHook(cmd.InOrStdin(), installed)
			case "posttool":
				return runPostToolRefresh(cmd)
			default:
				return fmt.Errorf("unknown hook %q", args[0])
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func init() { rootCmd.AddCommand(newHookCmd()) }
