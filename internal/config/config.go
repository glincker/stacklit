package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the settings loaded from a .stacklitrc.json file.
type Config struct {
	Ignore     []string `json:"ignore,omitempty"`
	MaxDepth   int      `json:"max_depth,omitempty"`
	MaxModules int      `json:"max_modules,omitempty"`
	MaxExports int      `json:"max_exports,omitempty"`
	// MaxModuleFiles caps the file names listed per module.
	MaxModuleFiles int `json:"max_module_files,omitempty"`
	// ModuleSplitFiles splits a module with more files than this into
	// subdirectory modules. A negative value turns splitting off.
	ModuleSplitFiles int          `json:"module_split_files,omitempty"`
	Output           OutputConfig `json:"output,omitempty"`

	// Scan controls (see scan.go for env overrides and validation).
	Roots          []string `json:"roots,omitempty"`            // scan only these subdirectories
	Include        []string `json:"include,omitempty"`          // gitignore-style globs; when set only matches are kept
	Exclude        []string `json:"exclude,omitempty"`          // gitignore-style globs always skipped
	MaxFileSize    int64    `json:"max_file_size,omitempty"`    // bytes; larger files are skipped with a warning
	Workers        int      `json:"workers,omitempty"`          // parser pool size; 0 means NumCPU
	ParseMemoryMB  int      `json:"parse_memory_mb,omitempty"`  // soft cap on concurrent parse memory; 0 means default
	ParseTimeoutMS int      `json:"parse_timeout_ms,omitempty"` // per-file tree-sitter budget; 0 means default, -1 disables
	TSMaxBytes     int64    `json:"ts_max_bytes,omitempty"`     // larger files use the lightweight parser; 0 means default, -1 disables
	MaxLineLen     int      `json:"max_line_len,omitempty"`     // longer lines mark a file minified; 0 means default, -1 disables
	SkipGenerated  *bool    `json:"skip_generated,omitempty"`   // generated files use the lightweight parser, default on
	MemoryLimitMB  int      `json:"memory_limit_mb,omitempty"`  // soft Go heap limit during a scan; 0 means default, -1 disables
	Cache          *bool    `json:"cache,omitempty"`            // incremental cache, default on
	Timestamps     *bool    `json:"timestamps,omitempty"`       // write generated_at, default off
	GitActivity    *bool    `json:"git_activity,omitempty"`     // include hot/recent files, default off
	MapIcons       string   `json:"map_icons,omitempty"`        // "remote" (default) or "off" for the HTML map
	// MapLayout picks the module list layout in the derived map: "flat" (one
	// line per module path, default) or "tree" (one line per parent directory
	// with its modules listed after it).
	MapLayout string `json:"map_layout,omitempty"`

	// Steering picks the rule wording in the generated CLAUDE.md block ("a", "b", "c", "d" the strict no-crawl wording, or "e" the routed wording).
	Steering string `json:"steering,omitempty"`
	// SmallRepoFiles is the source file count below which the session hook and
	// the rules block carry a one-line "grep is fine" note instead of the full
	// digest. Nil uses DefaultSmallRepoFiles; 0 turns small repo mode off.
	// Also STACKLIT_SMALL_REPO_FILES.
	SmallRepoFiles *int `json:"small_repo_files,omitempty"`
	// QuietRepoFiles is the source file count below which the session and
	// subagent hooks inject nothing, because measured sessions cost more with
	// them on repos this size. Nil uses DefaultQuietRepoFiles; 0 turns quiet
	// mode off. Also STACKLIT_QUIET_REPO_FILES.
	QuietRepoFiles *int `json:"quiet_repo_files,omitempty"`
	// CrawlGuard sets the Claude Code crawl guard hook mode: "off", "nudge" or
	// "redirect". Also STACKLIT_CRAWL_GUARD. Empty uses the mode setup installed.
	CrawlGuard string `json:"crawl_guard,omitempty"`
	// Stats turns on the local usage log (also STACKLIT_STATS=1). Default off.
	Stats *bool `json:"stats,omitempty"`

	// Check holds architecture rules evaluated by `stacklit check`.
	Check *CheckConfig `json:"check,omitempty"`
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

		MaxModuleFiles:   DefaultMaxModuleFiles,
		ModuleSplitFiles: DefaultModuleSplitFiles,
		MaxFileSize:      DefaultMaxFileSize,
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
	if cfg.MaxModuleFiles == 0 {
		cfg.MaxModuleFiles = DefaultMaxModuleFiles
	}
	if cfg.ModuleSplitFiles == 0 {
		cfg.ModuleSplitFiles = DefaultModuleSplitFiles
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

// CheckConfig lists the architecture rules `stacklit check` enforces. Every
// rule is off until configured; limits come from this file, never from code.
type CheckConfig struct {
	Forbidden []ForbiddenEdge `json:"forbidden,omitempty"`
	Cycles    bool            `json:"cycles,omitempty"`  // report import cycles between modules
	Orphans   bool            `json:"orphans,omitempty"` // report modules with no edges in or out
	MaxFanIn  int             `json:"max_fan_in,omitempty"`
	MaxFanOut int             `json:"max_fan_out,omitempty"`
}

// ForbiddenEdge bans dependencies from modules matching From to modules matching To.
// Patterns are module paths with optional * wildcards; "dir/**" matches dir and below.
type ForbiddenEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}
