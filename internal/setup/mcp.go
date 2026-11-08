package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// MCPStyle selects the config file format a tool expects.
type MCPStyle string

const (
	StyleMCPServers MCPStyle = "mcpServers"
	StyleVSCode     MCPStyle = "vscode"
	StyleZed        MCPStyle = "zed"
	StyleOpenCode   MCPStyle = "opencode"
	StyleCrush      MCPStyle = "crush"
	StyleCodexTOML  MCPStyle = "codex-toml"
	StyleContinue   MCPStyle = "continue"
)

const serverName = "stacklit"

// jsonShape returns the top-level key and the server entry for JSON styles.
func jsonShape(s MCPStyle) (string, any) {
	cmd := []string{"serve"}
	switch s {
	case StyleVSCode:
		return "servers", map[string]any{"type": "stdio", "command": "stacklit", "args": cmd}
	case StyleZed:
		return "context_servers", map[string]any{"command": "stacklit", "args": cmd}
	case StyleOpenCode:
		return "mcp", map[string]any{"type": "local", "command": []string{"stacklit", "serve"}, "enabled": true}
	case StyleCrush:
		return "mcp", map[string]any{"type": "stdio", "command": "stacklit", "args": cmd}
	default:
		return "mcpServers", map[string]any{"command": "stacklit", "args": cmd}
	}
}

const tomlBody = "[mcp_servers.stacklit]\ncommand = \"stacklit\"\nargs = [\"serve\"]\n"

const continueBody = "name: Stacklit\nversion: 0.0.1\nschema: v1\nmcpServers:\n  - name: stacklit\n    command: stacklit\n    args:\n      - serve\n"

func decodeJSON(content string) (map[string]any, error) {
	m := map[string]any{}
	if strings.TrimSpace(content) == "" {
		return m, nil
	}
	dec := json.NewDecoder(strings.NewReader(content))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func encodeJSON(m map[string]any) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(m)
	return buf.String()
}

// mergeJSONServer adds the stacklit entry to JSON config content.
func mergeJSONServer(content string, s MCPStyle) (string, error) {
	m, err := decodeJSON(content)
	if err != nil {
		return "", err
	}
	key, entry := jsonShape(s)
	sub, _ := m[key].(map[string]any)
	if sub == nil {
		sub = map[string]any{}
	}
	sub[serverName] = entry
	m[key] = sub
	if s == StyleOpenCode {
		if _, ok := m["$schema"]; !ok {
			m["$schema"] = "https://opencode.ai/config.json"
		}
	}
	return encodeJSON(m), nil
}

// dropJSONServer removes the stacklit entry. empty is true when nothing else is left.
func dropJSONServer(content string, s MCPStyle) (out string, changed, empty bool, err error) {
	m, err := decodeJSON(content)
	if err != nil {
		return content, false, false, err
	}
	key, _ := jsonShape(s)
	sub, _ := m[key].(map[string]any)
	if _, ok := sub[serverName]; !ok {
		return content, false, false, nil
	}
	delete(sub, serverName)
	if len(sub) == 0 {
		delete(m, key)
	} else {
		m[key] = sub
	}
	delete(m, "$schema")
	if len(m) == 0 {
		return "", true, true, nil
	}
	return encodeJSON(m), true, false, nil
}

// applyMCP installs or removes one MCP target.
func (r *Runner) applyMCP(m MCP, uninstall bool) error {
	old, existed, err := r.read(m.Path)
	if err != nil {
		return err
	}
	switch m.Style {
	case StyleCodexTOML:
		return r.applyTextBlock(m.Path, old, existed, tomlBody, StyleHash, uninstall)
	case StyleContinue:
		return r.applyTextBlock(m.Path, old, existed, continueBody, StyleHash, uninstall)
	}
	if uninstall {
		if !existed {
			return nil
		}
		out, changed, empty, err := dropJSONServer(old, m.Style)
		if err != nil {
			fmt.Fprintf(r.Out, "  skip %s (not plain JSON, remove the stacklit entry by hand)\n", m.Path)
			return nil
		}
		if !changed {
			return nil
		}
		if empty {
			return r.remove(m.Path)
		}
		return r.write(m.Path, old, existed, out)
	}
	out, err := mergeJSONServer(old, m.Style)
	if err != nil {
		fmt.Fprintf(r.Out, "  skip %s (not plain JSON, add the stacklit server by hand: command \"stacklit\", args [\"serve\"])\n", m.Path)
		return nil
	}
	return r.write(m.Path, old, existed, out)
}

// applyTextBlock manages a hash-marked block in a text config file.
func (r *Runner) applyTextBlock(path, old string, existed bool, body, style string, uninstall bool) error {
	if uninstall {
		if !existed {
			return nil
		}
		out, changed := removeBlock(old, style)
		if !changed {
			return nil
		}
		if strings.TrimSpace(out) == "" {
			return r.remove(path)
		}
		return r.write(path, old, existed, out)
	}
	return r.write(path, old, existed, upsertBlock(old, body, style))
}
