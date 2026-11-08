package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/config"
)

// benchRepo builds a synthetic repo with n Go files spread over packages.
func benchRepo(b *testing.B, n int) string {
	b.Helper()
	root := b.TempDir()
	for i := 0; i < n; i++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%02d", i%40))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			b.Fatal(err)
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "package pkg%02d\n\nimport \"fmt\"\n\n", i%40)
		for j := 0; j < 20; j++ {
			fmt.Fprintf(&sb, "// F%d_%d does work.\nfunc F%d_%d(x int) int { fmt.Println(x); return x + %d }\n\n", i, j, i, j, j)
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.go", i)), []byte(sb.String()), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	return root
}

func BenchmarkScanRepo(b *testing.B) {
	for _, tc := range []struct {
		name    string
		cache   bool
		workers int
	}{
		{"cold-1worker", false, 1},
		{"cold-parallel", false, 0},
		{"warm-cache", true, 0},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.Setenv("STACKLIT_CACHE_DIR", b.TempDir())
			root := benchRepo(b, 1500)
			cfg := config.DefaultConfig()
			cfg.Workers = tc.workers
			on := tc.cache
			cfg.Cache = &on
			if tc.cache {
				if _, err := scanRepo(root, cfg, true); err != nil {
					b.Fatal(err)
				}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := scanRepo(root, cfg, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
