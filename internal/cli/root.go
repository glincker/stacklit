package cli

import (
	"github.com/spf13/cobra"
)

// Version is set via ldflags at build time.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "stacklit",
	Short:   "Generate a token-efficient codebase index for AI agents",
	Version: Version,
}

// Execute runs the root command.
func Execute() error {
	defer startProfile()()
	return rootCmd.Execute()
}

func init() {
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &ExitError{Code: ExitUsage, Err: err}
	})
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(viewCmd)
	rootCmd.AddCommand(newDiffCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newDeriveCmd())
	rootCmd.AddCommand(newSetupCmd())
	rootCmd.AddCommand(newExportCmd())
	addGraphCommands()
}
