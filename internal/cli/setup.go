package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/glincker/stacklit/internal/derive"
	"github.com/glincker/stacklit/internal/engine"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/setup"
	"github.com/spf13/cobra"
)

type setupFlags struct {
	all, dryRun, uninstall, global, list, report bool
	noMCP, noGitHook                             bool
	preTool, session, skill                      bool
}

func newSetupCmd() *cobra.Command {
	var f setupFlags
	cmd := &cobra.Command{
		Use:   "setup [tool...]",
		Short: "Auto-configure AI coding tools for this repo",
		Long: `Configures AI coding assistants to use Stacklit.

For each tool it writes a marker-delimited codebase map into the tool's
rules file (re-runs replace the block, never duplicate it), registers the
stacklit MCP server where the tool supports project-level config, and
installs a git hook that keeps the map fresh.

Nothing outside the repo is touched unless you pass --global. Every step is
idempotent. Use --dry-run to preview and --uninstall to reverse.

Claude Code extras (opt-in):
  --pretool-hook   point a grepped identifier at its definition and callers
  --session-hook   inject the compact map at session start
  --skill          install the /stacklit skill
These go to .claude/settings.json in the repo, or ~/.claude with --global.

Examples:
  stacklit setup                    Configure tools detected in this repo
  stacklit setup --all              Configure every supported tool
  stacklit setup claude codex       Configure specific tools
  stacklit setup --all --dry-run    Preview the changes
  stacklit setup --all --uninstall  Remove everything setup added
  stacklit setup --list             Show supported tools`,
		RunE: func(cmd *cobra.Command, args []string) error { return runSetup(cmd, args, f) },
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			var ids []string
			for _, t := range setup.Registry() {
				ids = append(ids, t.ID)
			}
			return ids, cobra.ShellCompDirectiveNoFileComp
		},
	}
	fl := cmd.Flags()
	fl.BoolVar(&f.all, "all", false, "Configure every supported tool, not just detected ones")
	fl.BoolVar(&f.dryRun, "dry-run", false, "Print what would change without writing anything")
	fl.BoolVar(&f.uninstall, "uninstall", false, "Remove what setup added")
	fl.BoolVar(&f.global, "global", false, "Allow writing to home-directory configs (Claude settings, Windsurf MCP)")
	fl.BoolVar(&f.list, "list", false, "List supported tools and whether they were detected")
	fl.BoolVar(&f.report, "report", false, "Print the always-on tokens each integration adds to an agent's context")
	fl.BoolVar(&f.noMCP, "no-mcp", false, "Skip MCP server registration")
	fl.BoolVar(&f.noGitHook, "no-git-hook", false, "Skip the post-commit git hook")
	fl.BoolVar(&f.preTool, "pretool-hook", false, "Claude Code: add a PreToolUse hook that points a grepped identifier at its definition and callers (needs the symbol graph)")
	fl.BoolVar(&f.session, "session-hook", false, "Claude Code: add a SessionStart hook that injects the map")
	fl.BoolVar(&f.skill, "skill", false, "Claude Code: install the /stacklit skill")
	return cmd
}

func runSetup(cmd *cobra.Command, args []string, f setupFlags) error {
	out := cmd.OutOrStdout()
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	if f.list {
		for _, t := range setup.Registry() {
			mark := " "
			if t.Detected(root) {
				mark = "*"
			}
			fmt.Fprintf(out, "%s %-13s %s\n", mark, t.ID, t.Name)
		}
		fmt.Fprintln(out, "\n* detected in this environment")
		return nil
	}

	tools, err := pickTools(root, args, f.all)
	if err != nil {
		return err
	}
	if len(tools) == 0 {
		idx, err := loadOrScan(f.dryRun)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "No AI coding tools detected. Codebase map:")
		fmt.Fprintln(out)
		fmt.Fprint(out, derive.CompactMap(idx))
		fmt.Fprintln(out, "\nRun 'stacklit setup --all' or name a tool (see 'stacklit setup --list').")
		return nil
	}

	if f.report {
		idx, err := loadOrScan(true)
		if err != nil {
			return err
		}
		exe, _ := os.Executable()
		rows := setup.Report(idx, tools, setup.MCPSchemaTokens(exe, root))
		fmt.Fprintf(out, "%-48s %7s  %s\n", "integration", "tokens", "when")
		for _, r := range rows {
			tok := fmt.Sprintf("%d", r.Tokens)
			if r.Tokens < 0 {
				tok = "n/a"
			}
			fmt.Fprintf(out, "%-48s %7s  %s\n", r.Item, tok, r.Note)
		}
		fmt.Fprintln(out, "\ntokens = bytes/4 approximation. Rules rows load on every session of that tool.")
		return nil
	}

	var idx *schema.Index
	if !f.uninstall {
		if idx, err = loadOrScan(f.dryRun); err != nil {
			return err
		}
	}

	r := &setup.Runner{Root: root, DryRun: f.dryRun, Out: out}
	err = setup.Run(r, setup.Options{
		Root: root, Tools: tools, Index: idx,
		Uninstall: f.uninstall, NoMCP: f.noMCP, NoGitHook: f.noGitHook, Global: f.global,
		Claude: setup.ClaudeOptions{
			PreToolHook: f.preTool, SessionHook: f.session, Skill: f.skill, GlobalScope: f.global,
		},
	})
	if err != nil {
		return err
	}
	if idx != nil && !f.uninstall {
		body := setup.MarkdownBody(idx)
		fmt.Fprintf(out, "\ninjected digest: %d tokens (%d bytes, bytes/4)\n", setup.Tokens(body), len(body))
	}
	switch {
	case f.dryRun:
		fmt.Fprintf(out, "\nDry run: %d change(s) pending, nothing written.\n", r.Changed)
	case f.uninstall:
		fmt.Fprintf(out, "\nRemoved stacklit from %d file(s).\n", r.Changed)
	default:
		fmt.Fprintf(out, "\nDone, %d file(s) changed. Your AI tools now have codebase context.\n", r.Changed)
	}
	return nil
}

func pickTools(root string, args []string, all bool) ([]setup.Tool, error) {
	if len(args) > 0 {
		var tools []setup.Tool
		for _, a := range args {
			t, ok := setup.Lookup(strings.ToLower(a))
			if !ok {
				return nil, fmt.Errorf("unknown tool %q (see 'stacklit setup --list')", a)
			}
			tools = append(tools, t)
		}
		return tools, nil
	}
	if all {
		return setup.Registry(), nil
	}
	var tools []setup.Tool
	for _, t := range setup.Registry() {
		// The generic AGENTS.md entry only matters when no named tool claims it.
		if t.Detected(root) {
			tools = append(tools, t)
		}
	}
	return tools, nil
}

// loadOrScan reads stacklit.json, or builds one. In dry-run it never writes,
// so a missing index yields a minimal stand-in.
func loadOrScan(dryRun bool) (*schema.Index, error) {
	if data, err := os.ReadFile("stacklit.json"); err == nil {
		var idx schema.Index
		if json.Unmarshal(data, &idx) == nil {
			return &idx, nil
		}
	}
	if dryRun {
		wd, _ := os.Getwd()
		return &schema.Index{
			Project: schema.Project{Name: filepath.Base(wd)},
			Modules: map[string]schema.ModuleInfo{},
		}, nil
	}
	fmt.Println("[stacklit] scanning codebase...")
	result, err := engine.Run(engine.Options{Root: ".", Quiet: false})
	if err != nil {
		return nil, fmt.Errorf("scanning codebase: %w", err)
	}
	return result.Index, nil
}
