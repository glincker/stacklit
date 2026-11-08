package engine

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/session"
	"github.com/glincker/stacklit/internal/walker"
)

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeProj builds a small Go project at parent/proj. The base name is fixed so
// two copies produce comparable output.
func makeProj(t *testing.T, parent string) string {
	t.Helper()
	root := filepath.Join(parent, "proj")
	writeFile(t, root, "go.mod", "module example.com/proj\n\ngo 1.22\n")
	writeFile(t, root, "main.go", "package main\n\nimport \"example.com/proj/a\"\n\nfunc main() { a.Hello() }\n")
	writeFile(t, root, "a/a.go", "package a\n\nimport \"example.com/proj/b\"\n\nfunc Hello() string { return b.Name() }\n")
	writeFile(t, root, "b/b.go", "package b\n\nfunc Name() string { return \"x\" }\n")
	return root
}

func isolate(t *testing.T) {
	t.Helper()
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	t.Setenv("STACKLIT_REFRESH_DEBOUNCE_MS", "0")
}

// fullOutputs runs a full generate (and graph) in a fresh copy of root's files
// and returns the two output files.
func fullOutputs(t *testing.T, root string) (idx, gr []byte) {
	t.Helper()
	parent := t.TempDir()
	copy := filepath.Join(parent, "proj")
	if err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "stacklit.json" || rel == "stacklit.graph.json" || rel == "stacklit.html" || rel == "DEPENDENCIES.md" || filepath.Base(filepath.Dir(p)) == ".git" {
			return nil
		}
		b, _ := os.ReadFile(p)
		writeFile(t, copy, filepath.ToSlash(rel), string(b))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(Options{Root: copy, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Load(copy)
	files, _ := walker.Walk(copy, cfg.ScanIgnore())
	g := graph.BuildSymbols(copy, files, graph.OptionsFromEnv())
	g.Commit = git.Head(root) // the copy has no .git; compare against the original's HEAD
	if err := g.Save(filepath.Join(copy, GraphFile)); err != nil {
		t.Fatal(err)
	}
	idx, _ = os.ReadFile(filepath.Join(copy, "stacklit.json"))
	gr, _ = os.ReadFile(filepath.Join(copy, GraphFile))
	return idx, gr
}

func seed(t *testing.T) string {
	t.Helper()
	isolate(t)
	root := makeProj(t, t.TempDir())
	if _, err := Run(Options{Root: root, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	files, _ := walker.Walk(root, config.Load(root).ScanIgnore())
	g := graph.BuildSymbols(root, files, graph.OptionsFromEnv())
	g.Stamp(root)
	if err := g.Save(filepath.Join(root, GraphFile)); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertSameAsFull(t *testing.T, root string) {
	t.Helper()
	wantIdx, wantGr := fullOutputs(t, root)
	gotIdx, _ := os.ReadFile(filepath.Join(root, "stacklit.json"))
	gotGr, _ := os.ReadFile(filepath.Join(root, GraphFile))
	if !bytes.Equal(gotIdx, wantIdx) {
		t.Fatalf("index differs from full generate\nrefresh:\n%s\nfull:\n%s", gotIdx, wantIdx)
	}
	if !bytes.Equal(gotGr, wantGr) {
		t.Fatalf("graph differs from full generate (%d vs %d bytes)", len(gotGr), len(wantGr))
	}
}

func TestRefreshMatchesFullGenerate(t *testing.T) {
	root := seed(t)
	// First call builds the record, then edits are incremental.
	if _, err := Refresh(RefreshOptions{Root: root, Paths: []string{"b/b.go"}}); err != nil {
		t.Fatal(err)
	}
	assertSameAsFull(t, root)

	writeFile(t, root, "b/b.go", "package b\n\nfunc Name() string { return \"y\" }\n\nfunc Extra() int { return 2 }\n")
	res, err := Refresh(RefreshOptions{Root: root, Paths: []string{filepath.Join(root, "b", "b.go")}})
	if err != nil || res.Status != "refreshed" || res.Parsed != 1 || res.Bootstrapped {
		t.Fatalf("want 1 incremental parse, got %+v err=%v", res, err)
	}
	assertSameAsFull(t, root)

	// New module with a new edge, and a deleted file.
	writeFile(t, root, "c/c.go", "package c\n\nimport \"example.com/proj/b\"\n\nfunc Use() string { return b.Name() }\n")
	if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	res, err = Refresh(RefreshOptions{Root: root, Paths: []string{"c/c.go", "main.go"}})
	if err != nil || res.Parsed != 1 || res.Removed != 1 || !res.StructureChanged {
		t.Fatalf("got %+v err=%v", res, err)
	}
	assertSameAsFull(t, root)
}

func TestRefreshSkipsUnchangedContent(t *testing.T) {
	root := seed(t)
	if _, err := Refresh(RefreshOptions{Root: root, Paths: []string{"a/a.go"}}); err != nil {
		t.Fatal(err)
	}
	res, err := Refresh(RefreshOptions{Root: root, Paths: []string{"a/a.go"}})
	if err != nil || res.Status != "nothing" || res.Parsed != 0 {
		t.Fatalf("unchanged file must not be reparsed: %+v %v", res, err)
	}
}

func TestRefreshNoIndexIsSilent(t *testing.T) {
	isolate(t)
	root := makeProj(t, t.TempDir())
	res, err := Refresh(RefreshOptions{Root: root, Paths: []string{"main.go"}})
	if err != nil || res.Status != "no-index" {
		t.Fatalf("got %+v %v", res, err)
	}
	if _, err := os.Stat(filepath.Join(root, "stacklit.json")); err == nil {
		t.Fatal("refresh must not create an index")
	}
}

func TestRefreshConcurrentCallersStayConsistent(t *testing.T) {
	root := seed(t)
	if _, err := Refresh(RefreshOptions{Root: root, Paths: []string{"b/b.go"}}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "a/a.go", "package a\n\nimport \"example.com/proj/b\"\n\nfunc Hello() string { return b.Name() }\n\nfunc Second() {}\n")
	writeFile(t, root, "b/b.go", "package b\n\nfunc Name() string { return \"z\" }\n")
	var wg sync.WaitGroup
	for _, p := range []string{"a/a.go", "b/b.go", "a/a.go", "b/b.go"} {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			if _, err := Refresh(RefreshOptions{Root: root, Paths: []string{p}}); err != nil {
				t.Error(err)
			}
		}(p)
	}
	wg.Wait()
	// Callers that lost the lock queued their paths; one more drain settles it.
	if _, err := Refresh(RefreshOptions{Root: root}); err != nil {
		t.Fatal(err)
	}
	assertSameAsFull(t, root)
	if l, _ := session.TryLock(session.StateDir(root)); l == nil {
		t.Fatal("lock left behind")
	} else {
		l.Unlock()
	}
}

func TestRefreshRebuildsAfterBranchSwitch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := seed(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "base")
	if _, err := Refresh(RefreshOptions{Root: root, Paths: []string{"b/b.go"}}); err != nil {
		t.Fatal(err)
	}
	git("checkout", "-qb", "feature")
	writeFile(t, root, "d/d.go", "package d\n\nfunc D() {}\n")
	git("add", "-A")
	git("commit", "-qm", "feature")
	// The new file arrived with a checkout, not through a hooked edit.
	res, err := Refresh(RefreshOptions{Root: root, Paths: []string{"b/b.go"}})
	if err != nil || !res.Bootstrapped {
		t.Fatalf("HEAD change must rebuild the record: %+v %v", res, err)
	}
	assertSameAsFull(t, root)
}

func TestRefreshWithoutGit(t *testing.T) {
	root := seed(t)
	if _, err := Refresh(RefreshOptions{Root: root, Paths: []string{"b/b.go"}}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "b/b.go", "package b\n\nfunc Name() string { return \"q\" }\n")
	if res, err := Refresh(RefreshOptions{Root: root, Paths: []string{"b/b.go"}}); err != nil || res.Parsed != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	assertSameAsFull(t, root)
}
