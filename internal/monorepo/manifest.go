package monorepo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Manifest identifies a package by the name other code imports it with.
type Manifest struct {
	Name string // npm name, Go module path, crate name or python project name
	Kind string // "npm", "go", "cargo" or "python"
}

var (
	goModRe     = regexp.MustCompile(`(?m)^\s*module\s+(\S+)`)
	tomlNameRe  = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)
	tomlSection = regexp.MustCompile(`(?m)^\s*\[([^\]]+)\]`)
)

// ReadManifest returns the identity declared in dir, trying package.json,
// go.mod, Cargo.toml and pyproject.toml in that order. ok is false when none
// of them names the package.
func ReadManifest(dir string) (Manifest, bool) {
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		var pj struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pj) == nil && pj.Name != "" {
			return Manifest{Name: pj.Name, Kind: "npm"}, true
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		if m := goModRe.FindSubmatch(data); m != nil {
			return Manifest{Name: strings.Trim(string(m[1]), `"`), Kind: "go"}, true
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "Cargo.toml")); err == nil {
		if n := tomlSectionName(string(data), "package"); n != "" {
			return Manifest{Name: n, Kind: "cargo"}, true
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "pyproject.toml")); err == nil {
		for _, sec := range []string{"project", "tool.poetry"} {
			if n := tomlSectionName(string(data), sec); n != "" {
				return Manifest{Name: n, Kind: "python"}, true
			}
		}
	}
	return Manifest{}, false
}

// tomlSectionName returns the first name = "..." inside [section].
func tomlSectionName(content, section string) string {
	locs := tomlSection.FindAllStringSubmatchIndex(content, -1)
	for i, loc := range locs {
		if strings.TrimSpace(content[loc[2]:loc[3]]) != section {
			continue
		}
		end := len(content)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		if m := tomlNameRe.FindStringSubmatch(content[loc[1]:end]); m != nil {
			return m[1]
		}
	}
	return ""
}

// Matches reports whether import path imp refers to the package named name,
// either exactly or as a subpath (e.g. "@acme/ui/button" for "@acme/ui").
// Cargo crates are compared with dashes folded to underscores.
func (m Manifest) Matches(imp string) bool {
	name := m.Name
	if m.Kind == "cargo" {
		name = strings.ReplaceAll(name, "-", "_")
		imp = strings.ReplaceAll(imp, "-", "_")
		if i := strings.Index(imp, "::"); i >= 0 {
			imp = imp[:i]
		}
	}
	if name == "" {
		return false
	}
	return imp == name || strings.HasPrefix(imp, name+"/")
}
