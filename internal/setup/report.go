package setup

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/glincker/stacklit/internal/graph"

	"github.com/glincker/stacklit/internal/schema"
)

// ReportRow is the always-on context one integration adds, in tokens (bytes/4).
type ReportRow struct {
	Item   string
	Tokens int
	Note   string
}

// MCPSchemaTokens starts `exe serve` and measures the tools/list payload an
// agent loads into context. It returns -1 when the server cannot be queried.
func MCPSchemaTokens(exe, dir string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "serve")
	cmd.Dir = dir
	in, err := cmd.StdinPipe()
	if err != nil {
		return -1
	}
	out, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return -1
	}
	defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	send := func(s string) { _, _ = io.WriteString(in, s+"\n") }
	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"stacklit-report","version":"0"}}}`)
	rd := bufio.NewReaderSize(out, 1<<20)
	wait := func(id int) (json.RawMessage, bool) {
		for {
			line, err := rd.ReadBytes('\n')
			if err != nil {
				return nil, false
			}
			var m struct {
				ID     *int            `json:"id"`
				Result json.RawMessage `json:"result"`
			}
			if json.Unmarshal(line, &m) == nil && m.ID != nil && *m.ID == id {
				return m.Result, true
			}
		}
	}
	if _, ok := wait(1); !ok {
		return -1
	}
	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	res, ok := wait(2)
	if !ok {
		return -1
	}
	var tl struct {
		Tools json.RawMessage `json:"tools"`
	}
	if json.Unmarshal(res, &tl) != nil {
		return -1
	}
	return Tokens(string(tl.Tools))
}

func frontmatterField(md, key string) string {
	for _, l := range strings.Split(md, "\n") {
		if strings.HasPrefix(l, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(l, key+":"))
		}
	}
	return ""
}

// Report measures the always-on cost of each integration for idx.
// mcpTokens is the shared MCP schema cost, or -1 if unknown.
func Report(idx *schema.Index, tools []Tool, mcpTokens int) []ReportRow {
	block := wrap(MarkdownBody(idx), StyleMarkdown)
	var rows []ReportRow
	seen := map[string]bool{}
	for _, t := range tools {
		for _, r := range t.Rules {
			if seen[r.Path] {
				continue
			}
			seen[r.Path] = true
			rows = append(rows, ReportRow{Item: fmt.Sprintf("%s rules (%s)", t.ID, r.Path), Tokens: Tokens(r.Frontmatter + block), Note: "loaded every session"})
		}
	}
	if mcpTokens >= 0 {
		rows = append(rows, ReportRow{Item: "MCP server tool schemas", Tokens: mcpTokens, Note: "per tool that registers it"})
	} else {
		rows = append(rows, ReportRow{Item: "MCP server tool schemas", Tokens: -1, Note: "could not query `stacklit serve`"})
	}
	rows = append(rows,
		ReportRow{Item: "claude skill (name+description)", Tokens: Tokens(frontmatterField(SkillMD, "name") + frontmatterField(SkillMD, "description")), Note: "always on; body loads only when invoked"},
		ReportRow{Item: "claude skill body", Tokens: Tokens(SkillMD), Note: "only when invoked"},
		ReportRow{Item: "claude PreToolUse pointer", Tokens: graph.DefaultPointerTokens, Note: "max, only on Grep for a known identifier"},
		ReportRow{Item: "claude SessionStart nudge", Tokens: Tokens(sessionNudge), Note: "once per session"},
	)
	return rows
}
