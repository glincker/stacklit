package parser

import (
	"regexp"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

var (
	groovyTopFunc = regexp.MustCompile(`(?m)^(?:def|void|[A-Za-z_][\w<>\[\]]*)\s+([a-z_]\w*)\s*\(([^)]*)\)\s*\{`)
	groovyEnum    = regexp.MustCompile(`(?m)^(?:public\s+)?enum\s+([A-Za-z_]\w*)`)
)

// extractGroovy reads imports and type declarations. The grammar often wraps
// whole files in ERROR nodes, so top-level defs and enums fall back to line patterns.
func extractGroovy(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "groovy"}
	acc := newTypeAcc()

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "groovy_import":
			if q := childByType(n, lang, "qualified_name"); q != nil {
				info.Imports = append(info.Imports, nodeText(q, src))
			}
		case "class_definition":
			id := childByType(n, lang, "identifier")
			if id == nil {
				return gts.WalkContinue
			}
			name := nodeText(id, src)
			if name == "def" || name == "enum" {
				return gts.WalkSkipChildren
			}
			info.Exports = append(info.Exports, name)
			acc.add(name, "")
			if body := childByType(n, lang, "closure"); body != nil {
				for _, m := range namedKids(body) {
					switch m.Type(lang) {
					case "function_definition", "function_declaration":
						if am := childByType(m, lang, "access_modifier"); am != nil && nodeText(am, src) != "public" {
							continue
						}
						sig := nodeText(m, src)
						if i := strings.Index(sig, "{"); i >= 0 {
							sig = sig[:i]
						}
						acc.add(name, truncSig(sig, 80))
					}
				}
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})

	text := string(src)
	for _, m := range groovyTopFunc.FindAllStringSubmatch(text, -1) {
		info.Exports = append(info.Exports, "def "+m[1]+"("+strings.TrimSpace(m[2])+")")
	}
	for _, m := range groovyEnum.FindAllStringSubmatch(text, -1) {
		info.Exports = append(info.Exports, m[1])
		acc.add(m[1], "")
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
