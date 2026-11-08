package graph

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestImpactSymbolCallSitesMatchSources(t *testing.T) {
	root, files := writeTree(t, map[string]string{
		"go.mod":   "module example.com/m\n\ngo 1.22\n",
		"lib/l.go": "package lib\n\nfunc Target() int { return 1 }\n",
		"a/a.go":   "package a\n\nimport \"example.com/m/lib\"\n\n// Target is mentioned in a comment only.\nfunc A() int {\n\tx := 1\n\treturn x + lib.Target()\n}\n",
		"b/b.go":   "package b\n\nimport \"example.com/m/lib\"\n\nfunc B() int {\n\ta := lib.Target()\n\tb := lib.Target()\n\treturn a + b\n}\n",
	})
	g := BuildSymbols(root, files, ExtractOptions{})
	out, err := g.ImpactFor(root, "Target", 0, 1500, false, false)
	if err != nil {
		t.Fatal(err)
	}
	ms := regexp.MustCompile(`at (\S+?):([\d,]+)`).FindAllStringSubmatch(out, -1)
	if len(ms) != 2 {
		t.Fatalf("want 2 call-site tags, got %d\n%s", len(ms), out)
	}
	for _, m := range ms {
		data, err := os.ReadFile(filepath.Join(root, m[1]))
		if err != nil {
			t.Fatal(err)
		}
		src := strings.Split(string(data), "\n")
		for _, l := range strings.Split(m[2], ",") {
			n, _ := strconv.Atoi(l)
			if !strings.Contains(src[n-1], "Target()") {
				t.Errorf("%s:%d does not call Target: %q", m[1], n, src[n-1])
			}
		}
	}
	if !strings.Contains(out, "at a/a.go:8") || !strings.Contains(out, "at b/b.go:6,7") {
		t.Errorf("unexpected sites\n%s", out)
	}
}
