package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParseReposFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "repos.txt")
	if err := os.WriteFile(f, []byte("# comment\n\nrepoA\r\nrepoA\n./repoB/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repos, err := ParseReposFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 2 || repos[0] != filepath.Join(dir, "repoA") || repos[1] != filepath.Join(dir, "repoB") {
		t.Fatalf("repos: %v", repos)
	}
	if err := os.WriteFile(f, []byte("# nothing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseReposFile(f); err == nil {
		t.Fatal("empty list must error")
	}
	if _, err := ParseReposFile(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing file must error")
	}
}

func TestRunMultiCrossRepoEdgesAndNoWrites(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"lib/go.mod":       "module example.com/lib\n\ngo 1.22\n",
		"lib/core/core.go": "package core\n\nfunc Do() {}\n",
		"app/go.mod":       "module example.com/app\n\ngo 1.22\n",
		"app/main.go":      "package main\n\nimport \"example.com/lib/core\"\n\nfunc main() { core.Do() }\n",
		"web/package.json": `{"name":"@acme/web"}`,
		"web/src/index.ts": "import x from 'react'\nexport const a = x\n",
		"repos.txt":        "lib\napp\nweb\nmissing-repo\n",
	})
	out := filepath.Join(dir, "out", "multi.json")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := RunMulti(MultiOptions{ReposFile: filepath.Join(dir, "repos.txt"), Output: out, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.RepoCount != 3 || res.Failed != 1 {
		t.Fatalf("counts: %+v", res)
	}
	found := false
	for _, e := range res.Index.CrossEdges {
		if e.From == "app" && e.To == "lib" && e.Weight >= 1 {
			found = true
		}
		if e.From == "web" || e.To == "web" {
			t.Fatalf("unexpected edge touching web: %+v", e)
		}
	}
	if !found {
		t.Fatalf("app -> lib edge missing: %+v", res.Index.CrossEdges)
	}
	for _, r := range []string{"lib", "app", "web"} {
		if _, err := os.Stat(filepath.Join(dir, r, "stacklit.json")); err == nil {
			t.Fatalf("multi scan wrote into member repo %s", r)
		}
	}
	data, _ := os.ReadFile(out)
	if strings.Contains(string(data), "generated_at") {
		t.Fatal("multi index must not carry timestamps by default")
	}

	// A second run must be byte-identical.
	res2, err := RunMulti(MultiOptions{ReposFile: filepath.Join(dir, "repos.txt"), Output: out, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(out)
	if string(data) != string(data2) || res2.RepoCount != 3 {
		t.Fatal("multi output not deterministic")
	}
}

func TestRunMultiAllFail(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "r.txt"), []byte("nope1\nnope2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RunMulti(MultiOptions{ReposFile: filepath.Join(dir, "r.txt"), Output: filepath.Join(dir, "o.json"), Quiet: true}); err == nil {
		t.Fatal("expected error when every repo fails")
	}
}
