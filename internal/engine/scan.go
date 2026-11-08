package engine

import (
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/glincker/stacklit/internal/cache"
	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/detect"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/parser"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/walker"
)

// scanResult is everything the walk-and-parse stage hands to index assembly.
type scanResult struct {
	Files      []string
	Parsed     []*parser.FileInfo
	EnvVars    []string
	MerkleHash string
	// Leaves and FileEnv are per-file facts the incremental refresh keeps.
	Leaves      map[string][]byte
	FileEnv     map[string][]string
	Info        *schema.ScanInfo
	ParseErrors int
	CacheHits   int
	CacheMisses int
}

// scanRepo walks root, parses every file with a bounded worker pool and
// collects the data the index needs, without holding all file bodies in memory.
func scanRepo(root string, cfg *config.Config, quiet bool) (*scanResult, error) {
	walked, err := walker.WalkWith(root, walker.Options{
		ExtraIgnore: cfg.ScanIgnore(),
		Include:     cfg.Include,
		Roots:       cfg.NormalizedRoots(),
		MaxFileSize: cfg.MaxFileSize,
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", root, err)
	}
	if !quiet {
		fmt.Printf("[stacklit] found %d files\n", len(walked.Files))
	}

	if lim := cfg.MemoryLimitBytes(); lim > 0 {
		defer debug.SetMemoryLimit(debug.SetMemoryLimit(lim))
	}

	var store *cache.Store
	var pc parser.Cache
	if cfg.CacheEnabled() {
		store = cache.Open(root)
		pc = store
	}

	var mu sync.Mutex
	envSeen := map[string]bool{}
	fileEnv := map[string][]string{}
	results := parser.ScanFiles(walked.Files, parser.PoolOptions{
		Root:         root,
		Workers:      cfg.WorkerCount(),
		MemoryBudget: cfg.ParseMemoryBytes(),
		MaxFileSize:  cfg.MaxFileSize,
		Limits:       cfg.ParseLimits(),
		Cache:        pc,
		Visit: func(path string, content []byte) {
			vars := detect.FileEnvVars(content)
			if len(vars) == 0 {
				return
			}
			mu.Lock()
			fileEnv[path] = vars
			for _, v := range vars {
				envSeen[v] = true
			}
			mu.Unlock()
		},
	})

	out := &scanResult{Info: &schema.ScanInfo{}}
	leaves := make(map[string][]byte, len(results))
	var kept []string
	for _, r := range results {
		if r.Err != nil || r.Info == nil {
			out.ParseErrors++
			out.Info.ParseErrors = append(out.Info.ParseErrors, r.Path)
			if !quiet {
				fmt.Fprintf(os.Stderr, "[stacklit] warning: skipping %s: %v\n", r.Path, r.Err)
			}
			continue
		}
		if r.Info.Fallback != "" {
			out.Info.Lightweight = append(out.Info.Lightweight, r.Path+" ("+r.Info.Fallback+")")
		}
		if r.Dur >= traceSlow {
			fmt.Fprintf(os.Stderr, "[stacklit trace] slow parse %s %s\n", r.Path, r.Dur.Round(time.Millisecond))
		}
		kept = append(kept, r.Path)
		out.Parsed = append(out.Parsed, r.Info)
		leaves[r.Path] = r.Leaf
	}
	out.Files = kept
	out.Leaves, out.FileEnv = leaves, fileEnv
	out.MerkleHash = git.ComputeMerkleFromLeaves(leaves)

	for _, s := range walked.Skipped {
		switch s.Reason {
		case walker.ReasonLarge:
			out.Info.SkippedLarge = append(out.Info.SkippedLarge, s.Path)
		case walker.ReasonBrokenSymlink, walker.ReasonSymlinkEscape, walker.ReasonSymlinkDir:
			out.Info.SkippedSymlinks = append(out.Info.SkippedSymlinks, s.Path)
		default:
			out.Info.SkippedOther = append(out.Info.SkippedOther, s.Path)
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "[stacklit] warning: skipped %s (%s)\n", s.Path, s.Reason)
		}
	}
	sort.Strings(out.Info.ParseErrors)
	sort.Strings(out.Info.Lightweight)

	for v := range envSeen {
		out.EnvVars = append(out.EnvVars, v)
	}
	sort.Strings(out.EnvVars)

	if store != nil {
		out.CacheHits, out.CacheMisses = store.Stats()
		if err := store.Save(); err != nil && !quiet {
			fmt.Fprintf(os.Stderr, "[stacklit] warning: could not save cache: %v\n", err)
		}
	}
	if !quiet {
		fmt.Printf("[stacklit] parsed %d files (%d errors, %d cached)\n", len(out.Parsed), out.ParseErrors, out.CacheHits)
	}

	if len(out.Info.SkippedLarge)+len(out.Info.SkippedSymlinks)+len(out.Info.SkippedOther)+len(out.Info.ParseErrors)+len(out.Info.Lightweight) == 0 {
		out.Info = nil
	}
	return out, nil
}

// mergeEnvVars unions scanned env vars with the file-based sources (.env.example etc).
func mergeEnvVars(root string, scanned []string) []string {
	seen := map[string]bool{}
	for _, v := range detect.DetectEnvVars(root, nil) {
		seen[v] = true
	}
	for _, v := range scanned {
		seen[v] = true
	}
	out := make([]string, 0, len(seen))
	for v := range seen {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// traceSlow is the parse time above which STACKLIT_TRACE reports a file; it
// is 0 (never) unless tracing is on.
var traceSlow = func() time.Duration {
	if os.Getenv("STACKLIT_TRACE") == "" {
		return 1 << 62
	}
	return 250 * time.Millisecond
}()
