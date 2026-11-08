package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// extractHCL extracts Terraform-style blocks: resources, data sources, variables, outputs, modules.
func extractHCL(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "hcl"}
	acc := newTypeAcc()

	var blocks []*gts.Node
	gts.Walk(root, func(n *gts.Node, depth int) gts.WalkAction {
		if n.Type(lang) == "block" {
			blocks = append(blocks, n)
			return gts.WalkSkipChildren // only top-level blocks matter
		}
		return gts.WalkContinue
	})

	for _, b := range blocks {
		kind := ""
		var labels []string
		for _, c := range namedKids(b) {
			switch c.Type(lang) {
			case "identifier":
				if kind == "" {
					kind = nodeText(c, src)
				}
			case "string_lit":
				labels = append(labels, stripQuotes(nodeText(c, src)))
			}
		}
		var name string
		switch kind {
		case "resource":
			if len(labels) == 2 {
				name = labels[0] + "." + labels[1]
			}
		case "data":
			if len(labels) == 2 {
				name = "data." + labels[0] + "." + labels[1]
			}
		case "variable":
			if len(labels) == 1 {
				name = "var." + labels[0]
			}
		case "output":
			if len(labels) == 1 {
				name = "output." + labels[0]
			}
		case "module":
			if len(labels) == 1 {
				name = "module." + labels[0]
				if src := hclAttr(b, lang, src, "source"); src != "" {
					info.Imports = append(info.Imports, src)
				}
			}
		case "provider":
			if len(labels) == 1 {
				name = "provider." + labels[0]
			}
		}
		if name == "" {
			continue
		}
		info.Exports = append(info.Exports, name)
		acc.add(name, "")
		for _, a := range hclAttrNames(b, lang, src) {
			acc.add(name, a)
		}
	}

	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}

// hclAttrNames lists the attribute names set directly in a block body.
func hclAttrNames(b *gts.Node, lang *gts.Language, src []byte) []string {
	body := childByType(b, lang, "body")
	if body == nil {
		return nil
	}
	var out []string
	for _, a := range childrenByType(body, lang, "attribute") {
		if id := childByType(a, lang, "identifier"); id != nil {
			out = append(out, nodeText(id, src))
		}
	}
	return out
}

// hclAttr returns a string attribute's unquoted value, or "" when absent.
func hclAttr(b *gts.Node, lang *gts.Language, src []byte, key string) string {
	body := childByType(b, lang, "body")
	if body == nil {
		return ""
	}
	for _, a := range childrenByType(body, lang, "attribute") {
		if id := childByType(a, lang, "identifier"); id != nil && nodeText(id, src) == key {
			t := nodeText(a, src)
			if i := strings.Index(t, "="); i >= 0 {
				return stripQuotes(strings.TrimSpace(t[i+1:]))
			}
		}
	}
	return ""
}
