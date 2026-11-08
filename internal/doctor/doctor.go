// Package doctor diagnoses a repo's stacklit setup: config, hooks, index
// freshness, skipped files and parse failures. It never modifies the repo.
package doctor

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/cache"
	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/parser"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/walker"
)

// Level is the severity of a finding.
type Level string

const (
	OK    Level = "ok"
	Info  Level = "info"
	Warn  Level = "warn"
	Error Level = "error"
)

// Finding is one diagnosed fact with an optional suggested fix.
type Finding struct {
	Level   Level    `json:"level"`
	Check   string   `json:"check"`
	Message string   `json:"message"`
	Fix     string   `json:"fix,omitempty"`
	Details []string `json:"details,omitempty"`
}

// Report is the full result of Run.
type Report struct {
	Root     string    `json:"root"`
	Findings []Finding `json:"findings"`
}

// Count returns how many findings have the given level.
func (r *Report) Count(l Level) int {
	n := 0
	for _, f := range r.Findings {
		if f.Level == l {
			n++
		}
	}
	return n
}

func (r *Report) add(l Level, check, msg, fix string, details ...string) {
	r.Findings = append(r.Findings, Finding{Level: l, Check: check, Message: msg, Fix: fix, Details: details})
}

// maxDetails keeps reports readable on repos with thousands of problems.
const maxDetails = 10

func capList(in []string) []string {
	if len(in) <= maxDetails {
		return in
	}
	out := append([]string{}, in[:maxDetails]...)
	return append(out, fmt.Sprintf("... and %d more", len(in)-maxDetails))
}

// Run inspects root and returns a report.
func Run(root string) (*Report, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}
	r := &Report{Root: abs}

	cfg := checkConfig(r, abs)
	checkHooks(r, abs)
	files := checkScan(r, abs, cfg)
	checkIndex(r, abs, cfg, files)
	checkCache(r, abs, cfg)
	if runtime.GOOS == "windows" {
		r.add(Info, "platform", "running on Windows; paths in the index are always slash-separated", "")
	}
	return r, nil
}

func checkConfig(r *Report, root string) *config.Config {
	cfg, err := config.LoadStrict(root)
	if err != nil {
		r.add(Error, "config", err.Error(), "fix .stacklitrc.json; known keys are listed in the README")
		return config.Load(root)
	}
	if _, statErr := os.Stat(filepath.Join(root, ".stacklitrc.json")); statErr != nil {
		r.add(OK, "config", "no .stacklitrc.json, using defaults", "")
	} else {
		r.add(OK, "config", ".stacklitrc.json is valid", "")
	}
	return cfg
}

func gitHooksDir(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "--git-path", "hooks").Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return p
}

func checkHooks(r *Report, root string) {
	dir := gitHooksDir(root)
	if dir == "" {
		r.add(Info, "hooks", "not a git repository (or git is missing); hooks and git activity are unavailable", "")
		return
	}
	hook := filepath.Join(dir, "post-commit")
	data, err := os.ReadFile(hook)
	if err != nil {
		r.add(Info, "hooks", "no stacklit post-commit hook installed", "run `stacklit init --hook` to keep the index fresh")
		return
	}
	if !strings.Contains(string(data), "stacklit") {
		r.add(Info, "hooks", "post-commit hook exists but does not run stacklit", "run `stacklit init --hook` to append it")
		return
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(hook); err == nil && info.Mode()&0o111 == 0 {
			r.add(Error, "hooks", "post-commit hook mentions stacklit but is not executable", "chmod +x "+hook)
			return
		}
	}
	if _, err := exec.LookPath("stacklit"); err != nil {
		r.add(Warn, "hooks", "hook is installed but `stacklit` is not on PATH, so it will silently do nothing", "install stacklit on PATH")
		return
	}
	r.add(OK, "hooks", "post-commit hook installed", "")
}

type scanned struct {
	files  []string
	leaves map[string][]byte
}

func checkScan(r *Report, root string, cfg *config.Config) *scanned {
	walked, err := walker.WalkWith(root, walker.Options{
		ExtraIgnore: cfg.ScanIgnore(),
		Include:     cfg.Include,
		Roots:       cfg.NormalizedRoots(),
		MaxFileSize: cfg.MaxFileSize,
	})
	if err != nil {
		r.add(Error, "scan", "cannot walk the repo: "+err.Error(), "")
		return nil
	}
	r.add(OK, "scan", fmt.Sprintf("%d source files found", len(walked.Files)), "")

	by := map[string][]string{}
	for _, s := range walked.Skipped {
		by[s.Reason] = append(by[s.Reason], s.Path)
	}
	reasons := make([]string, 0, len(by))
	for k := range by {
		reasons = append(reasons, k)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		paths := by[reason]
		fix := ""
		lvl := Info
		switch reason {
		case walker.ReasonLarge:
			lvl = Warn
			fix = "raise max_file_size (or STACKLIT_MAX_FILE_SIZE) or add the files to .stacklitignore"
		case walker.ReasonMissingRoot, walker.ReasonRootEscape:
			lvl = Warn
			fix = "fix the roots setting in .stacklitrc.json"
		}
		r.add(lvl, "scan", fmt.Sprintf("%d skipped: %s", len(paths), reason), fix, capList(paths)...)
	}

	res := parser.ScanFiles(walked.Files, parser.PoolOptions{Root: root, Workers: cfg.WorkerCount(), MaxFileSize: cfg.MaxFileSize})
	sc := &scanned{leaves: map[string][]byte{}}
	var failed, panicked []string
	for _, x := range res {
		if x.Err != nil {
			if x.Panicked {
				panicked = append(panicked, x.Path)
			} else {
				failed = append(failed, fmt.Sprintf("%s: %v", x.Path, x.Err))
			}
			continue
		}
		sc.files = append(sc.files, x.Path)
		sc.leaves[x.Path] = x.Leaf
	}
	if len(panicked) > 0 {
		r.add(Error, "parse", fmt.Sprintf("%d files crashed a parser (skipped, scan continues)", len(panicked)), "report these files upstream", capList(panicked)...)
	}
	if len(failed) > 0 {
		r.add(Warn, "parse", fmt.Sprintf("%d files could not be parsed", len(failed)), "exclude them via .stacklitignore if they are generated", capList(failed)...)
	}
	if len(panicked)+len(failed) == 0 {
		r.add(OK, "parse", "all files parsed cleanly", "")
	}
	return sc
}

func checkIndex(r *Report, root string, cfg *config.Config, sc *scanned) {
	path := filepath.Join(root, cfg.Output.JSON)
	data, err := os.ReadFile(path)
	if err != nil {
		r.add(Warn, "index", cfg.Output.JSON+" does not exist", "run `stacklit generate`")
		return
	}
	var idx schema.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		r.add(Error, "index", cfg.Output.JSON+" is not valid JSON: "+err.Error(), "run `stacklit generate` to rewrite it")
		return
	}
	if idx.SchemaVersion == 0 {
		r.add(Warn, "index", cfg.Output.JSON+" has no schema_version (written by an older stacklit)", "run `stacklit generate`")
	} else if idx.SchemaVersion > schema.CurrentSchemaVersion {
		r.add(Warn, "index", fmt.Sprintf("index schema_version %d is newer than this binary supports (%d)", idx.SchemaVersion, schema.CurrentSchemaVersion), "upgrade stacklit")
	}
	if sc == nil || idx.MerkleHash == "" {
		return
	}
	if fresh := git.ComputeMerkleFromLeaves(sc.leaves); fresh != idx.MerkleHash {
		r.add(Warn, "index", cfg.Output.JSON+" is stale: source files changed since it was generated", "run `stacklit generate`")
		return
	}
	r.add(OK, "index", cfg.Output.JSON+" is up to date", "")
}

func checkCache(r *Report, root string, cfg *config.Config) {
	if !cfg.CacheEnabled() {
		r.add(Info, "cache", "incremental cache is disabled", "")
		return
	}
	p := cache.Path(root)
	if p == "" {
		r.add(Warn, "cache", "no user cache directory available; every run re-parses everything", "set STACKLIT_CACHE_DIR")
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		r.add(Warn, "cache", "cache directory is not writable: "+err.Error(), "set STACKLIT_CACHE_DIR")
		return
	}
	r.add(OK, "cache", "cache at "+p, "")
}
