package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/stats"
)

// parseSince accepts a duration like 7d or 36h, or a date like 2026-01-31.
func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	var n int
	var unit string
	if _, err := fmt.Sscanf(s, "%d%s", &n, &unit); err == nil && n > 0 {
		switch unit {
		case "d":
			return time.Now().Add(-time.Duration(n) * 24 * time.Hour), nil
		case "h":
			return time.Now().Add(-time.Duration(n) * time.Hour), nil
		}
	}
	return time.Time{}, fmt.Errorf("bad --since %q: use 7d, 36h or 2026-01-31", s)
}

func newStatsCmd() *cobra.Command {
	var since string
	var asJSON bool
	c := &cobra.Command{
		Use:   "stats",
		Short: "Report the opt-in local usage log (STACKLIT_STATS=1)",
		Long: `Summarizes ~/.cache/stacklit/stats.jsonl, written only when STACKLIT_STATS=1 or
"stats": true is set in .stacklitrc.json. Nothing leaves your machine.

served       tokens (bytes/4) stacklit answers put in the agent's context.
replaced_est ESTIMATE: tokens of the whole files those answers pointed into.
             It assumes the agent would otherwise have read them in full, which
             is not measured. It is not a measured saving.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := parseSince(since)
			if err != nil {
				return &ExitError{Code: ExitUsage, Err: err}
			}
			s, err := stats.Read(t)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON {
				return json.NewEncoder(out).Encode(s)
			}
			if s.Calls == 0 {
				fmt.Fprintf(out, "no entries in %s (logging is off unless STACKLIT_STATS=1)\n", stats.LogPath())
				return nil
			}
			fmt.Fprintf(out, "%-10s %6s %10s %14s\n", "command", "calls", "served", "replaced_est")
			for _, r := range s.ByCmd {
				fmt.Fprintf(out, "%-10s %6d %10d %14d\n", r.Cmd, r.Calls, r.Served, r.ReplacedEst)
			}
			fmt.Fprintf(out, "%-10s %6d %10d %14d\n", "total", s.Calls, s.Served, s.ReplacedEst)
			fmt.Fprintln(out, "\nreplaced_est is an estimate of whole-file read cost, not a measured saving.")
			return nil
		},
	}
	c.Flags().StringVar(&since, "since", "", "Only entries since 7d, 36h or a date (2026-01-31)")
	c.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return c
}

func init() { rootCmd.AddCommand(newStatsCmd()) }
