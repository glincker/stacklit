package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractKotlin extracts imports, exports, and type definitions from a Kotlin source file.
func extractKotlin(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{
		Language: "kotlin",
		TypeDefs: make(map[string]string),
	}

	type classEntry struct {
		name    string
		members []string
	}
	var classes []classEntry

	gts.Walk(root, func(node *gts.Node, depth int) gts.WalkAction {
		switch node.Type(lang) {
		case "import_header":
			// import foo.bar.Baz or import foo.bar.*
			text := strings.TrimSpace(nodeText(node, src))
			text = strings.TrimPrefix(text, "import")
			text = strings.TrimSpace(text)
			if text != "" {
				info.Imports = append(info.Imports, text)
			}
			return gts.WalkSkipChildren

		case "import_list":
			// Some grammars wrap import_header nodes in an import_list.
			// Walk into it and let the import_header case handle children.
			return gts.WalkContinue

		case "class_declaration", "object_declaration":
			// Include all classes/objects (public by default in Kotlin).
			// Skip if explicitly private or internal.
			if ktHasModifier(node, lang, src, "private") {
				return gts.WalkSkipChildren
			}
			name := ktDeclName(node, lang, src)
			if name == "" {
				return gts.WalkContinue
			}
			info.Exports = append(info.Exports, name)
			members := ktClassMembers(node, lang, src)
			classes = append(classes, classEntry{name: name, members: members})
			return gts.WalkSkipChildren

		case "infix_expression":
			// The grammar parses `object Name { ... }` as an infix expression.
			if depth <= 1 && childByType(node, lang, "object_literal") != nil {
				if id := childByType(node, lang, "simple_identifier"); id != nil {
					name := nodeText(id, src)
					info.Exports = append(info.Exports, name)
					var members []string
					if lam := childByType(node, lang, "lambda_literal"); lam != nil {
						gts.Walk(lam, func(c *gts.Node, d int) gts.WalkAction {
							if c.Type(lang) == "function_declaration" {
								if !ktHasModifier(c, lang, src, "private") {
									members = append(members, ktFunctionSig(c, lang, src))
								}
								return gts.WalkSkipChildren
							}
							return gts.WalkContinue
						})
					}
					classes = append(classes, classEntry{name: name, members: members})
				}
				return gts.WalkSkipChildren
			}
			return gts.WalkContinue

		case "function_declaration":
			// Top-level functions (not inside a class).
			if depth > 2 {
				// Likely nested; handled by ktClassMembers.
				return gts.WalkSkipChildren
			}
			if ktHasModifier(node, lang, src, "private") {
				return gts.WalkSkipChildren
			}
			sig := ktFunctionSig(node, lang, src)
			if sig != "" {
				info.Exports = append(info.Exports, sig)
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)

	for _, ce := range classes {
		if len(ce.members) > 0 {
			info.TypeDefs[ce.name] = strings.Join(ce.members, "; ")
		} else {
			info.TypeDefs[ce.name] = ce.name
		}
	}

	if len(info.TypeDefs) == 0 {
		info.TypeDefs = nil
	}

	return info
}

// ktHasModifier checks if a node has a specific modifier keyword.
func ktHasModifier(node *gts.Node, lang *gts.Language, src []byte, mod string) bool {
	modifiers := childByType(node, lang, "modifiers")
	if modifiers == nil {
		return false
	}
	for i := 0; i < modifiers.ChildCount(); i++ {
		child := modifiers.Child(i)
		if nodeText(child, src) == mod {
			return true
		}
	}
	return false
}

// ktDeclName extracts the simple name from a class/object declaration.
func ktDeclName(node *gts.Node, lang *gts.Language, src []byte) string {
	id := childByType(node, lang, "type_identifier")
	if id != nil {
		return nodeText(id, src)
	}
	id = childByType(node, lang, "simple_identifier")
	if id != nil {
		return nodeText(id, src)
	}
	return ""
}

// ktClassMembers collects public method and property names from a class/object body.
func ktClassMembers(node *gts.Node, lang *gts.Language, src []byte) []string {
	var members []string
	if pc := childByType(node, lang, "primary_constructor"); pc != nil {
		members = append(members, truncSig(nodeText(pc, src), 80))
	}
	if eb := childByType(node, lang, "enum_class_body"); eb != nil {
		for _, e := range childrenByType(eb, lang, "enum_entry") {
			members = append(members, nodeText(e, src))
		}
	}
	body := childByType(node, lang, "class_body")
	if body == nil {
		return members
	}
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type(lang) {
		case "function_declaration":
			if ktHasModifier(child, lang, src, "private") {
				continue
			}
			sig := ktFunctionSig(child, lang, src)
			if sig != "" {
				members = append(members, sig)
			}
		case "property_declaration":
			if ktHasModifier(child, lang, src, "private") {
				continue
			}
			if vd := childByType(child, lang, "variable_declaration"); vd != nil {
				if name := childByType(vd, lang, "simple_identifier"); name != nil {
					members = append(members, nodeText(name, src))
				}
			}
		}
	}
	return members
}

// ktFunctionSig returns the declaration text from `fun` up to the body, so generics,
// receivers and return types are kept verbatim.
func ktFunctionSig(node *gts.Node, lang *gts.Language, src []byte) string {
	start := node.StartByte()
	for i := 0; i < node.ChildCount(); i++ {
		if node.Child(i).Type(lang) == "fun" {
			start = node.Child(i).StartByte()
			break
		}
	}
	end := node.EndByte()
	if body := childByType(node, lang, "function_body"); body != nil {
		end = body.StartByte()
	}
	if end > uint32(len(src)) || start >= end {
		return ""
	}
	return truncSig(string(src[start:end]), 100)
}
