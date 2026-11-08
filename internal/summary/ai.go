package summary

import (
	"github.com/glincker/stacklit/internal/schema"
)

// systemPrompt is the instruction sent with the index when a summary command is configured.
const systemPrompt = "You are a senior software architect. Summarize this codebase architecture in 2-3 concise paragraphs. Focus on the overall structure, key patterns, and how data flows. Be specific to this codebase, not generic."

// indexSnapshot is the subset of the index passed to the summary command.
type indexSnapshot struct {
	Project      schema.Project               `json:"project"`
	Tech         schema.Tech                  `json:"tech"`
	Modules      map[string]schema.ModuleInfo `json:"modules"`
	Dependencies schema.Dependencies          `json:"dependencies"`
	Entrypoints  []string                     `json:"entrypoints"`
}
