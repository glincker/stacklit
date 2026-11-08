package graph

import (
	"regexp"
	"strings"
)

var (
	pyDefRe   = regexp.MustCompile(`^(\s*)(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pyClassRe = regexp.MustCompile(`^(\s*)class\s+([A-Za-z_]\w*)\s*(?:\(([^)]*)\))?\s*:`)

	pyImportRe = regexp.MustCompile(`^\s*import\s+([\w.,\s]+?)\s*$`)
	pyFromRe   = regexp.MustCompile(`^\s*from\s+(\.*[\w.]*)\s+import\s+(.+)$`)

	pyCallScanner = &refScanner{
		re:       regexp.MustCompile(`((?:[A-Za-z_]\w*\s*\.\s*)*)([A-Za-z_]\w*)\s*\(`),
		keywords: wordSet("if while for elif return and or not in is lambda with assert yield await print except def class raise else del global nonlocal"),
		builtins: wordSet("len range str int float bool list dict set tuple frozenset isinstance issubclass super open enumerate zip map filter sorted reversed min max sum any all getattr setattr hasattr delattr type repr format iter next abs round id hash vars dir bytes bytearray object property staticmethod classmethod callable input divmod pow chr ord hex bin oct slice"),
	}
)

func indentOf(s string) int {
	n := 0
	for _, c := range s {
		switch c {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

func extractPython(file string, src []byte) *FileSyms {
	fs := &FileSyms{Path: file, Lang: "python"}
	orig := splitLines(src)
	lines := splitLines(stripNoise(src, "python"))
	fs.Imports = pyImports(orig)

	type scope struct {
		name   string
		indent int
		end    int
	}
	var classes []scope
	var spans []span
	decl := map[int]bool{}

	// headerEnd returns the last line of a possibly multi-line def/class header.
	headerEnd := func(i int) int {
		bal := 0
		for j := i; j < len(lines) && j < i+60; j++ {
			bal += strings.Count(lines[j], "(") + strings.Count(lines[j], "[") - strings.Count(lines[j], ")") - strings.Count(lines[j], "]")
			if bal <= 0 && strings.HasSuffix(strings.TrimSpace(lines[j]), ":") {
				return j
			}
			if bal <= 0 && j > i {
				return j
			}
		}
		return i
	}
	bodyEnd := func(h, indent int) int {
		end := h
		for j := h + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			if indentOf(lines[j]) <= indent {
				break
			}
			end = j
		}
		return end
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		ind := indentOf(line)
		for len(classes) > 0 && (i > classes[len(classes)-1].end) {
			classes = classes[:len(classes)-1]
		}
		if m := pyClassRe.FindStringSubmatch(line); m != nil {
			h := headerEnd(i)
			end := bodyEnd(h, len(m[1]))
			fs.Defs = append(fs.Defs, Def{Name: m[2], Qual: m[2], Kind: KindClass, Line: i + 1, Sig: sigOf(orig[i])})
			idx := len(fs.Defs) - 1
			for _, b := range splitTypeList(m[3]) {
				if strings.Contains(b, "=") || b == "object" || b == "" {
					continue
				}
				fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeInherits, Name: lastSeg(b), Qual: firstQual(b)})
			}
			classes = append(classes, scope{m[2], len(m[1]), end})
			for k := i; k <= h; k++ {
				decl[k] = true
			}
			i = h
			continue
		}
		if m := pyDefRe.FindStringSubmatch(line); m != nil {
			h := headerEnd(i)
			end := bodyEnd(h, len(m[1]))
			d := Def{Name: m[2], Qual: m[2], Kind: KindFunc, Line: i + 1, Sig: sigOf(orig[i])}
			if len(classes) > 0 && ind > classes[len(classes)-1].indent {
				c := classes[len(classes)-1]
				d.Kind, d.Recv, d.Qual = KindMethod, c.name, c.name+"."+m[2]
			}
			fs.Defs = append(fs.Defs, d)
			spans = append(spans, span{i, end, len(fs.Defs) - 1})
			for k := i; k <= h; k++ {
				decl[k] = true
			}
			continue
		}
	}
	fillDefs(fs, orig, lines)
	own := ownerMap(len(lines), spans)
	fs.Refs = append(fs.Refs, scanBodies(lines, own, decl, pyCallScanner)...)
	return fs
}

func pyImports(lines []string) []Import {
	var out []Import
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if m := pyFromRe.FindStringSubmatch(l); m != nil {
			rest := strings.TrimSpace(strings.SplitN(m[2], "#", 2)[0])
			if strings.HasPrefix(rest, "(") {
				for !strings.Contains(rest, ")") && i+1 < len(lines) {
					i++
					rest += " " + strings.TrimSpace(strings.SplitN(lines[i], "#", 2)[0])
				}
				rest = strings.Trim(rest, "() ")
			}
			imp := Import{Spec: m[1]}
			for _, p := range strings.Split(rest, ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				if k := strings.Index(p, " as "); k >= 0 {
					imp.Names = append(imp.Names, strings.TrimSpace(p[:k])+"="+strings.TrimSpace(p[k+4:]))
				} else {
					imp.Names = append(imp.Names, p)
				}
			}
			out = append(out, imp)
		} else if m := pyImportRe.FindStringSubmatch(l); m != nil {
			for _, p := range strings.Split(m[1], ",") {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				imp := Import{Spec: p}
				if k := strings.Index(p, " as "); k >= 0 {
					imp.Spec, imp.Alias = strings.TrimSpace(p[:k]), strings.TrimSpace(p[k+4:])
				} else {
					imp.Alias = strings.SplitN(p, ".", 2)[0]
				}
				out = append(out, imp)
			}
		}
	}
	return out
}
