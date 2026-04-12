package renderer

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"

	"github.com/glincker/stacklit/assets"
	"github.com/glincker/stacklit/internal/schema"
)

// WriteHTML renders idx as a self-contained HTML file with an interactive
// force-directed dependency graph and writes it to path.
func WriteHTML(idx *schema.Index, path string) error {
	dataJSON, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	html := strings.Replace(assets.TemplateHTML, "{{STACKLIT_DATA}}", string(dataJSON), 1)
	html = strings.Replace(html, "{{LANG_ICONS_JS}}", assets.LangIconsJS, 1)
	data := []byte(html)
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return os.WriteFile(path, data, 0644)
}
