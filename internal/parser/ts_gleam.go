package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractGleam reads imports and pub functions, types and constants.
func extractGleam(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "gleam"}
	acc := newTypeAcc()

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "import":
			if m := childByType(n, lang, "module"); m != nil {
				info.Imports = append(info.Imports, nodeText(m, src))
			}
			continue
		}
		if childByType(n, lang, "visibility_modifier") == nil {
			continue
		}
		switch n.Type(lang) {
		case "function":
			info.Exports = append(info.Exports, truncSig("pub "+sigBeforeBody(strings.TrimPrefix(nodeText(n, src), "pub ")), 110))
		case "type_definition":
			if tn := childByType(n, lang, "type_name"); tn != nil {
				name := nodeText(tn, src)
				info.Exports = append(info.Exports, "type "+name)
				acc.add(name, "")
				if dc := childByType(n, lang, "data_constructors"); dc != nil {
					for _, c := range namedKids(dc) {
						acc.add(name, truncSig(nodeText(c, src), 60))
					}
				}
			}
		case "constant":
			if id := childByType(n, lang, "identifier"); id != nil {
				info.Exports = append(info.Exports, "const "+nodeText(id, src))
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
