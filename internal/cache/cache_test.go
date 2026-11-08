package cache

import (
	"os"
	"testing"

	"github.com/glincker/stacklit/internal/parser"
)

func TestRoundTrip(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()

	s := Open(root)
	s.Put("k1", &parser.FileInfo{Path: "a.go", Language: "go", LineCount: 3})
	s.Put("k2", &parser.FileInfo{Path: "b.go"})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	s2 := Open(root)
	got, ok := s2.Get("k1")
	if !ok || got.Path != "a.go" || got.LineCount != 3 {
		t.Fatalf("round trip failed: %v %v", got, ok)
	}
	if _, ok := s2.Get("nope"); ok {
		t.Fatal("unexpected hit")
	}
	h, m := s2.Stats()
	if h != 1 || m != 1 {
		t.Fatalf("stats %d/%d", h, m)
	}
	// k2 was never read this run, so it is pruned on save.
	if err := s2.Save(); err != nil {
		t.Fatal(err)
	}
	s3 := Open(root)
	if _, ok := s3.Get("k2"); ok {
		t.Fatal("unused entry should be pruned")
	}
	if _, ok := s3.Get("k1"); !ok {
		t.Fatal("used entry should survive")
	}
}

func TestCorruptCacheIgnored(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	p := Path(root)
	if err := os.WriteFile(p, []byte("{garbage"), 0644); err != nil {
		t.Fatal(err)
	}
	s := Open(root)
	if _, ok := s.Get("x"); ok {
		t.Fatal("corrupt cache must open empty")
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
}
