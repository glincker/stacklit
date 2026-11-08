package ingest

import (
	"path"
	"regexp"
	"strings"
)

// Table is a table or model discovered in SQL or Prisma files.
type Table struct {
	Name    string   `json:"name"`
	File    string   `json:"file"`
	Columns []string `json:"columns"`
}

var (
	createTable = regexp.MustCompile(`(?is)create\s+(?:or\s+replace\s+)?(?:temp(?:orary)?\s+)?table\s+(?:if\s+not\s+exists\s+)?([\w."` + "`" + `\[\]]+)\s*\(`)
	prismaModel = regexp.MustCompile(`(?m)^model\s+(\w+)\s*\{`)
	constraints = map[string]bool{"primary": true, "foreign": true, "constraint": true, "unique": true, "check": true, "key": true, "index": true, "exclude": true}
)

func collectTables(root string, files []string) []Table {
	var out []Table
	for _, rel := range files {
		ext := strings.ToLower(path.Ext(rel))
		if ext != ".sql" && ext != ".prisma" {
			continue
		}
		body, ok := readCapped(root, rel)
		if !ok {
			continue
		}
		if ext == ".sql" {
			out = append(out, parseSQL(body, rel)...)
		} else {
			out = append(out, parsePrisma(body, rel)...)
		}
		if len(out) >= maxItems() {
			out = out[:maxItems()]
			break
		}
	}
	return out
}

func parseSQL(body, rel string) []Table {
	var out []Table
	for _, m := range createTable.FindAllStringSubmatchIndex(body, -1) {
		name := strings.Trim(body[m[2]:m[3]], "`\"[]")
		open := m[1] - 1
		depth, end := 0, -1
		for i := open; i < len(body); i++ {
			if body[i] == '(' {
				depth++
			} else if body[i] == ')' {
				depth--
				if depth == 0 {
					end = i
					break
				}
			}
		}
		if end < 0 {
			continue
		}
		t := Table{Name: name, File: rel}
		for _, item := range splitTopLevel(body[open+1 : end]) {
			f := strings.Fields(item)
			if len(f) == 0 || constraints[strings.ToLower(f[0])] {
				continue
			}
			t.Columns = append(t.Columns, strings.Trim(f[0], "`\"[]"))
		}
		out = append(out, t)
	}
	return out
}

func splitTopLevel(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, s[start:])
}

func parsePrisma(body, rel string) []Table {
	var out []Table
	for _, m := range prismaModel.FindAllStringSubmatchIndex(body, -1) {
		end := strings.Index(body[m[1]:], "}")
		if end < 0 {
			continue
		}
		t := Table{Name: body[m[2]:m[3]], File: rel}
		for _, l := range strings.Split(body[m[1]:m[1]+end], "\n") {
			f := strings.Fields(l)
			if len(f) >= 2 && !strings.HasPrefix(f[0], "@") && !strings.HasPrefix(f[0], "//") {
				t.Columns = append(t.Columns, f[0])
			}
		}
		out = append(out, t)
	}
	return out
}
