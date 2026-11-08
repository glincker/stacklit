package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractPowerShell reads module imports, dot-sourced scripts, functions and classes.
func extractPowerShell(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "powershell"}
	acc := newTypeAcc()

	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "command":
			cn := childByType(n, lang, "command_name")
			if cn != nil {
				name := strings.ToLower(nodeText(cn, src))
				if name == "import-module" || name == "using" {
					if el := childByType(n, lang, "command_elements"); el != nil {
						toks := childrenByType(el, lang, "generic_token")
						if len(toks) > 0 {
							info.Imports = append(info.Imports, nodeText(toks[len(toks)-1], src))
						}
					}
				}
				return gts.WalkContinue
			}
			// Dot-sourcing (". ./file.ps1") has no command_name at the top.
			if childByType(n, lang, "command_invokation_operator") != nil {
				if e := childByType(n, lang, "command_name_expr"); e != nil {
					info.Imports = append(info.Imports, strings.TrimSpace(nodeText(e, src)))
				}
			}
		case "function_statement":
			if fn := childByType(n, lang, "function_name"); fn != nil {
				sig := "function " + nodeText(fn, src)
				if sb := childByType(n, lang, "script_block"); sb != nil {
					if pb := childByType(sb, lang, "param_block"); pb != nil {
						sig += " " + nodeText(pb, src)
					}
				}
				info.Exports = append(info.Exports, truncSig(sig, 110))
			}
			return gts.WalkSkipChildren
		case "class_statement":
			if k := namedKids(n); len(k) > 0 {
				name := nodeText(k[0], src)
				info.Exports = append(info.Exports, "class "+name)
				acc.add(name, "")
				for _, m := range childrenByType(n, lang, "class_method_definition") {
					acc.add(name, truncSig(sigBeforeBody(nodeText(m, src)), 70))
				}
				for _, p := range childrenByType(n, lang, "class_property_definition") {
					acc.add(name, truncSig(nodeText(p, src), 50))
				}
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
