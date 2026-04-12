package renderer

import (
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/glincker/stacklit/internal/schema"
)

const schemaURL = "https://stacklit.dev/schema/v1.json"

var version = "dev"

// WriteJSON populates metadata fields on idx and writes it as indented JSON to path.
func WriteJSON(idx *schema.Index, path string) error {
	idx.Schema = schemaURL
	idx.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	idx.StacklitVersion = version
	preserveGeneratedAtIfUnchanged(idx, path)

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}

	return os.WriteFile(path, data, 0644)
}
