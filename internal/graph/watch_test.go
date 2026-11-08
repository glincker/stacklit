package graph

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitFor(t *testing.T, out string, cond func(*SymGraph) bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if g, err := Load(out); err == nil && cond(g) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("graph file never reached the expected state")
}

func TestWatchIncremental(t *testing.T) {
	for _, poll := range []bool{false, true} {
		name := "fsnotify"
		if poll {
			name = "poll"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write := func(rel, src string) {
				p := filepath.Join(root, rel)
				_ = os.MkdirAll(filepath.Dir(p), 0o755)
				if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			write("a.go", "package a\n\nfunc One() {}\n")
			out := filepath.Join(root, "stacklit.graph.json")
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- Watch(ctx, WatchOptions{Root: root, Out: out, Debounce: 30 * time.Millisecond, PollInterval: 100 * time.Millisecond, ForcePoll: poll})
			}()
			has := func(name string) func(*SymGraph) bool {
				return func(g *SymGraph) bool {
					for _, n := range g.Nodes {
						if n.Name == name {
							return true
						}
					}
					return false
				}
			}
			waitFor(t, out, has("One"))
			write("a.go", "package a\n\nfunc One() {}\n\nfunc Two() {}\n")
			waitFor(t, out, has("Two"))
			write("sub/b.go", "package sub\n\nfunc Three() {}\n")
			waitFor(t, out, has("Three"))
			if err := os.Remove(filepath.Join(root, "sub/b.go")); err != nil {
				t.Fatal(err)
			}
			waitFor(t, out, func(g *SymGraph) bool { return !has("Three")(g) })
			cancel()
			if err := <-done; err != nil && !strings.Contains(err.Error(), "canceled") {
				t.Fatal(err)
			}
		})
	}
}

func TestWatchDirsOnlyCoverSources(t *testing.T) {
	dirs := watchDirs([]string{"a/b/c.go", "d.go"})
	want := []string{".", "a", "a/b"}
	if strings.Join(dirs, ",") != strings.Join(want, ",") {
		t.Fatalf("dirs = %v, want %v", dirs, want)
	}
}
