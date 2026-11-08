package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// parsed is what one stream-json session tells us.
type parsed struct {
	result     string
	isError    bool
	subtype    string
	turns      int
	cost       float64
	use        usage
	model      string
	denials    int
	calls      map[string]int
	files      map[string]bool
	readBytes  int
	stacklit   int
	hooks      int
	util       float64
	log        []string
	stop       string
	gotResult  bool
	rateStatus string
}

type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	Text      string          `json:"text"`
}

type event struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	Message struct {
		Model   string  `json:"model"`
		Content []block `json:"content"`
	} `json:"message"`
	IsError      bool            `json:"is_error"`
	Result       string          `json:"result"`
	NumTurns     int             `json:"num_turns"`
	Cost         float64         `json:"total_cost_usd"`
	Usage        usage           `json:"usage"`
	ModelUsage   map[string]any  `json:"modelUsage"`
	Denials      []any           `json:"permission_denials"`
	RateLimit    json.RawMessage `json:"rate_limit_info"`
	TerminalFrom string          `json:"terminal_reason"`
}

// contentLen counts the bytes of a tool_result body (string or text blocks).
func contentLen(raw json.RawMessage) int {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return len(s)
	}
	var bs []block
	if json.Unmarshal(raw, &bs) == nil {
		n := 0
		for _, b := range bs {
			n += len(b.Text)
		}
		return n
	}
	return len(raw)
}

func parseStream(b []byte) (parsed, error) {
	p := parsed{calls: map[string]int{}, files: map[string]bool{}}
	readIDs := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e event
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		switch e.Type {
		case "system":
			if strings.Contains(e.Subtype, "hook") {
				p.hooks++
			}
		case "rate_limit_event":
			var rl struct {
				Status  string `json:"status"`
				Unified map[string]struct {
					Utilization float64 `json:"utilization"`
				} `json:"unifiedWindows"`
			}
			if json.Unmarshal(e.RateLimit, &rl) == nil {
				p.rateStatus = rl.Status
				if rl.Status != "" && rl.Status != "allowed" && rl.Status != "allowed_warning" {
					p.stop = "rate limit status " + rl.Status
				}
				for _, w := range rl.Unified {
					if w.Utilization > p.util {
						p.util = w.Utilization
					}
				}
			}
		case "assistant":
			for _, bl := range e.Message.Content {
				if bl.Type != "tool_use" {
					continue
				}
				p.calls[bl.Name]++
				var in map[string]any
				_ = json.Unmarshal(bl.Input, &in)
				desc := bl.Name
				switch bl.Name {
				case "Read":
					readIDs[bl.ID] = true
					if f, _ := in["file_path"].(string); f != "" {
						p.files[f] = true
						desc += " " + f
					}
				case "Bash":
					cmd, _ := in["command"].(string)
					if strings.Contains(cmd, "stacklit") {
						p.stacklit++
					}
					desc += " " + truncate(cmd, 160)
				default:
					for _, k := range []string{"pattern", "path"} {
						if v, _ := in[k].(string); v != "" {
							desc += " " + k + "=" + truncate(v, 80)
						}
					}
				}
				p.log = append(p.log, desc)
			}
		case "user":
			for _, bl := range e.Message.Content {
				if bl.Type == "tool_result" && readIDs[bl.ToolUseID] {
					p.readBytes += contentLen(bl.Content)
				}
			}
		case "result":
			p.gotResult = true
			p.result, p.isError, p.subtype = e.Result, e.IsError, e.Subtype
			p.turns, p.cost, p.use = e.NumTurns, e.Cost, e.Usage
			p.denials = len(e.Denials)
			for m := range e.ModelUsage {
				p.model = m
			}
			if e.IsError && limitText.MatchString(e.Result) {
				p.stop = "result error: " + truncate(e.Result, 300)
			}
		}
	}
	if !p.gotResult {
		return p, errors.New("no result event in stream")
	}
	return p, nil
}

func fill(rec *record, p parsed) {
	rec.Answer = truncate(p.result, 2000)
	rec.Turns, rec.CostUSD, rec.Usage, rec.Model = p.turns, p.cost, p.use, p.model
	rec.ToolCalls, rec.FilesRead, rec.ReadBytes = p.calls, len(p.files), p.readBytes
	rec.StacklitCalls, rec.HookEvents, rec.Denials = p.stacklit, p.hooks, p.denials
	rec.Subtype, rec.WeekUtil, rec.Calls = p.subtype, p.util, p.log
	if p.isError && p.subtype == "" {
		rec.Error = fmt.Sprintf("agent error: %s", truncate(p.result, 200))
	}
}
