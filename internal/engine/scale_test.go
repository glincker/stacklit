package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// scaleBudget reads the wall-clock budget for the synthetic scale run.
func scaleBudget() time.Duration {
	if v, err := strconv.Atoi(os.Getenv("STACKLIT_SCALE_BUDGET_S")); err == nil && v > 0 {
		return time.Duration(v) * time.Second
	}
	return 90 * time.Second
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A 10k-file repo with generated, minified and shell files must finish inside
// the budget, and the awkward files must be reported, not silently dropped.
func TestScaleSyntheticRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("scale run skipped in -short")
	}
	root := t.TempDir()
	const pkgs, perPkg = 100, 100
	for p := 0; p < pkgs; p++ {
		for f := 0; f < perPkg; f++ {
			switch f % 4 {
			case 0:
				write(t, root, fmt.Sprintf("web/p%d/f%d.ts", p, f),
					fmt.Sprintf("import { a } from '../p%d/f0'\nexport function fn%d_%d() { return a }\n", (p+1)%pkgs, p, f))
			case 1:
				write(t, root, fmt.Sprintf("svc/p%d/f%d.go", p, f),
					fmt.Sprintf("package p%d\n\nfunc Fn%d() int { return %d }\n", p, f, f))
			case 2:
				write(t, root, fmt.Sprintf("web/p%d/f%d.json", p, f), fmt.Sprintf(`{"k%d":{"n":[1,2,3]}}`, f))
			default:
				write(t, root, fmt.Sprintf("scripts/p%d/f%d.sh", p, f), "#!/bin/bash\nsetup() { echo hi; }\nsource ./lib.sh\n")
			}
		}
	}
	write(t, root, "web/gen/routeTree.gen.ts", "// This file was automatically generated.\nexport const x = 1\n")
	write(t, root, "web/vendor.min.js", "var a="+strings.Repeat("1+", 5000)+"1;")

	t.Setenv("STACKLIT_NO_CACHE", "1")
	start := time.Now()
	res, err := Run(Options{Root: root, Quiet: true, NoWrite: true})
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > scaleBudget() {
		t.Fatalf("10k-file scan took %s, budget %s", d, scaleBudget())
	}
	if res.Index.Scan == nil || len(res.Index.Scan.Lightweight) < 2 {
		t.Fatalf("expected generated and minified files in scan.lightweight, got %+v", res.Index.Scan)
	}
}
