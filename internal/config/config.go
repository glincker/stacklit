package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the settings loaded from a .stacklitrc.json file.
type Config struct {
	Ignore     []string     `json:"ignore,omitempty"`
	MaxDepth   int          `json:"max_depth,omitempty"`
	MaxModules int          `json:"max_modules,omitempty"`
	MaxExports int          `json:"max_exports,omitempty"`
	Output     OutputConfig `json:"output,omitempty"`
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
		MaxDepth:   4,
		MaxModules: 200,
		MaxExports: 10,
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

	return cfg
}

// ScanIgnore returns ignore patterns plus Stacklit output files so generated artifacts
// never feed back into the next scan.
func (c *Config) ScanIgnore() []string {
	ignore := append([]string{}, c.Ignore...)
	for _, out := range []string{c.Output.JSON, c.Output.Mermaid, c.Output.HTML} {
		if out == "" {
			continue
		}
		ignore = append(ignore, filepath.ToSlash(out))
	}
	return ignore
}
