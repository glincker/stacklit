// Package cache persists parse results between runs, keyed by a hash of each
// file's path and content, so regeneration only re-parses what changed.
package cache

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/glincker/stacklit/internal/fsutil"
	"github.com/glincker/stacklit/internal/parser"
)

// formatVersion is bumped when the on-disk layout changes.
const formatVersion = 1

type fileFormat struct {
	Format      int                         `json:"format"`
	Fingerprint string                      `json:"fingerprint"`
	Entries     map[string]*parser.FileInfo `json:"entries"`
}

// Store is a concurrency-safe parse cache bound to one repo root.
type Store struct {
	path        string
	fingerprint string

	mu     sync.Mutex
	loaded map[string]*parser.FileInfo
	fresh  map[string]*parser.FileInfo // entries used or added this run
	hits   int
	misses int
}

// Open loads the cache for root. A missing, corrupt or stale (different
// fingerprint) cache file yields an empty store, never an error.
func Open(root string) *Store {
	s := &Store{
		fingerprint: fingerprint(),
		loaded:      map[string]*parser.FileInfo{},
		fresh:       map[string]*parser.FileInfo{},
	}
	s.path = Path(root)
	if s.path == "" {
		return s
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	var f fileFormat
	if json.Unmarshal(data, &f) != nil || f.Format != formatVersion || f.Fingerprint != s.fingerprint {
		return s
	}
	if f.Entries != nil {
		s.loaded = f.Entries
	}
	return s
}

// Path returns the cache file location for root, or "" when no cache
// directory is available. STACKLIT_CACHE_DIR overrides the default.
func Path(root string) string {
	dir := os.Getenv("STACKLIT_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(base, "stacklit")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(dir, fmt.Sprintf("%x.json", sum[:8]))
}

// Get implements parser.Cache.
func (s *Store) Get(key string) (*parser.FileInfo, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info, ok := s.loaded[key]
	if !ok {
		s.misses++
		return nil, false
	}
	s.hits++
	s.fresh[key] = info
	cp := *info
	return &cp, true
}

// Put implements parser.Cache.
func (s *Store) Put(key string, info *parser.FileInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *info
	s.fresh[key] = &cp
}

// Stats reports cache hits and misses for this run.
func (s *Store) Stats() (hits, misses int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits, s.misses
}

// Save writes only the entries touched this run, so deleted or edited files
// do not accumulate. Failure is reported but is never fatal to a scan.
func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	s.mu.Lock()
	data, err := json.Marshal(fileFormat{Format: formatVersion, Fingerprint: s.fingerprint, Entries: s.fresh})
	s.mu.Unlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data, 0o644)
}

// fingerprint ties the cache to the running binary, so any parser change
// (new release or a local rebuild) invalidates stale entries.
func fingerprint() string {
	h := sha256.New()
	fmt.Fprintf(h, "v%d", formatVersion)
	if exe, err := os.Executable(); err == nil {
		if st, err := os.Stat(exe); err == nil {
			fmt.Fprintf(h, "|%d|%d", st.Size(), st.ModTime().UnixNano())
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)[:12])
}
