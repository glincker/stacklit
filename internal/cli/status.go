package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/graph"
)

// artifactStatus is one saved output (index or graph) compared with git.
type artifactStatus struct {
	File    string   `json:"file"`
	Present bool     `json:"present"`
	AgeSec  int64    `json:"age_seconds,omitempty"`
	Commit  string   `json:"commit,omitempty"`
	Behind  int      `json:"commits_behind,omitempty"`
	Changed []string `json:"changed,omitempty"`
	Stale   bool     `json:"stale"`
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// indexTime prefers generated_at inside stacklit.json and falls back to mtime.
func indexTime(path string, mtime time.Time) time.Time {
	b, err := os.ReadFile(path)
	if err != nil {
		return mtime
	}
	var h struct {
		GeneratedAt string `json:"generated_at"`
	}
	if json.Unmarshal(b, &h) == nil {
		if t, err := time.Parse(time.RFC3339, h.GeneratedAt); err == nil {
			return t
		}
	}
	return mtime
}

func newStatusCmd() *cobra.Command {
	var root, gfile string
	var asJSON bool
	c := &cobra.Command{
		Use:   "status",
		Short: "Index and graph age versus git HEAD, and files changed since",
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now()
			idxPath, grPath := filepath.Join(root, "stacklit.json"), filepath.Join(root, gfile)
			var rows []artifactStatus
			if st, err := os.Stat(idxPath); err == nil {
				t := indexTime(idxPath, st.ModTime())
				s := git.CheckStale(root, "", t, func(p string) bool { return graph.LangOf(p) != "" })
				rows = append(rows, artifactStatus{File: "stacklit.json", Present: true, AgeSec: int64(now.Sub(t).Seconds()), Behind: s.CommitsBehind, Changed: s.Changed, Stale: len(s.Changed) > 0})
			} else {
				rows = append(rows, artifactStatus{File: "stacklit.json"})
			}
			if st, err := os.Stat(grPath); err == nil {
				row := artifactStatus{File: gfile, Present: true, AgeSec: int64(now.Sub(st.ModTime()).Seconds())}
				if g, err := graph.Load(grPath); err == nil {
					s := graph.StaleCheck(root, g, st.ModTime())
					row.Commit, row.Behind, row.Changed, row.Stale = g.Commit, s.CommitsBehind, s.Changed, len(s.Changed) > 0
				}
				rows = append(rows, row)
			} else {
				rows = append(rows, artifactStatus{File: gfile})
			}
			head := git.Head(root)
			out := cmd.OutOrStdout()
			if asJSON {
				b, _ := json.Marshal(map[string]any{"head": head, "artifacts": rows})
				fmt.Fprintln(out, string(b))
				return nil
			}
			if head != "" {
				fmt.Fprintf(out, "HEAD %s\n", head)
			}
			stale := false
			for _, r := range rows {
				if !r.Present {
					fmt.Fprintf(out, "%s: missing (run: stacklit generate --graph)\n", r.File)
					continue
				}
				line := fmt.Sprintf("%s: built %s", r.File, humanAge(time.Duration(r.AgeSec)*time.Second))
				if r.Commit != "" {
					line += " at " + r.Commit
				}
				if r.Behind > 0 {
					line += fmt.Sprintf(", %d commits behind", r.Behind)
				}
				if len(r.Changed) > 0 {
					show := r.Changed
					more := ""
					if len(show) > 5 {
						more = fmt.Sprintf(", +%d", len(show)-5)
						show = show[:5]
					}
					line += fmt.Sprintf(", %d files changed (%s%s)", len(r.Changed), strings.Join(show, ", "), more)
					stale = true
				} else {
					line += ", current"
				}
				fmt.Fprintln(out, line)
			}
			if stale {
				fmt.Fprintln(out, "stale: run stacklit generate --graph")
			}
			return nil
		},
	}
	c.Flags().StringVar(&root, "root", ".", "Project root")
	c.Flags().StringVar(&gfile, "graph", graphFile, "Symbol graph file name")
	c.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return c
}

func init() { rootCmd.AddCommand(newStatusCmd()) }
