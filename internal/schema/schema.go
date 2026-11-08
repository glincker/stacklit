package schema

type Index struct {
	Schema          string                `json:"$schema"`
	Version         string                `json:"version"`
	SchemaVersion   int                   `json:"schema_version"`
	GeneratedAt     string                `json:"generated_at,omitempty"`
	StacklitVersion string                `json:"stacklit_version"`
	MerkleHash      string                `json:"merkle_hash,omitempty"`
	Project         Project               `json:"project"`
	Tech            Tech                  `json:"tech"`
	Structure       Structure             `json:"structure"`
	Modules         map[string]ModuleInfo `json:"modules"`
	Dependencies    Dependencies          `json:"dependencies"`
	Git             GitInfo               `json:"git,omitempty"`
	Hints           Hints                 `json:"hints,omitempty"`
	Architecture    Architecture          `json:"architecture,omitempty"`
	Scan            *ScanInfo             `json:"scan,omitempty"`
	Packages        []PackageInfo         `json:"packages,omitempty"`
	PackageEdges    []PackageEdge         `json:"package_edges,omitempty"`
}

// PackageInfo is one workspace package in a monorepo: a hard boundary that
// groups modules and owns its manifest identity.
type PackageInfo struct {
	Name       string   `json:"name"`           // manifest name, or the path when none
	Path       string   `json:"path"`           // slash-separated, relative to the repo root
	Kind       string   `json:"kind,omitempty"` // npm, go, cargo, python
	Files      int      `json:"files"`
	Lines      int      `json:"lines"`
	Modules    []string `json:"modules,omitempty"`
	DependsOn  []string `json:"depends_on,omitempty"` // other package names
	DependedBy []string `json:"depended_by,omitempty"`
}

// PackageEdge is a dependency from one package to another.
type PackageEdge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"` // number of import sites backing the edge
}

// CurrentSchemaVersion is bumped on any breaking change to the index layout.
const CurrentSchemaVersion = 1

// ScanInfo records what a run deliberately left out, so readers can tell a
// partial index from a complete one. All lists are sorted for stable diffs.
type ScanInfo struct {
	SkippedLarge    []string `json:"skipped_large,omitempty"`
	SkippedSymlinks []string `json:"skipped_symlinks,omitempty"`
	SkippedOther    []string `json:"skipped_other,omitempty"`
	ParseErrors     []string `json:"parse_errors,omitempty"`
	// Lightweight lists files indexed without tree-sitter ("path (reason)"):
	// timeout, large, generated or minified. Their exports and imports are empty.
	Lightweight []string `json:"lightweight,omitempty"`
}

type Architecture struct {
	Pattern string `json:"pattern,omitempty"`    // e.g. "Clean Architecture"
	Summary string `json:"ai_summary,omitempty"` // AI-generated narrative
}

type Project struct {
	Name       string   `json:"name"`
	Root       string   `json:"root"`
	Type       string   `json:"type"`
	Workspaces []string `json:"workspaces,omitempty"`
}

type Tech struct {
	PrimaryLanguage   string               `json:"primary_language"`
	Languages         map[string]LangStats `json:"languages"`
	Frameworks        []string             `json:"frameworks,omitempty"`
	FrameworkPatterns []FrameworkPattern   `json:"framework_patterns,omitempty"`
}

// FrameworkPattern holds detected structural patterns for a framework.
type FrameworkPattern struct {
	Name       string   `json:"name"`
	Config     []string `json:"config_files,omitempty"`
	Routes     string   `json:"routes,omitempty"`
	API        string   `json:"api,omitempty"`
	Middleware string   `json:"middleware,omitempty"`
	Models     string   `json:"models,omitempty"`
	Entry      string   `json:"entry,omitempty"`
}

type LangStats struct {
	Files int `json:"files"`
	Lines int `json:"lines"`
}

type Structure struct {
	Entrypoints    []string          `json:"entrypoints"`
	TotalFiles     int               `json:"total_files"`
	TotalLines     int               `json:"total_lines"`
	KeyDirectories map[string]string `json:"key_directories,omitempty"`
}

type ModuleInfo struct {
	Purpose    string            `json:"purpose"`
	Language   string            `json:"language,omitempty"`
	Files      int               `json:"files"`
	Lines      int               `json:"lines"`
	FileList   []string          `json:"file_list,omitempty"`
	Exports    []string          `json:"exports,omitempty"`
	TypeDefs   map[string]string `json:"type_defs,omitempty"` // key type definitions
	DependsOn  []string          `json:"depends_on,omitempty"`
	DependedBy []string          `json:"depended_by,omitempty"`
	Activity   string            `json:"activity,omitempty"`
}

type Dependencies struct {
	Edges        [][2]string `json:"edges"`
	Entrypoints  []string    `json:"entrypoints"`
	MostDepended []string    `json:"most_depended,omitempty"`
	Isolated     []string    `json:"isolated,omitempty"`
}

type GitInfo struct {
	HotFiles []HotFile `json:"hot_files,omitempty"`
	Recent   []string  `json:"recent,omitempty"`
	Stable   []string  `json:"stable,omitempty"`
}

type HotFile struct {
	Path       string `json:"path"`
	Commits90d int    `json:"commits_90d"`
}

type Hints struct {
	AddFeature string   `json:"add_feature,omitempty"`
	TestCmd    string   `json:"test_command,omitempty"`
	EnvVars    []string `json:"env_vars,omitempty"`
	DoNotTouch []string `json:"do_not_touch,omitempty"`
}

type MultiIndex struct {
	Schema        string          `json:"$schema"`
	Version       string          `json:"version"`
	SchemaVersion int             `json:"schema_version"`
	Type          string          `json:"type"` // "polyrepo"
	GeneratedAt   string          `json:"generated_at,omitempty"`
	TotalFiles    int             `json:"total_files"`
	TotalLines    int             `json:"total_lines"`
	TotalModules  int             `json:"total_modules"`
	Repos         []RepoSummary   `json:"repos"`
	CrossEdges    []CrossRepoEdge `json:"cross_repo_edges,omitempty"`
	Failed        []RepoFailure   `json:"failed,omitempty"`
}

// CrossRepoEdge says that FromRepo imports something FromRepo does not own
// but ToRepo provides (matched through manifest names).
type CrossRepoEdge struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Weight int      `json:"weight"`
	Via    []string `json:"via,omitempty"` // importing modules in From, capped
}

// RepoFailure records a repo that could not be scanned.
type RepoFailure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type RepoSummary struct {
	Name            string                `json:"name"`
	Path            string                `json:"path"`
	PrimaryLanguage string                `json:"primary_language"`
	TotalFiles      int                   `json:"total_files"`
	TotalLines      int                   `json:"total_lines"`
	MerkleHash      string                `json:"merkle_hash,omitempty"`
	Provides        []string              `json:"provides,omitempty"` // manifest names this repo publishes
	Modules         map[string]ModuleInfo `json:"modules"`
	Packages        []PackageInfo         `json:"packages,omitempty"`
	PackageEdges    []PackageEdge         `json:"package_edges,omitempty"`
	Frameworks      []string              `json:"frameworks,omitempty"`
	Entrypoints     []string              `json:"entrypoints,omitempty"`
}
