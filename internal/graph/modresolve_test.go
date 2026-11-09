package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glincker/stacklit/internal/parser"
)

func fi(path string, imports ...string) *parser.FileInfo {
	return &parser.FileInfo{Path: path, Imports: imports, Language: "x"}
}

func dependedBy(g *Graph, mod string) []string {
	m := g.Module(mod)
	if m == nil {
		return nil
	}
	return m.DependedBy
}

func TestRustWorkspaceDependents(t *testing.T) {
	root := t.TempDir()
	for dir, name := range map[string]string{"crates/matcher": "grep-matcher", "crates/regex": "grep-regex", "crates/core": "ripgrep"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		body := fmt.Sprintf("[package]\nname = \"%s\"\n", name)
		if err := os.WriteFile(filepath.Join(root, dir, "Cargo.toml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := []*parser.FileInfo{
		fi("crates/matcher/src/lib.rs", "interpolate"),
		fi("crates/matcher/src/interpolate.rs"),
		fi("crates/regex/src/lib.rs", "grep_matcher", "config", "std::io"),
		fi("crates/regex/src/config.rs", "crate::error", "super::util"),
		fi("crates/regex/src/error.rs"),
		fi("crates/regex/src/util/mod.rs"),
		fi("crates/core/src/main.rs", "grep_regex::RegexMatcher", "crate::flags::parse"),
		fi("crates/core/src/flags/parse.rs", "crate::search"),
		fi("crates/core/src/search.rs"),
	}
	g := Build(files, BuildOptions{Root: root})
	if got, want := dependedBy(g, "crates/matcher/src"), []string{"crates/regex/src"}; !reflect.DeepEqual(got, want) {
		t.Errorf("matcher depended_by = %v, want %v", got, want)
	}
	if got, want := dependedBy(g, "crates/regex/src"), []string{"crates/core/src"}; !reflect.DeepEqual(got, want) {
		t.Errorf("regex depended_by = %v, want %v", got, want)
	}
	if got, want := dependedBy(g, "crates/core/src"), []string{"crates/core/src/flags"}; !reflect.DeepEqual(got, want) {
		t.Errorf("core depended_by = %v, want %v", got, want)
	}
	// crate::error from config.rs stays inside one module: no self edge.
	for _, d := range dependedBy(g, "crates/regex/src") {
		if d == "crates/regex/src" {
			t.Errorf("self edge recorded")
		}
	}
}

func TestRustCrateNameFallsBackToDirectory(t *testing.T) {
	files := []*parser.FileInfo{
		fi("crates/globset/src/lib.rs"),
		fi("crates/ignore/src/lib.rs", "globset::Glob"),
	}
	g := Build(files, BuildOptions{})
	if got := dependedBy(g, "crates/globset/src"); !reflect.DeepEqual(got, []string{"crates/ignore/src"}) {
		t.Errorf("globset depended_by = %v", got)
	}
}

func TestJavaPackageDependentsAndSplit(t *testing.T) {
	var files []*parser.FileInfo
	base := "gson/src/main/java/com/google/gson"
	files = append(files,
		fi(base+"/Gson.java", "com.google.gson.internal", "com.google.gson.reflect"),
		fi(base+"/internal/Streams.java", "com.google.gson.stream"),
		fi(base+"/internal/bind/Adapters.java", "com.google.gson.internal", "static com.google.gson.internal.Excluder.EXCLUDER"),
		fi(base+"/reflect/TypeToken.java"),
		fi(base+"/stream/JsonReader.java"),
		fi("extras/src/main/java/com/google/gson/extras/Foo.java", "com.google.gson.internal"),
	)
	g := Build(files, BuildOptions{MaxDepth: 4, SplitFiles: 1})
	internal := base + "/internal"
	if g.Module(internal) == nil {
		t.Fatalf("expected split module %q, have %v", internal, moduleNames(g.Modules()))
	}
	want := []string{"extras/src/main/java", base, base + "/internal/bind"}
	if got := dependedBy(g, internal); !reflect.DeepEqual(got, want) {
		t.Errorf("internal depended_by = %v, want %v", got, want)
	}
}

func TestRubyRequireDependents(t *testing.T) {
	files := []*parser.FileInfo{
		fi("lib/rack.rb", "rack/utils", "rack/multipart"),
		fi("lib/rack/utils.rb", "set", "uri"),
		fi("lib/rack/request.rb", "multipart/parser", "utils", "json"),
		fi("lib/rack/multipart.rb"),
		fi("lib/rack/multipart/parser.rb", "../utils"),
		fi("spec/spec_helper.rb", "rack/request"),
	}
	g := Build(files, BuildOptions{MaxDepth: 4})
	if got, want := dependedBy(g, "lib/rack"), []string{"lib", "lib/rack/multipart", "spec"}; !reflect.DeepEqual(got, want) {
		t.Errorf("lib/rack depended_by = %v, want %v", got, want)
	}
	if got, want := dependedBy(g, "lib/rack/multipart"), []string{"lib/rack"}; !reflect.DeepEqual(got, want) {
		t.Errorf("multipart depended_by = %v, want %v", got, want)
	}
}

func TestSplitIsDeterministicAndKeepsSmallModules(t *testing.T) {
	var files []*parser.FileInfo
	for i := 0; i < 5; i++ {
		files = append(files, fi(fmt.Sprintf("a/b/c/d/e/f%d.go", i)), fi(fmt.Sprintf("a/b/c/d/g/h%d.go", i)))
	}
	files = append(files, fi("x/y/z.go"))
	a := assignModules(files, 4, 6)
	b := assignModules(files, 4, 6)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("assignModules not deterministic")
	}
	if a["a/b/c/d/e/f0.go"] != "a/b/c/d/e" || a["a/b/c/d/g/h0.go"] != "a/b/c/d/g" {
		t.Errorf("oversized module not split: %v", a)
	}
	if a["x/y/z.go"] != "x/y" {
		t.Errorf("small module changed: %v", a["x/y/z.go"])
	}
	off := assignModules(files, 4, 0)
	if off["a/b/c/d/e/f0.go"] != "a/b/c/d" {
		t.Errorf("splitting should be off at 0, got %q", off["a/b/c/d/e/f0.go"])
	}
}

func TestModuleRankingPrefersRichFiles(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "m/a.go", Exports: []string{"A"}},
		{Path: "m/z.go", Exports: []string{"Z1", "Z2", "Z3"}},
		{Path: "m/b.go"},
	}
	m := Build(files, BuildOptions{}).Module("m")
	if !reflect.DeepEqual(m.FilesByRank, []string{"z.go", "a.go", "b.go"}) {
		t.Errorf("FilesByRank = %v", m.FilesByRank)
	}
	if !reflect.DeepEqual(m.ExportsByRank, []string{"Z1", "Z2", "Z3", "A"}) {
		t.Errorf("ExportsByRank = %v", m.ExportsByRank)
	}
}

// Absolute imports are written from the Python import root (apps/api), which
// is a subdirectory of the scanned root, so `app.core.db` has to match the
// tail of apps/api/app/core/db.py.
func TestPythonAbsoluteImportsFromSubdirectoryRoot(t *testing.T) {
	files := []*parser.FileInfo{
		fi("apps/api/app/__init__.py"),
		fi("apps/api/app/main.py", "app.core", "app.domains.spend.router", "fastapi"),
		fi("apps/api/app/core/__init__.py"),
		fi("apps/api/app/core/db.py", "alembic", "sqlalchemy"),
		fi("apps/api/app/core/logging.py"),
		fi("apps/api/app/domains/spend/__init__.py"),
		fi("apps/api/app/domains/spend/router.py", "app.core.db", "app.domains.spend.service", "logging"),
		fi("apps/api/app/domains/spend/service.py", "app.core.db.Session", ".router"),
		fi("apps/api/alembic/env.py"),
	}
	g := Build(files, BuildOptions{MaxDepth: 6})

	core := "apps/api/app/core"
	spend := "apps/api/app/domains/spend"
	if got, want := dependedBy(g, core), []string{"apps/api/app", spend}; !reflect.DeepEqual(got, want) {
		t.Errorf("core depended_by = %v, want %v", got, want)
	}
	// main.py reaches spend through app.domains.spend.router (a file, not a package).
	if got, want := dependedBy(g, spend), []string{"apps/api/app"}; !reflect.DeepEqual(got, want) {
		t.Errorf("spend depended_by = %v, want %v", got, want)
	}
	// `logging` is the standard library and `alembic` is a third-party package, even though
	// the repo has a logging.py and an alembic/ directory without an __init__.py.
	if got := dependedBy(g, "apps/api/alembic"); len(got) != 0 {
		t.Errorf("alembic depended_by = %v, want none", got)
	}
	for _, m := range g.Module(core).DependsOn {
		t.Errorf("core should depend on nothing, got %s (stdlib logging or third-party alembic matched a repo path)", m)
	}
}

func TestPythonSingleSegmentImportNeedsAPackage(t *testing.T) {
	files := []*parser.FileInfo{
		fi("svc/app/__init__.py"),
		fi("svc/app/models.py"),
		fi("svc/worker/run.py", "app", "app.models"),
		fi("svc/util/helpers.py"),
		fi("svc/worker/jobs.py", "helpers"),
	}
	g := Build(files, BuildOptions{})
	if got, want := dependedBy(g, "svc/app"), []string{"svc/worker"}; !reflect.DeepEqual(got, want) {
		t.Errorf("app depended_by = %v, want %v", got, want)
	}
	// helpers is a plain file with no package around it, so a bare `import helpers` stays unresolved.
	if got := dependedBy(g, "svc/util"); len(got) != 0 {
		t.Errorf("util depended_by = %v, want none", got)
	}
}
