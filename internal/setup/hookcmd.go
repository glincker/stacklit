package setup

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Nudge texts stay at or under 100 tokens (bytes/4); a test enforces it.
const (
	preToolNudge = "stacklit.json maps this repo's modules. Check it (or stacklit MCP find_module) before broad grep/glob."
	sessionNudge = "This repo has a stacklit map: read stacklit.json or call stacklit MCP get_overview before broad searches."
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
	return hookJSON("PreToolUse", preToolNudge)
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
