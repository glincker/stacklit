package engine

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glincker/stacklit/internal/cache"
	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/detect"
	"github.com/glincker/stacklit/internal/fsutil"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/monorepo"
	"github.com/glincker/stacklit/internal/parser"
	"github.com/glincker/stacklit/internal/renderer"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/session"
	"github.com/glincker/stacklit/internal/walker"
)

// GraphFile is the symbol graph refreshed alongside the index when present.
const GraphFile = "stacklit.graph.json"

// RefreshOptions selects what Refresh updates.
type RefreshOptions struct {
	Root string
	// Paths are files that changed, absolute or relative to Root.
	Paths []string
	// Debounce waits before processing so a burst of edits becomes one run.
	// Negative means none; zero reads STACKLIT_REFRESH_DEBOUNCE_MS (default 30).
	Debounce time.Duration
}

// RefreshResult reports what a Refresh did.
type RefreshResult struct {
	// Status is refreshed, queued (another process holds the lock and will
	// pick the paths up), no-index, or nothing (no file needed work).
	Status        string
	Parsed        int
	Removed       int
	Bootstrapped  bool
	StructureHash string
	// StructureChanged is true when the structure hash differs from before.
	StructureChanged bool
	Duration         time.Duration
}

const refreshStateVersion = 1

type fileState struct {
	// GraphDone marks files already offered to the symbol extractor, tests included.
	GraphDone bool             `json:"graph_done,omitempty"`
	Leaf      string           `json:"leaf"`
	Info      *parser.FileInfo `json:"info"`
	Env       []string         `json:"env,omitempty"`
	Syms      *graph.FileSyms  `json:"syms,omitempty"`
	Err       bool             `json:"err,omitempty"`
}

// refreshState is the per-worktree record of what the last run saw. The
// content hash (leaf) per file is the cache key: an unchanged file is never
// parsed again.
type refreshState struct {
	Version     int                   `json:"version"`
	Fingerprint string                `json:"fingerprint"`
	Head        string                `json:"head"`
	Graph       bool                  `json:"graph"`
	Files       map[string]*fileState `json:"files"`
	Bodies      graph.BodyCache       `json:"bodies,omitempty"`
}

func statePath(dir string) string { return filepath.Join(dir, "refresh-state.json") }

func loadState(dir string) *refreshState {
	b, err := os.ReadFile(statePath(dir))
	if err != nil {
		return nil
	}
	var st refreshState
	if json.Unmarshal(b, &st) != nil || st.Version != refreshStateVersion || st.Fingerprint != cache.Fingerprint() || st.Files == nil {
		return nil
	}
	return &st
}

func (st *refreshState) save(dir string) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(statePath(dir), b, 0o644)
}

func envMillis(name string, def time.Duration) time.Duration {
	if n, err := strconv.Atoi(os.Getenv(name)); err == nil && n >= 0 {
		return time.Duration(n) * time.Millisecond
	}
	return def
}

// relPaths normalizes inputs to slash-separated paths inside root.
func relPaths(root string, in []string) []string {
	realRoot, _ := filepath.EvalSymlinks(root)
	seen := map[string]bool{}
	var out []string
	for _, p := range in {
		if p == "" {
			continue
		}
		if filepath.IsAbs(p) {
			rel, err := filepath.Rel(root, p)
			if err != nil || strings.HasPrefix(rel, "..") {
				d, _ := filepath.EvalSymlinks(filepath.Dir(p))
				if rr, err2 := filepath.Rel(realRoot, filepath.Join(d, filepath.Base(p))); err2 == nil {
					rel = rr
				}
			}
			p = rel
		}
		p = filepath.ToSlash(filepath.Clean(p))
		if p == "." || strings.HasPrefix(p, "..") || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Refresh updates stacklit.json (and stacklit.graph.json when it exists) for
// the given files. Unchanged files are skipped by content hash, only changed
// files are parsed, and the outputs are rebuilt from stored per-file facts
// with the same code a full generate uses. Concurrent callers coalesce: the
// lock holder drains a queue the others append to.
func Refresh(o RefreshOptions) (*RefreshResult, error) {
	start := time.Now()
	root, err := filepath.Abs(o.Root)
	if err != nil {
		return nil, err
	}
	cfg := config.Load(root)
	if _, err := os.Stat(filepath.Join(root, cfg.Output.JSON)); err != nil {
		return &RefreshResult{Status: "no-index"}, nil
	}
	dir := session.StateDir(root)
	rels := relPaths(root, o.Paths)
	lock, err := session.TryLock(dir)
	if err != nil {
		return nil, err
	}
	if lock == nil {
		if err := session.Enqueue(dir, rels); err != nil {
			return nil, err
		}
		return &RefreshResult{Status: "queued", Duration: time.Since(start)}, nil
	}
	deb := o.Debounce
	if deb == 0 {
		deb = envMillis("STACKLIT_REFRESH_DEBOUNCE_MS", 30*time.Millisecond)
	}
	res := &RefreshResult{Status: "nothing"}
	pending := rels
	for round := 0; ; round++ {
		if deb > 0 {
			time.Sleep(deb)
		}
		pending = unionSorted(pending, session.Drain(dir))
		r, err := refreshOnce(root, cfg, dir, pending)
		if err != nil {
			lock.Unlock()
			return nil, err
		}
		mergeResult(res, r)
		lock.Touch()
		pending = nil
		if !session.HasPending(dir) || round >= 4 {
			break
		}
	}
	lock.Unlock()
	// A writer may have queued after the last check but before the unlock.
	if session.HasPending(dir) {
		if l2, _ := session.TryLock(dir); l2 != nil {
			if r, err := refreshOnce(root, cfg, dir, session.Drain(dir)); err == nil {
				mergeResult(res, r)
			}
			l2.Unlock()
		}
	}
	res.Duration = time.Since(start)
	return res, nil
}

func unionSorted(a, b []string) []string {
	m := map[string]bool{}
	for _, s := range a {
		m[s] = true
	}
	for _, s := range b {
		m[s] = true
	}
	out := make([]string, 0, len(m))
	for s := range m {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func mergeResult(dst, r *RefreshResult) {
	if r.Status == "refreshed" {
		dst.Status = "refreshed"
	}
	dst.Parsed += r.Parsed
	dst.Removed += r.Removed
	dst.Bootstrapped = dst.Bootstrapped || r.Bootstrapped
	dst.StructureHash = r.StructureHash
	dst.StructureChanged = dst.StructureChanged || r.StructureChanged
}

func walkOptions(cfg *config.Config) walker.Options {
	return walker.Options{ExtraIgnore: cfg.ScanIgnore(), Include: cfg.Include, Roots: cfg.NormalizedRoots(), MaxFileSize: cfg.MaxFileSize}
}

func graphOptions(cfg *config.Config) graph.ExtractOptions {
	eo := graph.OptionsFromEnv()
	if b := cfg.ParseMemoryBytes(); b > 0 {
		eo.MemoryBudget = b
	} else {
		eo.MemoryBudget = -1
	}
	return eo
}

func poolOptions(root string, cfg *config.Config, visit func(string, []byte)) parser.PoolOptions {
	return parser.PoolOptions{
		Root: root, Workers: cfg.WorkerCount(), MemoryBudget: cfg.ParseMemoryBytes(),
		MaxFileSize: cfg.MaxFileSize, Limits: cfg.ParseLimits(), Visit: visit,
	}
}

func refreshOnce(root string, cfg *config.Config, dir string, paths []string) (*RefreshResult, error) {
	jsonPath := filepath.Join(root, cfg.Output.JSON)
	old, err := session.LoadIndex(root)
	if err != nil {
		return &RefreshResult{Status: "no-index"}, nil
	}
	before := session.StructureHash(old)
	graphPath := filepath.Join(root, GraphFile)
	_, gerr := os.Stat(graphPath)
	wantGraph := gerr == nil
	head := git.Head(root)

	st := loadState(dir)
	res := &RefreshResult{Status: "refreshed"}
	eo := graphOptions(cfg)

	walked, err := walker.WalkWith(root, walkOptions(cfg))
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	present := make(map[string]bool, len(walked.Files))
	for _, f := range walked.Files {
		present[f] = true
	}

	var todo []string
	switch {
	case st == nil || st.Head != head || (wantGraph && !st.Graph):
		// No trusted record (first run, new binary, branch switch, rebase): rebuild the
		// record from every file, still skipping nothing but cheap through the parse cache.
		st = &refreshState{Version: refreshStateVersion, Fingerprint: cache.Fingerprint(), Head: head, Files: map[string]*fileState{}}
		todo = walked.Files
		res.Bootstrapped = true
	default:
		for p := range st.Files {
			if !present[p] {
				delete(st.Files, p)
				delete(st.Bodies, p)
				res.Removed++
			}
		}
		given := map[string]bool{}
		for _, p := range paths {
			given[p] = true
		}
		for _, f := range walked.Files {
			if _, known := st.Files[f]; !known || given[f] {
				todo = append(todo, f)
			}
		}
	}
	st.Graph = wantGraph

	// Hash first; parse only files whose content differs from the record.
	var changed []string
	var mu sync.Mutex
	env := map[string][]string{}
	leaves := map[string]string{}
	for _, f := range todo {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		lf := hex.EncodeToString(parser.LeafHash(f, b))
		if fs := st.Files[f]; fs != nil && fs.Leaf == lf && (!wantGraph || fs.GraphDone) {
			continue
		}
		leaves[f] = lf
		changed = append(changed, f)
	}
	if len(changed) > 0 {
		popt := poolOptions(root, cfg, func(p string, content []byte) {
			if vars := detect.FileEnvVars(content); len(vars) > 0 {
				mu.Lock()
				env[p] = vars
				mu.Unlock()
			}
		})
		if res.Bootstrapped && cfg.CacheEnabled() {
			store := cache.Open(root)
			popt.Cache = store
			defer func() { _ = store.Save() }()
		}
		for _, r := range parser.ScanFiles(changed, popt) {
			fsn := &fileState{Leaf: leaves[r.Path], Env: env[r.Path]}
			if r.Err != nil || r.Info == nil {
				fsn.Err = true
			} else {
				fsn.Info = r.Info
			}
			st.Files[r.Path] = fsn
			delete(st.Bodies, r.Path)
		}
		res.Parsed = len(changed)
		if wantGraph {
			for _, f := range changed {
				st.Files[f].GraphDone = true
			}
			for _, s := range graph.Extract(root, changed, eo) {
				if f := st.Files[s.Path]; f != nil {
					f.Syms = s
				}
			}
		}
	}
	if res.Parsed == 0 && res.Removed == 0 && !res.Bootstrapped {
		res.Status = "nothing"
		res.StructureHash = before
		return res, nil
	}

	sc := scanFromState(st, present, walked.Files, old.Scan)
	mono, merr := monorepo.Detect(root)
	if merr != nil {
		mono = &monorepo.Result{Type: "single"}
	}
	idx := buildIndex(root, cfg, mono, sc, activityFrom(old))
	idx.Architecture = old.Architecture
	renderer.SetTimestamps(cfg.TimestampsEnabled())
	if err := renderer.WriteJSON(idx, jsonPath); err != nil {
		return nil, fmt.Errorf("writing JSON: %w", err)
	}
	if wantGraph {
		var syms []*graph.FileSyms
		for _, f := range walked.Files {
			if s := st.Files[f]; s != nil && s.Syms != nil {
				syms = append(syms, s.Syms)
			}
		}
		if st.Bodies == nil {
			st.Bodies = graph.BodyCache{}
		}
		g := graph.LinkSymbols(root, walked.Files, syms, st.Bodies)
		g.Stamp(root)
		if err := g.Save(graphPath); err != nil {
			return nil, fmt.Errorf("writing graph: %w", err)
		}
	}
	if err := st.save(dir); err != nil {
		return nil, fmt.Errorf("saving refresh state: %w", err)
	}
	res.StructureHash = session.StructureHash(idx)
	res.StructureChanged = res.StructureHash != before
	return res, nil
}

func activityFrom(idx *schema.Index) *git.Activity {
	a := &git.Activity{RecentFiles: idx.Git.Recent, StableFiles: idx.Git.Stable}
	for _, h := range idx.Git.HotFiles {
		a.HotFiles = append(a.HotFiles, git.FileActivity{Path: h.Path, Commits90d: h.Commits90d})
	}
	return a
}

// scanFromState rebuilds the scan result a full run would produce.
func scanFromState(st *refreshState, present map[string]bool, order []string, prev *schema.ScanInfo) *scanResult {
	sc := &scanResult{Leaves: map[string][]byte{}, FileEnv: map[string][]string{}}
	info := &schema.ScanInfo{}
	if prev != nil {
		info.SkippedLarge, info.SkippedSymlinks, info.SkippedOther = prev.SkippedLarge, prev.SkippedSymlinks, prev.SkippedOther
	}
	envSeen := map[string]bool{}
	for _, f := range order {
		s := st.Files[f]
		if s == nil {
			continue
		}
		if s.Err || s.Info == nil {
			info.ParseErrors = append(info.ParseErrors, f)
			continue
		}
		if s.Info.Fallback != "" {
			info.Lightweight = append(info.Lightweight, f+" ("+s.Info.Fallback+")")
		}
		sc.Files = append(sc.Files, f)
		sc.Parsed = append(sc.Parsed, s.Info)
		if b, err := hex.DecodeString(s.Leaf); err == nil {
			sc.Leaves[f] = b
		}
		for _, v := range s.Env {
			envSeen[v] = true
		}
	}
	for v := range envSeen {
		sc.EnvVars = append(sc.EnvVars, v)
	}
	sort.Strings(sc.EnvVars)
	sort.Strings(info.ParseErrors)
	sort.Strings(info.Lightweight)
	sc.MerkleHash = git.ComputeMerkleFromLeaves(sc.Leaves)
	if len(info.SkippedLarge)+len(info.SkippedSymlinks)+len(info.SkippedOther)+len(info.ParseErrors)+len(info.Lightweight) == 0 {
		info = nil
	}
	sc.Info = info
	return sc
}
