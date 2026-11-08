package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractZig extracts @import targets, pub functions and pub container types from a Zig file.
func extractZig(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "zig"}
	acc := newTypeAcc()

	for _, decl := range childrenByType(root, lang, "Decl") {
		public := zigIsPub(decl, lang)

		// @import("x") may appear anywhere in a top-level const initializer.
		if vd := childByType(decl, lang, "VarDecl"); vd != nil {
			gts.Walk(vd, func(n *gts.Node, _ int) gts.WalkAction {
				if n.Type(lang) == "BUILTINIDENTIFIER" && nodeText(n, src) == "@import" {
					if p := n.Parent(); p != nil {
						if s := zigImportArg(nodeText(p, src)); s != "" {
							info.Imports = append(info.Imports, s)
						}
					}
				}
				return gts.WalkContinue
			})
		}

		if fn := childByType(decl, lang, "FnProto"); fn != nil {
			if public {
				info.Exports = append(info.Exports, truncSig(nodeText(fn, src), 100))
			}
			continue
		}
		vd := childByType(decl, lang, "VarDecl")
		if vd == nil || !public {
			continue
		}
		id := childByType(vd, lang, "IDENTIFIER")
		if id == nil {
			continue
		}
		name := nodeText(id, src)
		vtext := nodeText(vd, src)
		kind := ""
		for _, k := range []string{"struct", "enum", "union", "opaque"} {
			if strings.Contains(vtext, "= "+k) || strings.Contains(vtext, "= packed "+k) || strings.Contains(vtext, "= extern "+k) {
				kind = k
				break
			}
		}
		if kind == "" {
			// Public constant: export only when the value is not an @import alias.
			if !strings.Contains(vtext, "@import") {
				info.Exports = append(info.Exports, truncSig(strings.TrimSuffix(vtext, ";"), 80))
			}
			continue
		}
		info.Exports = append(info.Exports, name)
		acc.add(name, "")
		// Member fns of the container: nested FnProto nodes with a pub Decl parent.
		gts.Walk(vd, func(n *gts.Node, _ int) gts.WalkAction {
			if n.Type(lang) == "Decl" && zigIsPub(n, lang) {
				if fn := childByType(n, lang, "FnProto"); fn != nil {
					acc.add(name, truncSig(nodeText(fn, src), 80))
				}
			}
			return gts.WalkContinue
		})
		acc.add(name, "("+kind+")")
	}

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

// zigImportArg pulls the string out of `@import("x")`.
func zigImportArg(call string) string {
	i := strings.Index(call, "(")
	j := strings.LastIndex(call, ")")
	if i < 0 || j <= i {
		return ""
	}
	return stripQuotes(call[i+1 : j])
}

// zigIsPub reports whether a Decl is preceded by a `pub` token (the grammar emits it as a sibling).
func zigIsPub(decl *gts.Node, lang *gts.Language) bool {
	prev := decl.PrevSibling()
	return prev != nil && prev.Type(lang) == "pub"
}
