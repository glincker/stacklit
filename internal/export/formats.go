package export

import (
	"encoding/json"
	"fmt"
	"strings"
)

func xmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

func cypherEsc(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", "")
	return r.Replace(s)
}

func dotEsc(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return r.Replace(s)
}

// relType turns an edge kind into a valid Cypher relationship type.
func relType(kind string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(kind) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "RELATED"
	}
	return b.String()
}

// labelType turns a node kind into a valid Cypher label.
func labelType(kind string) string {
	s := relType(kind)
	if s == "RELATED" {
		return "Node"
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

func renderGraphML(m *Model) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<graphml xmlns="http://graphml.graphdrawing.org/xmlns">` + "\n")
	keys := []struct{ id, on, name, typ string }{
		{"label", "node", "label", "string"}, {"kind", "node", "kind", "string"},
		{"source_file", "node", "source_file", "string"}, {"line", "node", "line", "int"},
		{"group", "node", "group", "string"},
		{"ekind", "edge", "kind", "string"}, {"confidence", "edge", "confidence", "string"},
		{"score", "edge", "score", "double"},
	}
	for _, k := range keys {
		fmt.Fprintf(&b, `  <key id="%s" for="%s" attr.name="%s" attr.type="%s"/>`+"\n", k.id, k.on, k.name, k.typ)
	}
	fmt.Fprintf(&b, `  <graph id="stacklit-%s" edgedefault="directed">`+"\n", m.Scope)
	for _, n := range m.Nodes {
		fmt.Fprintf(&b, `    <node id="%s">`+"\n", xmlEsc(n.ID))
		fmt.Fprintf(&b, `      <data key="label">%s</data>`+"\n", xmlEsc(n.Label))
		fmt.Fprintf(&b, `      <data key="kind">%s</data>`+"\n", xmlEsc(n.Kind))
		if n.File != "" {
			fmt.Fprintf(&b, `      <data key="source_file">%s</data>`+"\n", xmlEsc(n.File))
		}
		if n.Line > 0 {
			fmt.Fprintf(&b, `      <data key="line">%d</data>`+"\n", n.Line)
		}
		if n.Group != "" {
			fmt.Fprintf(&b, `      <data key="group">%s</data>`+"\n", xmlEsc(n.Group))
		}
		b.WriteString("    </node>\n")
	}
	for i, e := range m.Edges {
		fmt.Fprintf(&b, `    <edge id="e%d" source="%s" target="%s">`+"\n", i, xmlEsc(e.From), xmlEsc(e.To))
		fmt.Fprintf(&b, `      <data key="ekind">%s</data>`+"\n", xmlEsc(e.Kind))
		if e.Conf != "" {
			fmt.Fprintf(&b, `      <data key="confidence">%s</data>`+"\n", xmlEsc(e.Conf))
			fmt.Fprintf(&b, `      <data key="score">%g</data>`+"\n", e.Score)
		}
		b.WriteString("    </edge>\n")
	}
	b.WriteString("  </graph>\n</graphml>\n")
	return b.String()
}

func renderCypher(m *Model) string {
	var b strings.Builder
	b.WriteString("// stacklit export, scope: " + m.Scope + "\n")
	for _, n := range m.Nodes {
		fmt.Fprintf(&b, "CREATE (:%s {id:'%s', label:'%s', kind:'%s'", labelType(n.Kind), cypherEsc(n.ID), cypherEsc(n.Label), cypherEsc(n.Kind))
		if n.File != "" {
			fmt.Fprintf(&b, ", source_file:'%s'", cypherEsc(n.File))
		}
		if n.Line > 0 {
			fmt.Fprintf(&b, ", line:%d", n.Line)
		}
		if n.Group != "" {
			fmt.Fprintf(&b, ", group:'%s'", cypherEsc(n.Group))
		}
		b.WriteString("});\n")
	}
	for _, e := range m.Edges {
		fmt.Fprintf(&b, "MATCH (a {id:'%s'}), (b {id:'%s'}) CREATE (a)-[:%s", cypherEsc(e.From), cypherEsc(e.To), relType(e.Kind))
		if e.Conf != "" {
			fmt.Fprintf(&b, " {confidence:'%s', score:%g}", cypherEsc(e.Conf), e.Score)
		}
		b.WriteString("]->(b);\n")
	}
	return b.String()
}

func renderJSON(m *Model) (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func renderDOT(m *Model) string {
	var b strings.Builder
	b.WriteString("digraph stacklit {\n  rankdir=LR;\n  node [shape=box, fontname=\"Helvetica\"];\n")
	for _, n := range m.Nodes {
		fmt.Fprintf(&b, "  \"%s\" [label=\"%s\", tooltip=\"%s\"];\n", dotEsc(n.ID), dotEsc(n.Label), dotEsc(n.Kind))
	}
	for _, e := range m.Edges {
		style := ""
		if e.Conf == "INFERRED" {
			style = ", style=dashed"
		}
		fmt.Fprintf(&b, "  \"%s\" -> \"%s\" [label=\"%s\"%s];\n", dotEsc(e.From), dotEsc(e.To), dotEsc(e.Kind), style)
	}
	b.WriteString("}\n")
	return b.String()
}

func renderMermaid(m *Model) string {
	ids := map[string]string{}
	var b strings.Builder
	b.WriteString("graph LR\n")
	for i, n := range m.Nodes {
		id := fmt.Sprintf("n%d", i)
		ids[n.ID] = id
		label := strings.NewReplacer(`"`, "'", "\n", " ").Replace(n.Label)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", id, label)
	}
	for _, e := range m.Edges {
		arrow := "-->"
		if e.Conf == "INFERRED" {
			arrow = "-.->"
		}
		fmt.Fprintf(&b, "  %s %s|%s| %s\n", ids[e.From], arrow, strings.NewReplacer("|", "/", "\n", " ").Replace(e.Kind), ids[e.To])
	}
	return b.String()
}
