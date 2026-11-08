package graph

import (
	"fmt"
	"strings"
	"testing"
)

func buildFrom(t *testing.T, srcs map[string]string, mods map[string]string) *SymGraph {
	t.Helper()
	var fs []*FileSyms
	paths := make([]string, 0, len(srcs))
	for p := range srcs {
		paths = append(paths, p)
	}
	sortStrings(paths)
	for _, p := range paths {
		f := ExtractFile(p, []byte(srcs[p]))
		if f == nil {
			t.Fatalf("extract %s failed", p)
		}
		fs = append(fs, f)
	}
	return Resolve(fs, ResolveOptions{GoModules: mods})
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// edgeOf finds an edge by endpoint names and kind.
func edgeOf(g *SymGraph, from, to, kind string) (SymEdge, bool) {
	for _, e := range g.Edges {
		if e.Kind == kind && strings.HasSuffix(g.Nodes[e.From].ID, from) && strings.HasSuffix(g.Nodes[e.To].ID, to) {
			return e, true
		}
	}
	return SymEdge{}, false
}

func TestResolveEdges(t *testing.T) {
	cases := []struct {
		name string
		srcs map[string]string
		mods map[string]string
		want []struct{ from, to, kind, conf string }
	}{
		{"go cross package", map[string]string{
			"cmd/main.go":   "package main\n\nimport \"example.com/app/lib\"\n\nfunc main() { lib.Do() }\n",
			"lib/lib.go":    "package lib\n\nfunc Do() { inner() }\n",
			"lib/helper.go": "package lib\n\nfunc inner() {}\n",
		}, map[string]string{"example.com/app": "."}, []struct{ from, to, kind, conf string }{
			{"main.go::main", "lib.go::Do", "calls", ConfExtracted},
			{"lib.go::Do", "helper.go::inner", "calls", ConfExtracted},
			{"cmd/main.go", "lib/lib.go", "imports", ConfExtracted},
		}},
		{"go structural interface", map[string]string{
			"a/a.go": "package a\n\ntype Speaker interface{ Speak() string }\n\ntype Dog struct{}\n\nfunc (Dog) Speak() string { return \"w\" }\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"a.go::Dog", "a.go::Speaker", "implements", ConfInferred},
		}},
		{"ts relative import", map[string]string{
			"src/a.ts": "import { b } from './b';\nimport def from './c';\nexport function a() { b(); def(); }\n",
			"src/b.ts": "export function b() {}\n",
			"src/c.ts": "export default function c() {}\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"a.ts::a", "b.ts::b", "calls", ConfExtracted},
			{"a.ts::a", "c.ts::c", "calls", ConfInferred},
			{"src/a.ts", "src/b.ts", "imports", ConfExtracted},
		}},
		{"ts class inheritance across files", map[string]string{
			"x.ts":    "import { Base } from './base';\nexport class Kid extends Base {\n  go() { this.run(); }\n  run() {}\n}\n",
			"base.ts": "export class Base {}\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"x.ts::Kid", "base.ts::Base", "inherits", ConfExtracted},
			{"Kid.go", "Kid.run", "calls", ConfExtracted},
		}},
		{"python module import", map[string]string{
			"app/main.py":  "from app import util\nfrom app.tools import tool\n\ndef run():\n    util.helper()\n    tool()\n",
			"app/util.py":  "def helper():\n    pass\n",
			"app/tools.py": "def tool():\n    pass\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"main.py::run", "util.py::helper", "calls", ConfExtracted},
			{"main.py::run", "tools.py::tool", "calls", ConfExtracted},
		}},
		{"rust use", map[string]string{
			"src/main.rs": "use crate::util::helper;\nfn main() { helper(); }\n",
			"src/util.rs": "pub fn helper() {}\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"main.rs::main", "util.rs::helper", "calls", ConfExtracted},
		}},
		{"java import", map[string]string{
			"p/A.java": "package p;\nimport q.B;\npublic class A {\n  void go() { B.help(); }\n}\n",
			"q/B.java": "package q;\npublic class B {\n  static void help() {}\n}\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"A.go", "B.help", "calls", ConfExtracted},
		}},
		{"go typed local receiver", map[string]string{
			"a/a.go": "package a\n\ntype T struct{}\n\nfunc (T) Run() {}\n\ntype U struct{}\n\nfunc (U) Run() {}\n\nfunc f() { t := &T{}; t.Run() }\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"a.go::f", "T.Run", "calls", ConfExtracted},
		}},
		{"unknown receiver is inferred", map[string]string{
			"a.py": "def f(x):\n    x.frobnicate()\n",
			"b.py": "class T:\n    def frobnicate(self):\n        pass\n",
		}, nil, []struct{ from, to, kind, conf string }{
			{"a.py::f", "T.frobnicate", "calls", ConfInferred},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := buildFrom(t, tc.srcs, tc.mods)
			for _, w := range tc.want {
				e, ok := edgeOf(g, w.from, w.to, w.kind)
				if !ok {
					t.Errorf("missing %s -%s-> %s; edges: %s", w.from, w.kind, w.to, dumpEdges(g))
					continue
				}
				if e.Conf != w.conf {
					t.Errorf("%s -> %s conf = %s, want %s", w.from, w.to, e.Conf, w.conf)
				}
				if e.Score <= 0 || e.Score > 1 {
					t.Errorf("score out of range: %v", e.Score)
				}
			}
		})
	}
}

func dumpEdges(g *SymGraph) string {
	var b strings.Builder
	for _, e := range g.Edges {
		fmt.Fprintf(&b, "[%s -%s-> %s %s]", g.Nodes[e.From].ID, e.Kind, g.Nodes[e.To].ID, tag(e))
	}
	return b.String()
}

func TestExternalCallsDropped(t *testing.T) {
	g := buildFrom(t, map[string]string{
		"a.ts": "import fs from 'fs';\nexport function a() { fs.readFileSync('x'); JSON.stringify({}); }\n",
		"b.ts": "export class S { readFileSync() {} stringify() {} }\n",
	}, nil)
	for _, e := range g.Edges {
		if e.Kind == EdgeCalls {
			t.Fatalf("unexpected call edge: %s", dumpEdges(g))
		}
	}
}
