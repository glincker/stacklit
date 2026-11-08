package parser

import (
	"strings"

	gts "github.com/odvcencio/gotreesitter"
)

// typeAcc collects member summaries per type so each extractor can fill
// FileInfo.TypeDefs the same way.
type typeAcc struct {
	order   []string
	members map[string][]string
}

func newTypeAcc() *typeAcc {
	return &typeAcc{members: map[string][]string{}}
}

// add registers a type and, when member is non-empty, one of its members.
func (a *typeAcc) add(name, member string) {
	if name == "" {
		return
	}
	if _, ok := a.members[name]; !ok {
		a.members[name] = nil
		a.order = append(a.order, name)
	}
	if member != "" {
		a.members[name] = append(a.members[name], member)
	}
}

// apply writes the collected types into info.TypeDefs (nil when empty).
func (a *typeAcc) apply(info *FileInfo) {
	if len(a.order) == 0 {
		return
	}
	info.TypeDefs = make(map[string]string, len(a.order))
	for _, name := range a.order {
		m := dedup(a.members[name])
		if len(m) == 0 {
			info.TypeDefs[name] = name
		} else {
			info.TypeDefs[name] = truncSig(strings.Join(m, "; "), 400)
		}
	}
}

// namedKids returns the named children of a node.
func namedKids(n *gts.Node) []*gts.Node {
	var out []*gts.Node
	for i := 0; i < n.ChildCount(); i++ {
		if c := n.Child(i); c.IsNamed() {
			out = append(out, c)
		}
	}
	return out
}

// sigBeforeBody returns the declaration text up to a top-level '{' or '='
// (ignoring '=' inside brackets and '=>' arrows), so bodies never leak into signatures.
func sigBeforeBody(text string) string {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(', '[', '<':
			depth++
		case ')', ']', '>':
			if depth > 0 && !(text[i] == '>' && i > 0 && text[i-1] == '=') {
				depth--
			}
		case '{':
			if depth == 0 {
				return strings.TrimSpace(text[:i])
			}
		case '=':
			if depth == 0 && (i+1 >= len(text) || text[i+1] != '>') {
				return strings.TrimSpace(text[:i])
			}
		}
	}
	return strings.TrimSpace(text)
}

// stripQuotes removes one layer of surrounding quotes.
func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'' || s[0] == '`') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
