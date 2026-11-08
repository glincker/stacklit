package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractSolidity reads imports, contracts, interfaces, libraries and their public surface.
func extractSolidity(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "solidity"}
	acc := newTypeAcc()

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "import_directive":
			if ss := childrenByType(n, lang, "string"); len(ss) > 0 {
				info.Imports = append(info.Imports, stripQuotes(nodeText(ss[len(ss)-1], src)))
			}
		case "contract_declaration", "interface_declaration", "library_declaration":
			id := childByType(n, lang, "identifier")
			if id == nil {
				continue
			}
			name := nodeText(id, src)
			kind := strings.TrimSuffix(n.Type(lang), "_declaration")
			info.Exports = append(info.Exports, kind+" "+name)
			acc.add(name, "")
			body := childByType(n, lang, "contract_body")
			if body == nil {
				continue
			}
			for _, m := range namedKids(body) {
				switch m.Type(lang) {
				case "function_definition":
					v := childByType(m, lang, "visibility")
					// Interface members carry no visibility; they are external by definition.
					if v != nil && (nodeText(v, src) == "internal" || nodeText(v, src) == "private") {
						continue
					}
					acc.add(name, truncSig("function "+sigBeforeBody(nodeText(m, src)), 110))
				case "event_definition", "struct_declaration", "enum_declaration", "error_declaration", "modifier_definition":
					acc.add(name, truncSig(sigBeforeBody(nodeText(m, src)), 90))
				case "state_variable_declaration":
					if v := childByType(m, lang, "visibility"); v != nil && nodeText(v, src) == "public" {
						acc.add(name, truncSig(sigBeforeBody(nodeText(m, src)), 70))
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
