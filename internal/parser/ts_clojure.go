package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// clojureHead returns the head symbol of a list form and its other named items.
func clojureHead(n *gts.Node, lang *gts.Language, src []byte) (string, []*gts.Node) {
	k := namedKids(n)
	if len(k) == 0 || k[0].Type(lang) != "sym_lit" {
		return "", nil
	}
	return nodeText(k[0], src), k[1:]
}

// extractClojure reads ns requires and public def forms.
func extractClojure(root *gts.Node, lang *gts.Language, src []byte, path string) *FileInfo {
	info := &FileInfo{Language: "clojure"}
	acc := newTypeAcc()

	for _, n := range namedKids(root) {
		if n.Type(lang) != "list_lit" {
			continue
		}
		head, rest := clojureHead(n, lang, src)
		if len(rest) == 0 {
			continue
		}
		switch head {
		case "ns":
			for _, c := range rest[1:] {
				if c.Type(lang) != "list_lit" {
					continue
				}
				ck := namedKids(c)
				if len(ck) == 0 {
					continue
				}
				kind := strings.TrimSpace(nodeText(ck[0], src))
				if kind != ":require" && kind != ":use" && kind != ":import" {
					continue
				}
				for _, item := range ck[1:] {
					switch item.Type(lang) {
					case "vec_lit":
						if first := namedKids(item); len(first) > 0 {
							info.Imports = append(info.Imports, nodeText(first[0], src))
						}
					case "sym_lit":
						info.Imports = append(info.Imports, nodeText(item, src))
					case "list_lit":
						// (java.util Date Map) imports each class under the package.
						names := namedKids(item)
						if len(names) > 1 && kind == ":import" {
							pkg := nodeText(names[0], src)
							for _, cl := range names[1:] {
								info.Imports = append(info.Imports, pkg+"."+nodeText(cl, src))
							}
						}
					}
				}
			}
		case "defn", "def", "defmacro", "defmulti", "defprotocol", "defrecord", "deftype", "definterface":
			name := nodeText(rest[0], src)
			sig := name
			switch head {
			case "defn", "defmacro":
				for _, c := range rest[1:] {
					if c.Type(lang) == "vec_lit" {
						sig += " " + nodeText(c, src)
						break
					}
				}
				info.Exports = append(info.Exports, truncSig(head+" "+sig, 110))
			case "def", "defmulti":
				info.Exports = append(info.Exports, head+" "+name)
			default:
				info.Exports = append(info.Exports, head+" "+name)
				acc.add(name, "")
				if head == "defrecord" || head == "deftype" {
					for _, c := range rest[1:] {
						if c.Type(lang) == "vec_lit" {
							acc.add(name, nodeText(c, src))
							break
						}
					}
				}
				if head == "defprotocol" {
					for _, c := range rest[1:] {
						if c.Type(lang) == "list_lit" {
							acc.add(name, truncSig(nodeText(c, src), 60))
						}
					}
				}
			}
		}
	}
	info.Imports = dedup(info.Imports)
	info.Exports = dedup(info.Exports)
	acc.apply(info)
	return info
}
