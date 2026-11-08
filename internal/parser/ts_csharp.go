package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractCSharp extracts imports, exports, and type definitions from a C# source file.
func extractCSharp(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{
		Language: "csharp",
		TypeDefs: make(map[string]string),
	}

	// Collect public type names for tracking methods.
	// key: type name, value: list of public method signatures.
	typeMethods := make(map[string][]string)

	gts.Walk(root, func(node *gts.Node, depth int) gts.WalkAction {
		switch node.Type(lang) {
		case "using_directive":
			// Extract the namespace identifier from the using directive.
			text := strings.TrimSpace(nodeText(node, src))
			// Strip leading "using " and trailing ";"
			text = strings.TrimPrefix(text, "using")
			text = strings.TrimSuffix(text, ";")
			text = strings.TrimSpace(text)
			// Drop "static" or "alias = " forms, keep the namespace.
			if idx := strings.Index(text, "="); idx >= 0 {
				text = strings.TrimSpace(text[idx+1:])
			}
			text = strings.TrimPrefix(text, "static ")
			text = strings.TrimSpace(text)
			if text != "" {
				info.Imports = append(info.Imports, text)
			}
			return gts.WalkSkipChildren

		case "class_declaration", "interface_declaration", "record_declaration",
			"struct_declaration", "enum_declaration":
			// Check for "public" modifier.
			if !csHasPublicModifier(node, lang, src) {
				return gts.WalkContinue
			}
			name := csTypeName(node, lang, src)
			if name == "" {
				return gts.WalkContinue
			}
			info.Exports = append(info.Exports, name)
			typeMethods[name] = csPublicMethods(node, lang, src, node.Type(lang) == "interface_declaration")
			return gts.WalkContinue
		}
		return gts.WalkContinue
	})

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)

	for typeName, methods := range typeMethods {
		if len(methods) > 0 {
			info.TypeDefs[typeName] = strings.Join(methods, "; ")
		} else {
			info.TypeDefs[typeName] = typeName
		}
	}

	if len(info.TypeDefs) == 0 {
		info.TypeDefs = nil
	}

	return info
}

// csHasPublicModifier checks if a declaration node has a "public" modifier.
func csHasPublicModifier(node *gts.Node, lang *gts.Language, src []byte) bool {
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Type(lang) == "modifier" && nodeText(child, src) == "public" {
			return true
		}
	}
	return false
}

// csTypeName extracts the identifier name from a type declaration node.
func csTypeName(node *gts.Node, lang *gts.Language, src []byte) string {
	id := childByType(node, lang, "identifier")
	if id != nil {
		return nodeText(id, src)
	}
	return ""
}

// csPublicMethods collects public method signatures from a type body.
// Interface members have no modifiers because they are implicitly public.
func csPublicMethods(node *gts.Node, lang *gts.Language, src []byte, isInterface bool) []string {
	var methods []string
	// Look for a declaration_list (body of class/struct/interface).
	body := childByType(node, lang, "declaration_list")
	if body == nil {
		return methods
	}
	for i := 0; i < body.ChildCount(); i++ {
		child := body.Child(i)
		t := child.Type(lang)
		if isInterface && t == "property_declaration" {
			methods = append(methods, truncSig(nodeText(child, src), 80))
			continue
		}
		if t == "method_declaration" || t == "constructor_declaration" {
			if isInterface || csHasPublicModifier(child, lang, src) {
				// Build a brief signature: return_type name(params)
				sig := csMethodSig(child, lang, src)
				if sig != "" {
					methods = append(methods, sig)
				}
			}
		}
	}
	return methods
}

// csMethodSig builds a short method signature string.
func csMethodSig(node *gts.Node, lang *gts.Language, src []byte) string {
	// For method_declaration: return_type identifier parameter_list
	// For constructor_declaration: identifier parameter_list
	var parts []string
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		t := child.Type(lang)
		switch t {
		case "modifier", "block", "arrow_expression_clause", "{", "}":
			continue
		case "identifier", "predefined_type", "generic_name", "array_type",
			"nullable_type", "qualified_name", "parameter_list":
			parts = append(parts, nodeText(child, src))
		}
	}
	sig := strings.Join(parts, " ")
	// Attach the parameter list to the name: "Total()" not "Total ()".
	sig = strings.Replace(sig, " (", "(", 1)
	return truncSig(sig, 80)
}
