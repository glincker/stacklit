package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractLua extracts require() calls and non-local functions from a Lua file.
func extractLua(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "lua"}

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "function_call":
			if id := childByType(n, lang, "identifier"); id != nil && nodeText(id, src) == "require" {
				if a := childByType(n, lang, "arguments"); a != nil {
					t := strings.Trim(strings.TrimSpace(nodeText(a, src)), "()")
					info.Imports = append(info.Imports, stripQuotes(t))
				}
			}
		}
		return gts.WalkContinue
	})

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "function_declaration":
			if strings.HasPrefix(nodeText(n, src), "local ") {
				continue
			}
			name := namedKids(n)
			params := childByType(n, lang, "parameters")
			if len(name) == 0 || params == nil {
				continue
			}
			info.Exports = append(info.Exports, truncSig(nodeText(name[0], src)+nodeText(params, src), 100))
		case "assignment_statement":
			vl := childByType(n, lang, "variable_list")
			el := childByType(n, lang, "expression_list")
			if vl == nil || el == nil {
				continue
			}
			fn := childByType(el, lang, "function_definition")
			if fn == nil {
				continue
			}
			if params := childByType(fn, lang, "parameters"); params != nil {
				info.Exports = append(info.Exports, truncSig(nodeText(vl, src)+nodeText(params, src), 100))
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}
