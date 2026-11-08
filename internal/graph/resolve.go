package graph

import (
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveOptions tunes cross-file resolution.
type ResolveOptions struct {
	// GoModules maps a Go module path to its root-relative directory.
	GoModules map[string]string
	// MaxCandidates caps how many same-name targets an ambiguous reference may fan out to.
	MaxCandidates int
}

type binding struct {
	files []int
	pkg   string // set when the target is a whole package (Go dir, Java wildcard)
}

type named struct {
	b    binding
	orig string // original exported name, "" when the local name is the default export
}

type fileCtx struct {
	ns       map[string]binding
	named    map[string]named
	external map[string]bool
}

type symEdgeKey struct {
	from, to int32
	kind     uint8
}

type resolver struct {
	files    []*FileSyms
	opts     ResolveOptions
	g        *SymGraph
	fileNode []int
	defNode  [][]int

	fileByPath map[string]int
	defsIn     []map[string][]int // per file: name -> def nodes
	pkgDefs    map[string]map[string][]int
	pkgFiles   map[string][]int
	byName     map[string][]int // top-level funcs and all types
	methods    map[string][]int // method name -> nodes
	methodsOf  map[string][]int // Recv\x00name -> nodes
	typeNames  map[string]bool
	nodeDef    map[int]Def
	nodeFile   []int // node -> file index

	mods    map[string][]int // dotted suffix -> files, for python/rust/java
	goDirs  map[string][]int
	edgeIdx map[symEdgeKey]int
	ctx     []fileCtx

	goPending [][2]string // file index, imported dir; linked after refs if unused
}

var kindCode = map[string]uint8{EdgeCalls: 1, EdgeImports: 2, EdgeInherits: 3, EdgeImplements: 4}

// ubiquitous names are too generic to link by name alone without a receiver type.
var ubiquitous = wordSet("get set add remove delete has put push pop run start stop close open read write find map filter reduce each format parse init new len size count copy clear update create build apply call handle emit on off")

// Resolve turns per-file extraction results into a linked symbol graph. Every
// lookup goes through hash maps built once, so cost grows linearly with refs.
func Resolve(files []*FileSyms, opts ResolveOptions) *SymGraph {
	if opts.MaxCandidates == 0 {
		opts.MaxCandidates = 4
	}
	r := &resolver{
		files: files, opts: opts, g: &SymGraph{Version: SchemaVersion},
		fileByPath: map[string]int{}, pkgDefs: map[string]map[string][]int{}, pkgFiles: map[string][]int{},
		byName: map[string][]int{}, methods: map[string][]int{}, methodsOf: map[string][]int{},
		typeNames: map[string]bool{}, nodeDef: map[int]Def{}, mods: map[string][]int{}, goDirs: map[string][]int{},
		edgeIdx: map[symEdgeKey]int{},
	}
	r.buildNodes()
	r.buildModIndex()
	r.buildContexts()
	r.resolveRefs()
	r.linkUnusedGoImports()
	r.goImplements()
	r.finish()
	return r.g
}

func (r *resolver) addNode(n SymNode, file int) int {
	idx := len(r.g.Nodes)
	r.g.Nodes = append(r.g.Nodes, n)
	r.nodeFile = append(r.nodeFile, file)
	return idx
}

func isTypeKind(k string) bool {
	return k == KindType || k == KindClass || k == KindInterface || k == KindEnum || k == KindTrait
}

func (r *resolver) buildNodes() {
	seen := map[string]bool{}
	r.defsIn = make([]map[string][]int, len(r.files))
	r.defNode = make([][]int, len(r.files))
	r.fileNode = make([]int, len(r.files))
	for fi, f := range r.files {
		r.fileByPath[f.Path] = fi
		r.fileNode[fi] = r.addNode(SymNode{ID: f.Path, Name: path.Base(f.Path), Kind: KindFile, File: f.Path, Lang: f.Lang}, fi)
		if f.Lang == "go" {
			r.goDirs[f.Pkg] = append(r.goDirs[f.Pkg], fi)
		}
		if f.Pkg != "" {
			r.pkgFiles[f.Pkg] = append(r.pkgFiles[f.Pkg], fi)
		}
		r.defsIn[fi] = map[string][]int{}
		r.defNode[fi] = make([]int, len(f.Defs))
		for di, d := range f.Defs {
			id := f.Path + "::" + d.Qual
			if seen[id] {
				id = id + "#" + itoa(d.Line)
			}
			seen[id] = true
			ni := r.addNode(SymNode{ID: id, Name: d.Name, Kind: d.Kind, File: f.Path, Line: d.Line, End: d.End, Doc: d.Doc, Lang: f.Lang, Sig: d.Sig}, fi)
			r.defNode[fi][di] = ni
			r.nodeDef[ni] = d
			r.defsIn[fi][d.Name] = append(r.defsIn[fi][d.Name], ni)
			if d.Kind == KindMethod {
				r.methods[d.Name] = append(r.methods[d.Name], ni)
				r.methodsOf[d.Recv+"\x00"+d.Name] = append(r.methodsOf[d.Recv+"\x00"+d.Name], ni)
				continue
			}
			if isTypeKind(d.Kind) {
				r.typeNames[d.Name] = true
			}
			r.byName[d.Name] = append(r.byName[d.Name], ni)
			if f.Pkg != "" {
				m := r.pkgDefs[f.Pkg]
				if m == nil {
					m = map[string][]int{}
					r.pkgDefs[f.Pkg] = m
				}
				m[d.Name] = append(m[d.Name], ni)
			}
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// buildModIndex registers every dotted path suffix of each non-Go file, so
// "a.b.C" finds a/b/C.java and "pkg.mod" finds src/pkg/mod.py in one lookup.
func (r *resolver) buildModIndex() {
	for fi, f := range r.files {
		if f.Lang == "go" || f.Lang == "typescript" || f.Lang == "javascript" {
			continue
		}
		p := strings.TrimSuffix(f.Path, path.Ext(f.Path))
		parts := strings.Split(p, "/")
		switch parts[len(parts)-1] {
		case "__init__", "mod", "lib", "main":
			if len(parts) > 1 {
				parts = parts[:len(parts)-1]
			}
		}
		for k := 0; k < len(parts); k++ {
			key := strings.ToLower(f.Lang[:1]) + ":" + strings.Join(parts[k:], ".")
			r.mods[key] = append(r.mods[key], fi)
		}
	}
}

// closest picks the candidate sharing the longest directory prefix with from.
func (r *resolver) closest(from string, cands []int) int {
	best, bestScore := cands[0], -1
	for _, c := range cands {
		a, b := strings.Split(path.Dir(from), "/"), strings.Split(path.Dir(r.files[c].Path), "/")
		n := 0
		for n < len(a) && n < len(b) && a[n] == b[n] {
			n++
		}
		if n > bestScore || (n == bestScore && r.files[c].Path < r.files[best].Path) {
			best, bestScore = c, n
		}
	}
	return best
}

func (r *resolver) lookupMod(lang, from, dotted string) (int, bool) {
	if dotted == "" {
		return 0, false
	}
	c := r.mods[strings.ToLower(lang[:1])+":"+dotted]
	if len(c) == 0 {
		return 0, false
	}
	return r.closest(from, c), true
}

var jsExts = []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts"}

func (r *resolver) resolveJS(from, spec string) (int, bool) {
	var base string
	switch {
	case strings.HasPrefix(spec, "."):
		base = path.Join(path.Dir(from), spec)
	case strings.HasPrefix(spec, "@/") || strings.HasPrefix(spec, "~/"):
		for _, pre := range []string{"src/", ""} {
			if fi, ok := r.tryJS(pre + spec[2:]); ok {
				return fi, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
	return r.tryJS(base)
}

func (r *resolver) tryJS(base string) (int, bool) {
	if fi, ok := r.fileByPath[base]; ok {
		return fi, true
	}
	// ESM sources import "./x.js" while the file on disk is x.ts.
	stem := strings.TrimSuffix(base, path.Ext(base))
	for _, cand := range []string{base, stem} {
		for _, e := range jsExts {
			if fi, ok := r.fileByPath[cand+e]; ok {
				return fi, true
			}
			if fi, ok := r.fileByPath[cand+"/index"+e]; ok {
				return fi, true
			}
		}
	}
	return 0, false
}

func (r *resolver) resolveGoDir(spec string) (string, bool) {
	best, bestDir := "", ""
	for mod, dir := range r.opts.GoModules {
		if (spec == mod || strings.HasPrefix(spec, mod+"/")) && len(mod) > len(best) {
			best, bestDir = mod, dir
		}
	}
	if best == "" {
		return "", false
	}
	d := path.Join(bestDir, strings.TrimPrefix(strings.TrimPrefix(spec, best), "/"))
	if _, ok := r.goDirs[d]; ok {
		return d, true
	}
	return "", false
}

// rustModule maps a use path to a file, dropping a trailing item name when needed.
func (r *resolver) rustModule(from, spec string) (fi int, item string, ok bool) {
	segs := strings.Split(spec, "::")
	for len(segs) > 0 && (segs[0] == "crate" || segs[0] == "self" || segs[0] == "super") {
		segs = segs[1:]
	}
	if len(segs) == 0 {
		return 0, "", false
	}
	if fi, ok := r.lookupMod("rust", from, strings.Join(segs, ".")); ok {
		return fi, "", true
	}
	if len(segs) > 1 {
		if fi, ok := r.lookupMod("rust", from, strings.Join(segs[:len(segs)-1], ".")); ok {
			return fi, segs[len(segs)-1], true
		}
	}
	return 0, "", false
}

func (r *resolver) pyModule(from, spec string) (int, bool) {
	if strings.HasPrefix(spec, ".") {
		n := 0
		for n < len(spec) && spec[n] == '.' {
			n++
		}
		dir := path.Dir(from)
		for i := 1; i < n; i++ {
			dir = path.Dir(dir)
		}
		rest := strings.ReplaceAll(spec[n:], ".", "/")
		for _, cand := range []string{path.Join(dir, rest) + ".py", path.Join(dir, rest, "__init__.py")} {
			if rest == "" {
				cand = path.Join(dir, "__init__.py")
			}
			if fi, ok := r.fileByPath[cand]; ok {
				return fi, true
			}
		}
		return 0, false
	}
	return r.lookupMod("python", from, spec)
}

func (r *resolver) buildContexts() {
	r.ctx = make([]fileCtx, len(r.files))
	for fi, f := range r.files {
		c := fileCtx{ns: map[string]binding{}, named: map[string]named{}, external: map[string]bool{}}
		for _, imp := range f.Imports {
			r.bindImport(fi, f, imp, &c)
		}
		r.ctx[fi] = c
	}
}

func (r *resolver) bindImport(fi int, f *FileSyms, imp Import, c *fileCtx) {
	markExternal := func() {
		if imp.Alias != "" {
			c.external[imp.Alias] = true
		}
		for _, n := range imp.Names {
			c.external[localName(n)] = true
		}
	}
	switch f.Lang {
	case "go":
		dir, ok := r.resolveGoDir(imp.Spec)
		if !ok {
			markExternal()
			return
		}
		if imp.Alias != "" {
			c.ns[imp.Alias] = binding{files: r.goDirs[dir], pkg: dir}
		}
		r.goPending = append(r.goPending, [2]string{itoa(fi), dir})
	case "typescript", "javascript":
		t, ok := r.resolveJS(f.Path, imp.Spec)
		if !ok {
			markExternal()
			return
		}
		r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1)
		b := binding{files: []int{t}}
		if imp.Alias != "" {
			c.ns[imp.Alias] = b
			c.named[imp.Alias] = named{b: b}
		}
		for _, n := range imp.Names {
			orig, local := splitAlias(n)
			c.named[local] = named{b: b, orig: orig}
		}
	case "python":
		t, ok := r.pyModule(f.Path, imp.Spec)
		if len(imp.Names) == 0 { // import a.b.c [as x]
			if !ok {
				markExternal()
				return
			}
			r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1)
			c.ns[imp.Alias] = binding{files: []int{t}}
			return
		}
		if ok {
			r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1)
		}
		for _, n := range imp.Names {
			orig, local := splitAlias(n)
			sub := imp.Spec + "." + orig
			if strings.HasSuffix(imp.Spec, ".") {
				sub = imp.Spec + orig
			}
			if st, sok := r.pyModule(f.Path, sub); sok {
				r.addEdge(r.fileNode[fi], r.fileNode[st], EdgeImports, ConfExtracted, 1)
				c.ns[local] = binding{files: []int{st}}
			} else if ok {
				c.named[local] = named{b: binding{files: []int{t}}, orig: orig}
			} else {
				c.external[local] = true
			}
		}
	case "rust":
		if strings.HasPrefix(imp.Spec, "mod:") {
			name := strings.TrimPrefix(imp.Spec, "mod:")
			dir := path.Dir(f.Path)
			for _, cand := range []string{path.Join(dir, name+".rs"), path.Join(dir, name, "mod.rs")} {
				if t, ok := r.fileByPath[cand]; ok {
					r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1)
					c.ns[name] = binding{files: []int{t}}
					return
				}
			}
			return
		}
		t, item, ok := r.rustModule(f.Path, imp.Spec)
		if !ok {
			markExternal()
			return
		}
		r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1)
		b := binding{files: []int{t}}
		if item == "" {
			if imp.Alias != "" {
				c.ns[imp.Alias] = b
			}
			return
		}
		local := imp.Alias
		if local == "" {
			local = item
		}
		c.named[local] = named{b: b, orig: item}
		c.ns[local] = b
	case "java":
		if strings.HasSuffix(imp.Spec, ".*") {
			pkg := strings.TrimSuffix(imp.Spec, ".*")
			if fs := r.pkgFiles[pkg]; len(fs) > 0 {
				c.ns["*"+pkg] = binding{files: fs, pkg: pkg}
			}
			return
		}
		t, ok := r.lookupMod("java", f.Path, imp.Spec)
		if !ok {
			markExternal()
			return
		}
		r.addEdge(r.fileNode[fi], r.fileNode[t], EdgeImports, ConfExtracted, 1)
		b := binding{files: []int{t}}
		c.named[imp.Alias] = named{b: b, orig: imp.Alias}
		c.ns[imp.Alias] = b
	}
}

func splitAlias(n string) (orig, local string) {
	if i := strings.IndexByte(n, '='); i >= 0 {
		return n[:i], n[i+1:]
	}
	return n, n
}

func localName(n string) string { _, l := splitAlias(n); return l }

func (r *resolver) addEdge(from, to int, kind, conf string, score float64) {
	if from == to {
		return
	}
	k := symEdgeKey{int32(from), int32(to), kindCode[kind]}
	if i, ok := r.edgeIdx[k]; ok {
		e := &r.g.Edges[i]
		if score > e.Score {
			e.Score = score
		}
		if conf == ConfExtracted {
			e.Conf = ConfExtracted
		}
		return
	}
	r.edgeIdx[k] = len(r.g.Edges)
	r.g.Edges = append(r.g.Edges, SymEdge{From: from, To: to, Kind: kind, Conf: conf, Score: score})
}

type cand struct {
	node  int
	conf  string
	score float64
}

// defsByName returns top-level defs of name inside a binding.
func (r *resolver) defsByName(b binding, name string, onlyTypes bool) []int {
	var src []int
	if b.pkg != "" {
		src = r.pkgDefs[b.pkg][name]
	} else {
		for _, fi := range b.files {
			src = append(src, r.defsIn[fi][name]...)
		}
	}
	var out []int
	for _, n := range src {
		d := r.nodeDef[n]
		if d.Kind == KindMethod || (onlyTypes && !isTypeKind(d.Kind)) {
			continue
		}
		out = append(out, n)
	}
	return out
}

func (r *resolver) resolveRefs() {
	for fi, f := range r.files {
		for _, ref := range f.Refs {
			owner := r.fileNode[fi]
			ownerRecv := ""
			if ref.From >= 0 && ref.From < len(r.defNode[fi]) {
				owner = r.defNode[fi][ref.From]
				ownerRecv = f.Defs[ref.From].Recv
				if isTypeKind(f.Defs[ref.From].Kind) {
					ownerRecv = f.Defs[ref.From].Name
				}
			}
			var cs []cand
			if ref.Kind == EdgeCalls {
				cs = r.resolveCall(fi, f, ref, ownerRecv)
			} else {
				cs = r.resolveType(fi, f, ref)
			}
			for _, c := range cs {
				if c.node == owner {
					continue
				}
				r.addEdge(owner, c.node, ref.Kind, c.conf, c.score)
				tf := r.nodeFile[c.node]
				if tf != fi && c.conf == ConfExtracted {
					r.addEdge(r.fileNode[fi], r.fileNode[tf], EdgeImports, ConfExtracted, 1)
				}
			}
		}
	}
}

func (r *resolver) fanout(nodes []int, conf string, uniq, multi float64) []cand {
	switch {
	case len(nodes) == 0 || len(nodes) > r.opts.MaxCandidates:
		return nil
	case len(nodes) == 1:
		return []cand{{nodes[0], conf, uniq}}
	}
	out := make([]cand, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, cand{n, ConfInferred, multi})
	}
	return out
}

func firstSeg(q string) string {
	if i := strings.IndexByte(q, '.'); i >= 0 {
		return q[:i]
	}
	return q
}

func (r *resolver) resolveCall(fi int, f *FileSyms, ref Ref, ownerRecv string) []cand {
	ctx := &r.ctx[fi]
	name := ref.Name
	if ref.Qual == "" {
		// Same file first, then named imports, then same package, then a unique global.
		var local []int
		for _, n := range r.defsIn[fi][name] {
			d := r.nodeDef[n]
			if d.Recv == "" || d.Recv == ownerRecv {
				local = append(local, n)
			}
		}
		if len(local) > 0 {
			return r.fanout(local, ConfExtracted, 1, 0.5)
		}
		if nm, ok := ctx.named[name]; ok {
			orig := nm.orig
			if orig == "" {
				orig = name
			}
			if hit := r.defsByName(nm.b, orig, false); len(hit) > 0 {
				return r.fanout(hit, ConfExtracted, 0.95, 0.5)
			}
			if orig == name && len(nm.b.files) == 1 { // default export: take the file's only top-level def
				if only := r.onlyTopLevel(nm.b.files[0]); only >= 0 {
					return []cand{{only, ConfInferred, 0.8}}
				}
			}
			return nil
		}
		if ctx.external[name] {
			return nil
		}
		if f.Pkg != "" {
			var pk []int
			for _, n := range r.pkgDefs[f.Pkg][name] {
				if r.nodeDef[n].Recv == "" {
					pk = append(pk, n)
				}
			}
			if len(pk) > 0 {
				return r.fanout(pk, ConfExtracted, 0.95, 0.5)
			}
		}
		for k, b := range ctx.ns { // Java wildcard imports
			if strings.HasPrefix(k, "*") {
				if hit := r.defsByName(b, name, false); len(hit) > 0 {
					return r.fanout(hit, ConfExtracted, 0.9, 0.4)
				}
			}
		}
		if f.Lang == "go" {
			return nil
		}
		return r.fanout(r.byName[name], ConfInferred, 0.5, 0.25)
	}

	q := ref.Qual
	first := firstSeg(q)
	if strings.HasPrefix(q, "@") {
		if hit := r.methodsOf[q[1:]+"\x00"+name]; len(hit) > 0 {
			return r.fanout(hit, ConfExtracted, 0.9, 0.45)
		}
		return r.methodGuess(name, 0.45)
	}
	switch first {
	case "this", "self", "cls", "super", "Self":
		if q == first && ownerRecv != "" {
			hit := r.methodsOf[ownerRecv+"\x00"+name]
			same := hit[:0:0]
			for _, n := range hit {
				if r.nodeFile[n] == fi {
					same = append(same, n)
				}
			}
			if len(same) > 0 {
				return r.fanout(same, ConfExtracted, 0.9, 0.45)
			}
			if len(hit) > 0 {
				return r.fanout(hit, ConfExtracted, 0.85, 0.4)
			}
			for _, n := range r.defsIn[fi][name] { // Go closures and embedded access
				if r.nodeDef[n].Recv == "" {
					return []cand{{n, ConfInferred, 0.6}}
				}
			}
		}
		return r.methodGuess(name, 0.5)
	}
	if b, ok := ctx.ns[first]; ok {
		if hit := r.defsByName(b, name, false); len(hit) > 0 && strings.Count(q, ".") == 0 {
			return r.fanout(hit, ConfExtracted, 0.95, 0.5)
		}
		if i := strings.LastIndexByte(q, '.'); i >= 0 || q != "" {
			recv := q[i+1:]
			var hit []int
			for _, fj := range b.files {
				for _, n := range r.defsIn[fj][name] {
					if r.nodeDef[n].Recv == recv {
						hit = append(hit, n)
					}
				}
			}
			if len(hit) > 0 {
				return r.fanout(hit, ConfExtracted, 0.9, 0.45)
			}
		}
		if hit := r.defsByName(b, name, false); len(hit) > 0 {
			return r.fanout(hit, ConfExtracted, 0.9, 0.45)
		}
		return nil
	}
	if nm, ok := ctx.named[first]; ok {
		orig := nm.orig
		if orig == "" {
			orig = first
		}
		var hit []int
		for _, fj := range nm.b.files {
			for _, n := range r.defsIn[fj][name] {
				if r.nodeDef[n].Recv == orig {
					hit = append(hit, n)
				}
			}
		}
		if len(hit) > 0 {
			return r.fanout(hit, ConfExtracted, 0.9, 0.45)
		}
		return nil
	}
	if ctx.external[first] {
		return nil
	}
	if q == first && r.typeNames[first] {
		if hit := r.methodsOf[first+"\x00"+name]; len(hit) > 0 {
			return r.fanout(hit, ConfExtracted, 0.85, 0.4)
		}
		return nil
	}
	if q == first && first != "" && first[0] >= 'A' && first[0] <= 'Z' {
		return nil // static call on a type that is not part of this repo
	}
	return r.methodGuess(name, 0.55)
}

// methodGuess links a call with an unknown receiver to same-named methods. Generic
// names such as "get" or "close" would link everything, so they score lower.
func (r *resolver) methodGuess(name string, uniq float64) []cand {
	if ubiquitous[name] {
		uniq = 0.3
	}
	return r.fanout(r.methods[name], ConfInferred, uniq, 0.25)
}

func (r *resolver) onlyTopLevel(fi int) int {
	found := -1
	for di, d := range r.files[fi].Defs {
		if d.Recv != "" {
			continue
		}
		if found >= 0 {
			return -1
		}
		found = r.defNode[fi][di]
	}
	return found
}

func (r *resolver) resolveType(fi int, f *FileSyms, ref Ref) []cand {
	ctx := &r.ctx[fi]
	name := ref.Name
	if ref.Qual != "" {
		if b, ok := ctx.ns[firstSeg(ref.Qual)]; ok {
			if hit := r.defsByName(b, name, true); len(hit) > 0 {
				return r.fanout(hit, ConfExtracted, 0.95, 0.5)
			}
		}
		if ctx.external[firstSeg(ref.Qual)] {
			return nil
		}
	}
	var local []int
	for _, n := range r.defsIn[fi][name] {
		if isTypeKind(r.nodeDef[n].Kind) {
			local = append(local, n)
		}
	}
	if len(local) > 0 {
		return r.fanout(local, ConfExtracted, 1, 0.5)
	}
	if nm, ok := ctx.named[name]; ok {
		orig := nm.orig
		if orig == "" {
			orig = name
		}
		if hit := r.defsByName(nm.b, orig, true); len(hit) > 0 {
			return r.fanout(hit, ConfExtracted, 0.95, 0.5)
		}
		return nil
	}
	if ctx.external[name] {
		return nil
	}
	if f.Pkg != "" {
		var pk []int
		for _, n := range r.pkgDefs[f.Pkg][name] {
			if isTypeKind(r.nodeDef[n].Kind) {
				pk = append(pk, n)
			}
		}
		if len(pk) > 0 {
			return r.fanout(pk, ConfExtracted, 0.95, 0.5)
		}
	}
	var types []int
	for _, n := range r.byName[name] {
		if isTypeKind(r.nodeDef[n].Kind) {
			types = append(types, n)
		}
	}
	return r.fanout(types, ConfInferred, 0.6, 0.3)
}

func (r *resolver) linkUnusedGoImports() {
	for _, p := range r.goPending {
		fi := 0
		for _, ch := range p[0] {
			fi = fi*10 + int(ch-'0')
		}
		dir := r.goDirs[p[1]]
		if len(dir) == 0 {
			continue
		}
		used := false
		for _, t := range dir {
			if _, ok := r.edgeIdx[symEdgeKey{int32(r.fileNode[fi]), int32(r.fileNode[t]), kindCode[EdgeImports]}]; ok {
				used = true
				break
			}
		}
		if !used {
			r.addEdge(r.fileNode[fi], r.fileNode[dir[0]], EdgeImports, ConfExtracted, 0.5)
		}
	}
}

// goImplements links Go types to interfaces whose method names they all provide.
// Go typing is structural, so these edges are INFERRED by name only.
func (r *resolver) goImplements() {
	type tkey struct{ pkg, name string }
	methodSet := map[tkey]map[string]bool{}
	typeNode := map[tkey]int{}
	typesBy := map[string][]tkey{}
	for fi, f := range r.files {
		if f.Lang != "go" {
			continue
		}
		for di, d := range f.Defs {
			k := tkey{f.Pkg, d.Recv}
			switch {
			case d.Kind == KindMethod:
				if methodSet[k] == nil {
					methodSet[k] = map[string]bool{}
				}
				if !methodSet[k][d.Name] {
					methodSet[k][d.Name] = true
					typesBy[d.Name] = append(typesBy[d.Name], k)
				}
			case d.Kind == KindType:
				typeNode[tkey{f.Pkg, d.Name}] = r.defNode[fi][di]
			}
		}
	}
	for fi, f := range r.files {
		if f.Lang != "go" {
			continue
		}
		for di, d := range f.Defs {
			if d.Kind != KindInterface || len(d.Methods) == 0 {
				continue
			}
			rare := d.Methods[0]
			for _, m := range d.Methods {
				if len(typesBy[m]) < len(typesBy[rare]) {
					rare = m
				}
			}
			var hits []int
			for _, k := range typesBy[rare] {
				tn, ok := typeNode[k]
				if !ok {
					continue
				}
				all := true
				for _, m := range d.Methods {
					if !methodSet[k][m] {
						all = false
						break
					}
				}
				if all {
					hits = append(hits, tn)
				}
			}
			if len(hits) > 15 {
				continue
			}
			for _, tn := range hits {
				r.addEdge(tn, r.defNode[fi][di], EdgeImplements, ConfInferred, 0.7)
			}
		}
	}
}

func (r *resolver) finish() {
	sort.SliceStable(r.g.Edges, func(i, j int) bool {
		a, b := r.g.Edges[i], r.g.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		return a.Kind < b.Kind
	})
	for i := range r.g.Edges {
		r.g.Edges[i].Score = float64(int(r.g.Edges[i].Score*100+0.5)) / 100
	}
	r.g.Index()
	r.g.ComputeCommunities()
}

// DiscoverGoModules reads go.mod files next to the given Go sources.
func DiscoverGoModules(root string, files []string) map[string]string {
	mods := map[string]string{}
	seen := map[string]bool{}
	for _, f := range files {
		if filepath.Ext(f) != ".go" {
			continue
		}
		for dir := path.Dir(f); ; dir = path.Dir(dir) {
			if seen[dir] {
				break
			}
			seen[dir] = true
			if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), "go.mod")); err == nil {
				for _, l := range strings.Split(string(data), "\n") {
					l = strings.TrimSpace(l)
					if strings.HasPrefix(l, "module ") {
						mods[strings.Trim(strings.TrimSpace(strings.TrimPrefix(l, "module ")), `"`)] = dir
						break
					}
				}
			}
			if dir == "." || dir == "/" {
				break
			}
		}
	}
	return mods
}

// BuildSymbols extracts and resolves the given root-relative files into a graph.
func BuildSymbols(root string, files []string, eo ExtractOptions) *SymGraph {
	fs := Extract(root, files, eo)
	g := Resolve(fs, ResolveOptions{GoModules: DiscoverGoModules(root, files)})
	g.Root = filepath.Base(root)
	g.attachBodies(root)
	return g
}
