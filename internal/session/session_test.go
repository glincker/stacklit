package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glincker/stacklit/internal/schema"
)

func idx(mods map[string][]string, edges [][2]string) *schema.Index {
	i := &schema.Index{Modules: map[string]schema.ModuleInfo{}}
	for n, ex := range mods {
		i.Modules[n] = schema.ModuleInfo{Exports: ex, Lines: 10, Files: 1}
	}
	i.Dependencies.Edges = edges
	return i
}

func TestStructureHashIgnoresNonStructure(t *testing.T) {
	a := idx(map[string][]string{"x": {"A", "B"}, "y": {"C"}}, [][2]string{{"x", "y"}})
	h := StructureHash(a)
	b := idx(map[string][]string{"y": {"C"}, "x": {"B", "A"}}, [][2]string{{"x", "y"}})
	for n, m := range b.Modules {
		m.Lines, m.Files, m.Activity, m.Purpose = 9999, 7, "high", "changed words"
		b.Modules[n] = m
	}
	b.GeneratedAt = time.Now().String()
	if StructureHash(b) != h {
		t.Fatal("line counts, order, timestamps and activity must not change the hash")
	}
	c := idx(map[string][]string{"x": {"A", "B", "Z"}, "y": {"C"}}, [][2]string{{"x", "y"}})
	d := idx(map[string][]string{"x": {"A", "B"}, "y": {"C"}}, [][2]string{{"y", "x"}})
	e := idx(map[string][]string{"x": {"A", "B"}, "y": {"C"}, "w": nil}, [][2]string{{"x", "y"}})
	for name, o := range map[string]*schema.Index{"export": c, "edge": d, "module": e} {
		if StructureHash(o) == h {
			t.Fatalf("a %s change must change the hash", name)
		}
	}
}

func TestInjectedRecord(t *testing.T) {
	dir := t.TempDir()
	if ReadInjected(dir).For("s1") != "" {
		t.Fatal("empty record must read empty")
	}
	RecordInjected(dir, "s1", "aaa")
	RecordInjected(dir, "s2", "bbb")
	r := ReadInjected(dir)
	if r.For("s1") != "aaa" || r.For("s2") != "bbb" || r.For("") != "bbb" || r.For("unknown") != "bbb" {
		t.Fatalf("%+v", r)
	}
}

func TestRootResolution(t *testing.T) {
	base := t.TempDir()
	// A linked worktree has a .git file, not a directory.
	wt := filepath.Join(base, "wt")
	sub := filepath.Join(wt, "pkg", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Root(sub); got != wt {
		t.Fatalf("worktree root: got %s want %s", got, wt)
	}
	// stacklit.json closer than .git wins (a nested package index).
	if err := os.WriteFile(filepath.Join(wt, "pkg", "stacklit.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Root(sub); got != filepath.Join(wt, "pkg") {
		t.Fatalf("nested index root: got %s", got)
	}
	plain := t.TempDir()
	if got := Root(plain); got != plain {
		t.Fatalf("no git, no index: got %s want %s", got, plain)
	}
}

func TestRootMatchesGitForRealWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	main := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(main, "init", "-q")
	if err := os.WriteFile(filepath.Join(main, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(main, "add", "-A")
	run(main, "commit", "-qm", "i")
	wt := filepath.Join(t.TempDir(), "linked")
	run(main, "worktree", "add", "-q", "-b", "side", wt)
	if err := os.MkdirAll(filepath.Join(wt, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{main, wt} {
		cwd := filepath.Join(d, "a")
		_ = os.MkdirAll(cwd, 0o755)
		want := GitToplevel(cwd)
		got, _ := filepath.EvalSymlinks(Root(cwd))
		if want == "" || got != want {
			t.Fatalf("Root %q != git toplevel %q", got, want)
		}
	}
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	if StateDir(main) == StateDir(wt) {
		t.Fatal("worktrees must not share state")
	}
}

func TestLockExcludesAndRecovers(t *testing.T) {
	dir := t.TempDir()
	l, err := TryLock(dir)
	if err != nil || l == nil {
		t.Fatalf("first lock: %v %v", l, err)
	}
	if l2, _ := TryLock(dir); l2 != nil {
		t.Fatal("second lock must fail while held")
	}
	l.Unlock()
	l3, _ := TryLock(dir)
	if l3 == nil {
		t.Fatal("lock must be free after unlock")
	}
	l3.Unlock()

	// A lock left by a crashed process is taken over once stale.
	t.Setenv("STACKLIT_LOCK_STALE_SEC", "1")
	l4, _ := TryLock(dir)
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(filepath.Join(dir, "refresh.lock"), old, old); err != nil {
		t.Fatal(err)
	}
	l5, _ := TryLock(dir)
	if l5 == nil {
		t.Fatal("stale lock must be reclaimed")
	}
	_ = l4
	l5.Unlock()
}

func TestLockContentionGoroutines(t *testing.T) {
	dir := t.TempDir()
	var holders, maxHolders int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				l, _ := TryLock(dir)
				if l == nil {
					continue
				}
				n := atomic.AddInt32(&holders, 1)
				for {
					m := atomic.LoadInt32(&maxHolders)
					if n <= m || atomic.CompareAndSwapInt32(&maxHolders, m, n) {
						break
					}
				}
				time.Sleep(time.Millisecond)
				atomic.AddInt32(&holders, -1)
				l.Unlock()
			}
		}()
	}
	wg.Wait()
	if maxHolders != 1 {
		t.Fatalf("mutual exclusion broken: %d holders at once", maxHolders)
	}
}

// Two real processes race for the lock; the child exits 0 if it got it.
func TestLockContentionAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	l, _ := TryLock(dir)
	if l == nil {
		t.Fatal("lock")
	}
	defer l.Unlock()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperTryLock")
	cmd.Env = append(os.Environ(), "STACKLIT_LOCK_HELPER="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child must be refused while parent holds the lock: %v\n%s", err, out)
	}
}

func TestHelperTryLock(t *testing.T) {
	dir := os.Getenv("STACKLIT_LOCK_HELPER")
	if dir == "" {
		t.Skip("helper")
	}
	if l, _ := TryLock(dir); l != nil {
		l.Unlock()
		t.Fatal("child acquired a held lock")
	}
}

func TestQueueDedupAndDrain(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Enqueue(dir, []string{"a.go", "b.go"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if !HasPending(dir) {
		t.Fatal("queue empty")
	}
	got := Drain(dir)
	if len(got) != 2 || got[0] != "a.go" || got[1] != "b.go" {
		t.Fatalf("drain: %v", got)
	}
	if HasPending(dir) || len(Drain(dir)) != 0 {
		t.Fatal("queue must be empty after drain")
	}
}
