package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractSwift extracts imports, exports, and type definitions from a Swift source file.
func extractSwift(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{
		Language: "swift",
		TypeDefs: make(map[string]string),
	}

	type typeEntry struct {
		name    string
		members []string
	}
	var types []typeEntry

	gts.Walk(root, func(node *gts.Node, depth int) gts.WalkAction {
		switch node.Type(lang) {
		case "import_declaration":
			// import Foundation or import UIKit.UIView
			text := strings.TrimSpace(nodeText(node, src))
			text = strings.TrimPrefix(text, "import")
			text = strings.TrimSpace(text)
			if text != "" {
				info.Imports = append(info.Imports, text)
			}
			return gts.WalkSkipChildren

		case "class_declaration", "struct_declaration", "protocol_declaration", "enum_declaration":
			// Only include public/open or unqualified declarations (public by default in a module).
			if swiftHasModifier(node, lang, src, "private") || swiftHasModifier(node, lang, src, "fileprivate") {
				return gts.WalkSkipChildren
			}
			name := swiftDeclName(node, lang, src)
			if name == "" {
				return gts.WalkContinue
			}
			info.Exports = append(info.Exports, name)
			members := swiftTypeMembers(node, lang, src)
			types = append(types, typeEntry{name: name, members: members})
			return gts.WalkSkipChildren

		case "function_declaration":
			// Top-level functions only (depth 1 or 2 in the tree).
			if depth > 3 {
				return gts.WalkSkipChildren
			}
			if swiftHasModifier(node, lang, src, "private") || swiftHasModifier(node, lang, src, "fileprivate") {
				return gts.WalkSkipChildren
			}
			sig := swiftFunctionSig(node, lang, src)
			if sig != "" {
				info.Exports = append(info.Exports, sig)
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)

	for _, te := range types {
		if len(te.members) > 0 {
			info.TypeDefs[te.name] = strings.Join(te.members, "; ")
		} else {
			info.TypeDefs[te.name] = te.name
		}
	}

	if len(info.TypeDefs) == 0 {
		info.TypeDefs = nil
	}

	return info
}

// swiftHasModifier checks if a node carries an access-control keyword (e.g. private).
func swiftHasModifier(node *gts.Node, lang *gts.Language, src []byte, mod string) bool {
	m := childByType(node, lang, "modifiers")
	if m == nil {
		return false
	}
	for _, w := range strings.Fields(nodeText(m, src)) {
		if w == mod {
			return true
		}
	}
	return false
}

// swiftDeclName extracts the type name from a Swift declaration node.
func swiftDeclName(node *gts.Node, lang *gts.Language, src []byte) string {
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

// swiftTypeMembers collects property and function names from a type body.
func swiftTypeMembers(node *gts.Node, lang *gts.Language, src []byte) []string {
	var members []string
	body := childByType(node, lang, "class_body")
	if body == nil {
		body = childByType(node, lang, "struct_body")
	}
	if body == nil {
		body = childByType(node, lang, "protocol_body")
	}
	if body == nil {
		body = childByType(node, lang, "enum_class_body")
	}
	if body == nil {
		return members
	}

	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		switch child.Type(lang) {
		case "function_declaration", "protocol_function_declaration":
			if swiftHasModifier(child, lang, src, "private") || swiftHasModifier(child, lang, src, "fileprivate") {
				continue
			}
			sig := swiftFunctionSig(child, lang, src)
			if sig != "" {
				members = append(members, sig)
			}
		case "property_declaration", "variable_declaration":
			if swiftHasModifier(child, lang, src, "private") || swiftHasModifier(child, lang, src, "fileprivate") {
				continue
			}
			name := swiftPropertyName(child, lang, src)
			if name != "" {
				members = append(members, name)
			}
		}
	}
	return members
}

// swiftFunctionSig returns the declaration from `func` up to the body, keeping
// parameter labels, generics and the return type verbatim.
func swiftFunctionSig(node *gts.Node, lang *gts.Language, src []byte) string {
	start := node.StartByte()
	for i := 0; i < node.ChildCount(); i++ {
		if node.Child(i).Type(lang) == "func" {
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

// swiftPropertyName extracts the name from a property declaration.
func swiftPropertyName(node *gts.Node, lang *gts.Language, src []byte) string {
	// The identifier sits inside a pattern node, so search descendants.
	var name string
	gts.Walk(node, func(n *gts.Node, _ int) gts.WalkAction {
		if name == "" && n.Type(lang) == "simple_identifier" {
			name = nodeText(n, src)
		}
		return gts.WalkContinue
	})
	return name
}
