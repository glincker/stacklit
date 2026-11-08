package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractObjC reads #import/#include, @interface, @protocol and their methods.
func extractObjC(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "objective-c"}
	acc := newTypeAcc()
	declared := map[string]bool{}

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "preproc_include", "preproc_import":
			for _, c := range namedKids(n) {
				switch c.Type(lang) {
				case "system_lib_string":
					info.Imports = append(info.Imports, strings.Trim(nodeText(c, src), "<>"))
				case "string_literal":
					info.Imports = append(info.Imports, stripQuotes(nodeText(c, src)))
				}
			}
		case "class_interface", "protocol_declaration":
			ids := childrenByType(n, lang, "identifier")
			if len(ids) == 0 {
				continue
			}
			name := nodeText(ids[0], src)
			declared[name] = true
			label := "@interface "
			if n.Type(lang) == "protocol_declaration" {
				label = "@protocol "
			}
			info.Exports = append(info.Exports, label+name)
			acc.add(name, "")
			for _, m := range childrenByType(n, lang, "method_declaration") {
				acc.add(name, objcMethodSig(m, src))
			}
			for _, p := range childrenByType(n, lang, "property_declaration") {
				acc.add(name, truncSig(strings.TrimSuffix(strings.TrimSpace(nodeText(p, src)), ";"), 80))
			}
		}
	}
	// Classes only implemented here (no @interface in this file) still get listed.
	for _, n := range namedKids(root) {
		if n.Type(lang) != "class_implementation" {
			continue
		}
		ids := childrenByType(n, lang, "identifier")
		if len(ids) == 0 {
			continue
		}
		name := nodeText(ids[0], src)
		if declared[name] {
			continue
		}
		info.Exports = append(info.Exports, "@implementation "+name)
		acc.add(name, "")
		if impl := childByType(n, lang, "implementation_definition"); impl != nil {
			for _, m := range childrenByType(impl, lang, "method_definition") {
				acc.add(name, objcMethodSig(m, src))
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

// objcMethodSig returns the selector line of a method without its body.
func objcMethodSig(m *gts.Node, src []byte) string {
	t := nodeText(m, src)
	if i := strings.IndexAny(t, "{;"); i >= 0 {
		t = t[:i]
	}
	return truncSig(t, 90)
}
