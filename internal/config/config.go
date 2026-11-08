package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the settings loaded from a .stacklitrc.json file.
type Config struct {
	Ignore     []string     `json:"ignore,omitempty"`
	MaxDepth   int          `json:"max_depth,omitempty"`
	MaxModules int          `json:"max_modules,omitempty"`
	MaxExports int          `json:"max_exports,omitempty"`
	Output     OutputConfig `json:"output,omitempty"`

	// Scan controls (see scan.go for env overrides and validation).
	Roots       []string `json:"roots,omitempty"`         // scan only these subdirectories
	Include     []string `json:"include,omitempty"`       // gitignore-style globs; when set only matches are kept
	Exclude     []string `json:"exclude,omitempty"`       // gitignore-style globs always skipped
	MaxFileSize int64    `json:"max_file_size,omitempty"` // bytes; larger files are skipped with a warning
	Workers     int      `json:"workers,omitempty"`       // parser pool size; 0 means NumCPU
	Cache       *bool    `json:"cache,omitempty"`         // incremental cache, default on
	Timestamps  *bool    `json:"timestamps,omitempty"`    // write generated_at, default off
	GitActivity *bool    `json:"git_activity,omitempty"`  // include hot/recent files, default off
}

// OutputConfig controls where output files are written.
type OutputConfig struct {
	JSON    string `json:"json,omitempty"`
	Mermaid string `json:"mermaid,omitempty"`
	HTML    string `json:"html,omitempty"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		MaxDepth:    4,
		MaxModules:  200,
		MaxExports:  10,
		MaxFileSize: DefaultMaxFileSize,
		Output: OutputConfig{
			JSON:    "stacklit.json",
			Mermaid: "DEPENDENCIES.md",
			HTML:    "stacklit.html",
		},
	}
}

// Load reads .stacklitrc.json from root and merges it over the defaults.
// If the file does not exist or cannot be parsed, defaults are returned.
func Load(root string) *Config {
	cfg := DefaultConfig()
	defer cfg.applyEnv()

	path := filepath.Join(root, ".stacklitrc.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}

	// Unmarshal on top of cfg so existing defaults survive missing keys.
	json.Unmarshal(data, cfg) //nolint:errcheck // best-effort; defaults remain

	// Re-apply defaults for any zero values introduced by an explicit null/0.
	if cfg.MaxDepth == 0 {
		cfg.MaxDepth = 4
	}
	if cfg.MaxModules == 0 {
		cfg.MaxModules = 200
	}
	if cfg.MaxExports == 0 {
		cfg.MaxExports = 10
	}
	if cfg.Output.JSON == "" {
		cfg.Output.JSON = "stacklit.json"
	}
	if cfg.Output.Mermaid == "" {
		cfg.Output.Mermaid = "DEPENDENCIES.md"
	}
	if cfg.Output.HTML == "" {
		cfg.Output.HTML = "stacklit.html"
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = DefaultMaxFileSize
	}

	return cfg
}

// ScanIgnore returns ignore patterns plus Stacklit output files so generated artifacts
// never feed back into the next scan. All patterns are normalized to forward slashes
// so they match the walker's normalized paths on every OS.
func (c *Config) ScanIgnore() []string {
	ignore := make([]string, 0, len(c.Ignore)+len(c.Exclude)+3)
	for _, pat := range c.Ignore {
		ignore = append(ignore, toSlash(pat))
	}
	for _, pat := range c.Exclude {
		ignore = append(ignore, toSlash(pat))
	}
	for _, out := range []string{c.Output.JSON, c.Output.Mermaid, c.Output.HTML} {
		if out == "" {
			continue
		}
		ignore = append(ignore, toSlash(out))
	}
	return ignore
}

// toSlash normalizes both OS path separators and literal backslashes to forward
// slashes. filepath.ToSlash only converts os.PathSeparator which is '/' on
// Unix, leaving Windows-style backslashes untouched on non-Windows builds.
func toSlash(p string) string {
	p = filepath.ToSlash(p)
	return strings.ReplaceAll(p, "\\", "/")
}
