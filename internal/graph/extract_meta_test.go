package graph

import (
	"strings"
	"testing"
)

type metaWant struct {
	qual, kind, doc, vis, sig string
	line, end                 int
}

// TestDefMetadata checks line span, signature, doc sentence and visibility per language.
func TestDefMetadata(t *testing.T) {
	cases := []struct {
		name, path, src string
		want            []metaWant
	}{
		{"python", "m.py", "class Dog:\n    \"\"\"A dog. More text.\"\"\"\n\n    def _hide(self):\n        return 1\n\ndef run(x: int) -> int:\n    return x\n", []metaWant{
			{"Dog", KindClass, "A dog.", "public", "class Dog:", 1, 5},
			{"Dog._hide", KindMethod, "", "private", "def _hide(self):", 4, 5},
			{"run", KindFunc, "", "public", "def run(x: int) -> int:", 7, 8},
		}},
		{"typescript", "a.ts", "/** Make it. Extra. */\nexport function make(a: number): number {\n  return a;\n}\nfunction hidden() {}\nexport class C {\n  private x = 1;\n  #y = 2;\n  go() {}\n}\n", []metaWant{
			{"make", KindFunc, "Make it.", "public", "function make(a: number): number", 2, 4},
			{"hidden", KindFunc, "", "private", "function hidden()", 5, 5},
			{"C.go", KindMethod, "", "public", "go()", 9, 9},
		}},
		{"rust", "src/lib.rs", "/// Adds.\npub fn add() {}\nfn sub() {}\npub(crate) struct S;\n", []metaWant{
			{"add", KindFunc, "Adds.", "public", "pub fn add()", 2, 2},
			{"sub", KindFunc, "", "private", "fn sub()", 3, 3},
			{"S", KindType, "", "crate", "pub(crate) struct S;", 4, 4},
		}},
		{"java", "A.java", "package p;\n\n/** Svc. */\npublic class A {\n    /** Run it. */\n    private void run() {}\n    void pkg() {}\n}\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "public class A", 4, 8},
			{"A.run", KindMethod, "Run it.", "private", "private void run()", 6, 6},
			{"A.pkg", KindMethod, "", "package", "void pkg()", 7, 7},
		}},
		{"csharp", "A.cs", "namespace N {\n  /// <summary>Svc.</summary>\n  public class A {\n    void M() {}\n    public void P() {}\n  }\n}\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "public class A", 3, 6},
			{"A.M", KindMethod, "", "private", "void M()", 4, 4},
			{"A.P", KindMethod, "", "public", "public void P()", 5, 5},
		}},
		{"kotlin", "A.kt", "/** Svc. */\nclass A {\n    private fun m() = 1\n    fun p() {}\n}\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "class A", 2, 5},
			{"A.m", KindMethod, "", "private", "private fun m() = 1", 3, 3},
		}},
		{"swift", "A.swift", "/// Svc.\npublic class A {\n    private func m() {}\n    func p() {}\n}\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "public class A", 2, 5},
			{"A.m", KindMethod, "", "private", "private func m()", 3, 3},
			{"A.p", KindMethod, "", "internal", "func p()", 4, 4},
		}},
		{"php", "a.php", "<?php\n/** Svc. */\nclass A {\n    private function m() {}\n    public function p() {}\n}\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "class A", 3, 6},
			{"A.m", KindMethod, "", "private", "private function m()", 4, 4},
		}},
		{"ruby", "a.rb", "# Svc.\nclass A\n  def p; end\n  private\n  def m; end\nend\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "class A", 2, 6},
			{"A.p", KindMethod, "", "public", "def p; end", 3, 3},
			{"A.m", KindMethod, "", "private", "def m; end", 5, 5},
		}},
		{"scala", "A.scala", "/** Svc. */\nclass A {\n  private def m = 1\n  def p = 2\n}\n", []metaWant{
			{"A", KindClass, "Svc.", "public", "class A", 2, 5},
			{"A.m", KindMethod, "", "private", "private def m", 3, 3},
		}},
		{"c", "a.c", "/* Adds. */\nint add(int a) { return a; }\nstatic int sub(int a) { return a; }\n", []metaWant{
			{"add", KindFunc, "Adds.", "public", "int add(int a)", 2, 2},
			{"sub", KindFunc, "", "private", "static int sub(int a)", 3, 3},
		}},
		{"elixir", "a.ex", "defmodule M do\n  def pub(x), do: x\n  defp priv(x), do: x\nend\n", []metaWant{
			{"M", KindClass, "", "public", "defmodule M do", 1, 4},
			{"M.pub", KindMethod, "", "public", "def pub(x), do: x", 2, 2},
			{"M.priv", KindMethod, "", "private", "defp priv(x), do: x", 3, 3},
		}},
		{"lua", "a.lua", "-- Adds.\nfunction add(a) return a end\nlocal function sub(a) return a end\n", []metaWant{
			{"add", KindFunc, "Adds.", "public", "function add(a)", 2, 2},
			{"sub", KindFunc, "", "private", "local function sub(a)", 3, 3},
		}},
		{"zig", "a.zig", "/// Adds.\npub fn add() void {}\nfn sub() void {}\n", []metaWant{
			{"add", KindFunc, "Adds.", "public", "fn add() void", 2, 2},
			{"sub", KindFunc, "", "private", "fn sub() void", 3, 3},
		}},
		{"solidity", "a.sol", "/// Token.\ncontract T {\n    function a() public {}\n    function b() internal {}\n}\n", []metaWant{
			{"T", KindClass, "Token.", "internal", "contract T", 2, 5},
			{"T.a", KindMethod, "", "public", "function a() public", 3, 3},
			{"T.b", KindMethod, "", "internal", "function b() internal", 4, 4},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := ExtractFile(tc.path, []byte(tc.src))
			if fs == nil {
				t.Fatal("nil result")
			}
			by := map[string]Def{}
			for _, d := range fs.Defs {
				by[d.Qual] = d
			}
			for _, w := range tc.want {
				d, ok := by[w.qual]
				if !ok {
					t.Errorf("missing def %s in %v", w.qual, defNames(fs))
					continue
				}
				if d.Kind != w.kind || d.Doc != w.doc || d.Vis != w.vis || d.Line != w.line || d.End != w.end || !strings.HasPrefix(d.Sig, w.sig) {
					t.Errorf("%s: got kind=%s doc=%q vis=%s lines=%d-%d sig=%q, want kind=%s doc=%q vis=%s lines=%d-%d sig=%q",
						w.qual, d.Kind, d.Doc, d.Vis, d.Line, d.End, d.Sig, w.kind, w.doc, w.vis, w.line, w.end, w.sig)
				}
			}
		})
	}
}
