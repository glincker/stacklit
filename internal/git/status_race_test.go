package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A post-commit hook regenerates the index within the commit's own second, so
// that commit must not count as newer than the index.
func TestCheckStaleSameSecondCommitIsNotNewer(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	built := time.Now()
	s := CheckStale(root, "", built, nil)
	if s.CommitsBehind != 0 || len(s.Changed) != 0 {
		t.Fatalf("fresh index reported stale: %+v", s)
	}
}
