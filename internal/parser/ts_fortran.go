package parser

import (
	gts "github.com/odvcencio/gotreesitter"
)

// extractFortran reads use statements, modules, derived types and procedures.
func extractFortran(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "fortran"}
	acc := newTypeAcc()

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "use_statement":
			if m := childByType(n, lang, "module_name"); m != nil {
				info.Imports = append(info.Imports, nodeText(m, src))
			}
		case "module_statement":
			if nm := childByType(n, lang, "name"); nm != nil {
				info.Exports = append(info.Exports, "module "+nodeText(nm, src))
			}
		case "function_statement", "subroutine_statement":
			nm := childByType(n, lang, "name")
			if nm == nil {
				return gts.WalkContinue
			}
			kind := "function "
			if n.Type(lang) == "subroutine_statement" {
				kind = "subroutine "
			}
			sig := kind + nodeText(nm, src) + "("
			if p := childByType(n, lang, "parameters"); p != nil {
				sig = kind + nodeText(nm, src) + nodeText(p, src)
			} else {
				sig += ")"
			}
			info.Exports = append(info.Exports, truncSig(sig, 100))
		case "derived_type_definition":
			if st := childByType(n, lang, "derived_type_statement"); st != nil {
				if tn := childByType(st, lang, "type_name"); tn != nil {
					name := nodeText(tn, src)
					info.Exports = append(info.Exports, "type "+name)
					acc.add(name, "")
					for _, v := range childrenByType(n, lang, "variable_declaration") {
						acc.add(name, truncSig(nodeText(v, src), 50))
					}
				}
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
