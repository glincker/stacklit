package cli

import (
	"github.com/glincker/stacklit/internal/mcp"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var profile string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Start MCP server for AI agent integration (stdio)",
		Long: `Starts a Model Context Protocol (MCP) server over stdio.

The server reads stacklit.json and exposes tools for AI agents to query
the codebase index. Run 'stacklit init' first to generate stacklit.json.

Tools exposed:
  get_overview        Project info, tech stack, entry points
  get_module          Full info for a specific module
  find_module         Search modules by name or purpose
  get_dependencies    Dependency edges for a module
  get_hot_files       Most frequently changed files (git)
  get_hints           Workflow hints and commands
  query_graph, shortest_path, explain_symbol, impact, get_hubs,
  get_communities, graph_report, slice_symbol   Symbol graph tools

Use --profile minimal to expose only query, slice, impact and map (about 230
tokens of tool schema instead of about 1850).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mcp.StartServerProfile(profile)
		},
	}
	c.Flags().StringVar(&profile, "profile", mcp.ProfileFull, "Tool profile: full (all tools) or minimal (query, slice, impact, map)")
	return c
}
