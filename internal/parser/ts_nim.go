package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// nimExportedName returns the identifier of an exported_symbol child, or "" when unexported.
func nimExportedName(n *gts.Node, lang *gts.Language, src []byte) string {
	if e := childByType(n, lang, "exported_symbol"); e != nil {
		if id := childByType(e, lang, "identifier"); id != nil {
			return nodeText(id, src)
		}
	}
	return ""
}

// extractNim reads imports and star-exported procs, types and constants.
func extractNim(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "nim"}
	acc := newTypeAcc()

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "import_statement", "include_statement", "export_statement":
			if n.Type(lang) == "export_statement" {
				continue
			}
			if l := childByType(n, lang, "expression_list"); l != nil {
				for _, id := range namedKids(l) {
					info.Imports = append(info.Imports, nodeText(id, src))
				}
			}
		case "import_from_statement":
			if id := childByType(n, lang, "identifier"); id != nil {
				info.Imports = append(info.Imports, nodeText(id, src))
			}
		case "proc_declaration", "func_declaration", "method_declaration",
			"template_declaration", "macro_declaration", "iterator_declaration", "converter_declaration":
			name := nimExportedName(n, lang, src)
			if name == "" {
				continue
			}
			kind := strings.TrimSuffix(n.Type(lang), "_declaration")
			sig := sigBeforeBody(nodeText(n, src))
			sig = strings.Replace(sig, name+"*", name, 1)
			sig = strings.TrimPrefix(strings.TrimSpace(sig), kind)
			info.Exports = append(info.Exports, truncSig(kind+" "+strings.TrimSpace(sig), 110))
		case "type_section":
			for _, td := range childrenByType(n, lang, "type_declaration") {
				decl := childByType(td, lang, "type_symbol_declaration")
				if decl == nil {
					continue
				}
				name := nimExportedName(decl, lang, src)
				if name == "" {
					continue
				}
				info.Exports = append(info.Exports, "type "+name)
				acc.add(name, "")
				nimMembers(td, lang, src, name, acc)
			}
		case "const_section", "let_section", "var_section":
			for _, v := range childrenByType(n, lang, "variable_declaration") {
				if l := childByType(v, lang, "symbol_declaration_list"); l != nil {
					for _, s := range childrenByType(l, lang, "symbol_declaration") {
						if name := nimExportedName(s, lang, src); name != "" {
							info.Exports = append(info.Exports, strings.TrimSuffix(n.Type(lang), "_section")+" "+name)
						}
					}
				}
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func nimMembers(td *gts.Node, lang *gts.Language, src []byte, name string, acc *typeAcc) {
	if e := childByType(td, lang, "enum_declaration"); e != nil {
		for _, f := range childrenByType(e, lang, "enum_field_declaration") {
			acc.add(name, strings.TrimSpace(nodeText(f, src)))
		}
	}
	if o := childByType(td, lang, "object_declaration"); o != nil {
		if l := childByType(o, lang, "field_declaration_list"); l != nil {
			for _, f := range childrenByType(l, lang, "field_declaration") {
				acc.add(name, truncSig(nodeText(f, src), 50))
			}
		}
	}
}
