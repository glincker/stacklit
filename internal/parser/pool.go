package parser

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
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
	info, err := ParseContent(rel, content)
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
