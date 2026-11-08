package renderer

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/glincker/stacklit/assets"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
)

// WriteHTMLWithGraph is WriteHTML plus an embedded, searchable symbol graph tab.
// A nil g produces the same page as WriteHTML.
func WriteHTMLWithGraph(idx *schema.Index, g *graph.SymGraph, maxNodes int, path string) error {
	dataJSON, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	graphJSON := []byte("null")
	if g != nil {
		if graphJSON, err = json.Marshal(g.HTMLData(maxNodes)); err != nil {
			return err
		}
	}
	html := strings.Replace(assets.TemplateHTML, "{{STACKLIT_DATA}}", string(dataJSON), 1)
	html = strings.Replace(html, "{{STACKLIT_GRAPH}}", string(graphJSON), 1)
	html = strings.Replace(html, "{{SYMBOLS_JS}}", assets.SymbolsJS, 1)
	html = strings.Replace(html, "{{LANG_ICONS_JS}}", assets.LangIconsJS, 1)
	data := []byte(html)
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0o644)
}
