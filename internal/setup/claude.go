package setup

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillMD is the /stacklit skill installed for Claude Code.
//
//go:embed skill/SKILL.md
var SkillMD string

const (
	hookPreCmd     = "stacklit hook pretool"
	hookSessionCmd = "stacklit hook session"
)

// ClaudeOptions selects the optional Claude Code extras.
type ClaudeOptions struct {
	PreToolHook  bool
	SessionHook  bool
	Skill        bool
	GlobalScope  bool
	UninstallAll bool
}

func claudeBase(global bool) (string, error) {
	if !global {
		return ".claude", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}

// hookEntry builds one settings.json hook group.
func hookEntry(matcher, cmd string) map[string]any {
	g := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": cmd}}}
	if matcher != "" {
		g["matcher"] = matcher
	}
	return g
}

func groupHasCmd(g any, cmd string) bool {
	gm, _ := g.(map[string]any)
	hs, _ := gm["hooks"].([]any)
	for _, h := range hs {
		hm, _ := h.(map[string]any)
		if c, _ := hm["command"].(string); c == cmd {
			return true
		}
	}
	return false
}

// setHook adds or removes one stacklit hook group under event.
func setHook(m map[string]any, event, matcher, cmd string, on bool) {
	hooks, _ := m["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	list, _ := hooks[event].([]any)
	var kept []any
	for _, g := range list {
		if !groupHasCmd(g, cmd) {
			kept = append(kept, g)
		}
	}
	if on {
		kept = append(kept, hookEntry(matcher, cmd))
	}
	if len(kept) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = kept
	}
	if len(hooks) == 0 {
		delete(m, "hooks")
	} else {
		m["hooks"] = hooks
	}
}

// ApplyClaudeExtras installs or removes the optional hooks and skill.
// Settings are only written under .claude/ unless GlobalScope is set.
func (r *Runner) ApplyClaudeExtras(o ClaudeOptions, uninstall bool) error {
	base, err := claudeBase(o.GlobalScope)
	if err != nil {
		return err
	}
	if o.PreToolHook || o.SessionHook || uninstall {
		path := filepath.Join(base, "settings.json")
		old, existed, err := r.read(path)
		if err != nil {
			return err
		}
		m, err := decodeJSON(old)
		if err != nil {
			fmt.Fprintf(r.Out, "  skip %s (not plain JSON)\n", path)
			return nil
		}
		before := encodeJSON(m)
		if o.PreToolHook || uninstall {
			setHook(m, "PreToolUse", "Grep|Glob", hookPreCmd, !uninstall)
		}
		if o.SessionHook || uninstall {
			setHook(m, "SessionStart", "startup|resume", hookSessionCmd, !uninstall)
		}
		switch {
		case len(m) == 0 && existed:
			if err := r.remove(path); err != nil {
				return err
			}
		case len(m) == 0:
		case uninstall && encodeJSON(m) == before:
		default:
			if err := r.write(path, old, existed, encodeJSON(m)); err != nil {
				return err
			}
		}
	}
	if o.Skill || uninstall {
		path := filepath.Join(base, "skills", "stacklit", "SKILL.md")
		old, existed, err := r.read(path)
		if err != nil {
			return err
		}
		if uninstall {
			// Only delete a skill file that still carries our name.
			if existed && strings.Contains(old, "name: stacklit") {
				return r.remove(path)
			}
			return nil
		}
		return r.write(path, old, existed, SkillMD)
	}
	return nil
}
