package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractDart extracts imports/exports, public declarations and class members from a Dart file.
func extractDart(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "dart"}
	acc := newTypeAcc()

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "import_or_export":
			if u := dartFirst(n, lang, "configurable_uri"); u != nil {
				info.Imports = append(info.Imports, stripQuotes(nodeText(u, src)))
			}
		case "class_definition", "mixin_declaration", "enum_declaration", "extension_declaration":
			id := childByType(n, lang, "identifier")
			if id == nil {
				continue
			}
			name := nodeText(id, src)
			if strings.HasPrefix(name, "_") {
				continue
			}
			info.Exports = append(info.Exports, name)
			acc.add(name, "")
			dartMembers(n, lang, src, name, acc)
		case "type_alias":
			if id := childByType(n, lang, "type_identifier"); id != nil {
				name := nodeText(id, src)
				if !strings.HasPrefix(name, "_") {
					info.Exports = append(info.Exports, name)
					acc.add(name, truncSig(nodeText(n, src), 80))
				}
			}
		case "function_signature":
			if id := childByType(n, lang, "identifier"); id != nil && !strings.HasPrefix(nodeText(id, src), "_") {
				info.Exports = append(info.Exports, truncSig(nodeText(n, src), 100))
			}
		}
	}

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func dartFirst(n *gts.Node, lang *gts.Language, kind string) *gts.Node {
	var found *gts.Node
	gts.Walk(n, func(c *gts.Node, _ int) gts.WalkAction {
		if found == nil && c.Type(lang) == kind {
			found = c
		}
		return gts.WalkContinue
	})
	return found
}

// dartMembers records public methods and fields of a class-like body, plus enum constants.
func dartMembers(n *gts.Node, lang *gts.Language, src []byte, owner string, acc *typeAcc) {
	if eb := childByType(n, lang, "enum_body"); eb != nil {
		for _, c := range childrenByType(eb, lang, "enum_constant") {
			acc.add(owner, nodeText(c, src))
		}
	}
	for _, kind := range []string{"class_body", "extension_body"} {
		body := childByType(n, lang, kind)
		if body == nil {
			continue
		}
		for _, m := range namedKids(body) {
			switch m.Type(lang) {
			case "method_signature", "declaration":
				if fs := childByType(m, lang, "function_signature"); fs != nil {
					id := childByType(fs, lang, "identifier")
					if id == nil || strings.HasPrefix(nodeText(id, src), "_") {
						continue
					}
					// Skip unnamed constructors, which repeat the class name.
					if nodeText(id, src) == owner {
						continue
					}
					acc.add(owner, truncSig(nodeText(fs, src), 80))
				} else if l := childByType(m, lang, "initialized_identifier_list"); l != nil {
					if t := nodeText(l, src); !strings.HasPrefix(t, "_") {
						acc.add(owner, t)
					}
				}
			}
		}
	}
}
