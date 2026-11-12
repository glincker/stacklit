package derive

import (
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/schema"
)

func testIndex() *schema.Index {
	return &schema.Index{
		Project: schema.Project{Name: "demo"},
		Tech:    schema.Tech{PrimaryLanguage: "go"},
		Structure: schema.Structure{
			TotalLines: 1200,
		},
		Modules: map[string]schema.ModuleInfo{
			"internal/auth/login":                             {Purpose: "Login"},
			"internal/auth/token":                             {Purpose: "Refresh tokens"},
			"internal/storage":                                {Purpose: "Storage"},
			"very/long/path/with/a/deeply/nested/module/name": {Purpose: "Deeply nested module"},
			"cmd": {Purpose: "CLI"},
		},
	}
}

func moduleLines(t *testing.T, out string) []string {
	t.Helper()
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if line == "modules:" {
			rest := lines[i+1:]
			var mods []string
			for _, l := range rest {
				if l == "" || (!strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t")) {
					break
				}
				mods = append(mods, l)
			}
			return mods
		}
	}
	t.Fatalf("no modules section in output:\n%s", out)
	return nil
}

func TestCompactMapFlatKeepsFullPaths(t *testing.T) {
	out := CompactMap(testIndex(), Options{})
	long := "very/long/path/with/a/deeply/nested/module/name"
	if !strings.Contains(out, long) {
		t.Errorf("flat map truncated the module path, want %q in:\n%s", long, out)
	}
	if strings.Contains(out, "  "+long[:30]+" ") {
		t.Errorf("flat map padded or truncated at 30 chars:\n%s", out)
	}
}

func TestCompactMapFlatDropsRedundantPurpose(t *testing.T) {
	out := CompactMap(testIndex(), Options{})
	lines := moduleLines(t, out)
	want := []string{
		"  cmd  cli",
		"  internal/auth/login",
		"  internal/auth/token  refresh tokens",
		"  internal/storage",
		"  very/long/path/with/a/deeply/nested/module/name  deeply nested module",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d module lines, want %d:\n%s", len(lines), len(want), out)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestCompactMapFlatAlignment(t *testing.T) {
	out := CompactMap(testIndex(), Options{})
	for _, line := range moduleLines(t, out) {
		if strings.Contains(line, "   ") && !strings.HasPrefix(line, "   ") {
			continue
		}
		if strings.HasPrefix(line, "   ") {
			t.Errorf("line starts with alignment padding, want two spaces: %q", line)
		}
	}
}

func TestCompactMapTreeGroupsByParent(t *testing.T) {
	out := CompactMap(testIndex(), Options{Layout: "tree"})
	lines := moduleLines(t, out)
	want := []string{
		"  cmd  cli",
		"  internal/auth/ login token{refresh tokens}",
		"  internal/ storage",
		"  very/long/path/with/a/deeply/nested/module/ name{deeply nested module}",
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d module lines, want %d:\n%s", len(lines), len(want), out)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestCompactMapUnknownLayoutFallsBackToFlat(t *testing.T) {
	got := CompactMap(testIndex(), Options{Layout: "bogus"})
	want := CompactMap(testIndex(), Options{})
	if got != want {
		t.Errorf("unknown layout changed output:\n got:\n%s\nwant:\n%s", got, want)
	}
}
