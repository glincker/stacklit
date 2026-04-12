// Package walker provides a gitignore-aware file walker that returns all
// source files worth indexing from a given root directory.
package walker

import (
	"io/fs"
	"os"
	"path/filepath"

	gitignore "github.com/sabhiram/go-gitignore"
)

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
// source files that are not excluded by .gitignore rules or alwaysIgnoreDirs.
// extraIgnore is an optional list of additional gitignore-style patterns to
// skip; it may be nil or empty to preserve the previous behaviour.
func Walk(root string, extraIgnore []string) ([]string, error) {
	// Load .gitignore from root if present.
	var gi *gitignore.GitIgnore
	gitignorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignorePath); err == nil {
		gi, _ = gitignore.CompileIgnoreFile(gitignorePath)
	}

	// Compile extra ignore patterns into a separate matcher when provided.
	var extraGi *gitignore.GitIgnore
	if len(extraIgnore) > 0 {
		extraGi = gitignore.CompileIgnoreLines(extraIgnore...)
	}

	var results []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute path relative to root for all comparisons and results.
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}

		// Normalize path separators so generated output and ignore matching stay stable across OSes.
		rel = filepath.ToSlash(rel)

		// Skip the root itself.
		if rel == "." {
			return nil
		}

		name := d.Name()

		if d.IsDir() {
			// Always skip known noise directories.
			if alwaysIgnoreDirs[name] {
				return filepath.SkipDir
			}
			// Skip directories matched by .gitignore.
			if gi != nil && gi.MatchesPath(rel+"/") {
				return filepath.SkipDir
			}
			// Skip directories matched by extra ignore patterns.
			if extraGi != nil && extraGi.MatchesPath(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files matched by .gitignore.
		if gi != nil && gi.MatchesPath(rel) {
			return nil
		}

		// Skip files matched by extra ignore patterns.
		if extraGi != nil && extraGi.MatchesPath(rel) {
			return nil
		}

		// Only keep recognised source extensions.
		ext := filepath.Ext(name)
		if !sourceExtensions[ext] {
			return nil
		}

		results = append(results, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}
