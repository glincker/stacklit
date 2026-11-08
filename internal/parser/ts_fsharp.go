package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractFSharp reads open declarations, modules, types and non-private lets.
func extractFSharp(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "fsharp"}
	acc := newTypeAcc()

	var visit func(parent *gts.Node)
	visit = func(parent *gts.Node) {
		for _, n := range namedKids(parent) {
			switch n.Type(lang) {
			case "named_module", "namespace", "module_defn":
				if id := childByType(n, lang, "long_identifier"); id != nil {
					info.Exports = append(info.Exports, "module "+nodeText(id, src))
				}
				visit(n)
			case "import_decl":
				if id := childByType(n, lang, "long_identifier"); id != nil {
					info.Imports = append(info.Imports, nodeText(id, src))
				}
			case "type_definition":
				fsType(n, lang, src, info, acc)
			case "declaration_expression":
				def := childByType(n, lang, "function_or_value_defn")
				if def == nil {
					continue
				}
				left := childByType(def, lang, "value_declaration_left")
				if left == nil || childByType(left, lang, "access_modifier") != nil {
					continue
				}
				info.Exports = append(info.Exports, truncSig("let "+strings.TrimSpace(sigBeforeBody(strings.TrimPrefix(nodeText(def, src), "let"))), 110))
			}
		}
	}
	visit(root)

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func fsType(td *gts.Node, lang *gts.Language, src []byte, info *FileInfo, acc *typeAcc) {
	for _, d := range namedKids(td) {
		tn := childByType(d, lang, "type_name")
		if tn == nil {
			continue
		}
		name := nodeText(tn, src)
		info.Exports = append(info.Exports, "type "+name)
		acc.add(name, "")
		for _, c := range namedKids(d) {
			switch c.Type(lang) {
			case "union_type_cases":
				for _, uc := range namedKids(c) {
					acc.add(name, strings.TrimSpace(nodeText(uc, src)))
				}
			case "record_fields":
				for _, f := range namedKids(c) {
					acc.add(name, truncSig(nodeText(f, src), 50))
				}
			case "type_extension_elements":
				for _, m := range namedKids(c) {
					acc.add(name, truncSig(sigBeforeBody(nodeText(m, src)), 80))
				}
			}
		}
	}
}
