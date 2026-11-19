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
	"time"

	"github.com/glincker/stacklit/internal/parser"
)

// DefaultMaxFileSize is the fallback cap for a single source file (bytes).
// Override with max_file_size in .stacklitrc.json or STACKLIT_MAX_FILE_SIZE.
const DefaultMaxFileSize int64 = 1 << 20

// Defaults for the per-module listing caps. Override with max_module_files and
// module_split_files in .stacklitrc.json or STACKLIT_MAX_MODULE_FILES and
// STACKLIT_MODULE_SPLIT_FILES.
const (
	DefaultMaxModuleFiles   = 30
	DefaultModuleSplitFiles = 40
)

// DefaultSmallRepoFiles is the source file count under which a repo counts as
// small. Override with small_repo_files in .stacklitrc.json or
// STACKLIT_SMALL_REPO_FILES; 0 turns small repo mode off.
const DefaultSmallRepoFiles = 600

// DefaultQuietRepoFiles is the source file count under which the hooks stay
// silent. The agent test (bench/agent-ab/RESULTS-round2.md) lost money at
// 412 to 2,919 files and won at 15,978; the break-even between is unmeasured.
const DefaultQuietRepoFiles = 5000

// QuietRepoLimit returns the effective quiet threshold (0 means off).
func (c *Config) QuietRepoLimit() int {
	if c.QuietRepoFiles == nil {
		return DefaultQuietRepoFiles
	}
	return *c.QuietRepoFiles
}

// SmallRepoLimit returns the effective small repo threshold (0 means off).
func (c *Config) SmallRepoLimit() int {
	if c.SmallRepoFiles == nil {
		return DefaultSmallRepoFiles
	}
	return *c.SmallRepoFiles
}

// applyEnv lets environment variables override file config, so CI and hooks
// can tune a run without editing the committed rc file.
func (c *Config) applyEnv() {
	if v := os.Getenv("STACKLIT_MAX_FILE_SIZE"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			c.MaxFileSize = n
		}
	}
	for _, e := range []struct {
		name string
		dst  *int
		min  int
	}{
		{"STACKLIT_MAX_EXPORTS", &c.MaxExports, 1},
		{"STACKLIT_MAX_MODULE_FILES", &c.MaxModuleFiles, 1},
		{"STACKLIT_MODULE_SPLIT_FILES", &c.ModuleSplitFiles, -1},
	} {
		if v := os.Getenv(e.name); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= e.min {
				*e.dst = n
			}
		}
	}
	if v := os.Getenv("STACKLIT_SMALL_REPO_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.SmallRepoFiles = &n
		}
	}
	if v := os.Getenv("STACKLIT_QUIET_REPO_FILES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.QuietRepoFiles = &n
		}
	}
	if v := os.Getenv("STACKLIT_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.Workers = n
		}
	}
	if v := os.Getenv("STACKLIT_PARSE_MEMORY_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			c.ParseMemoryMB = n
			if n == 0 {
				c.ParseMemoryMB = -1
			}
		}
	}
	if v := os.Getenv("STACKLIT_PARSE_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.ParseTimeoutMS = n
			if n == 0 {
				c.ParseTimeoutMS = -1
			}
		}
	}
	if v := os.Getenv("STACKLIT_TS_MAX_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.TSMaxBytes = n
			if n == 0 {
				c.TSMaxBytes = -1
			}
		}
	}
	if v := os.Getenv("STACKLIT_MAX_LINE_LEN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MaxLineLen = n
			if n == 0 {
				c.MaxLineLen = -1
			}
		}
	}
	if v := os.Getenv("STACKLIT_SKIP_GENERATED"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.SkipGenerated = &b
		}
	}
	if v := os.Getenv("STACKLIT_MEMORY_LIMIT_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MemoryLimitMB = n
			if n == 0 {
				c.MemoryLimitMB = -1
			}
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
	if v := os.Getenv("STACKLIT_GIT"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.GitActivity = &b
		}
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("STACKLIT_MAP_ICONS"))); v != "" {
		c.MapIcons = v
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("STACKLIT_MAP_LAYOUT"))); v != "" {
		c.MapLayout = v
	}
	if v := os.Getenv("STACKLIT_NO_GIT"); v != "" {
		off := false
		c.GitActivity = &off
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

// TimestampsEnabled reports whether generated_at should be written (default false so a committed index stays byte-stable).
func (c *Config) TimestampsEnabled() bool { return c.Timestamps != nil && *c.Timestamps }

// GitActivityEnabled reports whether git history feeds the index (default false, opt in with git_activity or STACKLIT_GIT).
func (c *Config) GitActivityEnabled() bool { return c.GitActivity != nil && *c.GitActivity }

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
	if c.MaxDepth < 0 || c.MaxModules < 0 || c.MaxExports < 0 || c.MaxModuleFiles < 0 {
		problems = append(problems, "max_depth, max_modules, max_exports and max_module_files must not be negative")
	}
	if c.Workers < 0 {
		problems = append(problems, "workers must not be negative")
	}
	if c.SmallRepoFiles != nil && *c.SmallRepoFiles < 0 {
		problems = append(problems, "small_repo_files must not be negative")
	}
	if c.MapIcons != "" && c.MapIcons != "remote" && c.MapIcons != "off" {
		problems = append(problems, `map_icons must be "remote" or "off"`)
	}
	if c.MapLayout != "" && c.MapLayout != "flat" && c.MapLayout != "tree" {
		problems = append(problems, `map_layout must be "flat" or "tree"`)
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

// Error marks a problem with the user's configuration, as opposed to an
// internal failure. The CLI maps it to its own exit code.
type Error struct{ Err error }

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

// Check returns a *Error when .stacklitrc.json exists but is not valid JSON or
// holds values that cannot work. Unknown keys are tolerated so an older binary
// still runs against a newer rc file; `stacklit doctor` reports them.
func Check(root string) error {
	data, err := os.ReadFile(filepath.Join(root, ".stacklitrc.json"))
	if err != nil {
		return nil
	}
	if !json.Valid(data) {
		return &Error{fmt.Errorf(".stacklitrc.json is not valid JSON (run `stacklit doctor` for details)")}
	}
	if err := Load(root).Validate(); err != nil {
		return &Error{err}
	}
	return nil
}

// DefaultParseMemoryMB bounds concurrent parser memory when nothing is set.
const DefaultParseMemoryMB = 256

// ParseMemoryBytes returns the concurrent-parse memory budget, or 0 for
// unlimited (STACKLIT_PARSE_MEMORY_MB=0).
func (c *Config) ParseMemoryBytes() int64 {
	switch {
	case c.ParseMemoryMB < 0:
		return 0
	case c.ParseMemoryMB == 0:
		return DefaultParseMemoryMB << 20
	}
	return int64(c.ParseMemoryMB) << 20
}

// Defaults for the per-file parse limits. Each is overridable in
// .stacklitrc.json or by env (a value of -1, or 0 in env, disables it).
const (
	DefaultParseTimeoutMS       = 3000
	DefaultTSMaxBytes     int64 = 128 << 10
	DefaultMaxLineLen           = 2000
)

// ParseLimits resolves the effective per-file cost limits.
func (c *Config) ParseLimits() parser.Limits {
	l := parser.Limits{
		ParseTimeout: time.Duration(DefaultParseTimeoutMS) * time.Millisecond,
		MaxBytes:     DefaultTSMaxBytes,
		MaxLineLen:   DefaultMaxLineLen,
		Generated:    c.SkipGenerated == nil || *c.SkipGenerated,
	}
	switch {
	case c.ParseTimeoutMS < 0:
		l.ParseTimeout = 0
	case c.ParseTimeoutMS > 0:
		l.ParseTimeout = time.Duration(c.ParseTimeoutMS) * time.Millisecond
	}
	switch {
	case c.TSMaxBytes < 0:
		l.MaxBytes = 0
	case c.TSMaxBytes > 0:
		l.MaxBytes = c.TSMaxBytes
	}
	switch {
	case c.MaxLineLen < 0:
		l.MaxLineLen = 0
	case c.MaxLineLen > 0:
		l.MaxLineLen = c.MaxLineLen
	}
	return l
}

// DefaultMemoryLimitMB is the soft heap limit applied during a scan. The
// pure-Go parser churns gigabytes of short-lived arenas; without a limit the
// GC lets RSS run several times above the live set.
const DefaultMemoryLimitMB = 448

// MemoryLimitBytes returns the soft heap limit, or 0 for none. An explicit
// GOMEMLIMIT in the environment always wins.
func (c *Config) MemoryLimitBytes() int64 {
	if os.Getenv("GOMEMLIMIT") != "" {
		return 0
	}
	switch {
	case c.MemoryLimitMB < 0:
		return 0
	case c.MemoryLimitMB == 0:
		return DefaultMemoryLimitMB << 20
	}
	return int64(c.MemoryLimitMB) << 20
}

// MapIconsOff reports whether the HTML map must not request any remote icon.
func (c *Config) MapIconsOff() bool { return c.MapIcons == "off" }

// MapLayoutName returns the configured derived-map layout: "flat" unless the
// rc file asks for "tree". Unknown values fall back to "flat"; Validate
// reports them in strict mode.
func (c *Config) MapLayoutName() string {
	if c.MapLayout == "tree" {
		return "tree"
	}
	return "flat"
}
