package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// copyTree copies src into a fresh temp dir so Run can write outputs freely.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func writeRC(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, ".stacklitrc.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runJSON(t *testing.T, root string) []byte {
	t.Helper()
	if _, err := Run(Options{Root: root, Quiet: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "stacklit.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRunDeterministicAndCacheTransparent(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := copyTree(t, "../../testdata/go-project")
	writeRC(t, root, `{"timestamps":false,"git_activity":false}`)

	cold := runJSON(t, root)
	warm := runJSON(t, root) // served from cache
	if !bytes.Equal(cold, warm) {
		t.Fatal("cached run differs from cold run")
	}
	for i := 0; i < 5; i++ {
		if !bytes.Equal(cold, runJSON(t, root)) {
			t.Fatalf("run %d differs", i)
		}
	}

	writeRC(t, root, `{"timestamps":false,"git_activity":false,"cache":false,"workers":1}`)
	if !bytes.Equal(cold, runJSON(t, root)) {
		t.Fatal("output depends on cache or worker count")
	}
	s := string(cold)
	if strings.Contains(s, "generated_at") {
		t.Fatal("timestamps disabled but generated_at present")
	}
	if !strings.Contains(s, `"schema_version": 1`) {
		t.Fatal("schema_version missing")
	}
}

func TestRunSkipsBadInputsGracefully(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "src"), 0o755))
	must(os.WriteFile(filepath.Join(root, "src", "ok.go"), []byte("package src\n\nfunc A() {}\n"), 0o644))
	must(os.WriteFile(filepath.Join(root, "src", "huge.go"), []byte("package src\n"+strings.Repeat("// pad\n", 400)), 0o644))
	must(os.WriteFile(filepath.Join(root, "src", "junk.go"), []byte("\x00\xff not go {{{"), 0o644))
	writeRC(t, root, `{"max_file_size":500,"timestamps":false}`)

	res, err := Run(Options{Root: root, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	idx := res.Index
	if idx.Scan == nil || len(idx.Scan.SkippedLarge) != 1 || idx.Scan.SkippedLarge[0] != "src/huge.go" {
		t.Fatalf("scan info: %+v", idx.Scan)
	}
	if idx.Structure.TotalFiles < 1 {
		t.Fatal("good file lost")
	}
}

func TestRunCustomRoots(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	for _, rel := range []string{"a/x.go", "b/y.go"} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("package p\n"), 0o644)
	}
	writeRC(t, root, `{"roots":["a"]}`)
	res, err := Run(Options{Root: root, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Index.Structure.TotalFiles != 1 {
		t.Fatalf("roots ignored: %d files", res.Index.Structure.TotalFiles)
	}
}
