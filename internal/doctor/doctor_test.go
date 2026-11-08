package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/engine"
)

func setup(t *testing.T) string {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "a.go"), []byte("package pkg\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func has(r *Report, l Level, check, sub string) bool {
	for _, f := range r.Findings {
		if f.Level == l && f.Check == check && strings.Contains(f.Message, sub) {
			return true
		}
	}
	return false
}

func TestMissingThenFreshThenStale(t *testing.T) {
	root := setup(t)
	r, err := Run(root)
	if err != nil {
		t.Fatal(err)
	}
	if !has(r, Warn, "index", "does not exist") {
		t.Fatalf("expected missing index warning: %+v", r.Findings)
	}

	if _, err := engine.Run(engine.Options{Root: root, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	r, _ = Run(root)
	if !has(r, OK, "index", "up to date") || r.Count(Error) != 0 {
		t.Fatalf("expected fresh index: %+v", r.Findings)
	}

	if err := os.WriteFile(filepath.Join(root, "pkg", "b.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, _ = Run(root)
	if !has(r, Warn, "index", "stale") {
		t.Fatalf("expected stale warning: %+v", r.Findings)
	}
}

func TestBadConfigAndLargeFile(t *testing.T) {
	root := setup(t)
	_ = os.WriteFile(filepath.Join(root, ".stacklitrc.json"), []byte(`{"max_file_size":10,"oops":1}`), 0o644)
	r, _ := Run(root)
	if !has(r, Error, "config", "oops") && !has(r, Error, "config", "unknown") {
		t.Fatalf("expected config error: %+v", r.Findings)
	}
	if !has(r, Warn, "scan", "skipped") {
		t.Fatalf("expected large-file warning: %+v", r.Findings)
	}
}

func TestNotADirectory(t *testing.T) {
	if _, err := Run(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error")
	}
}
