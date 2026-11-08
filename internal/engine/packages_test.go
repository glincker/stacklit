package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glincker/stacklit/internal/parser"
)

func TestBuildPackagesManifestAndRelativeEdges(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("apps/web/package.json", `{"name":"@acme/web"}`)
	write("packages/ui/package.json", `{"name":"@acme/ui"}`)
	write("packages/core/package.json", `{"name":"@acme/core"}`)

	parsed := []*parser.FileInfo{
		{Path: "apps/web/src/a.ts", LineCount: 10, Imports: []string{"@acme/ui/button", "react"}},
		{Path: "apps/web/src/b.ts", LineCount: 5, Imports: []string{"@acme/ui"}},
		{Path: "packages/ui/src/x.ts", LineCount: 7, Imports: []string{"../../core/src/y"}},
		{Path: "packages/core/src/y.ts", LineCount: 3},
	}
	pkgs, edges := buildPackages(root, []string{"apps/web", "packages/ui", "packages/core"}, parsed, nil, []string{"apps/web/src", "packages/ui/src", "packages/core/src"})

	if len(pkgs) != 3 || pkgs[0].Name != "@acme/web" || pkgs[0].Files != 2 || pkgs[0].Lines != 15 {
		t.Fatalf("packages: %+v", pkgs)
	}
	want := map[[2]string]int{
		{"@acme/web", "@acme/ui"}:  2,
		{"@acme/ui", "@acme/core"}: 1,
	}
	got := map[[2]string]int{}
	for _, e := range edges {
		got[[2]string{e.From, e.To}] = e.Weight
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edges = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(pkgs[2].DependedBy, []string{"@acme/web"}) {
		t.Fatalf("depended_by: %v", pkgs[1].DependedBy)
	}
}

func TestRunMonorepoReportsPackages(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := copyTree(t, "../../testdata/monorepo")
	writeRC(t, root, `{"timestamps":false,"git_activity":false}`)
	res, err := Run(Options{Root: root, Quiet: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Index.Packages) != 3 {
		t.Fatalf("packages: %+v", res.Index.Packages)
	}
	found := false
	for _, e := range res.Index.PackageEdges {
		if e.From == "@test/web" && e.To == "@test/shared" {
			found = true
		}
	}
	if !found {
		t.Fatalf("web -> shared edge missing: %+v", res.Index.PackageEdges)
	}
}
