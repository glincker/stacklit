package setup

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/glincker/stacklit/internal/graph"
)

// graphFile is the symbol graph the hook reads; absent means stay silent.
const graphFile = "stacklit.graph.json"

// sessionNudge stays at or under 100 tokens (bytes/4); a test enforces it.
const sessionNudge = "This repo has a stacklit map: read stacklit.json or call stacklit MCP get_overview before broad searches."

type hookInput struct {
	ToolName  string `json:"tool_name"`
	Cwd       string `json:"cwd"`
	ToolInput struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
		Glob    string `json:"glob"`
		Type    string `json:"type"`
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

// SessionHook returns a short, static SessionStart pointer. The digest is
// already in the rules file, so this stays tiny and byte-identical per run.
func SessionHook(stdin io.Reader) string {
	var in hookInput
	_ = json.NewDecoder(stdin).Decode(&in)
	dir := in.Cwd
	if dir == "" {
		dir = "."
	}
	if _, err := os.Stat(filepath.Join(dir, "stacklit.json")); err != nil {
		return ""
	}
	return hookJSON("SessionStart", sessionNudge)
}
