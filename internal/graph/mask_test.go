package graph

import (
	"strings"
	"testing"
	"time"
)

func TestLuaBlockCommentsDoNotBreakParsing(t *testing.T) {
	src := "--[[\n  Greets people.\n]]\nlocal function hello(a)\n  --[==[ inner\n  ]==]\n  return a\nend\n\nfunction other() end\n"
	fs := ExtractFile("a.lua", []byte(src))
	if fs == nil || fs.syntaxErrors || len(fs.Defs) != 2 {
		t.Fatalf("want 2 defs without syntax errors, got %+v", fs)
	}
	if fs.Defs[0].Name != "hello" || fs.Defs[0].Doc != "Greets people." {
		t.Fatalf("got %+v", fs.Defs[0])
	}
	if len(luaMaskBlockComments([]byte(src))) != len(src) {
		t.Fatal("masking must keep byte offsets")
	}
}

func TestDirectiveMaskKeepsFirstBranch(t *testing.T) {
	src := "#if A\nclass X { void One() {} }\n#else\nclass X { void Two() {} }\n#endif\n#region r\nclass Y {}\n#endregion\n"
	out := string(csharpMaskDirectives([]byte(src)))
	if len(out) != len(src) {
		t.Fatal("masking must keep byte offsets")
	}
	if !strings.Contains(out, "One") || strings.Contains(out, "Two") || strings.Contains(out, "#") {
		t.Fatalf("unexpected mask result %q", out)
	}
	fs := ExtractFile("a.cs", []byte(src))
	if fs == nil || len(fs.Defs) != 3 {
		t.Fatalf("want X, X.One and Y with nothing from the dropped branch, got %v", defNames(fs))
	}
}

func TestGuardedExtractGivesUpOnStall(t *testing.T) {
	spec := &tsSpec{lang: "lua", grammar: "lua", mask: func(src []byte) []byte { time.Sleep(300 * time.Millisecond); return src }, comments: set("comment")}
	if fs := extractGuarded(spec, "lua", "a.lua", []byte("x = 1\n"), 20*time.Millisecond); fs != nil {
		t.Fatalf("a stalled parse must yield nil so callers fall back, got %+v", fs)
	}
}
