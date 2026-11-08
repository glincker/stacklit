package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// DefaultMaxFileSize is the fallback cap for a single source file (bytes).
// Override with max_file_size in .stacklitrc.json or STACKLIT_MAX_FILE_SIZE.
const DefaultMaxFileSize int64 = 1 << 20

// applyEnv lets environment variables override file config, so CI and hooks
// can tune a run without editing the committed rc file.
func (c *Config) applyEnv() {
	if v := os.Getenv("STACKLIT_MAX_FILE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			c.MaxFileSize = n
		}
	}
	if v := os.Getenv("STACKLIT_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.Workers = n
		}
	}
	if v := os.Getenv("STACKLIT_NO_CACHE"); v != "" {
		off := false
		c.Cache = &off
	}
	if v := os.Getenv("STACKLIT_TIMESTAMPS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.Timestamps = &b
		}
	}
	if v := os.Getenv("STACKLIT_ROOTS"); v != "" {
		c.Roots = nil
		for _, r := range strings.Split(v, ",") {
			if r = strings.TrimSpace(r); r != "" {
				c.Roots = append(c.Roots, r)
			}
		}
	}
}

// WorkerCount returns the effective parser pool size.
func (c *Config) WorkerCount() int {
	if c.Workers > 0 {
		return c.Workers
	}
	return runtime.NumCPU()
}

// CacheEnabled reports whether the incremental cache is on (default true).
func (c *Config) CacheEnabled() bool { return c.Cache == nil || *c.Cache }

// TimestampsEnabled reports whether generated_at should be written (default true).
func (c *Config) TimestampsEnabled() bool { return c.Timestamps == nil || *c.Timestamps }

// NormalizedRoots returns cleaned, slash-separated scan roots relative to the
// repo root. Empty means "scan everything".
func (c *Config) NormalizedRoots() []string {
	var out []string
	for _, r := range c.Roots {
		r = path.Clean(toSlash(r))
		if r == "." || r == "" {
			return nil
		}
		out = append(out, r)
	}
	return out
}

// LoadStrict reads .stacklitrc.json and reports parse and validation errors
// instead of silently falling back to defaults. A missing file is not an error.
func LoadStrict(root string) (*Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(filepath.Join(root, ".stacklitrc.json"))
	if err != nil {
		if os.IsNotExist(err) {
			cfg.applyEnv()
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading .stacklitrc.json: %w", err)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cfg); err != nil {
		return Load(root), fmt.Errorf("parsing .stacklitrc.json: %w", err)
	}
	cfg = Load(root)
	return cfg, cfg.Validate()
}

// Validate checks the settings for values that cannot work.
func (c *Config) Validate() error {
	var problems []string
	if c.MaxDepth < 0 || c.MaxModules < 0 || c.MaxExports < 0 {
		problems = append(problems, "max_depth, max_modules and max_exports must not be negative")
	}
	if c.Workers < 0 {
		problems = append(problems, "workers must not be negative")
	}
	for _, r := range c.Roots {
		clean := path.Clean(toSlash(r))
		if filepath.IsAbs(r) || clean == ".." || strings.HasPrefix(clean, "../") {
			problems = append(problems, fmt.Sprintf("root %q must be a relative path inside the repo", r))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(problems, "; "))
	}
	return nil
}
