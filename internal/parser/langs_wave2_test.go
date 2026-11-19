package parser

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestConfigKeyCapFromEnv(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&b, "key%d: v\n", i)
	}
	t.Setenv(envConfigKeyCap, "3")
	info, err := (&TreeSitterParser{}).Parse("a.yaml", []byte(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Exports) != 3 {
		t.Errorf("got %d keys, want 3: %v", len(info.Exports), info.Exports)
	}
}

func TestConfigSizeLimitSkipsKeys(t *testing.T) {
	t.Setenv(envConfigMaxBytes, "10")
	info, err := (&TreeSitterParser{}).Parse("a.json", []byte(`{"alpha": 1, "beta": 2}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Exports) != 0 || info.LineCount != 1 {
		t.Errorf("oversized config should keep only line count, got %+v", info)
	}
}

func TestAmbiguousExtensionsAndBuildFiles(t *testing.T) {
	p := &TreeSitterParser{}
	for path, lang := range map[string]string{
		"src/App.fs":       "fsharp",
		"x/Dockerfile.dev": "dockerfile",
		"x/api.dockerfile": "dockerfile",
		"GNUmakefile":      "make",
		"rules.mk":         "make",
		"a/CMakeLists.txt": "cmake",
		"cmake/Find.cmake": "cmake",
	} {
		e := detectEntry(path)
		if e == nil || e.Name != lang || !p.CanParse(path) {
			t.Errorf("%s: want %s, got %v", path, lang, e)
		}
	}
}

func TestUnknownExtensionKeepsLineCount(t *testing.T) {
	for _, name := range []string{"notes.zzz", "LICENSE", "data.weird.ext"} {
		info, err := ParseFile(writeTemp(t, name, "a\nb\nc\n"))
		if err != nil {
			t.Fatal(err)
		}
		if info.LineCount != 3 {
			t.Errorf("%s: LineCount = %d, want 3", name, info.LineCount)
		}
	}
}

func TestMatlabStyleDotMFallsBack(t *testing.T) {
	// A .m file that is not Objective-C must still parse without panicking and count lines.
	info, err := (&TreeSitterParser{}).Parse("calc.m", []byte("function y = f(x)\n  y = x + 1;\nend\n"))
	if err != nil {
		t.Fatal(err)
	}
	if info.LineCount != 3 {
		t.Errorf("LineCount = %d", info.LineCount)
	}
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := t.TempDir() + "/" + name
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestGenericDetectsBuildFilesByName(t *testing.T) {
	for path, want := range map[string]string{"Makefile": "make", "sub/Dockerfile": "dockerfile", "CMakeLists.txt": "cmake", "Dockerfile.prod": "dockerfile", "x.fs": "fsharp"} {
		if got := detectLanguage(path, nil); got != want {
			t.Errorf("detectLanguage(%q) = %q, want %q", path, got, want)
		}
	}
}
