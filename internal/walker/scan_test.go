package walker

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func mk(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func skipReasons(r *Result) map[string]string {
	m := map[string]string{}
	for _, s := range r.Skipped {
		m[s.Path] = s.Reason
	}
	return m
}

func needSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on windows")
	}
}

func TestSymlinks(t *testing.T) {
	needSymlinks(t)
	root := t.TempDir()
	outside := t.TempDir()
	mk(t, root, "a/real.go", "package a")
	mk(t, outside, "secret.go", "package s")
	mk(t, outside, "dir/x.go", "package x")

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Symlink(filepath.Join(root, "a", "real.go"), filepath.Join(root, "a", "alias.go")))
	must(os.Symlink(filepath.Join(outside, "secret.go"), filepath.Join(root, "a", "escape.go")))
	must(os.Symlink(filepath.Join(root, "nowhere.go"), filepath.Join(root, "a", "broken.go")))
	must(os.Symlink(root, filepath.Join(root, "a", "loop")))
	must(os.Symlink(filepath.Join(outside, "dir"), filepath.Join(root, "linkdir")))
	must(os.Symlink("../a", filepath.Join(root, "a", "self")))

	res, err := WalkWith(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a/alias.go", "a/real.go"}
	if !reflect.DeepEqual(res.Files, want) {
		t.Fatalf("files = %v, want %v", res.Files, want)
	}
	sk := skipReasons(res)
	if sk["a/escape.go"] != ReasonSymlinkEscape || sk["a/broken.go"] != ReasonBrokenSymlink {
		t.Fatalf("skips: %v", sk)
	}
}

func TestMaxFileSize(t *testing.T) {
	root := t.TempDir()
	mk(t, root, "small.go", "package a")
	mk(t, root, "big.go", "package a\n"+strings.Repeat("// x\n", 1000))
	res, err := WalkWith(root, Options{MaxFileSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Files, []string{"small.go"}) || skipReasons(res)["big.go"] != ReasonLarge {
		t.Fatalf("%v %v", res.Files, res.Skipped)
	}
	res, _ = WalkWith(root, Options{})
	if len(res.Files) != 2 {
		t.Fatalf("no cap should keep both: %v", res.Files)
	}
}

func TestRootsIncludeExclude(t *testing.T) {
	root := t.TempDir()
	mk(t, root, "src/a.go", "package a")
	mk(t, root, "src/gen/b.go", "package b")
	mk(t, root, "lib/c.ts", "export {}")
	mk(t, root, "other/d.go", "package d")

	res, _ := WalkWith(root, Options{Roots: []string{"src", "lib", "src/gen", "missing"}, ExtraIgnore: []string{"gen/"}})
	if !reflect.DeepEqual(res.Files, []string{"lib/c.ts", "src/a.go"}) {
		t.Fatalf("roots: %v", res.Files)
	}
	if skipReasons(res)["missing"] != ReasonMissingRoot {
		t.Fatalf("missing root not reported: %v", res.Skipped)
	}

	res, _ = WalkWith(root, Options{Include: []string{"*.ts"}})
	if !reflect.DeepEqual(res.Files, []string{"lib/c.ts"}) {
		t.Fatalf("include: %v", res.Files)
	}

	res, _ = WalkWith(root, Options{Roots: []string{"../" + filepath.Base(root)}})
	if len(res.Files) != 0 {
		t.Fatalf("escaping root must scan nothing: %v", res.Files)
	}
}

func TestIgnoreFiles(t *testing.T) {
	root := t.TempDir()
	mk(t, root, ".gitignore", "ignored/\n")
	mk(t, root, ".stacklitignore", "skipme.go\n")
	mk(t, root, "ignored/a.go", "package a")
	mk(t, root, "skipme.go", "package a")
	mk(t, root, "keep.go", "package a")
	mk(t, root, "pkg/.gitignore", "local.go\n")
	mk(t, root, "pkg/local.go", "package p")
	mk(t, root, "pkg/ok.go", "package p")
	mk(t, root, "node_modules/x/y.go", "package y")

	res, _ := WalkWith(root, Options{})
	want := []string{"keep.go", "pkg/ok.go"}
	if !reflect.DeepEqual(res.Files, want) {
		t.Fatalf("files = %v, want %v", res.Files, want)
	}
}

func TestMissingRootDir(t *testing.T) {
	if _, err := WalkWith(filepath.Join(t.TempDir(), "nope"), Options{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeRootsWindowsStyle(t *testing.T) {
	roots, esc, whole := normalizeRoots([]string{`src\app`, `src\app\sub`, `..\x`, "lib/"})
	if whole || len(esc) != 1 || len(roots) != 2 || roots[0] != "lib" || roots[1] != "src/app" {
		t.Fatalf("roots=%v esc=%v whole=%v", roots, esc, whole)
	}
}
