// Package ingest adds non-code context (docs, API specs, SQL schemas and
// ownership) to a sidecar file, stacklit.context.json. It is optional and
// never alters stacklit.json.
package ingest

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ContextFile is the sidecar written next to stacklit.json.
const ContextFile = "stacklit.context.json"

// Options selects which connectors run.
type Options struct {
	Root      string
	Docs      bool
	API       bool
	Schema    bool
	Owners    bool
	GitOwners bool
	GitDays   int
	Modules   []string // module paths from stacklit.json, for ownership mapping
}

// Context is the sidecar payload.
type Context struct {
	Version string  `json:"version"`
	Docs    []Doc   `json:"docs,omitempty"`
	APIs    []API   `json:"apis,omitempty"`
	Tables  []Table `json:"tables,omitempty"`
	Owners  *Owners `json:"owners,omitempty"`

	// Concepts and Links come from external adapters (see adapter.go). They are
	// always INFERRED and carry the adapter name in Source.
	Concepts []Concept     `json:"concepts,omitempty"`
	Links    []ConceptLink `json:"concept_links,omitempty"`
}

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "dist": true, "build": true,
	"target": true, ".venv": true, ".next": true, ".turbo": true, "__pycache__": true,
}

// maxFileBytes caps how much of any one file is read. Override with STACKLIT_INGEST_MAX_BYTES.
func maxFileBytes() int64 { return envInt("STACKLIT_INGEST_MAX_BYTES", 262144) }

// maxItems caps entries per connector. Override with STACKLIT_INGEST_MAX_ITEMS.
func maxItems() int { return int(envInt("STACKLIT_INGEST_MAX_ITEMS", 500)) }

func envInt(name string, def int64) int64 {
	if v, err := strconv.ParseInt(os.Getenv(name), 10, 64); err == nil && v > 0 {
		return v
	}
	return def
}

// walk calls fn with the slash-relative path of every candidate file.
func walk(root string, fn func(rel string)) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != root && (skipDirs[d.Name()] || (strings.HasPrefix(d.Name(), ".") && d.Name() != ".github")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return nil
		}
		fn(filepath.ToSlash(rel))
		return nil
	})
}

func readCapped(root, rel string) (string, bool) {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	defer f.Close()
	buf := make([]byte, maxFileBytes())
	n, _ := f.Read(buf)
	return string(buf[:n]), true
}

// Run executes the selected connectors.
func Run(o Options) (*Context, error) {
	ctx := &Context{Version: "1"}
	var files []string
	if err := walk(o.Root, func(rel string) { files = append(files, rel) }); err != nil {
		return nil, err
	}
	sort.Strings(files)
	if o.Docs {
		ctx.Docs = collectDocs(o.Root, files)
	}
	if o.API {
		ctx.APIs = collectAPIs(o.Root, files)
	}
	if o.Schema {
		ctx.Tables = collectTables(o.Root, files)
	}
	if o.Owners || o.GitOwners {
		ctx.Owners = collectOwners(o)
	}
	return ctx, nil
}
