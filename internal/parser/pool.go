package parser

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

// Cache stores parse results keyed by a content-and-path hash so unchanged
// files skip parsing on the next run.
type Cache interface {
	Get(key string) (*FileInfo, bool)
	Put(key string, info *FileInfo)
}

// PoolOptions configures ScanFiles.
type PoolOptions struct {
	Root        string // files are read from Root/<rel>
	Workers     int    // pool size; values below 1 mean 1
	MaxFileSize int64  // content larger than this is rejected (race with growth); 0 disables
	Cache       Cache  // optional
	// Visit sees each file's bytes once, inside the worker, so callers can
	// extract extra data without keeping every file in memory. It must be
	// safe for concurrent use.
	Visit func(rel string, content []byte)
	// MemoryBudget caps the estimated working set of concurrent tree-sitter
	// parses, in bytes. One file larger than the budget still runs, alone.
	// 0 disables the cap.
	MemoryBudget int64

	gate *memGate
}

// Scanned is the per-file outcome of ScanFiles.
type Scanned struct {
	Path     string
	Info     *FileInfo
	Leaf     []byte // sha256(path || content), the Merkle leaf and cache key
	Err      error
	Cached   bool
	Panicked bool
}

// ParseContent runs the registered parsers on in-memory content. A panic in
// any parser is converted into an error so one bad file cannot kill a scan.
func ParseContent(path string, content []byte) (info *FileInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			info = nil
			err = &PanicError{Path: path, Value: r, Stack: string(debug.Stack())}
		}
	}()
	for _, p := range registry {
		if p.CanParse(path) {
			return p.Parse(path, content)
		}
	}
	return (&GenericParser{}).Parse(path, content)
}

// PanicError reports a recovered parser panic.
type PanicError struct {
	Path  string
	Value any
	Stack string
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("parser panic in %s: %v", e.Path, e.Value)
}

// LeafHash is the hash used both as Merkle leaf and cache key.
func LeafHash(path string, content []byte) []byte {
	h := sha256.New()
	h.Write([]byte(path))
	h.Write(content)
	return h.Sum(nil)
}

// ScanFiles parses files with a bounded worker pool. At most Workers file
// bodies are in memory at once. Results are returned in input order, so
// output does not depend on scheduling.
func ScanFiles(files []string, o PoolOptions) []Scanned {
	out := make([]Scanned, len(files))
	workers := o.Workers
	if workers < 1 {
		workers = 1
	}
	if workers > len(files) {
		workers = len(files)
	}

	o.gate = newMemGate(o.MemoryBudget)
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				out[i] = scanOne(files[i], o)
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out
}

func scanOne(rel string, o PoolOptions) (res Scanned) {
	res.Path = rel
	content, err := os.ReadFile(filepath.Join(o.Root, filepath.FromSlash(rel)))
	if err != nil {
		res.Err = err
		return res
	}
	if o.MaxFileSize > 0 && int64(len(content)) > o.MaxFileSize {
		res.Err = fmt.Errorf("%s: %d bytes exceeds max_file_size", rel, len(content))
		return res
	}
	res.Leaf = LeafHash(rel, content)
	key := fmt.Sprintf("%x", res.Leaf)

	if o.Visit != nil {
		func() {
			defer func() { _ = recover() }()
			o.Visit(rel, content)
		}()
	}
	if o.Cache != nil {
		if info, ok := o.Cache.Get(key); ok {
			res.Info, res.Cached = info, true
			return res
		}
	}
	cost := parseCost(rel, len(content))
	o.gate.acquire(cost)
	info, err := ParseContent(rel, content)
	o.gate.release(cost)
	if err != nil {
		res.Err = err
		_, res.Panicked = err.(*PanicError)
		return res
	}
	res.Info = info
	if o.Cache != nil && info != nil {
		o.Cache.Put(key, info)
	}
	return res
}

// parseCost estimates the peak bytes a tree-sitter parse of one file holds.
// The GLR parser allocates roughly 6.5 KB per source byte plus a fixed floor
// (measured on Python and TypeScript sources). Go uses the stdlib parser.
func parseCost(rel string, size int) int64 {
	if strings.HasSuffix(rel, ".go") {
		return 0
	}
	return 24<<20 + int64(size)*6500
}

// memGate is a weighted semaphore. A request larger than the whole budget
// waits for an empty gate and then runs alone.
type memGate struct {
	mu     sync.Mutex
	cond   *sync.Cond
	budget int64
	used   int64
}

func newMemGate(budget int64) *memGate {
	if budget <= 0 {
		return nil
	}
	g := &memGate{budget: budget}
	g.cond = sync.NewCond(&g.mu)
	return g
}

func (g *memGate) acquire(n int64) {
	if g == nil || n <= 0 {
		return
	}
	g.mu.Lock()
	for g.used > 0 && g.used+n > g.budget {
		g.cond.Wait()
	}
	g.used += n
	g.mu.Unlock()
}

func (g *memGate) release(n int64) {
	if g == nil || n <= 0 {
		return
	}
	g.mu.Lock()
	g.used -= n
	g.mu.Unlock()
	g.cond.Broadcast()
}
