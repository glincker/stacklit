package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/detect"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/monorepo"
	"github.com/glincker/stacklit/internal/parser"
	"github.com/glincker/stacklit/internal/renderer"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/summary"
	"github.com/glincker/stacklit/internal/walker"
)

// Options configures an engine Run.
type Options struct {
	Root        string
	Workspace   string
	InstallHook bool
	Quiet       bool
	Summary     bool
}

// Result holds the output paths and assembled index from a Run.
type Result struct {
	JSONPath    string
	HTMLPath    string
	MermaidPath string
	Index       *schema.Index
	Duration    time.Duration
}

// purposeMap maps common directory names to human-readable descriptions.
var purposeMap = map[string]string{
	"auth":       "Authentication and authorization",
	"api":        "API endpoints and handlers",
	"db":         "Database access layer",
	"models":     "Data models and types",
	"config":     "Configuration management",
	"components": "UI components",
	"hooks":      "React hooks",
	"cmd":        "Application entrypoints",
	"internal":   "Private application packages",
	"pkg":        "Public packages",
	"lib":        "Shared library code",
	"utils":      "Utility functions",
	"services":   "Business logic services",
	"middleware":  "HTTP middleware",
	"cli":        "Command-line interface",
	"schema":     "Data schema definitions",
	"renderer":   "Output renderers",
	"walker":     "File system walker",
	"graph":      "Dependency graph",
	"engine":     "Core orchestration engine",
	"git":        "Git integration",
	"assets":     "Static assets",
	"parser":     "Source code parsers",
	"monorepo":   "Monorepo detection",
	"detect":     "Framework and tool detection",
	"mcp":        "MCP server for AI agents",
	"summary":    "AI-powered codebase summaries",
}

// inferPurpose returns a human-readable description for a module path.
func inferPurpose(name string) string {
	// Use the last path segment for lookup.
	base := filepath.Base(name)
	if desc, ok := purposeMap[base]; ok {
		return desc
	}
	// Fall back to the name itself, capitalised.
	if base == "." || base == "" || base == "root" {
		return "Root package"
	}
	words := strings.Fields(strings.ReplaceAll(base, "_", " "))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// generateAddFeatureHint returns a hint string describing where to add a new feature.
func generateAddFeatureHint(modules map[string]schema.ModuleInfo, entrypoints []string) string {
	for name := range modules {
		base := filepath.Base(name)
		if base == "api" || base == "handler" || base == "handlers" ||
			strings.Contains(name, "/api") || strings.Contains(name, "/handler") {
			if len(entrypoints) > 0 {
				return fmt.Sprintf("Add handler in %s, register in %s", name, entrypoints[0])
			}
			return fmt.Sprintf("Add handler in %s", name)
		}
	}
	if len(entrypoints) > 0 {
		return fmt.Sprintf("Start from entrypoint %s", entrypoints[0])
	}
	return ""
}

// detectDoNotTouch returns paths that should generally not be modified by hand.
func detectDoNotTouch(root string) []string {
	candidates := []string{"migrations", "vendor", "generated", "proto", ".github"}
	var paths []string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(root, c)); err == nil {
			paths = append(paths, c+"/")
		}
	}
	return paths
}

// detectTestCommand inspects root for well-known build files and returns the
// appropriate test command string.
func detectTestCommand(root string) string {
	checks := []struct {
		file string
		cmd  string
	}{
		{"package.json", "npm test"},
		{"go.mod", "go test ./..."},
		{"requirements.txt", "pytest"},
		{"pyproject.toml", "pytest"},
		{"Cargo.toml", "cargo test"},
		{"Makefile", "make test"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(root, c.file)); err == nil {
			return c.cmd
		}
	}
	return ""
}

// Run executes the full stacklit pipeline and returns the result.
func Run(opts Options) (*Result, error) {
	start := time.Now()

	// 1. Resolve absolute root.
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolving root: %w", err)
	}

	// 1a. Load config (best-effort; uses defaults if absent).
	cfg := config.Load(root)

	// 2. Detect monorepo layout.
	mono, err := monorepo.Detect(root)
	if err != nil {
		// Non-fatal — treat as single repo.
		mono = &monorepo.Result{Type: "single"}
	}
	if !opts.Quiet && mono.Type == "monorepo" {
		fmt.Printf("[stacklit] monorepo detected: %s (%d workspaces)\n", mono.Tool, len(mono.Workspaces))
	}

	// 3. Walk the filesystem, honouring extra ignore patterns from config.
	files, err := walker.Walk(root, cfg.ScanIgnore())
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	if !opts.Quiet {
		fmt.Printf("[stacklit] found %d files\n", len(files))
	}

	// 4. Parse all files.
	parsed, parseErrs := parser.ParseAll(files)
	if !opts.Quiet {
		fmt.Printf("[stacklit] parsed %d files (%d errors)\n", len(parsed), len(parseErrs))
	}

	// 5. Build dependency graph with config-driven limits.
	g := graph.Build(parsed, graph.BuildOptions{
		MaxDepth:   cfg.MaxDepth,
		MaxModules: cfg.MaxModules,
	})

	// 6. Get git activity.
	activity, err := git.GetActivity(root, 90)
	if err != nil {
		// Non-fatal.
		activity = &git.Activity{}
	}

	// 7. Read file contents (needed for Merkle and env var detection).
	contents := make(map[string][]byte, len(files))
	for _, f := range files {
		data, readErr := os.ReadFile(f)
		if readErr == nil {
			contents[f] = data
		}
	}

	// 8. Assemble schema.Index.
	idx := assembleIndex(root, mono, files, parsed, g, activity, contents, cfg)

	// 9. Compute Merkle hash.
	idx.MerkleHash = git.ComputeMerkle(files, contents)

	// 10. Generate AI summary if requested.
	if opts.Summary {
		if !opts.Quiet {
			fmt.Println("[stacklit] generating AI summary...")
		}
		text, summaryErr := summary.Generate(idx)
		if summaryErr != nil {
			fmt.Printf("[stacklit] warning: summary failed: %v\n", summaryErr)
		} else {
			idx.Architecture = schema.Architecture{Summary: text}
		}
	}

	// 11. Write outputs using config-driven paths.
	jsonPath := filepath.Join(root, cfg.Output.JSON)
	mmdPath := filepath.Join(root, cfg.Output.Mermaid)
	htmlPath := filepath.Join(root, cfg.Output.HTML)

	if err := renderer.WriteJSON(idx, jsonPath); err != nil {
		return nil, fmt.Errorf("writing JSON: %w", err)
	}
	if err := renderer.WriteMermaid(idx, mmdPath); err != nil {
		return nil, fmt.Errorf("writing Mermaid: %w", err)
	}
	if err := renderer.WriteHTML(idx, htmlPath); err != nil {
		return nil, fmt.Errorf("writing HTML: %w", err)
	}

	// 12. Install git hook if requested.
	if opts.InstallHook {
		if hookErr := git.InstallHook(root); hookErr != nil && !opts.Quiet {
			fmt.Printf("[stacklit] warning: could not install hook: %v\n", hookErr)
		}
	}

	dur := time.Since(start)

	// 13. Print summary.
	if !opts.Quiet {
		fmt.Printf("[stacklit] done in %s — wrote %s, %s, %s\n",
			dur.Round(time.Millisecond),
			filepath.Base(jsonPath), filepath.Base(mmdPath), filepath.Base(htmlPath))
	}

	return &Result{
		JSONPath:    jsonPath,
		HTMLPath:    htmlPath,
		MermaidPath: mmdPath,
		Index:       idx,
		Duration:    dur,
	}, nil
}

// MultiOptions configures a RunMulti call.
type MultiOptions struct {
	ReposFile string
	Quiet     bool
}

// MultiResult holds the output of a RunMulti call.
type MultiResult struct {
	OutputPath string
	RepoCount  int
	Duration   time.Duration
}

// RunMulti reads a repos file, scans each repo, and writes a combined stacklit-multi.json.
func RunMulti(opts MultiOptions) (*MultiResult, error) {
	start := time.Now()

	// 1. Read repos file.
	data, err := os.ReadFile(opts.ReposFile)
	if err != nil {
		return nil, fmt.Errorf("reading repos file: %w", err)
	}

	// 2. Parse lines (skip empty lines and comments).
	var repos []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		repos = append(repos, line)
	}

	// 3. Run stacklit on each repo.
	var indices []*schema.Index
	for _, repo := range repos {
		if !opts.Quiet {
			fmt.Printf("[stacklit] scanning %s...\n", repo)
		}
		result, err := Run(Options{Root: repo, Quiet: true})
		if err != nil {
			fmt.Printf("[stacklit] warning: failed to scan %s: %v\n", repo, err)
			continue
		}
		indices = append(indices, result.Index)
	}

	// 4. Generate cross-repo summary.
	multi := buildMultiIndex(indices)

	// 5. Write to stacklit-multi.json in current directory.
	outputPath := "stacklit-multi.json"
	multiData, err := json.MarshalIndent(multi, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling multi-index: %w", err)
	}
	if err := os.WriteFile(outputPath, append(multiData, '\n'), 0644); err != nil {
		return nil, fmt.Errorf("writing %s: %w", outputPath, err)
	}

	dur := time.Since(start)
	if !opts.Quiet {
		fmt.Printf("[stacklit] multi-repo scan done in %s — %d repos, %d modules, %d files\n",
			dur.Round(time.Millisecond), len(indices), multi.TotalModules, multi.TotalFiles)
	}

	return &MultiResult{OutputPath: outputPath, RepoCount: len(indices), Duration: dur}, nil
}

// buildMultiIndex assembles a MultiIndex from individual repo indices.
func buildMultiIndex(indices []*schema.Index) *schema.MultiIndex {
	multi := &schema.MultiIndex{
		Schema:      "https://stacklit.dev/schema/v1-multi.json",
		Version:     "1",
		Type:        "polyrepo",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for _, idx := range indices {
		multi.TotalFiles += idx.Structure.TotalFiles
		multi.TotalLines += idx.Structure.TotalLines
		multi.TotalModules += len(idx.Modules)

		multi.Repos = append(multi.Repos, schema.RepoSummary{
			Name:            idx.Project.Name,
			Path:            idx.Project.Root,
			PrimaryLanguage: idx.Tech.PrimaryLanguage,
			TotalFiles:      idx.Structure.TotalFiles,
			TotalLines:      idx.Structure.TotalLines,
			Modules:         idx.Modules,
			Frameworks:      idx.Tech.Frameworks,
			Entrypoints:     idx.Structure.Entrypoints,
		})
	}

	return multi
}

// assembleIndex builds a schema.Index from the pipeline outputs.
func assembleIndex(
	root string,
	mono *monorepo.Result,
	files []string,
	parsed []*parser.FileInfo,
	g *graph.Graph,
	activity *git.Activity,
	contents map[string][]byte,
	cfg *config.Config,
) *schema.Index {
	// --- Project ---
	projectName := filepath.Base(root)
	projectType := mono.Type

	// --- Tech: count languages and collect imports ---
	langStats := map[string]schema.LangStats{}
	totalLines := 0
	var allImports []string
	for _, f := range parsed {
		lang := strings.ToLower(f.Language)
		ls := langStats[lang]
		ls.Files++
		ls.Lines += f.LineCount
		langStats[lang] = ls
		totalLines += f.LineCount
		// Skip testdata imports for framework detection
		if !strings.HasPrefix(f.Path, "testdata") {
			allImports = append(allImports, f.Imports...)
		}
	}

	// --- Frameworks ---
	frameworks := detect.DetectFrameworks(root, allImports)
	primaryLang := ""
	primaryCount := 0
	for lang, ls := range langStats {
		if ls.Files > primaryCount {
			primaryCount = ls.Files
			primaryLang = lang
		}
	}

	// --- Structure ---
	rawEntrypoints := g.Entrypoints()
	var entrypoints []string
	for _, ep := range rawEntrypoints {
		if !strings.HasPrefix(ep, "testdata") {
			entrypoints = append(entrypoints, ep)
		}
	}

	// --- Modules ---
	// maxModules and maxExports come from config.
	maxModules := cfg.MaxModules
	maxExports := cfg.MaxExports

	allMods := g.Modules()

	// If we exceed the cap, keep the modules with the most files.
	if maxModules > 0 && len(allMods) > maxModules {
		sort.SliceStable(allMods, func(i, j int) bool {
			return allMods[i].FileCount > allMods[j].FileCount
		})
		allMods = allMods[:maxModules]
	}

	modules := map[string]schema.ModuleInfo{}
	for _, mod := range allMods {
		if strings.HasPrefix(mod.Name, "testdata") {
			continue
		}
		exports := mod.Exports
		if maxExports > 0 && len(exports) > maxExports {
			exports = exports[:maxExports]
		}
		fileList := mod.Files
		if len(fileList) > 20 {
			fileList = fileList[:20]
		}
		// Compute activity level from git hot files
		activityLevel := "low"
		for _, hf := range activity.HotFiles {
			if strings.HasPrefix(hf.Path, mod.Name+"/") {
				if hf.Commits90d > 10 {
					activityLevel = "high"
					break
				} else if hf.Commits90d > 3 {
					activityLevel = "medium"
				}
			}
		}

		// Cap type defs to maxExports per module in a deterministic order.
		typeDefs := mod.TypeDefs
		if maxExports > 0 && len(typeDefs) > maxExports {
			keys := make([]string, 0, len(typeDefs))
			for k := range typeDefs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			trimmed := make(map[string]string, maxExports)
			for i, k := range keys {
				if i >= maxExports {
					break
				}
				trimmed[k] = typeDefs[k]
			}
			typeDefs = trimmed
		}

		modules[mod.Name] = schema.ModuleInfo{
			Purpose:    inferPurpose(mod.Name),
			Language:   mod.PrimaryLanguage,
			Files:      mod.FileCount,
			Lines:      mod.LineCount,
			FileList:   fileList,
			Exports:    exports,
			TypeDefs:   typeDefs,
			DependsOn:  mod.DependsOn,
			DependedBy: mod.DependedBy,
			Activity:   activityLevel,
		}
	}

	// --- Dependencies ---
	edges := g.Edges()
	var schemaEdges [][2]string
	for _, e := range edges {
		if strings.HasPrefix(e.From, "testdata") || strings.HasPrefix(e.To, "testdata") {
			continue
		}
		schemaEdges = append(schemaEdges, [2]string{e.From, e.To})
	}
	mostDepended := g.MostDepended()
	// Cap most-depended list.
	if len(mostDepended) > 10 {
		mostDepended = mostDepended[:10]
	}
	isolated := g.Isolated()

	// --- Git ---
	hotFiles := make([]schema.HotFile, len(activity.HotFiles))
	for i, hf := range activity.HotFiles {
		hotFiles[i] = schema.HotFile{Path: hf.Path, Commits90d: hf.Commits90d}
	}

	// --- Hints ---
	testCmd := detectTestCommand(root)
	envVars := detect.DetectEnvVars(root, contents)
	addFeature := generateAddFeatureHint(modules, entrypoints)
	doNotTouch := detectDoNotTouch(root)

	// --- Workspaces ---
	var workspaces []string
	if mono != nil {
		workspaces = mono.Workspaces
	}

	return &schema.Index{
		Version: "1",
		Project: schema.Project{
			Name:       projectName,
			Root:       ".",
			Type:       projectType,
			Workspaces: workspaces,
		},
		Tech: schema.Tech{
			PrimaryLanguage: primaryLang,
			Languages:       langStats,
			Frameworks:      frameworks,
		},
		Structure: schema.Structure{
			TotalFiles:  len(files),
			TotalLines:  totalLines,
			Entrypoints: entrypoints,
		},
		Modules: modules,
		Dependencies: schema.Dependencies{
			Edges:        schemaEdges,
			Entrypoints:  entrypoints,
			MostDepended: mostDepended,
			Isolated:     isolated,
		},
		Git: schema.GitInfo{
			HotFiles: hotFiles,
			Recent:   activity.RecentFiles,
			Stable:   activity.StableFiles,
		},
		Hints: schema.Hints{
			TestCmd:    testCmd,
			EnvVars:    envVars,
			AddFeature: addFeature,
			DoNotTouch: doNotTouch,
		},
	}
}
