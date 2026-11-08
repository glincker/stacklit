package parser

import (
	"fmt"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractErlang reads includes, exported functions as name/arity, records and types.
func extractErlang(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "erlang"}
	acc := newTypeAcc()
	exported := map[string]bool{}
	seen := map[string]bool{}

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "module_attribute":
			if a := childByType(n, lang, "atom"); a != nil {
				info.Exports = append(info.Exports, "module "+nodeText(a, src))
			}
		case "export_attribute":
			for _, fa := range childrenByType(n, lang, "fa") {
				exported[strings.Join(strings.Fields(nodeText(fa, src)), "")] = true
			}
		case "pp_include", "pp_include_lib":
			if s := childByType(n, lang, "string"); s != nil {
				info.Imports = append(info.Imports, stripQuotes(nodeText(s, src)))
			}
		case "import_attribute":
			if a := childByType(n, lang, "atom"); a != nil {
				info.Imports = append(info.Imports, nodeText(a, src))
			}
		case "record_decl":
			if a := childByType(n, lang, "atom"); a != nil {
				name := nodeText(a, src)
				info.Exports = append(info.Exports, "record "+name)
				acc.add(name, "")
				for _, f := range childrenByType(n, lang, "record_field") {
					acc.add(name, nodeText(f, src))
				}
			}
		case "type_alias", "opaque":
			if tn := childByType(n, lang, "type_name"); tn != nil {
				if a := childByType(tn, lang, "atom"); a != nil {
					info.Exports = append(info.Exports, "type "+nodeText(a, src))
				}
			}
		case "fun_decl":
			clause := childByType(n, lang, "function_clause")
			if clause == nil {
				continue
			}
			a := childByType(clause, lang, "atom")
			args := childByType(clause, lang, "expr_args")
			if a == nil {
				continue
			}
			arity := 0
			if args != nil {
				arity = len(namedKids(args))
			}
			key := fmt.Sprintf("%s/%d", nodeText(a, src), arity)
			if exported[key] && !seen[key] {
				seen[key] = true
				info.Exports = append(info.Exports, key)
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
