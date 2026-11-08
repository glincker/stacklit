package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractR reads library/require/source calls and top-level function and constant assignments.
func extractR(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "r"}

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "call":
			fn := childByType(n, lang, "identifier")
			args := childByType(n, lang, "arguments")
			if fn == nil || args == nil {
				continue
			}
			switch nodeText(fn, src) {
			case "library", "require", "requireNamespace", "source":
				if a := namedKids(args); len(a) > 0 {
					info.Imports = append(info.Imports, stripQuotes(nodeText(a[0], src)))
				}
			}
		case "binary_operator":
			k := namedKids(n)
			if len(k) < 2 || k[0].Type(lang) != "identifier" {
				continue
			}
			name := nodeText(k[0], src)
			// A leading dot marks a private helper by R convention.
			if strings.HasPrefix(name, ".") {
				continue
			}
			if k[1].Type(lang) == "function_definition" {
				sig := name + "()"
				if p := childByType(k[1], lang, "parameters"); p != nil {
					sig = name + nodeText(p, src)
				}
				info.Exports = append(info.Exports, truncSig(sig, 100))
			} else {
				info.Exports = append(info.Exports, name)
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	return info
}
