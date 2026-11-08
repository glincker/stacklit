package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glincker/stacklit/internal/engine"
	"github.com/glincker/stacklit/internal/session"
)

func execCLI(t *testing.T, stdin string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetIn(strings.NewReader(stdin))
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return out.String(), err
}

func project(t *testing.T) string {
	t.Helper()
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	t.Setenv("STACKLIT_REFRESH_DEBOUNCE_MS", "0")
	root := t.TempDir()
	for rel, body := range map[string]string{
		"go.mod":  "module example.com/p\n\ngo 1.22\n",
		"main.go": "package main\n\nfunc main() {}\n",
		"a/a.go":  "package a\n\nfunc A() {}\n",
		"b/b.go":  "package b\n\nfunc B() {}\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.Run(engine.Options{Root: root, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDigestCheckExitCodes(t *testing.T) {
	root := project(t)
	_, err := execCLI(t, "", "digest", "--check", "--root", root)
	if ExitCode(err) != 2 {
		t.Fatalf("nothing injected yet must exit 2, got %v", err)
	}
	idx, _ := session.LoadIndex(root)
	session.RecordInjected(session.StateDir(root), "s1", session.StructureHash(idx))
	if out, err := execCLI(t, "", "digest", "--check", "--root", root, "--session", "s1"); err != nil {
		t.Fatalf("unchanged structure must exit 0: %v %s", err, out)
	}
	// An ordinary edit keeps the hash; a new module moves it.
	if err := os.WriteFile(filepath.Join(root, "a", "a.go"), []byte("package a\n\nfunc A() {}\n\n// more lines\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(engine.Options{Root: root, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := execCLI(t, "", "digest", "--check", "--root", root, "--session", "s1"); err != nil {
		t.Fatalf("non-structural edit must keep exit 0: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "c"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "c", "c.go"), []byte("package c\n\nfunc C() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Run(engine.Options{Root: root, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	_, err = execCLI(t, "", "digest", "--check", "--root", root, "--session", "s1")
	if ExitCode(err) != 1 {
		t.Fatalf("structure change must exit 1, got %v", err)
	}
}

func TestPostToolHookRefreshesEditedFile(t *testing.T) {
	root := project(t)
	if err := os.WriteFile(filepath.Join(root, "b", "b.go"), []byte("package b\n\nfunc B() {}\n\nfunc Added() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse", "tool_name": "Edit", "cwd": root,
		"tool_input": map[string]string{"file_path": filepath.Join(root, "b", "b.go")},
	})
	out, err := execCLI(t, string(payload), "hook", "posttool")
	if err != nil || out != "" {
		t.Fatalf("hook must be silent and succeed: %q %v", out, err)
	}
	idx, err := session.LoadIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range idx.Modules["b"].Exports {
		found = found || strings.HasPrefix(e, "Added")
	}
	if !found {
		t.Fatalf("export not refreshed: %+v", idx.Modules["b"])
	}
	// Garbage never fails the edit loop.
	if out, err := execCLI(t, "not json", "hook", "posttool"); err != nil || out != "" {
		t.Fatalf("%q %v", out, err)
	}
}

func TestRefreshCommandJSON(t *testing.T) {
	root := project(t)
	out, err := execCLI(t, "", "refresh", "--root", root, "--files", "a/a.go", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var res engine.RefreshResult
	if json.Unmarshal([]byte(out), &res) != nil || res.Status == "" {
		t.Fatalf("bad JSON: %q", out)
	}
}

func TestStatsCommand(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	out, err := execCLI(t, "", "stats")
	if err != nil || !strings.Contains(out, "no entries") {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := execCLI(t, "", "stats", "--since", "bogus"); ExitCode(err) != ExitUsage {
		t.Fatalf("bad --since must be a usage error: %v", err)
	}
	if ts, err := parseSince("7d"); err != nil || time.Since(ts) < 6*24*time.Hour {
		t.Fatalf("%v %v", ts, err)
	}
}
