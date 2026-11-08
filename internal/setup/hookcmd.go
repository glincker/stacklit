package setup

import (
	"encoding/json"
	"io"
	"path/filepath"

	"github.com/glincker/stacklit/internal/graph"
)

// graphFile is the symbol graph the hook reads; absent means stay silent.
const graphFile = "stacklit.graph.json"

type hookInput struct {
	ToolName  string `json:"tool_name"`
	Cwd       string `json:"cwd"`
	SessionID string `json:"session_id"`
	Source    string `json:"source"`
	AgentType string `json:"agent_type"`
	ToolInput struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
		Type    string `json:"type"`
		// Edit, Write and MultiEdit carry file_path; NotebookEdit carries notebook_path.
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"tool_input"`
}

type hookOutput struct {
	Spec struct {
		Event   string `json:"hookEventName"`
		Context string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func hookJSON(event, ctx string) string {
	var o hookOutput
	o.Spec.Event = event
	o.Spec.Context = ctx
	b, _ := json.Marshal(o)
	return string(b) + "\n"
}

// PreToolHook answers a PreToolUse payload. For a Grep on a single identifier
// it points at the definition and top callers from the symbol graph; it never
// blocks or approves, and stays silent without a graph or for non-identifiers.
func PreToolHook(stdin io.Reader) string {
	var in hookInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil || in.ToolName != "Grep" {
		return ""
	}
	ident := graph.PatternIdent(in.ToolInput.Pattern)
	if ident == "" {
		return ""
	}
	dir := in.Cwd
	if dir == "" {
		dir = "."
	}
	g, err := graph.Load(filepath.Join(dir, graphFile))
	if err != nil {
		return ""
	}
	ptr := g.Pointer(ident)
	if ptr == "" {
		return ""
	}
	return hookJSON("PreToolUse", ptr)
}
