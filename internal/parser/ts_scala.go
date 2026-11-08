package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractScala extracts imports, public types and members from a Scala file.
func extractScala(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "scala"}
	acc := newTypeAcc()

	var visit func(n *gts.Node)
	visit = func(n *gts.Node) {
		for _, c := range namedKids(n) {
			switch c.Type(lang) {
			case "import_declaration":
				info.Imports = append(info.Imports, scalaImports(c, lang, src)...)
			case "package_clause":
				// Block-style packages hold declarations inside.
				if childByType(c, lang, "template_body") != nil {
					visit(childByType(c, lang, "template_body"))
				}
			case "trait_definition", "class_definition", "object_definition", "enum_definition":
				if scalaPrivate(c, lang, src) {
					continue
				}
				id := childByType(c, lang, "identifier")
				if id == nil {
					continue
				}
				name := nodeText(id, src)
				info.Exports = append(info.Exports, name)
				acc.add(name, "")
				if cp := childByType(c, lang, "class_parameters"); cp != nil {
					acc.add(name, nodeText(cp, src))
				}
				if body := childByType(c, lang, "template_body"); body != nil {
					scalaMembers(body, lang, src, name, acc)
				}
				if eb := childByType(c, lang, "enum_body"); eb != nil {
					gts.Walk(eb, func(e *gts.Node, _ int) gts.WalkAction {
						if e.Type(lang) == "simple_enum_case" {
							acc.add(name, nodeText(e, src))
						}
						return gts.WalkContinue
					})
				}
			case "type_definition":
				if id := childByType(c, lang, "type_identifier"); id != nil {
					info.Exports = append(info.Exports, nodeText(id, src))
					acc.add(nodeText(id, src), truncSig(nodeText(c, src), 80))
				}
			case "function_definition", "function_declaration":
				if !scalaPrivate(c, lang, src) {
					info.Exports = append(info.Exports, truncSig(sigBeforeBody(nodeText(c, src)), 100))
				}
			}
		}
	}
	visit(root)

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func scalaPrivate(n *gts.Node, lang *gts.Language, src []byte) bool {
	m := childByType(n, lang, "modifiers")
	if m == nil {
		return false
	}
	t := nodeText(m, src)
	return strings.Contains(t, "private") || strings.Contains(t, "protected")
}

// scalaImports expands `import a.b.{C, D}` into a.b.C and a.b.D.
func scalaImports(n *gts.Node, lang *gts.Language, src []byte) []string {
	var prefix []string
	var sel *gts.Node
	for _, c := range namedKids(n) {
		switch c.Type(lang) {
		case "identifier":
			prefix = append(prefix, nodeText(c, src))
		case "namespace_selectors":
			sel = c
		case "namespace_wildcard":
			prefix = append(prefix, "_")
		}
	}
	base := strings.Join(prefix, ".")
	if sel == nil {
		return []string{base}
	}
	var out []string
	for _, s := range namedKids(sel) {
		out = append(out, base+"."+nodeText(s, src))
	}
	return out
}

func scalaMembers(body *gts.Node, lang *gts.Language, src []byte, owner string, acc *typeAcc) {
	for _, m := range namedKids(body) {
		switch m.Type(lang) {
		case "function_definition", "function_declaration":
			if !scalaPrivate(m, lang, src) {
				acc.add(owner, truncSig(sigBeforeBody(nodeText(m, src)), 80))
			}
		case "val_definition", "var_definition":
			if !scalaPrivate(m, lang, src) {
				acc.add(owner, truncSig(sigBeforeBody(nodeText(m, src)), 60))
			}
		}
	}
}
