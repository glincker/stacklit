package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/glincker/stacklit/internal/engine"
	"github.com/glincker/stacklit/internal/session"
)

// postToolInput is the slice of a PostToolUse payload refresh needs.
type postToolInput struct {
	Cwd       string `json:"cwd"`
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

// runPostToolRefresh is the async PostToolUse hook: it never prints and never
// fails the edit loop, whatever happens.
func runPostToolRefresh(cmd *cobra.Command) error {
	b, _ := io.ReadAll(cmd.InOrStdin())
	var in postToolInput
	if json.Unmarshal(b, &in) != nil {
		return nil
	}
	p := in.ToolInput.FilePath
	if p == "" {
		p = in.ToolInput.NotebookPath
	}
	if p == "" {
		return nil
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = filepath.Dir(p)
	}
	_, _ = engine.Refresh(engine.RefreshOptions{Root: session.Root(cwd), Paths: []string{p}})
	return nil
}

func newRefreshCmd() *cobra.Command {
	var root string
	var files []string
	var fromStdin, asJSON bool
	c := &cobra.Command{
		Use:   "refresh",
		Short: "Update the index and symbol graph for just the files that changed",
		Long: `Updates stacklit.json (and stacklit.graph.json when present) for the given
files without rescanning the repo: unchanged files are skipped by content hash.
Safe to run from several sessions or worktrees at once; callers that find the
lock held queue their paths for the holder.

With --stdin it reads a Claude Code hook payload (tool_input.file_path).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths := append(files, args...)
			if fromStdin {
				var in postToolInput
				if b, _ := io.ReadAll(cmd.InOrStdin()); json.Unmarshal(b, &in) == nil {
					paths = append(paths, in.ToolInput.FilePath, in.ToolInput.NotebookPath)
					if in.Cwd != "" && root == "." {
						root = in.Cwd
					}
				}
			}
			r := root
			if r == "." || r == "" {
				wd, _ := os.Getwd()
				r = session.Root(wd)
			}
			res, err := engine.Refresh(engine.RefreshOptions{Root: r, Paths: paths})
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "refresh: %s, %d parsed, %d removed, structure %s%s, %s\n",
				res.Status, res.Parsed, res.Removed, res.StructureHash, map[bool]string{true: " (changed)", false: ""}[res.StructureChanged], res.Duration.Round(1000000))
			return nil
		},
	}
	c.Flags().StringVar(&root, "root", ".", "Project root (default: the worktree containing the current directory)")
	c.Flags().StringSliceVar(&files, "files", nil, "Changed files, comma separated or repeated")
	c.Flags().BoolVar(&fromStdin, "stdin", false, "Read a hook JSON payload on stdin")
	c.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return c
}

func newDigestCmd() *cobra.Command {
	var root, sid string
	var check, asJSON bool
	c := &cobra.Command{
		Use:   "digest",
		Short: "Print the orientation digest, or --check whether it changed since injection",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := root
			if r == "." || r == "" {
				wd, _ := os.Getwd()
				r = session.Root(wd)
			}
			idx, err := session.LoadIndex(r)
			if err != nil {
				return &ExitError{Code: 2, Err: fmt.Errorf("no index: run stacklit init")}
			}
			hash := session.StructureHash(idx)
			if !check {
				if asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"structure_hash": hash})
				}
				fmt.Fprintln(cmd.OutOrStdout(), hash)
				return nil
			}
			was := session.ReadInjected(session.StateDir(r)).For(sid)
			switch {
			case was == "":
				return &ExitError{Code: 2, Err: fmt.Errorf("no digest recorded as injected yet (current %s)", hash)}
			case was != hash:
				return &ExitError{Code: 1, Err: fmt.Errorf("structure changed: injected %s, now %s", was, hash)}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "current %s\n", strings.TrimSpace(hash))
			return nil
		},
	}
	c.Flags().StringVar(&root, "root", ".", "Project root")
	c.Flags().StringVar(&sid, "session", "", "Session id to compare against (default: the latest injection)")
	c.Flags().BoolVar(&check, "check", false, "Exit 1 when the structure hash differs from the one injected, 2 when none was recorded")
	c.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return c
}

func init() { rootCmd.AddCommand(newRefreshCmd(), newDigestCmd()) }
