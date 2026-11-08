package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractJulia reads using/import, exported functions, structs and constants.
func extractJulia(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "julia"}
	acc := newTypeAcc()
	exported := map[string]bool{}
	type sym struct{ name, export string }
	var syms []sym

	var visit func(block *gts.Node)
	visit = func(block *gts.Node) {
		for _, n := range namedKids(block) {
			switch n.Type(lang) {
			case "module_definition":
				if id := childByType(n, lang, "identifier"); id != nil {
					syms = append(syms, sym{nodeText(id, src), "module " + nodeText(id, src)})
				}
				if b := childByType(n, lang, "block"); b != nil {
					visit(b)
				}
			case "using_statement", "import_statement":
				for _, c := range namedKids(n) {
					switch c.Type(lang) {
					case "identifier":
						info.Imports = append(info.Imports, nodeText(c, src))
					case "selected_import":
						if k := namedKids(c); len(k) > 0 {
							info.Imports = append(info.Imports, nodeText(k[0], src))
						}
					}
				}
			case "export_statement":
				for _, c := range namedKids(n) {
					exported[nodeText(c, src)] = true
				}
			case "struct_definition", "abstract_definition":
				head := childByType(n, lang, "type_head")
				if head == nil {
					continue
				}
				name := strings.Fields(strings.ReplaceAll(nodeText(head, src), "<:", " "))
				if len(name) == 0 {
					continue
				}
				syms = append(syms, sym{name[0], truncSig(nodeText(head, src), 80)})
				acc.add(name[0], "")
				if b := childByType(n, lang, "block"); b != nil {
					for _, f := range namedKids(b) {
						acc.add(name[0], truncSig(nodeText(f, src), 50))
					}
				}
			case "function_definition":
				if s := childByType(n, lang, "signature"); s != nil {
					text := truncSig(nodeText(s, src), 100)
					syms = append(syms, sym{juliaFuncName(text), "function " + text})
				}
			case "assignment":
				if k := namedKids(n); len(k) > 0 && k[0].Type(lang) == "call_expression" {
					text := truncSig(nodeText(k[0], src), 100)
					syms = append(syms, sym{juliaFuncName(text), "function " + text})
				}
			case "const_statement":
				if a := childByType(n, lang, "assignment"); a != nil {
					if k := namedKids(a); len(k) > 0 {
						syms = append(syms, sym{nodeText(k[0], src), "const " + nodeText(k[0], src)})
					}
				}
			}
		}
	}
	visit(root)

	for _, s := range syms {
		if len(exported) == 0 || exported[s.name] || strings.HasPrefix(s.export, "module ") {
			info.Exports = append(info.Exports, s.export)
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	// Hide types the export list left out.
	for name := range info.TypeDefs {
		if len(exported) > 0 && !exported[name] {
			delete(info.TypeDefs, name)
		}
	}
	if len(info.TypeDefs) == 0 {
		info.TypeDefs = nil
	}
	return info
}

func juliaFuncName(sig string) string {
	if i := strings.IndexAny(sig, "({"); i >= 0 {
		return strings.TrimSpace(sig[:i])
	}
	return sig
}
