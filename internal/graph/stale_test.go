package graph

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "core.hooksPath=/dev/null", "-c", "user.email=t@t", "-c", "user.name=t"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}

func TestStaleLine(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "a.go")
	if err := os.WriteFile(src, []byte("package a\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "init", "-q")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-qm", "init")
	g := synth()
	g.Stamp(dir)
	if g.Commit == "" {
		t.Fatal("stamp recorded no commit")
	}
	built := time.Now()
	if line := StaleLine(dir, g, built); line != "" {
		t.Fatalf("fresh graph reported stale: %q", line)
	}
	later := built.Add(time.Hour)
	if err := os.WriteFile(src, []byte("package a\n\nfunc A() {}\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(src, later, later); err != nil {
		t.Fatal(err)
	}
	line := StaleLine(dir, g, built)
	if !strings.Contains(line, "1 source files changed") || !strings.Contains(line, "a.go") || strings.Count(line, "\n") != 1 {
		t.Fatalf("unexpected stale line %q", line)
	}
	t.Setenv("STACKLIT_NO_STALE", "1")
	if StaleLine(dir, g, built) != "" {
		t.Fatal("STACKLIT_NO_STALE ignored")
	}
}
