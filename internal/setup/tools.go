package setup

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Rule is one instruction file a tool reads. Owned files are ours alone and
// are deleted on uninstall once the block is gone; shared files only lose the block.
type Rule struct {
	Path        string
	Style       string
	Frontmatter string
}

// MCP describes where and how a tool registers the stacklit server.
type MCP struct {
	Path  string
	Style MCPStyle
	// Global paths are relative to the home directory and need --global.
	Global bool
}

// Tool is one supported AI coding assistant.
type Tool struct {
	ID    string
	Name  string
	Rules []Rule
	MCP   []MCP
	// Paths and Bins drive auto-detection.
	Paths []string
	Bins  []string
}

const (
	fmCursor   = "---\ndescription: Codebase map from stacklit\nalwaysApply: true\n---\n"
	fmWindsurf = "---\ntrigger: always_on\n---\n"
	fmKiro     = "---\ninclusion: always\n---\n"
	fmAugment  = "---\ntype: always_apply\n---\n"
	fmContinue = "---\nname: Stacklit codebase map\nalwaysApply: true\n---\n"
)

func md(path string) Rule { return Rule{Path: path, Style: StyleMarkdown} }

func mdFM(path, fm string) Rule { return Rule{Path: path, Style: StyleMarkdown, Frontmatter: fm} }

// Registry lists every supported tool. Order is the display order.
func Registry() []Tool {
	return []Tool{
		{ID: "claude", Name: "Claude Code", Rules: []Rule{md("CLAUDE.md")},
			MCP: []MCP{{Path: ".mcp.json", Style: StyleMCPServers}}, Paths: []string{".claude", "CLAUDE.md"}, Bins: []string{"claude"}},
		{ID: "cursor", Name: "Cursor", Rules: []Rule{mdFM(".cursor/rules/stacklit.mdc", fmCursor)},
			MCP: []MCP{{Path: ".cursor/mcp.json", Style: StyleMCPServers}}, Paths: []string{".cursor", ".cursorrules"}},
		{ID: "aider", Name: "Aider", Rules: []Rule{md("CONVENTIONS.md")},
			MCP: nil, Paths: []string{".aider.conf.yml"}, Bins: []string{"aider"}},
		{ID: "codex", Name: "Codex", Rules: []Rule{md("AGENTS.md")},
			MCP: []MCP{{Path: ".codex/config.toml", Style: StyleCodexTOML}}, Paths: []string{".codex"}, Bins: []string{"codex"}},
		{ID: "gemini", Name: "Gemini CLI", Rules: []Rule{md("GEMINI.md")},
			MCP: []MCP{{Path: ".gemini/settings.json", Style: StyleMCPServers}}, Paths: []string{".gemini", "GEMINI.md"}, Bins: []string{"gemini"}},
		{ID: "copilot", Name: "GitHub Copilot", Rules: []Rule{md(".github/copilot-instructions.md")},
			MCP: []MCP{{Path: ".vscode/mcp.json", Style: StyleVSCode}}, Paths: []string{".github/copilot-instructions.md"}},
		{ID: "windsurf", Name: "Windsurf", Rules: []Rule{mdFM(".windsurf/rules/stacklit.md", fmWindsurf)},
			MCP: []MCP{{Path: ".codeium/windsurf/mcp_config.json", Style: StyleMCPServers, Global: true}}, Paths: []string{".windsurf", ".windsurfrules"}},
		{ID: "cline", Name: "Cline", Rules: []Rule{md(".clinerules/stacklit.md")}, Paths: []string{".clinerules", ".clineignore"}},
		{ID: "zed", Name: "Zed", Rules: []Rule{md(".rules")},
			MCP: []MCP{{Path: ".zed/settings.json", Style: StyleZed}}, Paths: []string{".zed"}, Bins: []string{"zed"}},
		{ID: "continue", Name: "Continue", Rules: []Rule{mdFM(".continue/rules/stacklit.md", fmContinue)},
			MCP: []MCP{{Path: ".continue/mcpServers/stacklit.yaml", Style: StyleContinue}}, Paths: []string{".continue"}},
		{ID: "opencode", Name: "OpenCode", Rules: []Rule{md("AGENTS.md")},
			MCP: []MCP{{Path: "opencode.json", Style: StyleOpenCode}}, Paths: []string{"opencode.json", ".opencode"}, Bins: []string{"opencode"}},
		{ID: "amp", Name: "Amp", Rules: []Rule{md("AGENTS.md")}, Bins: []string{"amp"}},
		{ID: "roo", Name: "Roo Code", Rules: []Rule{md(".roo/rules/stacklit.md")},
			MCP: []MCP{{Path: ".roo/mcp.json", Style: StyleMCPServers}}, Paths: []string{".roo", ".roomodes"}},
		{ID: "kilo", Name: "Kilo Code", Rules: []Rule{md(".kilocode/rules/stacklit.md")},
			MCP: []MCP{{Path: ".kilocode/mcp.json", Style: StyleMCPServers}}, Paths: []string{".kilocode"}},
		{ID: "junie", Name: "JetBrains Junie", Rules: []Rule{md(".junie/guidelines.md")},
			MCP: []MCP{{Path: ".junie/mcp/mcp.json", Style: StyleMCPServers}}, Paths: []string{".junie"}},
		{ID: "jetbrains-ai", Name: "JetBrains AI Assistant", Rules: []Rule{md(".aiassistant/rules/stacklit.md")}, Paths: []string{".aiassistant"}},
		{ID: "augment", Name: "Augment", Rules: []Rule{mdFM(".augment/rules/stacklit.md", fmAugment)}, Paths: []string{".augment"}, Bins: []string{"auggie"}},
		{ID: "trae", Name: "Trae", Rules: []Rule{md(".trae/rules/stacklit.md")}, Paths: []string{".trae"}},
		{ID: "kiro", Name: "Kiro", Rules: []Rule{mdFM(".kiro/steering/stacklit.md", fmKiro)},
			MCP: []MCP{{Path: ".kiro/settings/mcp.json", Style: StyleMCPServers}}, Paths: []string{".kiro"}},
		{ID: "goose", Name: "Goose", Rules: []Rule{md(".goosehints")}, Paths: []string{".goosehints"}, Bins: []string{"goose"}},
		{ID: "warp", Name: "Warp", Rules: []Rule{md("WARP.md")}, Paths: []string{"WARP.md"}},
		{ID: "crush", Name: "Crush", Rules: []Rule{md("CRUSH.md")},
			MCP: []MCP{{Path: ".crush.json", Style: StyleCrush}}, Paths: []string{".crush.json", "CRUSH.md"}, Bins: []string{"crush"}},
		{ID: "qwen", Name: "Qwen Code", Rules: []Rule{md("QWEN.md")},
			MCP: []MCP{{Path: ".qwen/settings.json", Style: StyleMCPServers}}, Paths: []string{".qwen", "QWEN.md"}, Bins: []string{"qwen"}},
		{ID: "droid", Name: "Factory Droid", Rules: []Rule{md("AGENTS.md")},
			MCP: []MCP{{Path: ".factory/mcp.json", Style: StyleMCPServers}}, Paths: []string{".factory"}, Bins: []string{"droid"}},
		{ID: "replit", Name: "Replit Agent", Rules: []Rule{md("replit.md")}, Paths: []string{"replit.md", ".replit"}},
		{ID: "agents", Name: "AGENTS.md (generic)", Rules: []Rule{md("AGENTS.md")}, Paths: []string{"AGENTS.md"}},
	}
}

// Lookup finds a tool by id.
func Lookup(id string) (Tool, bool) {
	for _, t := range Registry() {
		if t.ID == id {
			return t, true
		}
	}
	return Tool{}, false
}

// Detected reports whether the tool looks installed or in use under root.
func (t Tool) Detected(root string) bool {
	for _, p := range t.Paths {
		if _, err := os.Stat(filepath.Join(root, p)); err == nil {
			return true
		}
	}
	for _, b := range t.Bins {
		if _, err := exec.LookPath(b); err == nil {
			return true
		}
	}
	return false
}
