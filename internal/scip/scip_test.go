package scip

import (
	"testing"

	"github.com/glincker/stacklit/internal/graph"
)

func varint(v uint64) []byte {
	var out []byte
	for v >= 0x80 {
		out = append(out, byte(v)|0x80)
		v >>= 7
	}
	return append(out, byte(v))
}

func ld(num int, payload []byte) []byte {
	out := varint(uint64(num<<3 | 2))
	out = append(out, varint(uint64(len(payload)))...)
	return append(out, payload...)
}

func vi(num int, v uint64) []byte {
	return append(varint(uint64(num<<3)), varint(v)...)
}

func occ(line int, sym string, roles uint64) []byte {
	rng := append(varint(uint64(line)), 0, 3) // start line, start char, end char
	b := ld(1, rng)
	b = append(b, ld(2, []byte(sym))...)
	if roles != 0 {
		b = append(b, vi(3, roles)...)
	}
	return b
}

func doc(path string, occs ...[]byte) []byte {
	b := ld(1, []byte(path))
	for _, o := range occs {
		b = append(b, ld(2, o)...)
	}
	b = append(b, ld(4, []byte("go"))...) // unknown field must be skipped
	return b
}

func testIndex() []byte {
	meta := ld(1, vi(1, 1)) // metadata, ignored
	idx := meta
	idx = append(idx, ld(2, doc("a.go", occ(1, "pkg/Caller().", RoleDefinition), occ(2, "pkg/Callee().", 0), occ(2, "pkg/Other().", 0)))...)
	idx = append(idx, ld(2, doc("b.go", occ(4, "pkg/Callee().", RoleDefinition), occ(9, "pkg/Other().", RoleDefinition)))...)
	return idx
}

func TestParse(t *testing.T) {
	idx, err := Parse(testIndex())
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Documents) != 2 || idx.Documents[0].Path != "a.go" || len(idx.Documents[0].Occurrences) != 3 {
		t.Fatalf("%+v", idx)
	}
	if o := idx.Documents[1].Occurrences[0]; o.Line != 4 || o.Roles != RoleDefinition || o.Symbol != "pkg/Callee()." {
		t.Fatalf("%+v", o)
	}
}

func TestParseTruncated(t *testing.T) {
	b := testIndex()
	if _, err := Parse(b[:len(b)-3]); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpgrade(t *testing.T) {
	g := &graph.SymGraph{Version: "1", Nodes: []graph.SymNode{
		{ID: "a.go", Name: "a.go", Kind: graph.KindFile, File: "a.go"},
		{ID: "a.go::Caller", Name: "Caller", Kind: graph.KindFunc, File: "a.go", Line: 2, End: 4},
		{ID: "b.go", Name: "b.go", Kind: graph.KindFile, File: "b.go"},
		{ID: "b.go::Callee", Name: "Callee", Kind: graph.KindFunc, File: "b.go", Line: 5, End: 7},
		{ID: "b.go::Other", Name: "Other", Kind: graph.KindFunc, File: "b.go", Line: 10, End: 12},
		{ID: "b.go::Ghost", Name: "Ghost", Kind: graph.KindFunc, File: "b.go", Line: 20, End: 22},
	}, Edges: []graph.SymEdge{
		{From: 1, To: 3, Kind: graph.EdgeCalls, Conf: graph.ConfInferred, Score: 0.5},
		{From: 1, To: 4, Kind: graph.EdgeCalls, Conf: graph.ConfExtracted, Score: 1},
		{From: 1, To: 5, Kind: graph.EdgeCalls, Conf: graph.ConfInferred, Score: 0.4},
		{From: 0, To: 2, Kind: graph.EdgeImports, Conf: graph.ConfInferred, Score: 0.5},
	}}
	g.Index()
	idx, _ := Parse(testIndex())
	st := Upgrade(g, idx)
	if st.Upgraded != 2 || st.AlreadyExact != 1 || st.StillInferred != 1 {
		t.Fatalf("%+v", st)
	}
	if e := g.Edges[0]; e.Conf != graph.ConfExact || e.Score != 1 || e.Src != SrcTag {
		t.Fatalf("%+v", e)
	}
	if g.Edges[2].Conf != graph.ConfInferred {
		t.Fatal("unconfirmed edge must stay inferred")
	}
	if g.Edges[3].Conf != graph.ConfExact {
		t.Fatal("file edge should upgrade")
	}
}
