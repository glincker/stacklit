package graph

import (
	"sort"
	"testing"
)

func defNames(fs *FileSyms) []string {
	var out []string
	for _, d := range fs.Defs {
		out = append(out, d.Kind+":"+d.Qual)
	}
	sort.Strings(out)
	return out
}

func refSet(fs *FileSyms) map[string]bool {
	m := map[string]bool{}
	for _, r := range fs.Refs {
		q := r.Name
		if r.Qual != "" {
			q = r.Qual + "." + r.Name
		}
		m[r.Kind+":"+q] = true
	}
	return m
}

func TestExtractFile(t *testing.T) {
	cases := []struct {
		name, path, src string
		defs, refs      []string
	}{
		{"go", "a/a.go", `package a

import "fmt"

// Greeter says hello.
type Greeter interface{ Greet() string }

type Base struct{}

type Impl struct{ Base }

func (i *Impl) Greet() string { return helper(i.name()) }

func (i *Impl) name() string { fmt.Println("x"); return "n" }

func helper(s string) string { return s }
`, []string{"func:helper", "interface:Greeter", "method:Impl.Greet", "method:Impl.name", "type:Base", "type:Impl"},
			[]string{"calls:helper", "calls:this.name", "calls:fmt.Println", "inherits:Base"}},
		{"ts", "a.ts", "import { dep } from './dep';\nexport interface Shape { area(): number }\nexport class Circle extends Base implements Shape {\n  area(): number { return dep(this.r) + this.helper(); }\n  helper() { return 1; }\n}\nexport const make = (r: number) => new Circle(r);\n",
			[]string{"class:Circle", "func:make", "interface:Shape", "method:Circle.area", "method:Circle.helper"},
			[]string{"inherits:Base", "implements:Shape", "calls:dep", "calls:this.helper", "calls:Circle"}},
		{"python", "m.py", "from .util import tool\n\nclass Dog(Animal):\n    \"\"\"A dog.\"\"\"\n    def bark(self):\n        return tool(self.name())\n\n    def name(self):\n        return 'x'\n\ndef main():\n    Dog().bark()\n",
			[]string{"class:Dog", "func:main", "method:Dog.bark", "method:Dog.name"},
			[]string{"inherits:Animal", "calls:tool", "calls:this.name", "calls:Dog"}},
		{"rust", "src/lib.rs", "use crate::util::{helper, Thing as T};\n\npub trait Speak: Named {\n    fn speak(&self);\n}\n\npub struct Dog;\n\nimpl Speak for Dog {\n    fn speak(&self) { helper(); self.bark(); }\n}\nimpl Dog {\n    fn bark(&self) {}\n}\n",
			[]string{"method:Dog.bark", "method:Dog.speak", "method:Speak.speak", "type:Dog", "trait:Speak"},
			[]string{"inherits:Named", "implements:Speak", "calls:helper", "calls:this.bark"}},
		{"java", "p/Dog.java", "package p;\nimport q.Animal;\npublic class Dog extends Animal implements Runnable {\n  public Dog() { super(); }\n  public void run() {\n    this.bark();\n    Util.help();\n  }\n  void bark() {}\n}\n",
			[]string{"class:Dog", "method:Dog.<init>", "method:Dog.bark", "method:Dog.run"},
			[]string{"inherits:Animal", "implements:Runnable", "calls:this.bark", "calls:Util.help"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := ExtractFile(tc.path, []byte(tc.src))
			if fs == nil {
				t.Fatal("nil result")
			}
			got := map[string]bool{}
			for _, n := range defNames(fs) {
				got[n] = true
			}
			for _, w := range tc.defs {
				if !got[w] {
					t.Errorf("missing def %s in %v", w, defNames(fs))
				}
			}
			rs := refSet(fs)
			for _, w := range tc.refs {
				w2 := w
				if w2 == "calls:this.bark" || w2 == "calls:this.name" || w2 == "calls:this.helper" {
					if !rs[w2] && !rs["calls:self."+w2[len("calls:this."):]] {
						t.Errorf("missing ref %s in %v", w, rs)
					}
					continue
				}
				if !rs[w2] {
					t.Errorf("missing ref %s in %v", w, rs)
				}
			}
		})
	}
}

func TestDefRangesAndDocs(t *testing.T) {
	src := "package a\n\n// Add sums two ints. Extra words.\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	fs := ExtractFile("a.go", []byte(src))
	d := fs.Defs[0]
	if d.Line != 4 || d.End != 6 || d.Doc != "Add sums two ints." {
		t.Fatalf("got line=%d end=%d doc=%q", d.Line, d.End, d.Doc)
	}
	py := ExtractFile("a.py", []byte("def f(x):\n    \"\"\"Do f. More.\"\"\"\n    return x\n\nz = 1\n"))
	if py.Defs[0].End != 3 || py.Defs[0].Doc != "Do f." {
		t.Fatalf("python got end=%d doc=%q", py.Defs[0].End, py.Defs[0].Doc)
	}
	js := ExtractFile("a.ts", []byte("/** Make it. */\nexport function make() {\n  return 1;\n}\n"))
	if js.Defs[0].End != 4 || js.Defs[0].Doc != "Make it." {
		t.Fatalf("ts got end=%d doc=%q", js.Defs[0].End, js.Defs[0].Doc)
	}
}
