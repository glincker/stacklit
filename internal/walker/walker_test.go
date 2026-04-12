package walker_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/glincker/stacklit/internal/walker"
)

// fixtureDir returns the absolute path to the go-project test fixture.
func fixtureDir(t *testing.T) string {
	t.Helper()
	// Walk up from the test file's package directory to the repo root.
	abs, err := filepath.Abs("../../testdata/go-project")
	if err != nil {
		t.Fatalf("resolve fixture dir: %v", err)
	}
	return abs
}

func TestWalkSourceFiles(t *testing.T) {
	root := fixtureDir(t)
	files, err := walker.Walk(root, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	// Must include Go source files.
	wantIncluded := []string{
		"main.go",
		filepath.ToSlash(filepath.Join("internal", "handler.go")),
		filepath.ToSlash(filepath.Join("internal", "handler_test.go")),
	}
	for _, want := range wantIncluded {
		if !fileSet[want] {
			t.Errorf("expected %q to be included, got files: %v", want, sortedKeys(fileSet))
		}
	}

	// Must exclude gitignored directories.
	wantExcluded := []string{
		filepath.ToSlash(filepath.Join("vendor", "lib.go")),
		filepath.ToSlash(filepath.Join("node_modules", "pkg.js")),
	}
	for _, exclude := range wantExcluded {
		if fileSet[exclude] {
			t.Errorf("expected %q to be excluded (gitignored), but it was included", exclude)
		}
	}

	// Must exclude non-source files.
	nonSource := []string{"README.md", "image.png", ".gitignore"}
	for _, ns := range nonSource {
		if fileSet[ns] {
			t.Errorf("expected non-source file %q to be excluded, but it was included", ns)
		}
	}
}

func TestWalkReturnsRelativePaths(t *testing.T) {
	root := fixtureDir(t)
	files, err := walker.Walk(root, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	for _, f := range files {
		if filepath.IsAbs(f) {
			t.Errorf("expected relative path but got absolute: %q", f)
		}
	}
}

func TestWalkEmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := walker.Walk(dir, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files in empty dir, got %d: %v", len(files), files)
	}
}

func TestWalkAlwaysIgnoresDirs(t *testing.T) {
	// Build a temp directory with always-ignore dirs that have no .gitignore.
	dir := t.TempDir()

	ignoredDirs := []string{"node_modules", ".git", "vendor", "dist", "build", "__pycache__"}
	for _, d := range ignoredDirs {
		subdir := filepath.Join(dir, d)
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subdir, "file.go"), []byte("package foo\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// A legitimate source file at root.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := walker.Walk(dir, nil)
	if err != nil {
		t.Fatalf("Walk returned error: %v", err)
	}

	fileSet := make(map[string]bool, len(files))
	for _, f := range files {
		fileSet[f] = true
	}

	if !fileSet["main.go"] {
		t.Error("expected main.go to be included")
	}

	for _, d := range ignoredDirs {
		p := filepath.ToSlash(filepath.Join(d, "file.go"))
		if fileSet[p] {
			t.Errorf("expected %q inside always-ignore dir to be excluded", p)
		}
	}
}

// sortedKeys returns sorted map keys for readable error messages.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
