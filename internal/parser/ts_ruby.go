package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractRuby extracts imports, exports, and type definitions from a Ruby source file.
func extractRuby(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{
		Language: "ruby",
		TypeDefs: make(map[string]string),
	}

	// Track current class/module context for method association.
	// We do a two-pass approach: first collect all classes and top-level methods,
	// then populate TypeDefs.
	type classEntry struct {
		name    string
		methods []string
	}
	var classes []classEntry
	var topMethods []string

	gts.Walk(root, func(node *gts.Node, depth int) gts.WalkAction {
		switch node.Type(lang) {
		case "call":
			if v := rubyRequire(node, lang, src); v != "" {
				info.Imports = append(info.Imports, v)
			}
			return gts.WalkSkipChildren

		case "class", "module":
			// Collect class/module name and their methods.
			// Requires inside class bodies still count as imports.
			gts.Walk(node, func(c *gts.Node, _ int) gts.WalkAction {
				if c.Type(lang) == "call" {
					if v := rubyRequire(c, lang, src); v != "" {
						info.Imports = append(info.Imports, v)
					}
				}
				return gts.WalkContinue
			})
			rubyCollect(node, lang, src, "", func(name string, methods []string) {
				info.Exports = append(info.Exports, name)
				classes = append(classes, classEntry{name: name, methods: methods})
			})
			return gts.WalkSkipChildren

		case "method", "singleton_method":
			// Top-level method (not inside a class).
			if name := rubyMethodName(node, lang, src); name != "" && !strings.HasPrefix(name, "_") {
				info.Exports = append(info.Exports, rubyMethodSig(node, lang, src, name))
				topMethods = append(topMethods, name)
			}
			return gts.WalkSkipChildren
		}
		return gts.WalkContinue
	})

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)

	for _, ce := range classes {
		if len(ce.methods) > 0 {
			info.TypeDefs[ce.name] = strings.Join(ce.methods, "; ")
		} else {
			info.TypeDefs[ce.name] = ce.name
		}
	}

	if len(info.TypeDefs) == 0 {
		info.TypeDefs = nil
	}

	return info
}

// rubyStringValue extracts the string content from a Ruby string node.
func rubyStringValue(node *gts.Node, lang *gts.Language, src []byte) string {
	// Try string_content child first.
	sc := childByType(node, lang, "string_content")
	if sc != nil {
		return nodeText(sc, src)
	}
	// Fall back to stripping quotes from raw text.
	raw := nodeText(node, src)
	raw = strings.Trim(raw, `'"`)
	return raw
}

// rubyDefName extracts the name from a class or module node.
func rubyDefName(node *gts.Node, lang *gts.Language, src []byte) string {
	// class/module name is typically a constant (identifier with uppercase).
	name := childByType(node, lang, "constant")
	if name != nil {
		return nodeText(name, src)
	}
	// scope resolution: Foo::Bar
	scope := childByType(node, lang, "scope_resolution")
	if scope != nil {
		return nodeText(scope, src)
	}
	return ""
}

// rubyMethodName extracts the name from a method definition node.
func rubyMethodName(node *gts.Node, lang *gts.Language, src []byte) string {
	id := childByType(node, lang, "identifier")
	if id != nil {
		return nodeText(id, src)
	}
	return ""
}

// rubyRequire returns the target of a require/require_relative call, or "".
func rubyRequire(node *gts.Node, lang *gts.Language, src []byte) string {
	method := childByType(node, lang, "identifier")
	if method == nil {
		return ""
	}
	if m := nodeText(method, src); m != "require" && m != "require_relative" {
		return ""
	}
	args := childByType(node, lang, "argument_list")
	if args == nil {
		return ""
	}
	str := childByType(args, lang, "string")
	if str == nil {
		return ""
	}
	return rubyStringValue(str, lang, src)
}

// rubyMethodSig renders `name(params)` for a method node.
func rubyMethodSig(node *gts.Node, lang *gts.Language, src []byte, name string) string {
	if p := childByType(node, lang, "method_parameters"); p != nil {
		return truncSig(name+nodeText(p, src), 80)
	}
	return name
}

// rubyCollect records a class/module (qualified by its enclosing modules) and
// recurses into nested ones. Methods after a bare private/protected are skipped.
func rubyCollect(node *gts.Node, lang *gts.Language, src []byte, prefix string, emit func(name string, methods []string)) {
	name := rubyDefName(node, lang, src)
	if name == "" {
		return
	}
	full := prefix + name
	var methods []string
	var nested []*gts.Node
	if body := childByType(node, lang, "body_statement"); body != nil {
		public := true
		for i := 0; i < body.ChildCount(); i++ {
			child := body.Child(i)
			switch child.Type(lang) {
			case "identifier":
				if t := nodeText(child, src); t == "private" || t == "protected" {
					public = false
				} else if t == "public" {
					public = true
				}
			case "method", "singleton_method":
				if n := rubyMethodName(child, lang, src); n != "" && public {
					sig := rubyMethodSig(child, lang, src, n)
					if child.Type(lang) == "singleton_method" {
						sig = "self." + sig
					}
					methods = append(methods, sig)
				}
			case "class", "module":
				nested = append(nested, child)
			}
		}
	}
	emit(full, methods)
	for _, n := range nested {
		rubyCollect(n, lang, src, full+"::", emit)
	}
}
