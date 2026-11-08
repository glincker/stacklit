package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/glincker/stacklit/internal/schema"
)

var cachedIndex *schema.Index
var cachedIndexModTime time.Time

func loadIndex() (*schema.Index, error) {
	const indexFile = "stacklit.json"

	info, err := os.Stat(indexFile)
	if err != nil {
		return nil, fmt.Errorf("stacklit.json not found — run 'stacklit init' first")
	}

	if cachedIndex != nil && !info.ModTime().After(cachedIndexModTime) {
		return cachedIndex, nil
	}

	data, err := os.ReadFile(indexFile)
	if err != nil {
		return nil, fmt.Errorf("stacklit.json not found — run 'stacklit init' first")
	}
	var idx schema.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("failed to parse stacklit.json: %w", err)
	}
	cachedIndex = &idx
	cachedIndexModTime = info.ModTime()
	return cachedIndex, nil
}

func textResult(v any) (*sdkmcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal error: %w", err)
	}
	return &sdkmcp.CallToolResult{
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}

func errResult(err error) (*sdkmcp.CallToolResult, any, error) {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{
			&sdkmcp.TextContent{Text: err.Error()},
		},
	}, nil, nil
}

// StartServer runs the full-profile MCP server over stdio.
func StartServer() error { return StartServerProfile(ProfileFull) }

// StartServerProfile runs the MCP server for the named profile over stdio.
func StartServerProfile(profile string) error {
	s, err := NewServer(profile)
	if err != nil {
		return err
	}
	err = s.Run(context.Background(), &sdkmcp.StdioTransport{})
	// EOF on stdin is normal shutdown, not an error to report.
	if err != nil && strings.Contains(err.Error(), "EOF") {
		return nil
	}
	return err
}

// NewServer builds the MCP server with the tools of one profile.
func NewServer(profile string) (*sdkmcp.Server, error) {
	s := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "stacklit", Version: "0.1.0"},
		&sdkmcp.ServerOptions{Instructions: Instructions(profile)})
	switch profile {
	case "", ProfileFull:
	case ProfileMinimal:
		registerMinimalTools(s)
		return s, nil
	default:
		return nil, fmt.Errorf("unknown profile %q (use %s or %s)", profile, ProfileFull, ProfileMinimal)
	}

	// get_overview — project info, tech stack, entry points
	type emptyArgs struct{}
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_overview",
		Description: "Get codebase overview: project info, tech stack, entry points, file counts",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ emptyArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		overview := map[string]any{
			"project":          idx.Project.Name,
			"type":             idx.Project.Type,
			"primary_language": idx.Tech.PrimaryLanguage,
			"languages":        idx.Tech.Languages,
			"frameworks":       idx.Tech.Frameworks,
			"entrypoints":      idx.Structure.Entrypoints,
			"total_files":      idx.Structure.TotalFiles,
			"total_lines":      idx.Structure.TotalLines,
		}
		return textResult(overview)
	})

	// get_module — look up a specific module by name
	type getModuleArgs struct {
		Name string `json:"name" jsonschema:"the exact module name to look up"`
	}
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_module",
		Description: "Get full info for a specific module: purpose, exports, dependencies",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, args getModuleArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		mod, ok := idx.Modules[args.Name]
		if !ok {
			return errResult(fmt.Errorf("module %q not found", args.Name))
		}
		return textResult(map[string]any{
			"name":   args.Name,
			"module": mod,
		})
	})

	// find_module — fuzzy search by name or purpose
	type findModuleArgs struct {
		Query string `json:"query" jsonschema:"substring to search in module names and purpose"`
	}
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "find_module",
		Description: "Search modules by name or purpose (case-insensitive substring match, returns top 5)",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, args findModuleArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		q := strings.ToLower(args.Query)
		type match struct {
			Name   string            `json:"name"`
			Module schema.ModuleInfo `json:"module"`
		}
		var results []match
		for name, mod := range idx.Modules {
			if strings.Contains(strings.ToLower(name), q) ||
				strings.Contains(strings.ToLower(mod.Purpose), q) {
				results = append(results, match{Name: name, Module: mod})
			}
			if len(results) >= 5 {
				break
			}
		}
		return textResult(results)
	})

	// get_dependencies — edges for a specific module
	type getDepsArgs struct {
		Module string `json:"module" jsonschema:"module name to get dependency edges for"`
	}
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_dependencies",
		Description: "Get dependency edges (imports/imports-by) for a specific module",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, args getDepsArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		var edges [][2]string
		for _, edge := range idx.Dependencies.Edges {
			if edge[0] == args.Module || edge[1] == args.Module {
				edges = append(edges, edge)
			}
		}
		return textResult(map[string]any{
			"module": args.Module,
			"edges":  edges,
		})
	})

	// list_modules — all modules with summary info, sorted by name
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "list_modules",
		Description: "List all modules with their purpose, file/line counts, and activity — sorted by name",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ emptyArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		type moduleSummary struct {
			Name     string `json:"name"`
			Purpose  string `json:"purpose"`
			Files    int    `json:"files"`
			Lines    int    `json:"lines"`
			Activity string `json:"activity,omitempty"`
		}
		names := make([]string, 0, len(idx.Modules))
		for name := range idx.Modules {
			names = append(names, name)
		}
		sort.Strings(names)
		summaries := make([]moduleSummary, 0, len(names))
		for _, name := range names {
			mod := idx.Modules[name]
			summaries = append(summaries, moduleSummary{
				Name:     name,
				Purpose:  mod.Purpose,
				Files:    mod.Files,
				Lines:    mod.Lines,
				Activity: mod.Activity,
			})
		}
		return textResult(summaries)
	})

	// get_hot_files — git churn list
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_hot_files",
		Description: "Get the most frequently changed files (git hot files) in the last 90 days",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ emptyArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		return textResult(idx.Git.HotFiles)
	})

	// get_hints — workflow hints
	sdkmcp.AddTool(s, &sdkmcp.Tool{
		Name:        "get_hints",
		Description: "Get codebase hints: add_feature guidance, test command, env vars, do-not-touch list",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, _ emptyArgs) (*sdkmcp.CallToolResult, any, error) {
		idx, err := loadIndex()
		if err != nil {
			return errResult(err)
		}
		return textResult(idx.Hints)
	})

	registerGraphTools(s)
	return s, nil
}
