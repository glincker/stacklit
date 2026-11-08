package graph

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// "Who imports X" has one exact answer: the files whose import statements
// name X. A grep answers it in one call, so impact must too: list the importing
// files with the line of the import, nothing else, inside the token budget.

// Importer is one file that imports the target and the line of the import (0 if unknown).
type Importer struct {
	File string `json:"file"`
	Line int    `json:"line,omitempty"`
}

// ImpactFor answers an impact question for a symbol, file or package. A symbol
// name (exact ID or name) keeps the call-level blast radius. A file or
// directory lists its importers with file:line. calls forces the call-level
// view for every target. root locates sources for the line numbers; an empty
// root omits them.
func (g *SymGraph) ImpactFor(root, target string, depth, budget int, asJSON, calls bool) (string, error) {
	t := cleanTarget(target)
	if !calls && !g.isSymbolName(t) {
		if files, label, also := g.importTargets(t); len(files) > 0 {
			return g.importersView(root, t, label, also, files, budget, asJSON), nil
		}
	}
	return g.Impact(target, depth, budget, asJSON)
}

func cleanTarget(s string) string {
	s = strings.TrimSpace(filepath.ToSlash(s))
	s = strings.TrimPrefix(s, "./")
	return strings.TrimRight(s, "/")
}

// isSymbolName is true when q is exactly a symbol ID or name, so an exact name
// is answered as a symbol before any path guess.
func (g *SymGraph) isSymbolName(q string) bool {
	if i, ok := g.byID[q]; ok {
		return g.Nodes[i].Kind != KindFile
	}
	for _, i := range g.byName[q] {
		if g.Nodes[i].Kind != KindFile {
			return true
		}
	}
	return false
}

// importTargets resolves q to the file nodes it denotes: an exact file path, a
// directory (its direct files, or all below when it has none directly), or a
// path suffix of either. When several directories match, the shallowest wins
// and the others are returned as also.
func (g *SymGraph) importTargets(q string) (files map[int]bool, label string, also []string) {
	if q == "" {
		return nil, "", nil
	}
	var exact, suffix, stem []int
	dirs := map[string][]int{}
	below := map[string][]int{}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Kind != KindFile {
			continue
		}
		f := n.File
		switch {
		case f == q:
			exact = append(exact, i)
		case strings.HasSuffix(f, "/"+q):
			suffix = append(suffix, i)
		default:
			// A bare name without its extension, as written in an import.
			if st := strings.TrimSuffix(f, path.Ext(f)); st == q || strings.HasSuffix(st, "/"+q) {
				stem = append(stem, i)
			}
		}
		d := path.Dir(f)
		if d == q || strings.HasSuffix(d, "/"+q) {
			dirs[d] = append(dirs[d], i)
		}
		for p := d; p != "." && p != "/"; p = path.Dir(p) {
			if p != d && (p == q || strings.HasSuffix(p, "/"+q)) {
				below[p] = append(below[p], i)
			}
		}
	}
	pick := func(m map[string][]int) (map[int]bool, string, []string) {
		names := make([]string, 0, len(m))
		for d := range m {
			names = append(names, d)
		}
		sort.Slice(names, func(a, b int) bool {
			if len(names[a]) != len(names[b]) {
				return len(names[a]) < len(names[b])
			}
			return names[a] < names[b]
		})
		set := map[int]bool{}
		for _, i := range m[names[0]] {
			set[i] = true
		}
		return set, names[0], names[1:]
	}
	switch {
	case len(exact) > 0:
		return map[int]bool{exact[0]: true}, g.Nodes[exact[0]].File, nil
	case len(dirs) > 0:
		return pick(dirs)
	case len(suffix) > 0:
		return oneFile(g, suffix)
	case len(below) > 0:
		return pick(below)
	case len(stem) > 0:
		return oneFile(g, stem)
	}
	return nil, "", nil
}

// oneFile picks the first file by path and reports the others as also.
func oneFile(g *SymGraph, ids []int) (map[int]bool, string, []string) {
	sort.Slice(ids, func(a, b int) bool { return g.Nodes[ids[a]].File < g.Nodes[ids[b]].File })
	var also []string
	for _, i := range ids[1:] {
		also = append(also, g.Nodes[i].File)
	}
	return map[int]bool{ids[0]: true}, g.Nodes[ids[0]].File, also
}

// importerFiles returns the sorted files outside targets that import any of them.
func (g *SymGraph) importerFiles(targets map[int]bool) []string {
	inside := map[string]bool{}
	for i := range targets {
		inside[g.Nodes[i].File] = true
	}
	seen := map[string]bool{}
	for _, e := range g.Edges {
		if e.Kind != EdgeImports || !targets[e.To] {
			continue
		}
		f := g.Nodes[e.From].File
		if !inside[f] {
			seen[f] = true
		}
	}
	out := make([]string, 0, len(seen))
	for f := range seen {
		out = append(out, f)
	}
	sort.Strings(out)
	return out
}

// importSpec is the path fragment an importing file names, as a regexp that
// must be followed by a quote or a path separator. A Go import names the whole
// package path. Script imports are relative, so only the last segment is used
// and a wrong line number is the worst a collision can cause.
func importSpec(label string, isDir bool, ext string) *regexp.Regexp {
	frag := label
	switch {
	case ext == ".go":
		if !isDir {
			frag = path.Dir(label)
		}
	default:
		if !isDir {
			frag = strings.TrimSuffix(label, path.Ext(label))
			if b := path.Base(frag); b == "index" || b == "mod" || b == "__init__" {
				frag = path.Dir(label)
			}
		}
		frag = path.Base(frag)
	}
	return regexp.MustCompile(`["'/.]` + regexp.QuoteMeta(frag) + `(\.[A-Za-z]+)?(["'/]|$)`)
}

// importLine finds the first line of file that names the target in an import.
func importLine(root, file string, re *regexp.Regexp) int {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if len(line) > 400 {
			continue
		}
		if re.MatchString(line) && importWord.MatchString(line) {
			return n
		}
	}
	return 0
}

// importWord keeps comment and prose matches out: an import line has a keyword
// or is a bare quoted path (a Go import block entry).
var importWord = regexp.MustCompile(`\b(import|from|require|use|using|include|mod)\b|^\s*([A-Za-z_.]+\s+)?"[^"]+"\s*$`)

func (g *SymGraph) importersView(root, q, label string, also []string, targets map[int]bool, budget int, asJSON bool) string {
	if budget <= 0 {
		budget = DefaultImpactBudget
	}
	isDir, ext := false, ""
	for i := range targets {
		ext = path.Ext(g.Nodes[i].File)
		if g.Nodes[i].File != label {
			isDir = true
		}
	}
	files := g.importerFiles(targets)
	re := importSpec(label, isDir, ext)
	all := make([]Importer, len(files))
	known := map[string]bool{}
	for i, f := range files {
		all[i] = Importer{File: f}
		known[f] = true
		if root != "" {
			all[i].Line = importLine(root, f, re)
		}
	}
	var tfiles []string
	inside := map[string]bool{}
	for i := range targets {
		tfiles = append(tfiles, g.Nodes[i].File)
		inside[g.Nodes[i].File] = true
	}
	if extra := g.scanJSImporters(root, tfiles, known, inside); len(extra) > 0 {
		for f, ln := range extra {
			all = append(all, Importer{File: f, Line: ln})
		}
		sort.Slice(all, func(a, b int) bool { return all[a].File < all[b].File })
	}
	kind := "file"
	if isDir {
		kind = "package"
	}
	head := fmt.Sprintf("impact %s: %d importers of %s %s", q, len(all), kind, label)
	if len(all) == 1 {
		head = strings.Replace(head, "importers", "importer", 1)
	}
	if len(also) > 0 {
		if len(also) > 3 {
			also = also[:3]
		}
		head += "\nalso matches: " + strings.Join(also, ", ")
	}
	hint := fmt.Sprintf("stacklit impact %s --budget %d", q, budget*2)
	render := func(k int) string {
		if asJSON {
			data, _ := json.MarshalIndent(struct {
				Target    string     `json:"target"`
				Resolved  string     `json:"resolved"`
				Total     int        `json:"total"`
				Shown     int        `json:"shown"`
				Importers []Importer `json:"importers"`
			}{q, label, len(all), k, all[:k]}, "", " ")
			return string(data) + "\n"
		}
		var b strings.Builder
		b.WriteString(head + "\n")
		for _, im := range all[:k] {
			b.WriteString(im.File)
			if im.Line > 0 {
				fmt.Fprintf(&b, ":%d", im.Line)
			}
			b.WriteByte('\n')
		}
		if k < len(all) {
			fmt.Fprintf(&b, "[truncated: %d of %d omitted; raise it with: %s]\n", len(all)-k, len(all), hint)
		}
		return b.String()
	}
	lo, hi := 0, len(all)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if approxTokens(render(mid)) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return clampTokens(render(lo), budget)
}
