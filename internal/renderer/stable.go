package renderer

import (
	"encoding/json"
	"os"

	"github.com/glincker/stacklit/internal/schema"
)

func normalizedIndexJSON(idx *schema.Index) ([]byte, error) {
	clone := *idx
	clone.GeneratedAt = ""
	return json.Marshal(clone)
}

// preserveGeneratedAtIfUnchanged keeps the previous GeneratedAt value when the
// semantic index content is unchanged. This avoids needless output churn across
// repeated runs, especially on Windows where unchanged locked files should not
// trigger rewrite attempts.
func preserveGeneratedAtIfUnchanged(idx *schema.Index, path string) {
	existingBytes, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var existing schema.Index
	if err := json.Unmarshal(existingBytes, &existing); err != nil {
		return
	}
	currentNorm, err := normalizedIndexJSON(idx)
	if err != nil {
		return
	}
	existingNorm, err := normalizedIndexJSON(&existing)
	if err != nil {
		return
	}
	if string(currentNorm) == string(existingNorm) {
		idx.GeneratedAt = existing.GeneratedAt
	}
}
