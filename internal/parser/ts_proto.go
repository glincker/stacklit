package parser

import (
	gts "github.com/odvcencio/gotreesitter"
)

// extractProto extracts imports, messages, enums and services from a Protobuf file.
func extractProto(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "protobuf"}
	acc := newTypeAcc()

	var visit func(parent *gts.Node, prefix string)
	visit = func(parent *gts.Node, prefix string) {
		for _, n := range namedKids(parent) {
			switch n.Type(lang) {
			case "import":
				if s := childByType(n, lang, "string"); s != nil {
					info.Imports = append(info.Imports, stripQuotes(nodeText(s, src)))
				}
			case "message":
				nm := childByType(n, lang, "message_name")
				if nm == nil {
					continue
				}
				name := prefix + nodeText(nm, src)
				info.Exports = append(info.Exports, name)
				acc.add(name, "")
				if body := childByType(n, lang, "message_body"); body != nil {
					for _, f := range childrenByType(body, lang, "field") {
						acc.add(name, truncSig(sigField(nodeText(f, src)), 60))
					}
					visit(body, name+".")
				}
			case "enum":
				nm := childByType(n, lang, "enum_name")
				if nm == nil {
					continue
				}
				name := prefix + nodeText(nm, src)
				info.Exports = append(info.Exports, name)
				acc.add(name, "")
				if body := childByType(n, lang, "enum_body"); body != nil {
					for _, f := range childrenByType(body, lang, "enum_field") {
						if id := childByType(f, lang, "identifier"); id != nil {
							acc.add(name, nodeText(id, src))
						}
					}
				}
			case "service":
				nm := childByType(n, lang, "service_name")
				if nm == nil {
					continue
				}
				name := nodeText(nm, src)
				info.Exports = append(info.Exports, name)
				acc.add(name, "")
				for _, r := range childrenByType(n, lang, "rpc") {
					acc.add(name, truncSig(sigField(nodeText(r, src)), 100))
				}
			}
		}
	}
	visit(root, "")

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

// sigField drops the trailing semicolon and field number noise from a declaration.
func sigField(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ';' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
