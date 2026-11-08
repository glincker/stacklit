package setup

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glincker/stacklit/internal/derive"
	"github.com/glincker/stacklit/internal/schema"
)

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

// isBroad reports whether a Grep or Glob call sweeps the whole repo.
func isBroad(in hookInput) bool {
	switch in.ToolName {
	case "Glob":
		p := in.ToolInput.Pattern
		return strings.HasPrefix(p, "**") || p == "*" || strings.HasPrefix(p, "*.") && !strings.Contains(p, "/")
	case "Grep":
		t := in.ToolInput
		return t.Glob == "" && t.Type == "" && (t.Path == "" || t.Path == "." || t.Path == in.Cwd)
	}
	return false
}

// PreToolHook answers a PreToolUse payload. It only adds context, never
// blocks or approves, and stays silent when there is nothing to say.
func PreToolHook(stdin io.Reader) string {
	var in hookInput
	if err := json.NewDecoder(stdin).Decode(&in); err != nil || !isBroad(in) {
		return ""
	}
	dir := in.Cwd
	if dir == "" {
		dir = "."
	}
	if _, err := os.Stat(filepath.Join(dir, "stacklit.json")); err != nil {
		return ""
	}
	return hookJSON("PreToolUse", "This repo has stacklit.json (module map). Consult it, or the stacklit MCP tools, to pick the right module before broad grep or glob searches.")
}

// SessionHook returns SessionStart context: the compact map of the repo in cwd.
func SessionHook(stdin io.Reader) string {
	var in hookInput
	_ = json.NewDecoder(stdin).Decode(&in)
	dir := in.Cwd
	if dir == "" {
		dir = "."
	}
	data, err := os.ReadFile(filepath.Join(dir, "stacklit.json"))
	if err != nil {
		return ""
	}
	var idx schema.Index
	if json.Unmarshal(data, &idx) != nil {
		return ""
	}
	return hookJSON("SessionStart", "Codebase map (stacklit). Prefer it over broad grep.\n"+derive.CompactMap(&idx))
}
