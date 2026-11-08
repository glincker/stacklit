package graph

import (
	"testing"

	"github.com/glincker/stacklit/internal/parser"
)

// assertStringSliceContains fails the test if want is not present in got.
func assertStringSliceContains(t *testing.T, got []string, want string) {
	t.Helper()
	for _, v := range got {
		if v == want {
			return
		}
	}
	t.Errorf("expected slice to contain %q, got %v", want, got)
}

// TestDetectModules verifies that 5 files across 3 directories produce 3 modules
// and that the auth module depends on db.
func TestDetectModules(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "src/auth/login.ts", Language: "TypeScript", Imports: []string{"src/db"}, LineCount: 40},
		{Path: "src/auth/register.ts", Language: "TypeScript", Imports: []string{"src/db"}, LineCount: 30},
		{Path: "src/db/client.ts", Language: "TypeScript", Exports: []string{"Client"}, LineCount: 60},
		{Path: "src/db/models.ts", Language: "TypeScript", Exports: []string{"User"}, LineCount: 50},
		{Path: "src/api/routes.ts", Language: "TypeScript", Imports: []string{"src/auth", "src/db"}, LineCount: 80},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	mods := g.Modules()
	if len(mods) != 3 {
		t.Fatalf("expected 3 modules, got %d: %v", len(mods), moduleNames(mods))
	}

	authMod := g.Module("src/auth")
	if authMod == nil {
		t.Fatal("expected module src/auth to exist")
	}
	assertStringSliceContains(t, authMod.DependsOn, "src/db")
}

// TestModuleExports verifies that exports from multiple files in the same module
// are aggregated on the module.
func TestModuleExports(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "src/utils/format.ts", Language: "TypeScript", Exports: []string{"formatDate", "formatCurrency"}, LineCount: 25},
		{Path: "src/utils/validate.ts", Language: "TypeScript", Exports: []string{"validateEmail", "validatePhone"}, LineCount: 35},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	utilsMod := g.Module("src/utils")
	if utilsMod == nil {
		t.Fatal("expected module src/utils to exist")
	}
	if utilsMod.FileCount != 2 {
		t.Errorf("expected FileCount=2, got %d", utilsMod.FileCount)
	}

	assertStringSliceContains(t, utilsMod.Exports, "formatDate")
	assertStringSliceContains(t, utilsMod.Exports, "formatCurrency")
	assertStringSliceContains(t, utilsMod.Exports, "validateEmail")
	assertStringSliceContains(t, utilsMod.Exports, "validatePhone")

	expectedLines := 60
	if utilsMod.LineCount != expectedLines {
		t.Errorf("expected LineCount=%d, got %d", expectedLines, utilsMod.LineCount)
	}
}

// TestDependencyEdges verifies that 3 modules with known imports produce the
// correct edges and MostDepended ordering.
func TestDependencyEdges(t *testing.T) {
	// Layout:
	//   api   → auth, db
	//   auth  → db
	//   db    (no deps)
	files := []*parser.FileInfo{
		{Path: "src/api/index.ts", Language: "TypeScript", Imports: []string{"src/auth", "src/db"}, LineCount: 50},
		{Path: "src/auth/service.ts", Language: "TypeScript", Imports: []string{"src/db"}, LineCount: 40},
		{Path: "src/db/pool.ts", Language: "TypeScript", Exports: []string{"Pool"}, LineCount: 30},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	edges := g.Edges()
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d: %v", len(edges), edges)
	}

	// Verify MostDepended: db should be first (depended on by api and auth).
	mostDepended := g.MostDepended()
	if len(mostDepended) == 0 {
		t.Fatal("MostDepended returned empty slice")
	}
	if mostDepended[0] != "src/db" {
		t.Errorf("expected src/db to be most depended, got %q", mostDepended[0])
	}

	// db module should be depended on by both api and auth.
	dbMod := g.Module("src/db")
	if dbMod == nil {
		t.Fatal("expected module src/db to exist")
	}
	if len(dbMod.DependedBy) != 2 {
		t.Errorf("expected db.DependedBy length=2, got %d: %v", len(dbMod.DependedBy), dbMod.DependedBy)
	}
	assertStringSliceContains(t, dbMod.DependedBy, "src/api")
	assertStringSliceContains(t, dbMod.DependedBy, "src/auth")
}

// TestEntrypoints verifies that files flagged IsEntrypoint are tracked.
func TestEntrypoints(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "src/main.ts", Language: "TypeScript", IsEntrypoint: true, LineCount: 10},
		{Path: "src/auth/login.ts", Language: "TypeScript", IsEntrypoint: false, LineCount: 20},
		{Path: "src/api/index.ts", Language: "TypeScript", IsEntrypoint: true, LineCount: 15},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	eps := g.Entrypoints()
	if len(eps) != 2 {
		t.Fatalf("expected 2 entrypoints, got %d: %v", len(eps), eps)
	}
	assertStringSliceContains(t, eps, "src/main.ts")
	assertStringSliceContains(t, eps, "src/api/index.ts")
}

// TestIsolatedModules verifies that modules with no edges are correctly identified.
func TestIsolatedModules(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "src/api/handler.ts", Imports: []string{"../db/client"}, LineCount: 30},
		{Path: "src/db/client.ts", LineCount: 120},
		{Path: "src/scripts/migrate.ts", LineCount: 50}, // no imports, nobody imports it
	}
	g := Build(files, BuildOptions{MaxDepth: 4})
	isolated := g.Isolated()
	if len(isolated) != 1 || isolated[0] != "src/scripts" {
		t.Errorf("expected [src/scripts] as isolated, got %v", isolated)
	}
}

// TestPythonRelativeImports verifies that Python dot-notation relative imports
// produce correct dependency edges.
func TestPythonRelativeImports(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "myapp/api/views.py", Language: "python", Imports: []string{"flask", ".models", ".config"}, LineCount: 30},
		{Path: "myapp/api/models.py", Language: "python", Exports: []string{"User", "Post"}, LineCount: 20},
		{Path: "myapp/api/config.py", Language: "python", Exports: []string{"Settings"}, LineCount: 10},
		{Path: "myapp/db/client.py", Language: "python", Exports: []string{"get_conn"}, LineCount: 15},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	apiMod := g.Module("myapp/api")
	if apiMod == nil {
		t.Fatal("expected module myapp/api to exist")
	}

	// ".models" and ".config" are in the same directory so they resolve to same module.
	// They should NOT create self-edges. External "flask" should not appear.
	if len(apiMod.DependsOn) != 0 {
		t.Errorf("expected no external deps (self-refs filtered), got %v", apiMod.DependsOn)
	}
}

// TestPythonParentRelativeImport verifies that ".." style Python imports resolve
// to the parent module.
func TestPythonParentRelativeImport(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "myapp/api/views.py", Language: "python", Imports: []string{"..db"}, LineCount: 30},
		{Path: "myapp/db/client.py", Language: "python", Exports: []string{"get_conn"}, LineCount: 15},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	apiMod := g.Module("myapp/api")
	if apiMod == nil {
		t.Fatal("expected module myapp/api to exist")
	}
	assertStringSliceContains(t, apiMod.DependsOn, "myapp/db")
}

// TestPythonDottedAbsoluteImport verifies that Python dotted absolute imports
// (e.g. "myapp.db.client") resolve to the correct module.
func TestPythonDottedAbsoluteImport(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "myapp/api/views.py", Language: "python", Imports: []string{"myapp.db.client"}, LineCount: 30},
		{Path: "myapp/db/client.py", Language: "python", Exports: []string{"get_conn"}, LineCount: 15},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	apiMod := g.Module("myapp/api")
	if apiMod == nil {
		t.Fatal("expected module myapp/api to exist")
	}
	assertStringSliceContains(t, apiMod.DependsOn, "myapp/db")
}

// TestGoImportWithDotsNotMangled verifies that Go-style imports containing dots
// (e.g. "github.com/user/repo") are not incorrectly slash-converted.
func TestGoImportWithDotsNotMangled(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "cmd/main.go", Language: "go", Imports: []string{"github.com/lib/pq"}, LineCount: 20},
		{Path: "internal/db/conn.go", Language: "go", Exports: []string{"Connect"}, LineCount: 30},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	cmdMod := g.Module("cmd")
	if cmdMod == nil {
		t.Fatal("expected module cmd to exist")
	}
	// "github.com/lib/pq" is external — should not resolve to internal/db or any module.
	if len(cmdMod.DependsOn) != 0 {
		t.Errorf("expected no deps for external Go import, got %v", cmdMod.DependsOn)
	}
}

// TestPythonRelativeImportShorthand verifies that "from . import module" style
// imports resolve to dependency edges.
func TestPythonRelativeImportShorthand(t *testing.T) {
	files := []*parser.FileInfo{
		{Path: "pkg/api/views.py", Language: "python", Imports: []string{".models", "..shared"}, LineCount: 20},
		{Path: "pkg/api/models/user.py", Language: "python", LineCount: 10},
		{Path: "pkg/shared/util.py", Language: "python", LineCount: 10},
	}

	g := Build(files, BuildOptions{MaxDepth: 4})

	apiMod := g.Module("pkg/api")
	if apiMod == nil {
		t.Fatal("expected module pkg/api to exist")
	}
	assertStringSliceContains(t, apiMod.DependsOn, "pkg/api/models")
	assertStringSliceContains(t, apiMod.DependsOn, "pkg/shared")
}

// moduleNames is a helper to extract names for error messages.
func moduleNames(mods []*Module) []string {
	names := make([]string, len(mods))
	for i, m := range mods {
		names[i] = m.Name
	}
	return names
}

// TestNormalizePythonImportLeavesPathStyleAlone guards JS-style relative paths
// with extensions from being rewritten as dotted Python modules.
func TestNormalizePythonImportLeavesPathStyleAlone(t *testing.T) {
	for _, imp := range []string{"./utils.js", "../lib/x.ts", "./a/b"} {
		if got, ok := normalizePythonImport(imp); ok {
			t.Errorf("normalizePythonImport(%q) = %q, want unchanged", imp, got)
		}
	}
	if got, ok := normalizePythonImport("..db.client"); !ok || got != ".././db/client" {
		t.Errorf("normalizePythonImport(..db.client) = %q, %v", got, ok)
	}
}
