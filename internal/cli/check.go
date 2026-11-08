package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/check"
	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/schema"
)

func init() { rootCmd.AddCommand(newCheckCmd()) }

func newCheckCmd() *cobra.Command {
	var root, source string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Enforce architecture rules from .stacklitrc.json",
		Long: `Evaluates the "check" block of .stacklitrc.json against stacklit.json.

Rules (each is off until configured):
  forbidden   [{"from": "ui/**", "to": "core/db", "reason": "..."}] banned module edges
  cycles      true to report import cycles between modules
  max_fan_in  fail when a module has more dependents than this
  max_fan_out fail when a module depends on more modules than this
  orphans     true to report modules with no edges in or out

Exit code 3 when any rule is violated, 0 otherwise. Use --json for CI.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := source
			if !filepath.IsAbs(p) && root != "." {
				p = filepath.Join(root, p)
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("could not read %s: %w (run 'stacklit generate' first)", p, err)
			}
			var idx schema.Index
			if err := json.Unmarshal(data, &idx); err != nil {
				return fmt.Errorf("could not parse %s: %w", p, err)
			}
			res := check.Run(&idx, config.Load(root).Check)
			out := cmd.OutOrStdout()
			if asJSON {
				b, err := json.MarshalIndent(res, "", "  ")
				if err != nil {
					return err
				}
				fmt.Fprintln(out, string(b))
			} else if res.OK {
				fmt.Fprintln(out, "stacklit check: ok")
			} else {
				for _, v := range res.Violations {
					fmt.Fprintf(out, "[%s] %s\n", v.Rule, v.Message)
				}
				fmt.Fprintf(out, "stacklit check: %d violation(s)\n", len(res.Violations))
			}
			if !res.OK {
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				return &ExitError{Code: ExitProblems, Err: fmt.Errorf("%d architecture violation(s)", len(res.Violations))}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&root, "root", ".", "Project root holding .stacklitrc.json")
	cmd.Flags().StringVar(&source, "source", "stacklit.json", "Path to the index JSON")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return cmd
}
