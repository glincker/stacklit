package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractOCaml reads open/include, top-level values, types, modules and exceptions.
func extractOCaml(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "ocaml"}
	acc := newTypeAcc()

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "open_module", "include_module":
			if p := childByType(n, lang, "module_path"); p != nil {
				info.Imports = append(info.Imports, nodeText(p, src))
			}
		case "value_definition":
			for _, b := range childrenByType(n, lang, "let_binding") {
				name := childByType(b, lang, "value_name")
				if name == nil {
					continue
				}
				sig := nodeText(name, src)
				for _, p := range childrenByType(b, lang, "parameter") {
					sig += " " + nodeText(p, src)
				}
				info.Exports = append(info.Exports, "let "+truncSig(sig, 100))
			}
		case "value_specification":
			info.Exports = append(info.Exports, truncSig(nodeText(n, src), 120))
		case "type_definition":
			for _, b := range childrenByType(n, lang, "type_binding") {
				name := childByType(b, lang, "type_constructor")
				if name == nil {
					continue
				}
				tn := nodeText(name, src)
				info.Exports = append(info.Exports, "type "+tn)
				acc.add(tn, "")
				ocamlTypeMembers(b, lang, src, tn, acc)
			}
		case "exception_definition":
			if c := childByType(n, lang, "constructor_declaration"); c != nil {
				info.Exports = append(info.Exports, "exception "+truncSig(nodeText(c, src), 80))
			}
		case "module_definition":
			for _, b := range childrenByType(n, lang, "module_binding") {
				if name := childByType(b, lang, "module_name"); name != nil {
					info.Exports = append(info.Exports, "module "+nodeText(name, src))
				}
			}
		case "module_type_definition":
			if name := childByType(n, lang, "module_type_name"); name != nil {
				info.Exports = append(info.Exports, "module type "+nodeText(name, src))
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func ocamlTypeMembers(b *gts.Node, lang *gts.Language, src []byte, name string, acc *typeAcc) {
	if v := childByType(b, lang, "variant_declaration"); v != nil {
		for _, c := range childrenByType(v, lang, "constructor_declaration") {
			acc.add(name, strings.TrimSpace(nodeText(c, src)))
		}
	}
	if r := childByType(b, lang, "record_declaration"); r != nil {
		for _, f := range childrenByType(r, lang, "field_declaration") {
			acc.add(name, truncSig(nodeText(f, src), 60))
		}
	}
}
