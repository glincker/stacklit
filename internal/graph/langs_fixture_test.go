package graph

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// fixtureCase lists the symbols and edges a mini project under testdata/graphlangs must yield.
// Edges read "kind from -> to (confidence)"; scores are covered by the resolver tests.
type fixtureCase struct {
	lang  string
	syms  []string
	edges []string
}

func fixtureGraph(t *testing.T, lang string, o ExtractOptions) *SymGraph {
	t.Helper()
	dir := filepath.Join("..", "..", "testdata", "graphlangs", lang)
	var files []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(dir, p)
			files = append(files, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return BuildSymbols(dir, files, o)
}

func graphLines(g *SymGraph) (syms, edges []string) {
	for _, n := range g.Nodes {
		if n.Kind != KindFile {
			syms = append(syms, n.Kind+" "+n.ID)
		}
	}
	for _, e := range g.Edges {
		edges = append(edges, fmt.Sprintf("%s %s -> %s (%s)", e.Kind, g.Nodes[e.From].ID, g.Nodes[e.To].ID, e.Conf))
	}
	sort.Strings(syms)
	sort.Strings(edges)
	return
}

func diffLines(t *testing.T, what string, got, want []string) {
	t.Helper()
	g, w := map[string]bool{}, map[string]bool{}
	for _, x := range got {
		g[x] = true
	}
	for _, x := range want {
		w[x] = true
		if !g[x] {
			t.Errorf("%s: missing %q", what, x)
		}
	}
	for _, x := range got {
		if !w[x] {
			t.Errorf("%s: unexpected %q", what, x)
		}
	}
}

func TestLanguageFixtures(t *testing.T) {
	cases := []fixtureCase{
		{lang: "python",
			syms: []string{
				"class pkg/app.py::App",
				"class pkg/base.py::Base",
				"func pkg/base.py::helper",
				"method pkg/app.py::App.run",
				"method pkg/base.py::Base.greet",
			},
			edges: []string{
				"calls pkg/app.py::App.run -> pkg/base.py::Base.greet (EXTRACTED)",
				"calls pkg/app.py::App.run -> pkg/base.py::helper (EXTRACTED)",
				"imports pkg/app.py -> pkg/base.py (EXTRACTED)",
				"inherits pkg/app.py::App -> pkg/base.py::Base (EXTRACTED)",
			}},
		{lang: "typescript",
			syms: []string{
				"class src/app.ts::App",
				"class src/base.ts::Base",
				"func src/base.ts::helper",
				"interface src/app.ts::Runner",
				"method src/app.ts::App.constructor",
				"method src/app.ts::App.run",
				"method src/app.ts::Runner.run",
				"method src/base.ts::Base.greet",
			},
			edges: []string{
				"calls src/app.ts::App.run -> src/base.ts::Base.greet (EXTRACTED)",
				"calls src/app.ts::App.run -> src/base.ts::helper (EXTRACTED)",
				"implements src/app.ts::App -> src/app.ts::Runner (EXTRACTED)",
				"imports src/app.ts -> src/base.ts (EXTRACTED)",
				"inherits src/app.ts::App -> src/base.ts::Base (EXTRACTED)",
			}},
		{lang: "javascript",
			syms: []string{
				"class src/app.js::App",
				"class src/base.js::Base",
				"func src/app.js::make",
				"func src/base.js::helper",
				"method src/app.js::App.run",
				"method src/base.js::Base.greet",
			},
			edges: []string{
				"calls src/app.js::App.run -> src/base.js::Base.greet (EXTRACTED)",
				"calls src/app.js::App.run -> src/base.js::helper (EXTRACTED)",
				"calls src/app.js::make -> src/app.js::App (EXTRACTED)",
				"imports src/app.js -> src/base.js (EXTRACTED)",
				"inherits src/app.js::App -> src/base.js::Base (EXTRACTED)",
			}},
		{lang: "rust",
			syms: []string{
				"func src/base.rs::helper",
				"method src/base.rs::Base.greet",
				"method src/lib.rs::App.run",
				"type src/base.rs::Base",
				"type src/lib.rs::App",
			},
			edges: []string{
				"calls src/lib.rs::App.run -> src/base.rs::Base.greet (EXTRACTED)",
				"calls src/lib.rs::App.run -> src/base.rs::helper (EXTRACTED)",
				"imports src/lib.rs -> src/base.rs (EXTRACTED)",
			}},
		{lang: "java",
			syms: []string{
				"class app/App.java::App",
				"class com/x/Base.java::Base",
				"method app/App.java::App.run",
				"method com/x/Base.java::Base.greet",
				"method com/x/Base.java::Base.helper",
			},
			edges: []string{
				"calls app/App.java::App.run -> com/x/Base.java::Base.greet (EXTRACTED)",
				"calls app/App.java::App.run -> com/x/Base.java::Base.helper (EXTRACTED)",
				"imports app/App.java -> com/x/Base.java (EXTRACTED)",
				"inherits app/App.java::App -> com/x/Base.java::Base (EXTRACTED)",
			}},
		{lang: "csharp",
			syms: []string{
				"class App/App.cs::AppMain",
				"class Lib/Base.cs::Base",
				"method App/App.cs::AppMain.Run",
				"method Lib/Base.cs::Base.Greet",
				"method Lib/Base.cs::Base.Helper",
			},
			edges: []string{
				"calls App/App.cs::AppMain.Run -> Lib/Base.cs::Base.Greet (EXTRACTED)",
				"calls App/App.cs::AppMain.Run -> Lib/Base.cs::Base.Helper (EXTRACTED)",
				"imports App/App.cs -> Lib/Base.cs (EXTRACTED)",
				"inherits App/App.cs::AppMain -> Lib/Base.cs::Base (EXTRACTED)",
			}},
		{lang: "kotlin",
			syms: []string{
				"class app/App.kt::App",
				"class lib/Base.kt::Base",
				"func lib/Base.kt::helper",
				"method app/App.kt::App.run",
				"method lib/Base.kt::Base.greet",
			},
			edges: []string{
				"calls app/App.kt::App.run -> lib/Base.kt::Base.greet (EXTRACTED)",
				"calls app/App.kt::App.run -> lib/Base.kt::helper (EXTRACTED)",
				"imports app/App.kt -> lib/Base.kt (EXTRACTED)",
				"inherits app/App.kt::App -> lib/Base.kt::Base (EXTRACTED)",
			}},
		{lang: "swift",
			syms: []string{
				"class Sources/App.swift::App",
				"class Sources/Base.swift::Base",
				"func Sources/Base.swift::helper",
				"method Sources/App.swift::App.run",
				"method Sources/Base.swift::Base.greet",
			},
			edges: []string{
				"calls Sources/App.swift::App.run -> Sources/Base.swift::Base.greet (EXTRACTED)",
				"calls Sources/App.swift::App.run -> Sources/Base.swift::helper (INFERRED)",
				"imports Sources/App.swift -> Sources/Base.swift (EXTRACTED)",
				"inherits Sources/App.swift::App -> Sources/Base.swift::Base (INFERRED)",
			}},
		{lang: "php",
			syms: []string{
				"class src/App.php::Main",
				"class src/Lib/Base.php::Base",
				"func src/Lib/Base.php::helper",
				"method src/App.php::Main.run",
				"method src/Lib/Base.php::Base.greet",
			},
			edges: []string{
				"calls src/App.php::Main.run -> src/Lib/Base.php::Base (EXTRACTED)",
				"calls src/App.php::Main.run -> src/Lib/Base.php::Base.greet (EXTRACTED)",
				"imports src/App.php -> src/Lib/Base.php (EXTRACTED)",
				"inherits src/App.php::Main -> src/Lib/Base.php::Base (EXTRACTED)",
			}},
		{lang: "ruby",
			syms: []string{
				"class lib/app.rb::App",
				"class lib/base.rb::Base",
				"func lib/base.rb::helper",
				"method lib/app.rb::App.run",
				"method lib/base.rb::Base.greet",
			},
			edges: []string{
				"calls lib/app.rb::App.run -> lib/base.rb::Base (EXTRACTED)",
				"calls lib/app.rb::App.run -> lib/base.rb::Base.greet (EXTRACTED)",
				"calls lib/app.rb::App.run -> lib/base.rb::helper (EXTRACTED)",
				"imports lib/app.rb -> lib/base.rb (EXTRACTED)",
				"inherits lib/app.rb::App -> lib/base.rb::Base (EXTRACTED)",
			}},
		{lang: "c",
			syms: []string{
				"func base.c::helper",
				"func base.h::helper",
				"func main.c::run",
				"type base.h::Base",
			},
			edges: []string{
				"calls main.c::run -> base.c::helper (EXTRACTED)",
				"imports base.c -> base.h (EXTRACTED)",
				"imports main.c -> base.c (EXTRACTED)",
				"imports main.c -> base.h (EXTRACTED)",
			}},
		{lang: "cpp",
			syms: []string{
				"class app.cpp::App",
				"class base.hpp::Base",
				"method app.cpp::App.run",
				"method base.hpp::Base.greet",
			},
			edges: []string{
				"calls app.cpp::App.run -> base.hpp::Base.greet (EXTRACTED)",
				"imports app.cpp -> base.hpp (EXTRACTED)",
				"inherits app.cpp::App -> base.hpp::Base (EXTRACTED)",
			}},
		{lang: "scala",
			syms: []string{
				"class app/App.scala::App",
				"class lib/Base.scala::Base",
				"class lib/Base.scala::Util",
				"method app/App.scala::App.run",
				"method lib/Base.scala::Base.greet",
				"method lib/Base.scala::Util.helper",
			},
			edges: []string{
				"calls app/App.scala::App.run -> lib/Base.scala::Base.greet (EXTRACTED)",
				"calls app/App.scala::App.run -> lib/Base.scala::Util.helper (EXTRACTED)",
				"imports app/App.scala -> lib/Base.scala (EXTRACTED)",
				"inherits app/App.scala::App -> lib/Base.scala::Base (EXTRACTED)",
			}},
		{lang: "dart",
			syms: []string{
				"class lib/app.dart::App",
				"class lib/base.dart::Base",
				"func lib/base.dart::helper",
				"method lib/app.dart::App.<init>",
				"method lib/app.dart::App.run",
				"method lib/base.dart::Base.greet",
			},
			edges: []string{
				"calls lib/app.dart::App.run -> lib/base.dart::Base.greet (EXTRACTED)",
				"calls lib/app.dart::App.run -> lib/base.dart::helper (EXTRACTED)",
				"imports lib/app.dart -> lib/base.dart (EXTRACTED)",
				"inherits lib/app.dart::App -> lib/base.dart::Base (EXTRACTED)",
			}},
		{lang: "elixir",
			syms: []string{
				"class lib/app/base.ex::Base",
				"class lib/app/main.ex::Main",
				"method lib/app/base.ex::Base.greet",
				"method lib/app/base.ex::Base.helper",
				"method lib/app/main.ex::Main.local",
				"method lib/app/main.ex::Main.run",
			},
			edges: []string{
				"calls lib/app/main.ex::Main.run -> lib/app/base.ex::Base.helper (EXTRACTED)",
				"calls lib/app/main.ex::Main.run -> lib/app/main.ex::Main.local (EXTRACTED)",
				"imports lib/app/main.ex -> lib/app/base.ex (EXTRACTED)",
			}},
		{lang: "lua",
			syms: []string{
				"method app.lua::App.run",
				"method base.lua::Base.greet",
				"method base.lua::Base.helper",
			},
			edges: []string{
				"calls app.lua::App.run -> base.lua::Base.greet (EXTRACTED)",
				"calls app.lua::App.run -> base.lua::Base.helper (EXTRACTED)",
				"imports app.lua -> base.lua (EXTRACTED)",
			}},
		{lang: "zig",
			syms: []string{
				"func app.zig::run",
				"func base.zig::helper",
				"method base.zig::Base.greet",
				"type base.zig::Base",
			},
			edges: []string{
				"calls app.zig::run -> base.zig::Base.greet (INFERRED)",
				"calls app.zig::run -> base.zig::helper (EXTRACTED)",
				"imports app.zig -> base.zig (EXTRACTED)",
			}},
		{lang: "haskell",
			syms: []string{
				"func src/App.hs::local",
				"func src/App.hs::run",
				"func src/Lib/Base.hs::helper",
				"type src/Lib/Base.hs::Shape",
			},
			edges: []string{
				"calls src/App.hs::run -> src/App.hs::local (EXTRACTED)",
				"calls src/App.hs::run -> src/Lib/Base.hs::helper (EXTRACTED)",
				"imports src/App.hs -> src/Lib/Base.hs (EXTRACTED)",
			}},
		{lang: "ocaml",
			syms: []string{
				"func app.ml::local",
				"func app.ml::run",
				"func base.ml::helper",
				"type base.ml::shape",
			},
			edges: []string{
				"calls app.ml::run -> app.ml::local (EXTRACTED)",
				"calls app.ml::run -> base.ml::helper (EXTRACTED)",
				"imports app.ml -> base.ml (EXTRACTED)",
			}},
		{lang: "solidity",
			syms: []string{
				"class App.sol::App",
				"class Base.sol::Base",
				"interface Base.sol::IRunner",
				"method App.sol::App.local",
				"method App.sol::App.run",
				"method Base.sol::Base.greet",
				"method Base.sol::IRunner.run",
			},
			edges: []string{
				"calls App.sol::App.run -> App.sol::App.local (EXTRACTED)",
				"calls App.sol::App.run -> Base.sol::Base.greet (EXTRACTED)",
				"implements App.sol::App -> Base.sol::IRunner (EXTRACTED)",
				"imports App.sol -> Base.sol (EXTRACTED)",
				"inherits App.sol::App -> Base.sol::Base (EXTRACTED)",
			}},
	}
	for _, tc := range cases {
		t.Run(tc.lang, func(t *testing.T) {
			g := fixtureGraph(t, tc.lang, ExtractOptions{})
			syms, edges := graphLines(g)
			diffLines(t, "symbols", syms, tc.syms)
			diffLines(t, "edges", edges, tc.edges)
		})
	}
}
