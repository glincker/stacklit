package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractElixir extracts directives, public defs and module types from an Elixir file.
func extractElixir(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "elixir"}
	acc := newTypeAcc()

	var visit func(n *gts.Node, module string)
	visit = func(n *gts.Node, module string) {
		switch n.Type(lang) {
		case "unary_operator":
			// Module attributes such as @type, @callback and @opaque.
			if call := childByType(n, lang, "call"); call != nil && module != "" {
				exElixirAttr(call, lang, src, module, acc)
			}
			return
		case "call":
		default:
			for _, c := range namedKids(n) {
				visit(c, module)
			}
			return
		}

		kw := childByType(n, lang, "identifier")
		args := childByType(n, lang, "arguments")
		block := childByType(n, lang, "do_block")
		if kw == nil {
			return
		}
		switch nodeText(kw, src) {
		case "defmodule", "defprotocol", "defimpl":
			name := ""
			if args != nil {
				if a := childByType(args, lang, "alias"); a != nil {
					name = nodeText(a, src)
				}
			}
			if name == "" || block == nil {
				return
			}
			full := name
			if module != "" && nodeText(kw, src) == "defmodule" && !strings.Contains(name, ".") {
				full = module + "." + name
			}
			if nodeText(kw, src) != "defimpl" {
				info.Exports = append(info.Exports, full)
				acc.add(full, "")
			}
			for _, c := range namedKids(block) {
				visit(c, full)
			}
		case "import", "alias", "require", "use":
			if args != nil {
				info.Imports = append(info.Imports, exDirectiveTargets(args, lang, src)...)
			}
		case "def", "defmacro", "defdelegate", "defguard":
			if args == nil || module == "" {
				return
			}
			sig := exHead(args, lang, src)
			if sig == "" {
				return
			}
			full := truncSig(nodeText(kw, src)+" "+sig, 100)
			info.Exports = append(info.Exports, module+"."+sig)
			acc.add(module, full)
		case "defstruct":
			if args != nil && module != "" {
				acc.add(module, truncSig("defstruct "+nodeText(args, src), 100))
			}
		}
	}

	for _, c := range namedKids(root) {
		visit(c, "")
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

// exHead returns the function head (name and params) from a def's arguments, dropping guards.
func exHead(args *gts.Node, lang *gts.Language, src []byte) string {
	kids := namedKids(args)
	if len(kids) == 0 {
		return ""
	}
	h := kids[0]
	if h.Type(lang) == "binary_operator" { // `name(args) when guard`
		if k := namedKids(h); len(k) > 0 {
			h = k[0]
		}
	}
	return truncSig(nodeText(h, src), 80)
}

// exDirectiveTargets expands `alias A.{B, C}` into A.B and A.C; other forms keep their module name.
func exDirectiveTargets(args *gts.Node, lang *gts.Language, src []byte) []string {
	kids := namedKids(args)
	if len(kids) == 0 {
		return nil
	}
	first := kids[0]
	switch first.Type(lang) {
	case "alias":
		return []string{nodeText(first, src)}
	case "dot":
		parts := namedKids(first)
		if len(parts) == 2 && parts[1].Type(lang) == "tuple" {
			var out []string
			for _, m := range namedKids(parts[1]) {
				out = append(out, nodeText(parts[0], src)+"."+nodeText(m, src))
			}
			return out
		}
		return []string{nodeText(first, src)}
	}
	return nil
}

// exElixirAttr records @type, @opaque and @callback names as members of the module.
func exElixirAttr(call *gts.Node, lang *gts.Language, src []byte, module string, acc *typeAcc) {
	kw := childByType(call, lang, "identifier")
	args := childByType(call, lang, "arguments")
	if kw == nil || args == nil {
		return
	}
	k := nodeText(kw, src)
	if k != "type" && k != "opaque" && k != "callback" && k != "macrocallback" {
		return
	}
	kids := namedKids(args)
	if len(kids) == 0 {
		return
	}
	h := kids[0]
	if h.Type(lang) == "binary_operator" {
		if s := namedKids(h); len(s) > 0 {
			h = s[0]
		}
	}
	if h.Type(lang) == "call" {
		if id := childByType(h, lang, "identifier"); id != nil {
			h = id
		}
	}
	acc.add(module, "@"+k+" "+nodeText(h, src))
}
