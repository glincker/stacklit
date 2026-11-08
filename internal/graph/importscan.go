package graph

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// The import edges come from an extractor that can miss an import in a long
// file (a measured case: a file importing a module that also imports it back).
// For script languages an import is a plain string, so impact checks the
// sources too and takes the union, which keeps "who imports X" as exact as a
// grep. STACKLIT_IMPACT_SCAN=0 turns the check off; STACKLIT_IMPACT_SCAN_MAX
// caps the files read (default 20000).
const defaultImpactScanMax = 20000

var scanExts = map[string]bool{
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".mts": true, ".cts": true, ".vue": true, ".svelte": true, ".astro": true,
}

// jsSpecRe finds a module specifier in an import, export-from, dynamic import or require.
var jsSpecRe = regexp.MustCompile(`(?:\bfrom|\bimport|\brequire)\s*\(?\s*['"]([^'"]+)['"]`)

func impactScanMax() int {
	if v := strings.TrimSpace(os.Getenv("STACKLIT_IMPACT_SCAN")); v == "0" || strings.EqualFold(v, "false") {
		return 0
	}
	return envNonNeg("STACKLIT_IMPACT_SCAN_MAX", defaultImpactScanMax)
}

func stripJSExt(p string) string {
	if e := path.Ext(p); scanExts[e] {
		return strings.TrimSuffix(p, e)
	}
	return p
}

// jsAccept lists the specifier paths (no extension) that denote the targets.
func jsAccept(files []string) map[string]bool {
	acc := map[string]bool{}
	for _, f := range files {
		stem := stripJSExt(f)
		acc[stem] = true
		if b := path.Base(stem); b == "index" {
			acc[path.Dir(stem)] = true
		}
	}
	return acc
}

// specDenotes reports whether spec, written in importer, names one of accept.
func specDenotes(importer, spec string, accept map[string]bool) bool {
	spec = stripJSExt(strings.TrimSuffix(strings.TrimSuffix(spec, ".js"), ".jsx"))
	if strings.HasPrefix(spec, ".") {
		return accept[path.Join(path.Dir(importer), spec)]
	}
	// Alias or package-relative path: accept a multi-segment suffix match only.
	for _, pre := range []string{"@/", "~/", "#/"} {
		spec = strings.TrimPrefix(spec, pre)
	}
	if !strings.Contains(spec, "/") {
		return false
	}
	for a := range accept {
		if a == spec || strings.HasSuffix(a, "/"+spec) {
			return true
		}
	}
	return false
}

// scanJSImporters reads script files for imports of the targets that the edges
// missed. known holds the importers already found; inside the target files.
func (g *SymGraph) scanJSImporters(root string, targetFiles []string, known, inside map[string]bool) map[string]int {
	max := impactScanMax()
	isJS := false
	for _, f := range targetFiles {
		if scanExts[path.Ext(f)] {
			isJS = true
		}
	}
	if max == 0 || root == "" || !isJS {
		return nil
	}
	accept := jsAccept(targetFiles)
	found := map[string]int{}
	read := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind != KindFile || !scanExts[path.Ext(n.File)] || known[n.File] || inside[n.File] {
			continue
		}
		if read++; read > max {
			break
		}
		if ln := firstJSImport(root, n.File, accept); ln > 0 {
			found[n.File] = ln
		}
	}
	return found
}

func firstJSImport(root, file string, accept map[string]bool) int {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if len(line) > 400 || !strings.ContainsAny(line, `'"`) {
			continue
		}
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") || strings.HasPrefix(t, "/*") {
			continue
		}
		for _, m := range jsSpecRe.FindAllStringSubmatch(line, -1) {
			if specDenotes(file, m[1], accept) {
				return n
			}
		}
	}
	return 0
}
