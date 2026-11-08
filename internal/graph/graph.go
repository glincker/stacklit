package graph

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/parser"
)

// defaultMaxModuleDepth is the default maximum path depth for module detection.
const defaultMaxModuleDepth = 4

// skipModuleSegments is the set of path segment names that indicate a module
// (or any ancestor) should be excluded from the graph.
var skipModuleSegments = map[string]bool{
	"test":         true,
	"tests":        true,
	"__tests__":    true,
	"__test__":     true,
	"testdata":     true,
	"test_data":    true,
	"fixtures":     true,
	"__fixtures__": true,
	"docs_src":     true,
	"e2e":          true,
	"benchmarks":   true,
	"bench":        true,
}

// shouldSkipModule returns true if any segment of the module path matches a
// known non-essential directory name (tests, fixtures, examples, etc.).
func shouldSkipModule(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if skipModuleSegments[strings.ToLower(part)] {
			return true
		}
	}
	return false
}

// Module represents a logical grouping of source files by directory.
type Module struct {
	Name            string
	FileCount       int
	Files           []string // basenames of files in this module
	Exports         []string
	TypeDefs        map[string]string
	DependsOn       []string
	DependedBy      []string
	Languages       []string
	PrimaryLanguage string
	LineCount       int
	langCounts      map[string]int // internal: file count per language
}

// Edge represents a dependency relationship between two modules.
type Edge struct {
	From  string
	To    string
	Count int // number of imports crossing this edge
}

// Graph holds the full dependency graph built from parsed files.
type Graph struct {
	modules     map[string]*Module
	edges       map[edgeKey]*Edge
	entrypoints []string
}

type edgeKey struct {
	from string
	to   string
}

// BuildOptions configures the behaviour of Build.
type BuildOptions struct {
	// MaxDepth is the maximum number of path segments a module name may have.
	// Deeper paths are collapsed to their ancestor. Defaults to 4 when 0.
	MaxDepth int
	// MaxModules caps how many modules are retained. 0 means no cap.
	MaxModules int
}

// Build constructs a dependency Graph from a slice of parsed FileInfo.
func Build(files []*parser.FileInfo, opts BuildOptions) *Graph {
	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxModuleDepth
	}

	g := &Graph{
		modules: make(map[string]*Module),
		edges:   make(map[edgeKey]*Edge),
	}

	// First pass: create modules and collect entrypoints.
	for _, f := range files {
		mod := detectModuleWithDepth(f.Path, maxDepth)
		if shouldSkipModule(mod) {
			continue
		}
		m, ok := g.modules[mod]
		if !ok {
			m = &Module{Name: mod, langCounts: map[string]int{}}
			g.modules[mod] = m
		}
		m.FileCount++
		m.LineCount += f.LineCount
		m.Exports = appendUnique(m.Exports, f.Exports...)
		m.Languages = appendUnique(m.Languages, f.Language)
		m.Files = append(m.Files, filepath.Base(f.Path))
		if f.Language != "" {
			m.langCounts[f.Language]++
		}
		if len(f.TypeDefs) > 0 {
			if m.TypeDefs == nil {
				m.TypeDefs = map[string]string{}
			}
			for name, def := range f.TypeDefs {
				m.TypeDefs[name] = def
			}
		}

		if f.IsEntrypoint {
			g.entrypoints = append(g.entrypoints, f.Path)
		}
	}

	// Build a set of all known module names for import resolution.
	knownModules := make([]string, 0, len(g.modules))
	for name := range g.modules {
		knownModules = append(knownModules, name)
	}

	// Second pass: resolve imports to edges.
	for _, f := range files {
		fromMod := detectModuleWithDepth(f.Path, maxDepth)
		if shouldSkipModule(fromMod) {
			continue
		}
		fileDir := filepath.Dir(f.Path)

		for _, imp := range f.Imports {
			toMod := resolveImport(imp, fileDir, knownModules)
			if toMod == "" || toMod == fromMod {
				continue
			}
			if shouldSkipModule(toMod) {
				continue
			}
			key := edgeKey{from: fromMod, to: toMod}
			e, ok := g.edges[key]
			if !ok {
				e = &Edge{From: fromMod, To: toMod}
				g.edges[key] = e
			}
			e.Count++
		}
	}

	// Build DependsOn and DependedBy from edges.
	for key := range g.edges {
		from := g.modules[key.from]
		to := g.modules[key.to]
		from.DependsOn = appendUnique(from.DependsOn, key.to)
		to.DependedBy = appendUnique(to.DependedBy, key.from)
	}

	// Sort all slice fields for determinism and compute primary language.
	for _, m := range g.modules {
		sort.Strings(m.Exports)
		sort.Strings(m.DependsOn)
		sort.Strings(m.DependedBy)
		sort.Strings(m.Languages)
		sort.Strings(m.Files)

		// Determine the language with the most files in this module.
		bestLang, bestCount := "", 0
		for lang, count := range m.langCounts {
			if count > bestCount || (count == bestCount && lang < bestLang) {
				bestLang = lang
				bestCount = count
			}
		}
		m.PrimaryLanguage = bestLang
	}
	sort.Strings(g.entrypoints)

	return g
}

// Modules returns all modules sorted by name.
func (g *Graph) Modules() []*Module {
	out := make([]*Module, 0, len(g.modules))
	for _, m := range g.modules {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// Module returns a module by name, or nil if not found.
func (g *Graph) Module(name string) *Module {
	return g.modules[name]
}

// Edges returns all edges sorted by (From, To).
func (g *Graph) Edges() []Edge {
	out := make([]Edge, 0, len(g.edges))
	for _, e := range g.edges {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

// MostDepended returns module names sorted by DependedBy count descending.
// Ties are broken alphabetically.
func (g *Graph) MostDepended() []string {
	mods := g.Modules()
	sort.SliceStable(mods, func(i, j int) bool {
		ci := len(mods[i].DependedBy)
		cj := len(mods[j].DependedBy)
		if ci != cj {
			return ci > cj
		}
		return mods[i].Name < mods[j].Name
	})
	names := make([]string, len(mods))
	for i, m := range mods {
		names[i] = m.Name
	}
	return names
}

// Entrypoints returns file paths that were flagged as IsEntrypoint.
func (g *Graph) Entrypoints() []string {
	return g.entrypoints
}

// Isolated returns the names of modules that have no incoming and no outgoing
// dependency edges, sorted alphabetically.
func (g *Graph) Isolated() []string {
	var isolated []string
	for _, m := range g.modules {
		if len(m.DependsOn) == 0 && len(m.DependedBy) == 0 {
			isolated = append(isolated, m.Name)
		}
	}
	sort.Strings(isolated)
	return isolated
}

// detectModule returns the parent directory of a file path as the module name,
// collapsed to at most defaultMaxModuleDepth segments. Files at the root level
// return "root".
func detectModule(filePath string) string {
	return detectModuleWithDepth(filePath, defaultMaxModuleDepth)
}

// detectModuleWithDepth is like detectModule but uses an explicit depth cap.
func detectModuleWithDepth(filePath string, maxDepth int) string {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" || dir == "/" {
		return "root"
	}
	dir = filepath.ToSlash(dir)
	parts := strings.Split(dir, "/")
	if maxDepth > 0 && len(parts) > maxDepth {
		parts = parts[:maxDepth]
	}
	return strings.Join(parts, "/")
}

// resolveImport attempts to match an import string to a known module name.
// Returns the matched module name, or "" if no match (external dependency).
func resolveImport(imp, fileDir string, knownModules []string) string {
	// Normalize Python dot-notation imports to path-style.
	// ".models" → "./models", "..utils" → "../utils", "...core" → "../../core"
	if normalized, ok := normalizePythonImport(imp); ok {
		imp = normalized
	}

	// Relative imports: ./foo or ../foo
	if strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, "../") {
		resolved := filepath.ToSlash(filepath.Clean(filepath.Join(fileDir, imp)))
		// Try exact match first, then prefix match (longest wins).
		best := ""
		for _, mod := range knownModules {
			if mod == resolved {
				return mod
			}
		}
		for _, mod := range knownModules {
			if strings.HasPrefix(resolved, mod+"/") || strings.HasPrefix(resolved+"/", mod+"/") {
				if len(mod) > len(best) {
					best = mod
				}
			}
		}
		return best
	}

	// Absolute/package-style imports: match suffix against known module names.
	// Convert dotted imports to slash-separated only when the import has no slash
	// (Python "os.path" → "os/path"). Imports with slashes (Go "github.com/...")
	// are left as-is to avoid false matches.
	slashImp := imp
	if !strings.Contains(imp, "/") {
		slashImp = strings.ReplaceAll(imp, ".", "/")
	}

	best := ""
	for _, mod := range knownModules {
		if mod == imp || mod == slashImp {
			return mod
		}
		// Suffix match: "internal/auth" matches module "internal/auth".
		if strings.HasSuffix(imp, "/"+mod) || strings.HasSuffix(slashImp, "/"+mod) {
			if len(mod) > len(best) {
				best = mod
			}
			continue
		}
		// Module is a prefix of the import path (package.submodule → package/).
		if strings.HasPrefix(slashImp, mod+"/") || strings.HasPrefix(imp, mod+"/") {
			if len(mod) > len(best) {
				best = mod
			}
			continue
		}
		// Module is a suffix of the import path (Go-style).
		if strings.HasSuffix(imp, mod) || strings.HasSuffix(slashImp, mod) {
			if len(mod) > len(best) {
				best = mod
			}
		}
	}

	return best
}

// normalizePythonImport converts Python dot-prefix relative imports to path-style.
// ".models" → ("./models", true), "..utils" → ("../utils", true), "os" → ("", false).
func normalizePythonImport(imp string) (string, bool) {
	if !strings.HasPrefix(imp, ".") {
		return "", false
	}
	// Count leading dots.
	dots := 0
	for dots < len(imp) && imp[dots] == '.' {
		dots++
	}
	rest := imp[dots:]
	if rest == "" {
		// Bare relative like "." or ".." without module name — can't resolve.
		return "", false
	}
	// "./x.js" and "../x" are already path-style (JS/TS), not Python dot notation.
	if rest != "" && rest[0] == '/' {
		return "", false
	}
	// Convert dots to path: 1 dot → "./", 2 dots → "../", 3 dots → "../../", etc.
	prefix := "./"
	for i := 1; i < dots; i++ {
		prefix = "../" + prefix
	}
	// Replace remaining dots in module name with slashes (e.g. ".foo.bar" → "./foo/bar").
	rest = strings.ReplaceAll(rest, ".", "/")
	return prefix + rest, true
}

// appendUnique appends values to s, skipping duplicates.
func appendUnique(s []string, values ...string) []string {
	set := make(map[string]struct{}, len(s))
	for _, v := range s {
		set[v] = struct{}{}
	}
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := set[v]; !ok {
			set[v] = struct{}{}
			s = append(s, v)
		}
	}
	return s
}
