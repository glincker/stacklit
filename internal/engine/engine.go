package engine

import (
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
)

// Options configures an engine Run.
type Options struct {
	Root        string
	Workspace   string
	InstallHook bool
	Quiet       bool
	Summary     bool
	// NoWrite builds the index in memory only (used by multi-repo scans so
	// member repos are never modified).
	NoWrite bool
}

// Result holds the output paths and assembled index from a Run.
type Result struct {
	JSONPath    string
	HTMLPath    string
	MermaidPath string
	Index       *schema.Index
	Duration    time.Duration
	// Imports maps module name to its sorted unique import strings. Only
	// filled when Options.NoWrite is set.
	Imports map[string][]string
	// Provides lists manifest names (root and workspaces) this repo publishes.
	Provides []string
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
	"middleware": "HTTP middleware",
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

func filterRetainedModules(names []string, retained map[string]bool) []string {
	if len(names) == 0 {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, name := range names {
		if retained[name] {
			out = append(out, name)
		}
	}
	return out
}

func rankMostDepended(modules map[string]schema.ModuleInfo) []string {
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		ci := len(modules[names[i]].DependedBy)
		cj := len(modules[names[j]].DependedBy)
		if ci != cj {
			return ci > cj
		}
		return names[i] < names[j]
	})
	return names
}

func findIsolatedModules(modules map[string]schema.ModuleInfo) []string {
	var isolated []string
	for name, mod := range modules {
		if len(mod.DependsOn) == 0 && len(mod.DependedBy) == 0 {
			isolated = append(isolated, name)
		}
	}
	sort.Strings(isolated)
	return isolated
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
	if err := config.Check(root); err != nil {
		return nil, err
	}
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

	// 3-4, 7, 9. Walk, parse in parallel (with cache) and hash.
	sc, err := scanRepo(root, cfg, opts.Quiet)
	if err != nil {
		return nil, err
	}
	files, parsed := sc.Files, sc.Parsed

	// 5. Build dependency graph with config-driven limits.
	g := graph.Build(parsed, graph.BuildOptions{
		MaxDepth:   cfg.MaxDepth,
		MaxModules: cfg.MaxModules,
		Root:       root,
		SplitFiles: cfg.ModuleSplitFiles,
	})

	// 6. Get git activity (optional: it is the main source of churn in a committed index).
	activity := &git.Activity{}
	if cfg.GitActivityEnabled() {
		if a, gerr := git.GetActivity(root, 90); gerr == nil {
			activity = a
		}
	}

	// 8. Assemble schema.Index.
	idx := assembleIndex(root, mono, files, parsed, g, activity, map[string][]byte{}, cfg)
	idx.Hints.EnvVars = mergeEnvVars(root, sc.EnvVars)
	idx.Scan = sc.Info
	if mono.Type == "monorepo" {
		names := make([]string, 0, len(idx.Modules))
		for n := range idx.Modules {
			names = append(names, n)
		}
		idx.Packages, idx.PackageEdges = buildPackages(root, mono.Workspaces, parsed, idx.Dependencies.Edges, names)
	}
	idx.MerkleHash = sc.MerkleHash
	renderer.SetTimestamps(cfg.TimestampsEnabled())

	// 10. Generate AI summary if requested.
	if opts.Summary {
		if !opts.Quiet {
			fmt.Println("[stacklit] generating AI summary...")
		}
		text, summaryErr := summary.Run(idx)
		if summaryErr != nil {
			fmt.Printf("[stacklit] warning: summary failed: %v\n", summaryErr)
		} else {
			idx.Architecture = schema.Architecture{Summary: text}
		}
	}

	jsonPath := filepath.Join(root, cfg.Output.JSON)
	mmdPath := filepath.Join(root, cfg.Output.Mermaid)
	htmlPath := filepath.Join(root, cfg.Output.HTML)

	if opts.NoWrite {
		return &Result{
			Index:    idx,
			Duration: time.Since(start),
			Imports:  importsByModule(parsed, idx.Modules),
			Provides: repoProvides(root, mono),
		}, nil
	}

	// 11. Write outputs using config-driven paths.
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
		if _, hookErr := git.InstallRefreshHooks(root); hookErr != nil && !opts.Quiet {
			fmt.Printf("[stacklit] warning: could not install refresh hooks: %v\n", hookErr)
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
	detectedPatterns := detect.DetectFrameworkPatterns(root)
	frameworkPatterns := make([]schema.FrameworkPattern, len(detectedPatterns))
	for i, p := range detectedPatterns {
		frameworkPatterns[i] = schema.FrameworkPattern{
			Name:       p.Name,
			Config:     p.Config,
			Routes:     p.Routes,
			API:        p.API,
			Middleware: p.Middleware,
			Models:     p.Models,
			Entry:      p.Entry,
		}
	}
	primaryLang := ""
	primaryCount := 0
	// Config files only decide the language when nothing else is present.
	for pass := 0; pass < 2 && primaryLang == ""; pass++ {
		for lang, ls := range langStats {
			if pass == 0 && parser.IsDataLanguage(lang) {
				continue
			}
			if ls.Files > primaryCount || (ls.Files == primaryCount && lang < primaryLang) {
				primaryCount = ls.Files
				primaryLang = lang
			}
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

	retainedModules := make(map[string]bool, len(allMods))
	for _, mod := range allMods {
		if strings.HasPrefix(mod.Name, "testdata") {
			continue
		}
		retainedModules[mod.Name] = true
	}

	modules := map[string]schema.ModuleInfo{}
	for _, mod := range allMods {
		if strings.HasPrefix(mod.Name, "testdata") {
			continue
		}
		exports := mod.ExportsByRank
		if maxExports > 0 && len(exports) > maxExports {
			exports = exports[:maxExports]
		}
		fileList := mod.FilesByRank
		if cfg.MaxModuleFiles > 0 && len(fileList) > cfg.MaxModuleFiles {
			fileList = fileList[:cfg.MaxModuleFiles]
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
			DependsOn:  filterRetainedModules(mod.DependsOn, retainedModules),
			DependedBy: filterRetainedModules(mod.DependedBy, retainedModules),
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
		if !retainedModules[e.From] || !retainedModules[e.To] {
			continue
		}
		schemaEdges = append(schemaEdges, [2]string{e.From, e.To})
	}
	mostDepended := rankMostDepended(modules)
	// Cap most-depended list.
	if len(mostDepended) > 10 {
		mostDepended = mostDepended[:10]
	}
	isolated := findIsolatedModules(modules)

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
			PrimaryLanguage:   primaryLang,
			Languages:         langStats,
			Frameworks:        frameworks,
			FrameworkPatterns: frameworkPatterns,
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
