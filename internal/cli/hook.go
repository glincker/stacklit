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
		Use:       "hook <pretool|session>",
		Short:     "Claude Code hook handler (used by setup)",
		Hidden:    true,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"pretool", "session"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var out string
			switch args[0] {
			case "pretool":
				out = setup.PreToolHook(cmd.InOrStdin())
			case "session":
				out = setup.SessionHook(cmd.InOrStdin())
			default:
				return fmt.Errorf("unknown hook %q", args[0])
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func init() { rootCmd.AddCommand(newHookCmd()) }
