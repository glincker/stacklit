package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractSQL extracts tables, views and functions from CREATE statements.
func extractSQL(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "sql"}
	acc := newTypeAcc()

	// Walk everything: dialect syntax the grammar rejects wraps statements in ERROR nodes.
	gts.Walk(root, func(n *gts.Node, _ int) gts.WalkAction {
		switch n.Type(lang) {
		case "create_table_statement":
			name := sqlObjectName(n, lang, src)
			if name == "" {
				return gts.WalkContinue
			}
			info.Exports = append(info.Exports, "table "+name)
			acc.add(name, "")
			if tp := childByType(n, lang, "table_parameters"); tp != nil {
				for _, col := range childrenByType(tp, lang, "table_column") {
					f := strings.Fields(nodeText(col, src))
					if len(f) >= 2 {
						acc.add(name, f[0]+" "+f[1])
					} else if len(f) == 1 {
						acc.add(name, f[0])
					}
				}
			}
			return gts.WalkSkipChildren
		case "create_view_statement", "create_materialized_view_statement":
			if name := sqlObjectName(n, lang, src); name != "" {
				info.Exports = append(info.Exports, "view "+name)
				acc.add(name, "")
			}
			return gts.WalkSkipChildren
		case "create_function_statement":
			name := sqlObjectName(n, lang, src)
			if name == "" {
				return gts.WalkContinue
			}
			sig := name
			if p := childByType(n, lang, "create_function_parameters"); p != nil {
				sig += nodeText(p, src)
			}
			info.Exports = append(info.Exports, "function "+truncSig(sig, 90))
		}
		return gts.WalkContinue
	})

	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

func sqlObjectName(n *gts.Node, lang *gts.Language, src []byte) string {
	for _, c := range namedKids(n) {
		switch c.Type(lang) {
		case "identifier", "dotted_name", "object_reference":
			return nodeText(c, src)
		}
	}
	return ""
}
