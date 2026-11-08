package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractHaskell reads imports, exported top-level signatures and data/class declarations.
func extractHaskell(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "haskell"}
	acc := newTypeAcc()

	// An explicit module export list limits what counts as exported.
	var allowed map[string]bool
	if h := childByType(root, lang, "header"); h != nil {
		if ex := childByType(h, lang, "exports"); ex != nil {
			allowed = map[string]bool{}
			for _, e := range namedKids(ex) {
				if k := namedKids(e); len(k) > 0 {
					allowed[nodeText(k[0], src)] = true
				}
			}
		}
	}
	visible := func(name string) bool { return allowed == nil || allowed[name] }

	if imps := childByType(root, lang, "imports"); imps != nil {
		for _, imp := range childrenByType(imps, lang, "import") {
			if m := childByType(imp, lang, "module"); m != nil {
				info.Imports = append(info.Imports, strings.ReplaceAll(nodeText(m, src), " ", ""))
			}
		}
	}

	decls := childByType(root, lang, "declarations")
	if decls == nil {
		info.Imports = dedup(info.Imports)
		return info
	}
	for _, d := range namedKids(decls) {
		switch d.Type(lang) {
		case "signature":
			k := namedKids(d)
			if len(k) == 0 {
				continue
			}
			if name := nodeText(k[0], src); visible(name) {
				info.Exports = append(info.Exports, truncSig(nodeText(d, src), 120))
			}
		case "data_type", "newtype", "type_synomym", "type_synonym", "class":
			k := namedKids(d)
			if len(k) == 0 {
				continue
			}
			name := nodeText(k[0], src)
			if !visible(name) {
				continue
			}
			info.Exports = append(info.Exports, name)
			acc.add(name, "")
			hsMembers(d, lang, src, name, acc)
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

// hsMembers records constructors or class method signatures under a type.
func hsMembers(d *gts.Node, lang *gts.Language, src []byte, name string, acc *typeAcc) {
	for _, c := range namedKids(d) {
		switch c.Type(lang) {
		case "data_constructors":
			for _, dc := range namedKids(c) {
				acc.add(name, truncSig(nodeText(dc, src), 60))
			}
		case "newtype_constructor":
			acc.add(name, truncSig(nodeText(c, src), 60))
		case "class_declarations":
			for _, s := range childrenByType(c, lang, "signature") {
				acc.add(name, truncSig(nodeText(s, src), 80))
			}
		}
	}
}
