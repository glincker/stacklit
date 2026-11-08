package graph

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glincker/stacklit/internal/parser"
)

// langResolver maps language-specific import strings (Rust paths, Java
// packages, Ruby requires) to module names. The generic resolver only knows
// directory-style paths, which these languages never use.
type langResolver struct {
	root     string
	fileMod  map[string]string   // slash path -> module of that file
	dirMod   map[string]string   // slash dir -> module of files in that dir
	javaPkgs map[string][]string // trailing dir path -> sorted unique modules
	rubyLibs map[string][]string // require path without .rb -> sorted unique file paths
	crates   map[string]string   // Rust crate identifier -> its source dir
	seenSrc  map[string]bool
}

var cargoNameRe = regexp.MustCompile(`(?m)^\s*name\s*=\s*"([^"]+)"`)

func newLangResolver(files []*parser.FileInfo, root string, modOf map[string]string) *langResolver {
	r := &langResolver{
		root:     root,
		fileMod:  map[string]string{},
		dirMod:   map[string]string{},
		javaPkgs: map[string][]string{},
		rubyLibs: map[string][]string{},
		crates:   map[string]string{},
		seenSrc:  map[string]bool{},
	}
	javaSeen := map[string]map[string]bool{}
	for _, f := range files {
		p := filepath.ToSlash(f.Path)
		mod := modOf[f.Path]
		r.fileMod[p] = mod
		dir := path.Dir(p)
		r.dirMod[dir] = mod
		switch strings.ToLower(path.Ext(p)) {
		case ".java":
			for _, suf := range dirSuffixes(dir) {
				if javaSeen[suf] == nil {
					javaSeen[suf] = map[string]bool{}
				}
				if !javaSeen[suf][mod] {
					javaSeen[suf][mod] = true
					r.javaPkgs[suf] = append(r.javaPkgs[suf], mod)
				}
			}
		case ".rb":
			noExt := strings.TrimSuffix(p, path.Ext(p))
			for _, suf := range dirSuffixes(noExt) {
				r.rubyLibs[suf] = append(r.rubyLibs[suf], p)
			}
		case ".rs":
			r.noteRustCrate(dir)
		}
	}
	for k := range r.javaPkgs {
		sort.Strings(r.javaPkgs[k])
	}
	for k := range r.rubyLibs {
		sort.Strings(r.rubyLibs[k])
	}
	return r
}

// dirSuffixes returns every trailing sub-path of p, longest first.
func dirSuffixes(p string) []string {
	parts := strings.Split(p, "/")
	out := make([]string, 0, len(parts))
	for i := range parts {
		out = append(out, strings.Join(parts[i:], "/"))
	}
	return out
}

// noteRustCrate registers the crate whose source dir contains dir. The crate
// name comes from the sibling Cargo.toml, else the directory name.
func (r *langResolver) noteRustCrate(dir string) {
	var src string
	switch {
	case dir == "src" || strings.HasPrefix(dir, "src/"):
		src = "src"
	default:
		i := strings.Index(dir, "/src")
		if i < 0 || (len(dir) > i+4 && dir[i+4] != '/') {
			return
		}
		src = dir[:i+4]
	}
	if r.seenSrc[src] {
		return
	}
	r.seenSrc[src] = true
	crateDir := path.Dir(src)
	name := ""
	if r.root != "" {
		if b, err := os.ReadFile(filepath.Join(r.root, filepath.FromSlash(crateDir), "Cargo.toml")); err == nil {
			if m := cargoNameRe.FindSubmatch(b); m != nil {
				name = string(m[1])
			}
		}
	}
	if name == "" && crateDir != "." {
		name = path.Base(crateDir)
	}
	if name == "" {
		return
	}
	ident := strings.ReplaceAll(name, "-", "_")
	if _, ok := r.crates[ident]; !ok {
		r.crates[ident] = src
	}
	// Workspace members often drop the shared prefix from the directory name.
	if crateDir != "." {
		if base := strings.ReplaceAll(path.Base(crateDir), "-", "_"); base != ident {
			if _, ok := r.crates[base]; !ok {
				r.crates[base] = src
			}
		}
	}
}

// resolve returns the module an import refers to. handled is false for
// languages this resolver does not own, so the generic resolver runs instead.
func (r *langResolver) resolve(f *parser.FileInfo, imp string) (mod string, handled bool) {
	p := filepath.ToSlash(f.Path)
	switch strings.ToLower(path.Ext(p)) {
	case ".rs":
		return r.rust(p, imp), true
	case ".java":
		return r.java(p, imp), true
	case ".rb":
		return r.ruby(p, imp), true
	}
	return "", false
}

// ---- Rust ----

// rustCrateRoot is the src dir that holds file p.
func (r *langResolver) rustCrateRoot(p string) string {
	dir := path.Dir(p)
	for d := dir; ; d = path.Dir(d) {
		if path.Base(d) == "src" {
			return d
		}
		if d == "." || d == "/" || d == "" {
			return ""
		}
	}
}

// rustModDir is the directory holding p's child modules.
func rustModDir(p string) string {
	dir := path.Dir(p)
	switch path.Base(p) {
	case "lib.rs", "main.rs", "mod.rs":
		return dir
	}
	return path.Join(dir, strings.TrimSuffix(path.Base(p), ".rs"))
}

// rustDescend follows module segments below base and returns the module of
// the deepest file or directory found, or the module at base.
func (r *langResolver) rustDescend(base string, segs []string) string {
	cur := base
	found := ""
	for _, seg := range segs {
		if seg == "" || seg == "self" {
			continue
		}
		file := path.Join(cur, seg+".rs")
		if m, ok := r.fileMod[file]; ok {
			found = m
			cur = path.Join(cur, seg)
			continue
		}
		if m, ok := r.fileMod[path.Join(cur, seg, "mod.rs")]; ok {
			found = m
			cur = path.Join(cur, seg)
			continue
		}
		if m, ok := r.dirMod[path.Join(cur, seg)]; ok {
			found = m
			cur = path.Join(cur, seg)
			continue
		}
		break
	}
	if found != "" {
		return found
	}
	return r.dirMod[base]
}

func (r *langResolver) rust(p, imp string) string {
	segs := strings.Split(imp, "::")
	head := segs[0]
	switch head {
	case "crate":
		if root := r.rustCrateRoot(p); root != "" {
			return r.rustDescend(root, segs[1:])
		}
		return ""
	case "self":
		return r.rustDescend(rustModDir(p), segs[1:])
	case "super":
		base := path.Dir(rustModDir(p))
		rest := segs[1:]
		for len(rest) > 0 && rest[0] == "super" {
			base = path.Dir(base)
			rest = rest[1:]
		}
		return r.rustDescend(base, rest)
	}
	if len(segs) == 1 {
		// `mod name;` declares a child file; a bare crate name is an extern crate.
		dir := rustModDir(p)
		if m := r.rustDescend(dir, segs); m != "" && (r.fileMod[path.Join(dir, head+".rs")] != "" || r.fileMod[path.Join(dir, head, "mod.rs")] != "") {
			return m
		}
	}
	if src, ok := r.crates[head]; ok {
		return r.rustDescend(src, segs[1:])
	}
	return ""
}

// ---- Java ----

func (r *langResolver) java(p, imp string) string {
	segs := strings.Split(imp, ".")
	// Static and nested-class imports keep trailing identifiers; shorten until a package dir matches.
	for n := len(segs); n >= 2; n-- {
		mods := r.javaPkgs[strings.Join(segs[:n], "/")]
		if len(mods) == 0 {
			continue
		}
		return closestModule(p, mods)
	}
	return ""
}

// closestModule picks the candidate sharing the longest path prefix with p.
func closestModule(p string, mods []string) string {
	best, bestScore := mods[0], -1
	for _, m := range mods {
		score := 0
		for i := 0; i < len(m) && i < len(p) && m[i] == p[i]; i++ {
			score++
		}
		if score > bestScore {
			best, bestScore = m, score
		}
	}
	return best
}

// ---- Ruby ----

func (r *langResolver) ruby(p, imp string) string {
	imp = strings.TrimSuffix(imp, ".rb")
	if strings.HasPrefix(imp, "./") || strings.HasPrefix(imp, "../") {
		target := path.Clean(path.Join(path.Dir(p), imp)) + ".rb"
		if m, ok := r.fileMod[target]; ok {
			return m
		}
		return ""
	}
	// require_relative targets are stored without "./", so try the file's own dir first.
	if m, ok := r.fileMod[path.Join(path.Dir(p), imp)+".rb"]; ok {
		return m
	}
	cands := r.rubyLibs[imp]
	if len(cands) == 0 {
		return ""
	}
	// Prefer lib/ layouts: that is where the load path points.
	var lib []string
	for _, c := range cands {
		if strings.HasPrefix(c, "lib/") || strings.Contains(c, "/lib/") {
			lib = append(lib, c)
		}
	}
	if len(lib) > 0 {
		cands = lib
	}
	mods := make([]string, 0, len(cands))
	for _, c := range cands {
		mods = append(mods, r.fileMod[c])
	}
	return closestModule(p, mods)
}
