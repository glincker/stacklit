package graph

import (
	"os"
	"path/filepath"
	"testing"
)

// The graph root must not depend on the checkout folder name, or a committed
// graph would differ between machines and between generate, refresh and watch.
func TestGraphRootIsStable(t *testing.T) {
	root := filepath.Join(t.TempDir(), "some-checkout-name")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\nfunc One() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := BuildSymbols(root, []string{"a.go"}, ExtractOptions{})
	if g.Root != "." {
		t.Fatalf("root = %q, want %q", g.Root, ".")
	}
}
