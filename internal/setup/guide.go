package setup

import (
	"encoding/json"
	"strings"
)

// strictRules is steering variant d and the rules text of the agent guide, so
// the CLAUDE.md block and `stacklit guide` can never drift apart.
const strictRules = "Stacklit is the source of truth for this repo's structure. Do not crawl the repo.\n" +
	"- Structure questions: `stacklit query \"<question>\" --budget 800`, then `stacklit explain <symbol>`, `stacklit impact <symbol>` (before editing shared code) or `stacklit path <from> <to>`.\n" +
	"- Do not run recursive listings, find, tree or repo wide grep to learn structure. Open a file only after the map points to it, and read it with `stacklit slice <symbol>`.\n" +
	"- If stacklit reports a stale index or a missing symbol, run `stacklit refresh --files <paths>` or `stacklit generate`.\n" +
	"- Subagents follow the same rules.\n"

// GuideCommand is one row of the command list in the guide.
type GuideCommand struct {
	Command string `json:"command"`
	Use     string `json:"use"`
}

// GuideCommands lists the commands an agent should reach for, in order.
func GuideCommands() []GuideCommand {
	return []GuideCommand{
		{`stacklit query "<question>" --budget 800`, "Ranked files and symbols for a question, within a token budget. Start here."},
		{"stacklit explain <symbol>", "Definition, callers and callees of one symbol."},
		{"stacklit impact <symbol|file>", "What depends on it. Run before editing shared code."},
		{"stacklit path <from> <to>", "How two symbols connect."},
		{"stacklit slice <symbol>", "Source of one symbol. Use instead of reading the whole file."},
		{"stacklit report", "Repo overview: hubs, communities, surprises."},
		{"stacklit status", "Is the index current against git HEAD."},
		{"stacklit refresh --files <paths>", "Update the index for just the files that changed."},
		{"stacklit generate --graph", "Rebuild the index and symbol graph."},
	}
}

var guideExamples = []string{
	`Where is login handled?   stacklit query "where is login handled" --budget 800`,
	`Who calls parseToken?     stacklit explain parseToken`,
	`Change Config safely?     stacklit impact Config`,
	`Read one function?        stacklit slice parseToken`,
}

// GuideText is the full agent usage guide, plain text, byte stable.
func GuideText() string {
	var b strings.Builder
	b.WriteString("# Stacklit agent guide\n\n")
	b.WriteString(strictRules)
	b.WriteString("\n## Commands\n")
	for _, c := range GuideCommands() {
		b.WriteString("- `" + c.Command + "`: " + c.Use + "\n")
	}
	b.WriteString("\n## Examples\n")
	for _, e := range guideExamples {
		b.WriteString("- " + e + "\n")
	}
	return b.String()
}

// GuideJSON is the same guide as JSON for tools that want structured input.
func GuideJSON() string {
	out := struct {
		Rules    string         `json:"rules"`
		Commands []GuideCommand `json:"commands"`
		Examples []string       `json:"examples"`
		Text     string         `json:"text"`
	}{strictRules, GuideCommands(), guideExamples, GuideText()}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b) + "\n"
}
