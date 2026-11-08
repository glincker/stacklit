package graph

import (
	"regexp"
	"strings"
)

const rsVis = `(?:pub(?:\([^)]*\))?\s+)?`

var (
	rsFnRe    = regexp.MustCompile(`^` + rsVis + `(?:default\s+)?(?:const\s+)?(?:async\s+)?(?:unsafe\s+)?(?:extern\s+"?\w*"?\s+)?fn\s+([A-Za-z_]\w*)`)
	rsTypeRe  = regexp.MustCompile(`^` + rsVis + `(struct|enum|trait|union|type)\s+([A-Za-z_]\w*)(?:\s*<[^{;=]*?>)?(?:\s*:\s*([^{;=]+?))?\s*(?:where\b[^{]*)?(?:[{;=(]|$)`)
	rsImplRe  = regexp.MustCompile(`^(?:unsafe\s+)?impl\s*(?:<[^>]*>)?\s*(?:([\w:]+)(?:<[^{]*?>)?\s+for\s+)?([\w:]+)`)
	rsUseRe   = regexp.MustCompile(`^` + rsVis + `use\s+([^;]+);`)
	rsModRe   = regexp.MustCompile(`^` + rsVis + `mod\s+([A-Za-z_]\w*)\s*;`)
	rsScanner = &refScanner{
		re:       regexp.MustCompile(`((?:[A-Za-z_]\w*\s*(?:::|\.)\s*)*)([A-Za-z_]\w*)\s*(?:::\s*<[^<>()]*>)?\s*\(`),
		keywords: wordSet("if while for match return loop fn let in as move unsafe where impl dyn ref mut await async"),
		builtins: wordSet("Some None Ok Err Box Vec drop"),
	}
)

func extractRust(file string, src []byte) *FileSyms {
	fs := &FileSyms{Path: file, Lang: "rust"}
	orig := splitLines(src)
	lines := splitLines(stripNoise(src, "rust"))
	depth := braceDepths(lines)

	// use statements may span lines; join from the original text.
	for i := 0; i < len(orig); i++ {
		t := strings.TrimSpace(orig[i])
		if !strings.HasPrefix(t, "use ") && !strings.HasPrefix(t, "pub use ") && !strings.HasPrefix(t, "pub(crate) use ") {
			if m := rsModRe.FindStringSubmatch(t); m != nil {
				fs.Imports = append(fs.Imports, Import{Spec: "mod:" + m[1], Alias: m[1]})
			}
			continue
		}
		for !strings.Contains(t, ";") && i+1 < len(orig) {
			i++
			t += " " + strings.TrimSpace(orig[i])
		}
		if m := rsUseRe.FindStringSubmatch(t); m != nil {
			fs.Imports = append(fs.Imports, expandUse(m[1])...)
		}
	}

	type scope struct {
		name       string
		depth, end int
		isImpl     bool
	}
	var scopes []scope
	var spans []span
	decl := map[int]bool{}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		for len(scopes) > 0 && i > scopes[len(scopes)-1].end {
			scopes = scopes[:len(scopes)-1]
		}
		inScope := len(scopes) > 0 && depth[i] == scopes[len(scopes)-1].depth+1
		if depth[i] > 0 && !inScope {
			continue
		}
		if m := rsFnRe.FindStringSubmatch(line); m != nil {
			d := Def{Name: m[1], Qual: m[1], Kind: KindFunc, Line: i + 1, Sig: sigOf(orig[i])}
			if inScope {
				d.Kind, d.Recv, d.Qual = KindMethod, scopes[len(scopes)-1].name, scopes[len(scopes)-1].name+"."+m[1]
			}
			fs.Defs = append(fs.Defs, d)
			spans = append(spans, span{i, blockEnd(lines, i), len(fs.Defs) - 1})
			decl[i] = true
			continue
		}
		if inScope {
			continue
		}
		if m := rsImplRe.FindStringSubmatch(line); m != nil {
			target := lastSeg(strings.ReplaceAll(m[2], "::", "."))
			if m[1] != "" {
				// Link the type to the trait it implements when both live in this file.
				if idx := findDef(fs, target); idx >= 0 {
					tr := strings.ReplaceAll(m[1], "::", ".")
					fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeImplements, Name: lastSeg(tr), Qual: firstQual(tr)})
				}
			}
			scopes = append(scopes, scope{target, depth[i], blockEnd(lines, i), true})
			continue
		}
		if m := rsTypeRe.FindStringSubmatch(line); m != nil {
			k := KindType
			switch m[1] {
			case "enum":
				k = KindEnum
			case "trait":
				k = KindTrait
			}
			fs.Defs = append(fs.Defs, Def{Name: m[2], Qual: m[2], Kind: k, Line: i + 1, Sig: sigOf(orig[i])})
			idx := len(fs.Defs) - 1
			if m[1] == "trait" {
				for _, b := range strings.Split(m[3], "+") {
					b = strings.TrimSpace(b)
					if i := strings.IndexAny(b, "<"); i >= 0 {
						b = b[:i]
					}
					b = strings.ReplaceAll(b, "::", ".")
					if b != "" && !strings.HasPrefix(b, "'") && b != "Sized" {
						fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeInherits, Name: lastSeg(b), Qual: firstQual(b)})
					}
				}
				scopes = append(scopes, scope{m[2], depth[i], blockEnd(lines, i), false})
			}
		}
	}
	fillDefs(fs, orig, lines)
	own := ownerMap(len(lines), spans)
	fs.Refs = append(fs.Refs, scanBodies(lines, own, decl, rsScanner)...)
	return fs
}

func findDef(fs *FileSyms, name string) int {
	for i, d := range fs.Defs {
		if d.Name == name && d.Recv == "" && d.Kind != KindFunc {
			return i
		}
	}
	return -1
}

var useSpace = regexp.MustCompile(`\s*(::|,|[{}])\s*`)

// expandUse flattens a use tree such as a::{b, c::d as e} into one Import per leaf.
func expandUse(s string) []Import {
	s = useSpace.ReplaceAllString(strings.TrimSpace(s), "$1")
	var out []Import
	var walk func(prefix, rest string)
	walk = func(prefix, rest string) {
		i := strings.IndexByte(rest, '{')
		if i < 0 {
			spec, alias := rest, ""
			if k := strings.Index(rest, " as "); k > 0 {
				spec, alias = rest[:k], rest[k+4:]
			}
			full := prefix + spec
			if full == "" {
				return
			}
			if full == "self" || strings.HasSuffix(full, "::self") {
				full = strings.TrimSuffix(strings.TrimSuffix(full, "self"), "::")
				if alias == "" && full != "" {
					alias = full[strings.LastIndex(full, "::")+2:]
				}
				out = append(out, Import{Spec: full, Alias: alias})
				return
			}
			if strings.HasSuffix(full, "::*") {
				out = append(out, Import{Spec: strings.TrimSuffix(full, "::*")})
				return
			}
			name := full[strings.LastIndex(full, "::")+2:]
			if alias == "" {
				alias = name
			}
			out = append(out, Import{Spec: full, Alias: alias, Names: []string{name}})
			return
		}
		head := rest[:i]
		depth := 0
		start := i + 1
		for j := i; j < len(rest); j++ {
			switch rest[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					walk(prefix+head, rest[start:j])
					return
				}
			case ',':
				if depth == 1 {
					walk(prefix+head, rest[start:j])
					start = j + 1
				}
			}
		}
	}
	walk("", s)
	return out
}
