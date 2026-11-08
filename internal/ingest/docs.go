package ingest

import (
	"path"
	"regexp"
	"strings"
)

// Doc is one markdown document worth pointing an agent at.
type Doc struct {
	Path     string   `json:"path"`
	Kind     string   `json:"kind"`
	Title    string   `json:"title,omitempty"`
	Status   string   `json:"status,omitempty"`
	Headings []string `json:"headings,omitempty"`
}

var (
	numbered   = regexp.MustCompile(`^\d{2,5}[-_].+\.md$`)
	statusLine = regexp.MustCompile(`(?im)^\s*(?:\*\*)?status(?:\*\*)?\s*[:\-]\s*(.+)$`)
	rootDocs   = map[string]bool{"README.md": true, "ARCHITECTURE.md": true, "DESIGN.md": true, "CONTRIBUTING.md": true}
)

func docKind(rel string) string {
	low := strings.ToLower(rel)
	base := strings.ToLower(path.Base(rel))
	switch {
	case strings.Contains(low, "/adr/") || strings.HasPrefix(low, "adr/") || strings.HasPrefix(base, "adr-") ||
		strings.Contains(low, "/decisions/") || strings.HasPrefix(low, "decisions/"):
		return "adr"
	case strings.Contains(low, "/rfc/") || strings.HasPrefix(low, "rfc/") || strings.Contains(low, "/rfcs/") ||
		strings.HasPrefix(low, "rfcs/") || strings.HasPrefix(base, "rfc-"):
		return "rfc"
	case rootDocs[rel]:
		return "doc"
	case (strings.HasPrefix(low, "docs/") || strings.HasPrefix(low, "doc/")) && strings.HasSuffix(low, ".md"):
		return "doc"
	}
	return ""
}

func collectDocs(root string, files []string) []Doc {
	var out []Doc
	for _, rel := range files {
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			continue
		}
		kind := docKind(rel)
		if kind == "" && numbered.MatchString(path.Base(rel)) && (strings.Contains(rel, "adr") || strings.Contains(rel, "rfc")) {
			kind = "adr"
		}
		if kind == "" {
			continue
		}
		body, ok := readCapped(root, rel)
		if !ok {
			continue
		}
		d := parseDoc(body)
		d.Path, d.Kind = rel, kind
		out = append(out, d)
		if len(out) >= maxItems() {
			break
		}
	}
	return out
}

func parseDoc(body string) Doc {
	var d Doc
	inFence := false
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case strings.HasPrefix(l, "# ") && d.Title == "":
			d.Title = strings.TrimSpace(l[2:])
		case strings.HasPrefix(l, "## ") && len(d.Headings) < 8:
			h := strings.TrimSpace(l[3:])
			d.Headings = append(d.Headings, h)
			// MADR style: a "## Status" heading with the value on the next non-blank line.
			if strings.EqualFold(h, "status") && d.Status == "" {
				for _, n := range lines[i+1:] {
					if t := strings.TrimSpace(n); t != "" {
						d.Status = t
						break
					}
				}
			}
		}
	}
	if d.Status == "" {
		if m := statusLine.FindStringSubmatch(body); m != nil {
			d.Status = strings.TrimSpace(strings.Trim(m[1], "*"))
		}
	}
	return d
}
