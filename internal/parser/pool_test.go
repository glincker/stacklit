package parser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type memCache struct {
	mu sync.Mutex
	m  map[string]*FileInfo
}

func (c *memCache) Get(k string) (*FileInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}
func (c *memCache) Put(k string, v *FileInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = v
}

func poolFixture(t *testing.T, n int) (string, []string) {
	root := t.TempDir()
	var files []string
	for i := 0; i < n; i++ {
		rel := filepath.ToSlash(filepath.Join("pkg", "f"+string(rune('a'+i%26))+strings.Repeat("x", i/26)+".go"))
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		body := "package pkg\n\nfunc F" + strings.Repeat("A", i+1) + "() {}\n"
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		files = append(files, rel)
	}
	return root, files
}

func TestScanFilesOrderAndDeterminism(t *testing.T) {
	root, files := poolFixture(t, 60)
	a := ScanFiles(files, PoolOptions{Root: root, Workers: 1})
	b := ScanFiles(files, PoolOptions{Root: root, Workers: 8})
	if len(a) != len(files) {
		t.Fatal("length mismatch")
	}
	for i := range a {
		if a[i].Err != nil || b[i].Err != nil {
			t.Fatalf("errors: %v %v", a[i].Err, b[i].Err)
		}
		if a[i].Path != files[i] || !reflect.DeepEqual(a[i].Info, b[i].Info) || !reflect.DeepEqual(a[i].Leaf, b[i].Leaf) {
			t.Fatalf("file %d differs between worker counts", i)
		}
	}
}

func TestScanFilesCache(t *testing.T) {
	root, files := poolFixture(t, 10)
	c := &memCache{m: map[string]*FileInfo{}}
	first := ScanFiles(files, PoolOptions{Root: root, Workers: 4, Cache: c})
	second := ScanFiles(files, PoolOptions{Root: root, Workers: 4, Cache: c})
	for i := range first {
		if first[i].Cached || !second[i].Cached {
			t.Fatalf("file %d cached flags: %v %v", i, first[i].Cached, second[i].Cached)
		}
		if !reflect.DeepEqual(first[i].Info, second[i].Info) {
			t.Fatal("cached info differs")
		}
	}
	// Editing a file changes its key and forces a re-parse.
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(files[0])), []byte("package pkg\n\nfunc Changed() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	third := ScanFiles(files, PoolOptions{Root: root, Workers: 2, Cache: c})
	if third[0].Cached || !third[1].Cached {
		t.Fatal("edit did not invalidate exactly one entry")
	}
}

func TestScanFilesReadAndSizeErrors(t *testing.T) {
	root, files := poolFixture(t, 2)
	res := ScanFiles(append(files, "pkg/missing.go"), PoolOptions{Root: root, Workers: 2, MaxFileSize: 20})
	if res[2].Err == nil {
		t.Fatal("missing file should error")
	}
	if res[0].Err == nil || !strings.Contains(res[0].Err.Error(), "max_file_size") {
		t.Fatalf("size guard: %v", res[0].Err)
	}
}

func TestScanFilesEmpty(t *testing.T) {
	if len(ScanFiles(nil, PoolOptions{Root: t.TempDir(), Workers: 4})) != 0 {
		t.Fatal("expected empty")
	}
}

type panicParser struct{}

func (panicParser) CanParse(f string) bool { return strings.HasSuffix(f, ".boom") }
func (panicParser) Parse(string, []byte) (*FileInfo, error) {
	panic("kaboom")
}

func TestParseContentRecoversPanic(t *testing.T) {
	registry = append([]Parser{panicParser{}}, registry...)
	defer func() { registry = registry[1:] }()

	info, err := ParseContent("x.boom", []byte("data"))
	if info != nil || err == nil {
		t.Fatalf("expected recovered error, got %v %v", info, err)
	}
	if _, ok := err.(*PanicError); !ok {
		t.Fatalf("want PanicError, got %T", err)
	}

	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "x.boom"), []byte("a"), 0644)
	_ = os.WriteFile(filepath.Join(root, "ok.go"), []byte("package a\n"), 0644)
	res := ScanFiles([]string{"x.boom", "ok.go"}, PoolOptions{Root: root, Workers: 2})
	if !res[0].Panicked || res[1].Err != nil || res[1].Info == nil {
		t.Fatalf("panic must stay isolated: %+v %+v", res[0], res[1])
	}
}

func TestMemGateRunsOversizedAlone(t *testing.T) {
	g := newMemGate(100)
	g.acquire(500) // larger than the whole budget, admitted because the gate is empty
	done := make(chan struct{})
	go func() { g.acquire(10); close(done) }()
	select {
	case <-done:
		t.Fatal("second request admitted while an oversized one is running")
	default:
	}
	g.release(500)
	<-done
	g.release(10)
}
