// Package walker provides a gitignore-aware file walker that returns all
// source files worth indexing from a given root directory.
package walker

import "strings"

// sourceExtensions is the set of file extensions considered indexable source.
var sourceExtensions = map[string]bool{
	".go":         true,
	".ts":         true,
	".tsx":        true,
	".js":         true,
	".jsx":        true,
	".mjs":        true,
	".cjs":        true,
	".py":         true,
	".rs":         true,
	".java":       true,
	".cs":         true,
	".rb":         true,
	".php":        true,
	".swift":      true,
	".kt":         true,
	".scala":      true,
	".c":          true,
	".cpp":        true,
	".h":          true,
	".hpp":        true,
	".vue":        true,
	".svelte":     true,
	".dart":       true,
	".ex":         true,
	".exs":        true,
	".zig":        true,
	".nim":        true,
	".lua":        true,
	".sh":         true,
	".bash":       true,
	".sql":        true,
	".graphql":    true,
	".gql":        true,
	".proto":      true,
	".gd":         true,
	".tf":         true,
	".hcl":        true,
	".zsh":        true,
	".hs":         true,
	".ml":         true,
	".mli":        true,
	".clj":        true,
	".cljs":       true,
	".cljc":       true,
	".r":          true,
	".pl":         true,
	".pm":         true,
	".jl":         true,
	".m":          true,
	".mm":         true,
	".groovy":     true,
	".erl":        true,
	".hrl":        true,
	".fs":         true,
	".fsx":        true,
	".f90":        true,
	".f95":        true,
	".f03":        true,
	".f08":        true,
	".elm":        true,
	".sol":        true,
	".cmake":      true,
	".mk":         true,
	".ps1":        true,
	".gleam":      true,
	".json":       true,
	".yaml":       true,
	".yml":        true,
	".toml":       true,
	".dockerfile": true,
}

// sourceBasenames are extensionless build files worth indexing.
var sourceBasenames = map[string]bool{
	"Makefile":       true,
	"GNUmakefile":    true,
	"Dockerfile":     true,
	"CMakeLists.txt": true,
}

// skipBasenames are generated files that would only add noise.
var skipBasenames = map[string]bool{
	"package-lock.json":   true,
	"npm-shrinkwrap.json": true,
	"pnpm-lock.yaml":      true,
	"stacklit.json":       true,
	"stacklit.graph.json": true,
	".stacklitrc.json":    true,
}

// alwaysIgnoreDirs contains directory names that are always skipped regardless
// of .gitignore configuration.
var alwaysIgnoreDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".next":        true,
	".nuxt":        true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	".tox":         true,
	"target":       true,
	"coverage":     true,
	".cache":       true,
	".turbo":       true,
	// Agent and editor state: never source, often large, and rewritten by tools.
	".claude":       true,
	".agents":       true,
	".cursor":       true,
	".codex":        true,
	".idea":         true,
	".vscode":       true,
	".svelte-kit":   true,
	".parcel-cache": true,
}

// isToolStateDir also covers prefixed families such as .aider.chat.history.
func isToolStateDir(name string) bool {
	return alwaysIgnoreDirs[name] || strings.HasPrefix(name, ".aider")
}

// Walk traverses root and returns a sorted slice of relative paths for all
// source files that are not excluded by ignore rules or alwaysIgnoreDirs.
// extraIgnore is an optional list of additional gitignore-style patterns; it
// may be nil. Use WalkWith for roots, include globs, size caps and skip reports.
func Walk(root string, extraIgnore []string) ([]string, error) {
	res, err := WalkWith(root, Options{ExtraIgnore: extraIgnore})
	if err != nil {
		return nil, err
	}
	return res.Files, nil
}
