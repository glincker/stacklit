package ingest

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Operation is one HTTP operation from an OpenAPI document.
type Operation struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	ID      string `json:"id,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// API summarises one OpenAPI/Swagger file.
type API struct {
	File       string      `json:"file"`
	Title      string      `json:"title,omitempty"`
	Operations []Operation `json:"operations"`
}

var httpMethods = map[string]bool{"get": true, "put": true, "post": true, "delete": true, "options": true, "head": true, "patch": true}

func isAPIFile(rel string) bool {
	b := strings.ToLower(path.Base(rel))
	ext := path.Ext(b)
	if ext != ".json" && ext != ".yaml" && ext != ".yml" {
		return false
	}
	return strings.HasPrefix(b, "openapi") || strings.HasPrefix(b, "swagger")
}

func collectAPIs(root string, files []string) []API {
	var out []API
	for _, rel := range files {
		if !isAPIFile(rel) {
			continue
		}
		body, ok := readCapped(root, rel)
		if !ok {
			continue
		}
		var a API
		if strings.HasSuffix(strings.ToLower(rel), ".json") {
			a = parseOpenAPIJSON(body)
		} else {
			a = parseOpenAPIYAML(body)
		}
		if len(a.Operations) == 0 {
			continue
		}
		a.File = rel
		if len(a.Operations) > maxItems() {
			a.Operations = a.Operations[:maxItems()]
		}
		out = append(out, a)
	}
	return out
}

func parseOpenAPIJSON(body string) API {
	var doc struct {
		Info  struct{ Title string }                `json:"info"`
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	var a API
	if json.Unmarshal([]byte(body), &doc) != nil {
		return a
	}
	a.Title = doc.Info.Title
	paths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		methods := make([]string, 0)
		for m := range doc.Paths[p] {
			if httpMethods[m] {
				methods = append(methods, m)
			}
		}
		sort.Strings(methods)
		for _, m := range methods {
			var op struct {
				ID      string `json:"operationId"`
				Summary string `json:"summary"`
			}
			_ = json.Unmarshal(doc.Paths[p][m], &op)
			a.Operations = append(a.Operations, Operation{Method: strings.ToUpper(m), Path: p, ID: op.ID, Summary: op.Summary})
		}
	}
	return a
}

var kvLine = regexp.MustCompile(`^(\s*)["']?([^"':#]+?)["']?\s*:\s*(.*)$`)

// parseOpenAPIYAML is a line scanner, not a YAML parser: it reads the
// conventional layout (paths, then path keys, then method keys) so the tool
// needs no YAML dependency. Flow-style or anchored specs may yield fewer operations.
func parseOpenAPIYAML(body string) API {
	var a API
	inPaths, inInfo := false, false
	pathIndent, methodIndent := -1, -1
	cur := -1
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		m := kvLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent, key, val := len(m[1]), strings.TrimSpace(m[2]), strings.Trim(strings.TrimSpace(m[3]), `"'`)
		if indent == 0 {
			inPaths, inInfo = key == "paths", key == "info"
			pathIndent, methodIndent, cur = -1, -1, -1
			continue
		}
		if inInfo && key == "title" && a.Title == "" {
			a.Title = val
			continue
		}
		if !inPaths {
			continue
		}
		if strings.HasPrefix(key, "/") && (pathIndent < 0 || indent == pathIndent) {
			pathIndent, methodIndent, cur = indent, -1, -1
			a.Operations = append(a.Operations, Operation{Path: key})
			// Placeholder path entry; methods replace it below.
			cur = len(a.Operations) - 1
			continue
		}
		if cur < 0 {
			continue
		}
		if httpMethods[key] && (methodIndent < 0 || indent == methodIndent) {
			methodIndent = indent
			op := a.Operations[cur]
			if op.Method != "" {
				op = Operation{Path: op.Path}
				a.Operations = append(a.Operations, op)
				cur = len(a.Operations) - 1
			}
			a.Operations[cur].Method = strings.ToUpper(key)
			continue
		}
		if methodIndent >= 0 && indent > methodIndent && a.Operations[cur].Method != "" {
			switch key {
			case "operationId":
				a.Operations[cur].ID = val
			case "summary":
				a.Operations[cur].Summary = val
			}
		}
	}
	// Drop path entries that never received a method (e.g. $ref-only paths).
	kept := a.Operations[:0]
	for _, op := range a.Operations {
		if op.Method != "" {
			kept = append(kept, op)
		}
	}
	a.Operations = kept
	return a
}
