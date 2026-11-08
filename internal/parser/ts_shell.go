package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractShell extracts sourced files and function definitions from a shell script.
func extractShell(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "shell"}

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "command":
			cn := childByType(n, lang, "command_name")
			if cn == nil {
				return gts.WalkContinue
			}
			if name := nodeText(cn, src); name == "source" || name == "." {
				kids := namedKids(n)
				if len(kids) > 1 {
					info.Imports = append(info.Imports, stripQuotes(nodeText(kids[1], src)))
				}
			}
		case "function_definition":
			// Underscore-prefixed names follow the private-helper convention.
			if w := childByType(n, lang, "word"); w != nil {
				if name := nodeText(w, src); !strings.HasPrefix(name, "_") {
					info.Exports = append(info.Exports, name+"()")
				}
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}
