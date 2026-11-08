package parser

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestParseConcurrentDeterministic parses the same sources from many goroutines
// and requires identical results. Rust sources with error-recovery paths used
// to hit a pooled tree-sitter token source after it was released, which made
// parses panic or return empty results at random.
func TestParseConcurrentDeterministic(t *testing.T) {
	dir := os.Getenv("STACKLIT_STRESS_DIR")
	rounds := 1
	if dir == "" {
		dir = "../../testdata/rust-concurrency"
		rounds = 4
	}
	var files []string
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && filepath.Ext(p) == ".rs" {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no rust files")
	}
	signature := func(info *FileInfo) [3]int {
		return [3]int{len(info.Exports), len(info.Imports), info.LineCount}
	}
	want := map[string][3]int{}
	for _, f := range files {
		b, _ := os.ReadFile(f)
		info, err := ParseContent(f, b)
		if err != nil {
			t.Fatalf("serial %s: %v", f, err)
		}
		want[f] = signature(info)
	}
	var wg sync.WaitGroup
	for w := 0; w < 16; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds*len(files); i++ {
				f := files[i%len(files)]
				b, _ := os.ReadFile(f)
				info, err := ParseContent(f, b)
				if err != nil {
					t.Errorf("%s: %v", f, err)
					continue
				}
				if got := signature(info); got != want[f] {
					t.Errorf("%s: nondeterministic %v vs %v", f, got, want[f])
				}
			}
		}()
	}
	wg.Wait()
}
