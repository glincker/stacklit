package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

func elmQid(n *gts.Node, src []byte) string {
	return strings.ReplaceAll(nodeText(n, src), " ", "")
}

// extractElm reads imports plus annotated values and types the module exposes.
func extractElm(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "elm"}
	acc := newTypeAcc()
	exposed := map[string]bool{}
	exposeAll := true

	if md := childByType(root, lang, "module_declaration"); md != nil {
		if el := childByType(md, lang, "exposing_list"); el != nil {
			exposeAll = strings.Contains(nodeText(el, src), "..") && len(namedKids(el)) == 0
			for _, e := range namedKids(el) {
				for _, id := range namedKids(e) {
					exposed[nodeText(id, src)] = true
				}
			}
			if len(exposed) > 0 {
				exposeAll = false
			}
		}
	}
	visible := func(name string) bool { return exposeAll || exposed[name] }

	for _, n := range namedKids(root) {
		switch n.Type(lang) {
		case "import_clause":
			if q := childByType(n, lang, "upper_case_qid"); q != nil {
				info.Imports = append(info.Imports, elmQid(q, src))
			}
		case "type_annotation":
			if id := childByType(n, lang, "lower_case_identifier"); id != nil && visible(nodeText(id, src)) {
				info.Exports = append(info.Exports, truncSig(nodeText(n, src), 110))
			}
		case "type_declaration", "type_alias_declaration":
			id := childByType(n, lang, "upper_case_identifier")
			if id == nil || !visible(nodeText(id, src)) {
				continue
			}
			name := nodeText(id, src)
			info.Exports = append(info.Exports, "type "+name)
			acc.add(name, "")
			for _, v := range childrenByType(n, lang, "union_variant") {
				acc.add(name, truncSig(nodeText(v, src), 50))
			}
			if te := childByType(n, lang, "type_expression"); te != nil {
				acc.add(name, truncSig(nodeText(te, src), 120))
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
