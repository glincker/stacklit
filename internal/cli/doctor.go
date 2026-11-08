package cli

import (
	"encoding/json"
	"fmt"

	"github.com/glincker/stacklit/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var asJSON, strict bool
	cmd := &cobra.Command{
		Use:   "doctor [path]",
		Short: "Diagnose config, hooks, stale index and parse errors",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			rep, err := doctor.Run(root)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				if err := enc.Encode(rep); err != nil {
					return err
				}
			} else {
				printReport(cmd, rep)
			}
			errs, warns := rep.Count(doctor.Error), rep.Count(doctor.Warn)
			if errs > 0 || (strict && warns > 0) {
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				return &ExitError{Code: ExitProblems, Err: fmt.Errorf("doctor found %d errors and %d warnings", errs, warns)}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the report as JSON")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero on warnings too")
	return cmd
}

func printReport(cmd *cobra.Command, rep *doctor.Report) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "stacklit doctor: %s\n\n", rep.Root)
	for _, f := range rep.Findings {
		fmt.Fprintf(out, "[%-5s] %s: %s\n", f.Level, f.Check, f.Message)
		for _, d := range f.Details {
			fmt.Fprintf(out, "         - %s\n", d)
		}
		if f.Fix != "" {
			fmt.Fprintf(out, "         fix: %s\n", f.Fix)
		}
	}
	fmt.Fprintf(out, "\n%d errors, %d warnings\n", rep.Count(doctor.Error), rep.Count(doctor.Warn))
}
