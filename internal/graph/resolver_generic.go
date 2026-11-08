package graph

import (
	"path"
	"sort"
	"strings"
)

// modLang groups languages that share header or module files.
func modLang(lang string) string {
	if lang == "cpp" {
		return "c"
	}
	return lang
}

// fileExts lists the extensions a path-style import may omit.
var fileExts = map[string][]string{
	"ruby": {".rb"}, "dart": {".dart"}, "zig": {".zig"}, "solidity": {".sol"}, "lua": {".lua"},
	"c": {""}, "cpp": {""}, "php": {".php"},
}

var pathExts = map[string]bool{".h": true, ".hpp": true, ".hh": true, ".hxx": true, ".c": true, ".cc": true, ".cpp": true, ".rb": true, ".dart": true, ".zig": true, ".sol": true, ".lua": true, ".php": true}

// buildSupers records each type's direct parents by name, for inherited-method lookup.
func (r *resolver) buildSupers() {
	r.supers = map[string][]string{}
	for _, f := range r.files {
		for _, ref := range f.Refs {
			if (ref.Kind != EdgeInherits && ref.Kind != EdgeImplements) || ref.From < 0 || ref.From >= len(f.Defs) {
				continue
			}
			d := f.Defs[ref.From]
			if !isTypeKind(d.Kind) {
				continue
			}
			r.supers[d.Name] = append(r.supers[d.Name], ref.Name)
		}
	}
}

// inheritedMethod finds name on an ancestor of typ, nearest ancestor first. Depth is
// bounded so cyclic or very deep hierarchies stay cheap.
func (r *resolver) inheritedMethod(typ, name string) []int {
	const maxDepth = 6
	seen := map[string]bool{typ: true}
	level := []string{typ}
	for depth := 0; depth < maxDepth && len(level) > 0; depth++ {
		var next []string
		for _, t := range level {
			for _, p := range r.supers[t] {
				if seen[p] {
					continue
				}
				seen[p] = true
				if hit := r.methodsOf[p+"\x00"+name]; len(hit) > 0 {
					return hit
				}
				next = append(next, p)
			}
		}
		level = next
	}
	return nil
}

// relFile resolves a path-style import against the importing file's directory.
func (r *resolver) relFile(lang, from, spec string) (int, bool) {
	if !strings.HasPrefix(spec, ".") && !strings.Contains(spec, "/") && !pathExts[path.Ext(spec)] {
		return 0, false
	}
	base := spec
	if strings.HasPrefix(spec, "package:") {
		if i := strings.IndexByte(spec, '/'); i > 0 {
			base = spec[i+1:]
		}
		if fi, ok := r.fileByPath["lib/"+base]; ok {
			return fi, true
		}
	} else if strings.HasPrefix(spec, ".") {
		base = path.Join(path.Dir(from), spec)
	} else if fi, ok := r.fileByPath[path.Join(path.Dir(from), spec)]; ok {
		return fi, true
	}
	exts := fileExts[lang]
	if len(exts) == 0 {
		exts = []string{""}
	}
	for _, e := range exts {
		if fi, ok := r.fileByPath[base+e]; ok {
			return fi, true
		}
	}
	// Include paths and package roots are unknown, so match on the path suffix.
	stem := strings.TrimSuffix(base, path.Ext(base))
	return r.lookupMod(lang, from, strings.ReplaceAll(strings.Trim(stem, "./"), "/", "."))
}

// siblingsOf returns files that share a directory and stem with fi (a header and its source).
func (r *resolver) siblingsOf(fi int) []int {
	p := r.files[fi].Path
	stem := strings.TrimSuffix(p, path.Ext(p))
	var out []int
	for _, e := range []string{".c", ".cc", ".cpp", ".cxx", ".h", ".hpp", ".hh", ".hxx"} {
		if t, ok := r.fileByPath[stem+e]; ok {
			out = append(out, t)
		}
	}
	sort.Ints(out)
	return out
}

// bindGeneric binds an import for the languages without a dedicated resolver:
// namespace or package imports, class imports, path imports and dotted module imports.
func (r *resolver) bindGeneric(fi int, f *FileSyms, imp Import, c *fileCtx) {
	spec := imp.Spec
	alias := imp.Alias
	external := func() {
		if alias != "" {
			c.external[alias] = true
		}
	}
	link := func(t int) { r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1) }

	// 1. namespace or package import, written "a.b.*" or just "a.b".
	if pkg := strings.TrimSuffix(spec, ".*"); r.pkgFiles[pkg] != nil {
		b := binding{files: r.pkgFiles[pkg], pkg: pkg}
		c.ns["*"+pkg] = b
		if alias != "" {
			c.ns[alias] = b
		}
		return
	}
	if strings.HasSuffix(spec, ".*") {
		return
	}
	// 2. a path to a file.
	if t, ok := r.relFile(f.Lang, f.Path, spec); ok {
		files := []int{t}
		if modLang(f.Lang) == "c" {
			files = r.siblingsOf(t)
			if len(files) == 0 {
				files = []int{t}
			}
		}
		for _, x := range files {
			link(x)
		}
		b := binding{files: files}
		c.ns["*"+spec] = b
		if alias != "" {
			c.ns[alias] = b
			c.named[alias] = named{b: b, orig: alias}
		}
		return
	}
	// 3. one type imported from a package: a.b.Name.
	if i := strings.LastIndexByte(spec, '.'); i > 0 {
		pkg, name := spec[:i], spec[i+1:]
		if hits := r.pkgDefs[pkg][name]; len(hits) > 0 {
			var files []int
			seen := map[int]bool{}
			for _, n := range hits {
				if fj := r.nodeFile[n]; !seen[fj] {
					seen[fj] = true
					files = append(files, fj)
					link(fj)
				}
			}
			b := binding{files: files}
			if alias == "" {
				alias = name
			}
			c.named[alias] = named{b: b, orig: name}
			c.ns[alias] = b
			return
		}
	}
	// 4. a dotted module path matched against file paths.
	if t, ok := r.lookupMod(f.Lang, f.Path, spec); ok {
		link(t)
		b := binding{files: []int{t}}
		if alias == "" {
			alias = lastSeg(spec)
		}
		c.ns[alias] = b
		c.named[alias] = named{b: b, orig: alias}
		c.ns["*"+spec] = b
		return
	}
	external()
}
