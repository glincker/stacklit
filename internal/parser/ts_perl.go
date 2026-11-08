package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// perlPragmas are compiler directives, not module dependencies.
var perlPragmas = map[string]bool{
	"strict": true, "warnings": true, "utf8": true, "feature": true, "vars": true,
	"constant": true, "lib": true, "integer": true, "overload": true, "bytes": true,
}

// extractPerl reads use/require, packages and public subs.
func extractPerl(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "perl"}
	acc := newTypeAcc()
	current := ""

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "package_statement":
			// The keyword shares its node type with the name, so take the named child.
			if k := namedKids(n); len(k) > 0 {
				current = nodeText(k[0], src)
				info.Exports = append(info.Exports, "package "+current)
				acc.add(current, "")
			}
		case "use_statement":
			p := childByType(n, lang, "package")
			if p == nil {
				continue
			}
			name := nodeText(p, src)
			if name == "parent" || name == "base" {
				if s := childByType(n, lang, "string_literal"); s != nil {
					info.Imports = append(info.Imports, stripQuotes(nodeText(s, src)))
				}
				continue
			}
			if !perlPragmas[name] {
				info.Imports = append(info.Imports, name)
			}
		case "expression_statement":
			if r := childByType(n, lang, "require_expression"); r != nil {
				if b := childByType(r, lang, "bareword"); b != nil {
					info.Imports = append(info.Imports, nodeText(b, src))
				}
			}
		case "subroutine_declaration_statement":
			b := childByType(n, lang, "bareword")
			if b == nil {
				continue
			}
			name := nodeText(b, src)
			if strings.HasPrefix(name, "_") {
				continue
			}
			info.Exports = append(info.Exports, "sub "+name)
			if current != "" {
				acc.add(current, "sub "+name)
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
