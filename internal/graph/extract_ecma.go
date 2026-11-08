package graph

import (
	"regexp"
	"strings"
)

var (
	jsFuncRe   = regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:async\s+)?function\s*\*?\s*([A-Za-z_$][\w$]*)`)
	jsClassRe  = regexp.MustCompile(`^(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)(?:\s*<[^{]*?>)?(?:\s+extends\s+([\w$.]+))?(?:\s*<[^{]*?>)?(?:\s+implements\s+([^{]+?))?\s*(?:\{.*)?$`)
	jsIfaceRe  = regexp.MustCompile(`^(?:export\s+)?(?:declare\s+)?interface\s+([A-Za-z_$][\w$]*)(?:\s*<[^{]*?>)?(?:\s+extends\s+([^{]+?))?\s*(?:\{.*)?$`)
	jsTypeRe   = regexp.MustCompile(`^(?:export\s+)?(?:declare\s+)?(?:const\s+)?(type|enum)\s+([A-Za-z_$][\w$]*)`)
	jsArrowRe  = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(?:async\s+)?(?:function\b|\([^)]*\)\s*(?::[^=]+)?=>|[A-Za-z_$][\w$]*\s*(?::[^=]+)?=>|<[^>]*>\s*\()`)
	jsMethodRe = regexp.MustCompile(`^(?:(?:public|private|protected|static|async|override|readonly|abstract|get|set|declare)\s+)*\*?\s*(#?[A-Za-z_$][\w$]*)\s*(?:<[^>(]*>)?\s*\(`)
	jsPropFnRe = regexp.MustCompile(`^(?:(?:public|private|protected|static|readonly|override)\s+)*(#?[A-Za-z_$][\w$]*)\s*(?::[^=]+)?=\s*(?:async\s+)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*(?::[^=]+)?=>`)

	jsImportFrom = regexp.MustCompile(`(?:^|\s)(?:import|export)\s+(?:type\s+)?([^'"]*?)\s*from\s*['"]([^'"]+)['"]`)
	jsImportSide = regexp.MustCompile(`^\s*import\s*['"]([^'"]+)['"]`)
	jsRequire    = regexp.MustCompile(`(?:const|let|var)\s+(\{[^}]*\}|[A-Za-z_$][\w$]*)\s*=\s*require\(\s*['"]([^'"]+)['"]\s*\)`)

	jsCallScanner = &refScanner{
		re:       regexp.MustCompile(`((?:[A-Za-z_$][\w$]*\s*(?:\?\.|\.)\s*)*)([A-Za-z_$][\w$]*)\s*(?:<[^<>()]*>)?\s*\(`),
		keywords: wordSet("if for while switch catch function return typeof new void await yield delete import require super do else with constructor"),
		builtins: wordSet("String Number Boolean Array Object Symbol Date Error Promise Map Set parseInt parseFloat isNaN setTimeout setInterval clearTimeout clearInterval BigInt RegExp"),
	}
	jsMemberKeywords = wordSet("if for while switch catch function return typeof new void await yield delete import require do else with")
)

func extractJS(file, lang string, src []byte) *FileSyms {
	fs := &FileSyms{Path: file, Lang: lang}
	orig := splitLines(src)
	lines := splitLines(stripNoise(src, lang))
	depth := braceDepths(lines)
	fs.Imports = jsImports(orig)

	type scope struct {
		name       string
		depth, end int
	}
	var classes []scope
	var spans []span
	decl := map[int]bool{}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		for len(classes) > 0 && i > classes[len(classes)-1].end {
			classes = classes[:len(classes)-1]
		}
		add := func(d Def, callable bool) int {
			d.Line = i + 1
			d.Sig = sigOf(orig[i])
			fs.Defs = append(fs.Defs, d)
			idx := len(fs.Defs) - 1
			if callable {
				end := blockEnd(lines, i)
				spans = append(spans, span{i, end, idx})
				decl[i] = true
			}
			return idx
		}
		if len(classes) > 0 && depth[i] == classes[len(classes)-1].depth+1 {
			c := classes[len(classes)-1]
			var mname string
			if m := jsPropFnRe.FindStringSubmatch(line); m != nil {
				mname = m[1]
			} else if m := jsMethodRe.FindStringSubmatch(line); m != nil && !jsMemberKeywords[m[1]] && !strings.HasSuffix(line, ";") {
				mname = m[1]
			}
			if mname != "" {
				add(Def{Name: mname, Qual: c.name + "." + mname, Kind: KindMethod, Recv: c.name}, true)
			}
			continue
		}
		if depth[i] > 0 && len(classes) == 0 {
			continue
		}
		if len(classes) > 0 {
			continue
		}
		switch {
		case jsFuncRe.MatchString(line):
			m := jsFuncRe.FindStringSubmatch(line)
			add(Def{Name: m[1], Qual: m[1], Kind: KindFunc}, true)
		case jsClassRe.MatchString(line):
			m := jsClassRe.FindStringSubmatch(line)
			idx := add(Def{Name: m[1], Qual: m[1], Kind: KindClass}, false)
			if m[2] != "" {
				fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeInherits, Name: lastSeg(m[2]), Qual: firstQual(m[2])})
			}
			for _, b := range splitTypeList(m[3]) {
				fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeImplements, Name: lastSeg(b), Qual: firstQual(b)})
			}
			classes = append(classes, scope{m[1], depth[i], blockEnd(lines, i)})
		case jsIfaceRe.MatchString(line):
			m := jsIfaceRe.FindStringSubmatch(line)
			idx := add(Def{Name: m[1], Qual: m[1], Kind: KindInterface}, false)
			for _, b := range splitTypeList(m[2]) {
				fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeInherits, Name: lastSeg(b), Qual: firstQual(b)})
			}
		case jsTypeRe.MatchString(line):
			m := jsTypeRe.FindStringSubmatch(line)
			k := KindType
			if m[1] == "enum" {
				k = KindEnum
			}
			add(Def{Name: m[2], Qual: m[2], Kind: k}, false)
		case depth[i] == 0 && jsArrowRe.MatchString(line):
			m := jsArrowRe.FindStringSubmatch(line)
			add(Def{Name: m[1], Qual: m[1], Kind: KindFunc}, true)
		}
	}
	fillDefs(fs, orig, lines)
	own := ownerMap(len(lines), spans)
	fs.Refs = append(fs.Refs, scanBodies(lines, own, decl, jsCallScanner)...)
	return fs
}

// splitTypeList splits "A, B<T>, C" on top-level commas and drops generics.
func splitTypeList(s string) []string {
	var out []string
	depth, start := 0, 0
	flush := func(end int) {
		t := strings.TrimSpace(s[start:end])
		if i := strings.IndexByte(t, '<'); i >= 0 {
			t = t[:i]
		}
		if t != "" {
			out = append(out, strings.TrimSpace(t))
		}
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
		case ',':
			if depth == 0 {
				flush(i)
				start = i + 1
			}
		}
	}
	flush(len(s))
	return out
}

func lastSeg(s string) string {
	if i := strings.LastIndexAny(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}

func firstQual(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[:i]
	}
	return ""
}

func jsImports(lines []string) []Import {
	var out []Import
	for _, l := range lines {
		if m := jsImportFrom.FindStringSubmatch(l); m != nil {
			imp := Import{Spec: m[2]}
			clause := strings.TrimSpace(m[1])
			if strings.HasPrefix(clause, "*") {
				if i := strings.Index(clause, " as "); i >= 0 {
					imp.Alias = strings.TrimSpace(clause[i+4:])
				}
			} else {
				def := clause
				if i := strings.Index(clause, "{"); i >= 0 {
					def = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(clause[:i]), ","))
					if j := strings.Index(clause, "}"); j > i {
						imp.Names = braceNames(clause[i+1 : j])
					}
				}
				if def != "" && !strings.HasPrefix(def, "{") {
					imp.Alias = def
				}
			}
			out = append(out, imp)
		} else if m := jsImportSide.FindStringSubmatch(l); m != nil {
			out = append(out, Import{Spec: m[1]})
		} else if m := jsRequire.FindStringSubmatch(l); m != nil {
			imp := Import{Spec: m[2]}
			if strings.HasPrefix(m[1], "{") {
				imp.Names = braceNames(strings.Trim(m[1], "{}"))
			} else {
				imp.Alias = m[1]
			}
			out = append(out, imp)
		}
	}
	return out
}

// braceNames parses "a, b as c, type d" into the local names a, c, d. Names are
// stored as "orig=local" only when renamed, so the resolver can find the definition.
func braceNames(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p), "type "))
		if p == "" {
			continue
		}
		if i := strings.Index(p, " as "); i >= 0 {
			out = append(out, strings.TrimSpace(p[:i])+"="+strings.TrimSpace(p[i+4:]))
		} else {
			out = append(out, p)
		}
	}
	return out
}
