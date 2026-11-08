package graph

import (
	"regexp"
	"strings"
)

const javaMods = `(?:(?:public|protected|private|abstract|final|static|sealed|non-sealed|strictfp|synchronized|native|default)\s+)*`

var (
	javaTypeRe = regexp.MustCompile(`^(?:@\w+(?:\([^)]*\))?\s*)*` + javaMods + `(class|interface|enum|record|@interface)\s+([A-Za-z_]\w*)(?:\s*<[^{]*?>)?(?:\s*\([^)]*\))?(?:\s+extends\s+([^{]+?))?(?:\s+implements\s+([^{]+?))?(?:\s+permits\s+[^{]+?)?\s*\{?\s*$`)
	javaPkgRe  = regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)
	javaImpRe  = regexp.MustCompile(`^\s*import\s+(static\s+)?([\w.]+?)(\.\*)?\s*;`)
	javaMethRe = regexp.MustCompile(`^(?:@\w+(?:\([^)]*\))?\s*)*` + javaMods + `(?:<[^>]+>\s+)?[\w.<>\[\]?,]+(?:\s*\[\s*\])*\s+([A-Za-z_]\w*)\s*\(`)
	javaCtorRe = regexp.MustCompile(`^(?:@\w+(?:\([^)]*\))?\s*)*` + javaMods + `([A-Z]\w*)\s*\(`)

	javaScanner = &refScanner{
		re:       regexp.MustCompile(`((?:[A-Za-z_]\w*\s*\.\s*)*)([A-Za-z_]\w*)\s*(?:<[^<>()]*>)?\s*\(`),
		keywords: wordSet("if for while switch catch return synchronized throw else do try assert super this instanceof"),
		builtins: wordSet("String Integer Long Double Boolean Object Math"),
	}
	javaStmtKeywords = wordSet("return new throw else if for while switch catch try assert do case yield")
)

func extractJava(file string, src []byte) *FileSyms {
	fs := &FileSyms{Path: file, Lang: "java"}
	orig := splitLines(src)
	lines := splitLines(stripNoise(src, "java"))
	depth := braceDepths(lines)

	for _, l := range orig {
		if m := javaPkgRe.FindStringSubmatch(l); m != nil {
			fs.Pkg = m[1]
		} else if m := javaImpRe.FindStringSubmatch(l); m != nil && m[1] == "" {
			imp := Import{Spec: m[2]}
			if m[3] != "" {
				imp.Spec = m[2] + ".*"
			} else {
				imp.Alias = m[2][strings.LastIndex(m[2], ".")+1:]
			}
			fs.Imports = append(fs.Imports, imp)
		}
	}

	type scope struct {
		name       string
		depth, end int
	}
	var classes []scope
	var spans []span
	decl := map[int]bool{}
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "@") && !strings.Contains(line, " ") {
			continue
		}
		for len(classes) > 0 && i > classes[len(classes)-1].end {
			classes = classes[:len(classes)-1]
		}
		inClass := len(classes) > 0 && depth[i] == classes[len(classes)-1].depth+1
		if depth[i] > 0 && !inClass {
			continue
		}
		if m := javaTypeRe.FindStringSubmatch(line); m != nil {
			kind := KindClass
			switch m[1] {
			case "interface", "@interface":
				kind = KindInterface
			case "enum":
				kind = KindEnum
			}
			d := Def{Name: m[2], Qual: m[2], Kind: kind, Line: i + 1, Sig: sigOf(orig[i])}
			if inClass {
				d.Qual = classes[len(classes)-1].name + "." + m[2]
			}
			fs.Defs = append(fs.Defs, d)
			idx := len(fs.Defs) - 1
			for _, b := range splitTypeList(m[3]) {
				fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeInherits, Name: lastSeg(b), Qual: firstQual(b)})
			}
			for _, b := range splitTypeList(m[4]) {
				fs.Refs = append(fs.Refs, Ref{From: idx, Kind: EdgeImplements, Name: lastSeg(b), Qual: firstQual(b)})
			}
			classes = append(classes, scope{m[2], depth[i], blockEnd(lines, i)})
			continue
		}
		if !inClass {
			continue
		}
		c := classes[len(classes)-1]
		name := ""
		if m := javaCtorRe.FindStringSubmatch(line); m != nil && m[1] == c.name {
			name = "<init>"
		} else if m := javaMethRe.FindStringSubmatch(line); m != nil {
			first := strings.Fields(line)
			if len(first) > 0 && javaStmtKeywords[first[0]] || strings.Contains(line[:strings.Index(line, "(")], "=") || strings.HasPrefix(line, "new ") {
				continue
			}
			name = m[1]
		}
		if name == "" {
			continue
		}
		fs.Defs = append(fs.Defs, Def{Name: name, Qual: c.name + "." + name, Kind: KindMethod, Recv: c.name, Line: i + 1, Sig: sigOf(orig[i])})
		spans = append(spans, span{i, blockEnd(lines, i), len(fs.Defs) - 1})
		decl[i] = true
	}
	fillDefs(fs, orig, lines)
	own := ownerMap(len(lines), spans)
	fs.Refs = append(fs.Refs, scanBodies(lines, own, decl, javaScanner)...)
	return fs
}
