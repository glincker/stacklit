package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractC extracts structured info from C, C++, or Objective-C source files
// using the tree-sitter AST. The Language field is set based on the file extension.
func extractC(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{
		Language: cLanguage(path),
		TypeDefs: make(map[string]string),
	}

	gts.Walk(root, func(node *gts.Node, depth int) gts.WalkAction {
		switch node.Type(lang) {
		case "preproc_include":
			// #include <stdio.h> or #include "myheader.h"
			// The path is in a string_literal or system_lib_string child.
			for i := 0; i < node.ChildCount(); i++ {
				c := node.Child(i)
				t := c.Type(lang)
				if t == "string_literal" || t == "system_lib_string" {
					text := nodeText(c, src)
					// Strip enclosing <> or ""
					text = strings.Trim(text, "<>\"")
					if text != "" {
						info.Imports = append(info.Imports, text)
					}
					break
				}
			}
			return gts.WalkSkipChildren

		case "function_definition":
			// Only file or namespace scope, and not static (internal linkage).
			if !cAtScope(node, lang) || cIsStatic(node, lang, src) {
				return gts.WalkSkipChildren
			}
			declNode := childByType(node, lang, "function_declarator")
			if declNode == nil {
				// Try pointer_declarator wrapping a function_declarator.
				ptrNode := childByType(node, lang, "pointer_declarator")
				if ptrNode != nil {
					declNode = childByType(ptrNode, lang, "function_declarator")
				}
			}
			if declNode != nil {
				nameNode := childByType(declNode, lang, "identifier")
				if nameNode != nil {
					name := nodeText(nameNode, src)
					params := ""
					paramsNode := childByType(declNode, lang, "parameter_list")
					if paramsNode != nil {
						params = truncSig(nodeText(paramsNode, src), 60)
					}
					info.Exports = append(info.Exports, name+params)
					if cIsMain(name, params) {
						info.IsEntrypoint = true
					}
				}
			}
			return gts.WalkSkipChildren

		case "declaration":
			// File-scope declarations: function declarations (prototypes), typedefs.
			if !cAtScope(node, lang) || cIsStatic(node, lang, src) {
				return gts.WalkContinue
			}
			// Look for function_declarator to catch prototypes.
			cExtractDeclaration(node, lang, src, info)
			return gts.WalkContinue

		case "struct_specifier", "union_specifier", "enum_specifier", "class_specifier":
			// Only top-level (or namespace-level) type declarations.
			if !cAtScope(node, lang) {
				return gts.WalkSkipChildren
			}
			nameNode := childByType(node, lang, "type_identifier")
			if nameNode == nil {
				return gts.WalkSkipChildren
			}
			name := nodeText(nameNode, src)
			if name == "" {
				return gts.WalkSkipChildren
			}
			kind := strings.TrimSuffix(node.Type(lang), "_specifier")
			if kind == "class" {
				info.Exports = append(info.Exports, name)
			} else {
				info.Exports = append(info.Exports, kind+" "+name)
			}

			// Extract public fields and methods for structs, unions and classes.
			if bodyNode := childByType(node, lang, "field_declaration_list"); bodyNode != nil {
				if members := cMembers(bodyNode, lang, src, kind == "class"); len(members) > 0 {
					info.TypeDefs[name] = strings.Join(members, ", ")
				}
			}
			if eb := childByType(node, lang, "enumerator_list"); eb != nil {
				var vals []string
				for _, e := range childrenByType(eb, lang, "enumerator") {
					if id := childByType(e, lang, "identifier"); id != nil {
						vals = append(vals, nodeText(id, src))
					}
				}
				if len(vals) > 0 {
					info.TypeDefs[name] = strings.Join(vals, ", ")
				}
			}
			return gts.WalkSkipChildren

		case "type_definition":
			// typedef declarations.
			if !cAtScope(node, lang) {
				return gts.WalkSkipChildren
			}
			// The aliased name is typically the last type_identifier child.
			var lastName string
			for i := 0; i < node.ChildCount(); i++ {
				c := node.Child(i)
				if c.Type(lang) == "type_identifier" {
					lastName = nodeText(c, src)
				}
			}
			if lastName != "" {
				info.Exports = append(info.Exports, "typedef "+lastName)
			}
			// Still descend so `typedef struct {...} T` records its fields under T.
			return gts.WalkContinue
		}

		return gts.WalkContinue
	})

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	if len(info.TypeDefs) == 0 {
		info.TypeDefs = nil
	}
	return info
}

// cLanguage maps file extension to a language label.
func cLanguage(filename string) string {
	switch {
	case extIs(filename, ".cpp", ".cxx", ".cc", ".hpp", ".hxx"):
		return "cpp"
	case extIs(filename, ".m", ".mm"):
		return "objective-c"
	default:
		return "c"
	}
}

// cIsMain checks whether a function name/params represents int main(...).
func cIsMain(name, params string) bool {
	return name == "main"
}

// cExtractDeclaration handles a top-level declaration node, looking for
// function prototypes and other named declarations to add to exports.
func cExtractDeclaration(node *gts.Node, lang *gts.Language, src []byte, info *FileInfo) {
	// Walk direct children for function_declarator or pointer_declarator.
	for i := 0; i < node.ChildCount(); i++ {
		c := node.Child(i)
		switch c.Type(lang) {
		case "function_declarator":
			nameNode := childByType(c, lang, "identifier")
			if nameNode != nil {
				name := nodeText(nameNode, src)
				params := ""
				paramsNode := childByType(c, lang, "parameter_list")
				if paramsNode != nil {
					params = truncSig(nodeText(paramsNode, src), 60)
				}
				info.Exports = append(info.Exports, name+params)
			}
		case "pointer_declarator":
			inner := childByType(c, lang, "function_declarator")
			if inner != nil {
				nameNode := childByType(inner, lang, "identifier")
				if nameNode != nil {
					name := nodeText(nameNode, src)
					params := ""
					paramsNode := childByType(inner, lang, "parameter_list")
					if paramsNode != nil {
						params = truncSig(nodeText(paramsNode, src), 60)
					}
					info.Exports = append(info.Exports, name+params)
				}
			}
		}
	}
}

// cFieldName extracts the field name from a field_declaration node.
func cFieldName(field *gts.Node, lang *gts.Language, src []byte) string {
	// Check for field_identifier directly.
	fn := childByType(field, lang, "field_identifier")
	if fn != nil {
		return nodeText(fn, src)
	}
	// May be wrapped in a pointer_declarator.
	ptrNode := childByType(field, lang, "pointer_declarator")
	if ptrNode != nil {
		fn = childByType(ptrNode, lang, "field_identifier")
		if fn != nil {
			return nodeText(fn, src)
		}
	}
	return ""
}

// cAtScope reports whether a node sits at file, namespace or extern "C" scope,
// looking through wrappers that do not introduce a new scope.
func cAtScope(node *gts.Node, lang *gts.Language) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		switch p.Type(lang) {
		case "translation_unit":
			return true
		case "namespace_definition", "declaration_list", "linkage_specification",
			"template_declaration", "declaration", "type_definition",
			"preproc_ifdef", "preproc_if", "preproc_else", "preproc_elif":
			continue
		default:
			return false
		}
	}
	return true
}

// cIsStatic reports whether a declaration has internal linkage.
func cIsStatic(node *gts.Node, lang *gts.Language, src []byte) bool {
	for _, c := range childrenByType(node, lang, "storage_class_specifier") {
		if nodeText(c, src) == "static" {
			return true
		}
	}
	return false
}

// cMembers lists fields and methods of a struct/union/class body. Classes default
// to private access, structs and unions to public.
func cMembers(body *gts.Node, lang *gts.Language, src []byte, isClass bool) []string {
	public := !isClass
	var out []string
	for i := 0; i < body.ChildCount(); i++ {
		c := body.Child(i)
		switch c.Type(lang) {
		case "access_specifier":
			public = nodeText(c, src) == "public"
		case "field_declaration":
			if !public {
				continue
			}
			if fd := childByType(c, lang, "function_declarator"); fd != nil {
				if n := childByType(fd, lang, "field_identifier"); n != nil {
					out = append(out, nodeText(n, src)+truncSig(nodeText(childByType(fd, lang, "parameter_list"), src), 50))
				}
			} else {
				// `float x, y;` declares several fields in one node.
				for _, f := range childrenByType(c, lang, "field_identifier") {
					out = append(out, nodeText(f, src))
				}
				if len(childrenByType(c, lang, "field_identifier")) == 0 {
					if fn := cFieldName(c, lang, src); fn != "" {
						out = append(out, fn)
					}
				}
			}
		case "function_definition":
			if !public {
				continue
			}
			if fd := childByType(c, lang, "function_declarator"); fd != nil {
				if n := childByType(fd, lang, "field_identifier"); n != nil {
					out = append(out, nodeText(n, src)+truncSig(nodeText(childByType(fd, lang, "parameter_list"), src), 50))
				}
			}
		}
	}
	return out
}
