// Package walker provides a gitignore-aware file walker that returns all
// source files worth indexing from a given root directory.
package walker

// sourceExtensions is the set of file extensions considered indexable source.
var sourceExtensions = map[string]bool{
	".go":      true,
	".ts":      true,
	".tsx":     true,
	".js":      true,
	".jsx":     true,
	".mjs":     true,
	".cjs":     true,
	".py":      true,
	".rs":      true,
	".java":    true,
	".cs":      true,
	".rb":      true,
	".php":     true,
	".swift":   true,
	".kt":      true,
	".scala":   true,
	".c":       true,
	".cpp":     true,
	".h":       true,
	".hpp":     true,
	".vue":     true,
	".svelte":  true,
	".dart":    true,
	".ex":      true,
	".exs":     true,
	".zig":     true,
	".nim":     true,
	".lua":     true,
	".sh":      true,
	".bash":    true,
	".sql":     true,
	".graphql": true,
	".gql":     true,
	".proto":   true,
	".gd":      true,
	".tf":      true,
	".hcl":     true,
	".zsh":     true,
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
