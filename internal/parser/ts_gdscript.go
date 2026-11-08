package parser

import (
	"path/filepath"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractGDScript extracts class_name, public members and preload/load paths from a GDScript file.
func extractGDScript(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "gdscript"}
	acc := newTypeAcc()

	owner := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, n := range namedKids(root) {
		if n.Type(lang) == "class_name_statement" {
			if nm := childByType(n, lang, "name"); nm != nil {
				owner = nodeText(nm, src)
				info.Exports = append(info.Exports, owner)
			}
		}
	}
	acc.add(owner, "")

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		if n.Type(lang) == "call" {
			if id := childByType(n, lang, "identifier"); id != nil {
				if t := nodeText(id, src); t == "preload" || t == "load" {
					if a := childByType(n, lang, "arguments"); a != nil {
						info.Imports = append(info.Imports, stripQuotes(strings.Trim(nodeText(a, src), "()")))
					}
				}
			}
		}
		return gts.WalkContinue
	})

	gdMembers(root, lang, src, owner, acc, info, true)
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func gdMembers(parent *gts.Node, lang *gts.Language, src []byte, owner string, acc *typeAcc, info *FileInfo, top bool) {
	for _, n := range namedKids(parent) {
		name := ""
		if nm := childByType(n, lang, "name"); nm != nil {
			name = nodeText(nm, src)
		}
		switch n.Type(lang) {
		case "extends_statement":
			if s := childByType(n, lang, "string"); s != nil {
				info.Imports = append(info.Imports, stripQuotes(nodeText(s, src)))
			} else if t := childByType(n, lang, "type"); t != nil {
				acc.add(owner, "extends "+nodeText(t, src))
			}
		case "function_definition":
			// Underscore methods are engine callbacks or private; keep the API surface clean.
			if name == "" || strings.HasPrefix(name, "_") {
				continue
			}
			sig := truncSig(strings.TrimSuffix(strings.SplitN(nodeText(n, src), "\n", 2)[0], ":"), 100)
			acc.add(owner, sig)
			if top {
				info.Exports = append(info.Exports, sig)
			}
		case "signal_statement":
			acc.add(owner, "signal "+name)
			if top {
				info.Exports = append(info.Exports, "signal "+name)
			}
		case "const_statement", "enum_definition":
			if name != "" && !strings.HasPrefix(name, "_") {
				acc.add(owner, name)
				if top {
					info.Exports = append(info.Exports, name)
				}
			}
		case "variable_statement":
			if ann := childByType(n, lang, "annotations"); ann != nil && strings.Contains(nodeText(ann, src), "export") {
				acc.add(owner, "@export "+name)
			}
		case "class_definition":
			if name == "" {
				continue
			}
			full := owner + "." + name
			acc.add(full, "")
			if top {
				info.Exports = append(info.Exports, full)
			}
			if body := childByType(n, lang, "class_body"); body != nil {
				gdMembers(body, lang, src, full, acc, info, false)
			}
		}
	}
}
