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

// routedRules is steering variant e: it sends exact-name lookups to grep or
// impact and asks for a neighbour check on multi-file answers, because the
// first A/B showed a single query being taken as the whole answer.
const routedRules = "Use stacklit when you do not know where something is or how things connect: `stacklit query \"<question>\" --budget 800`.\n" +
	"Not for an exact name or a small file: use grep or read it. `stacklit impact <name>`: importers and callers, file:line.\n" +
	"For several files, check the neighbours with `stacklit explain <symbol>` or `stacklit path <from> <to>`. One query is a lead, not the answer.\n" +
	"Read one symbol with `stacklit slice <symbol>`.\n"

// guideRulesE is the guide's rules text under the shipped wording.
const guideRulesE = routedRules +
	"If stacklit reports a stale index, run `stacklit refresh --files <paths>`.\n"

// GuideRules returns the guide's rules text for a steering variant. The
// strict no-crawl wording stays available as variant d for comparison; every
// other value gets the shipped wording.
func GuideRules(variant string) string {
	if strings.ToLower(strings.TrimSpace(variant)) == "d" {
		return strictRules
	}
	return guideRulesE
}

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
		{"stacklit impact <name>", "Who imports a file or package, or calls a symbol, with file:line. Run before editing shared code."},
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

// GuideText is the full agent usage guide under the shipped wording.
func GuideText() string { return GuideTextFor("") }

// GuideTextFor is the agent usage guide, plain text, byte stable, with the
// rules text of one steering variant.
func GuideTextFor(variant string) string {
	var b strings.Builder
	b.WriteString("# Stacklit agent guide\n\n")
	b.WriteString(GuideRules(variant))
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
func GuideJSON() string { return GuideJSONFor("") }

// GuideJSONFor is GuideJSON for one steering variant.
func GuideJSONFor(variant string) string {
	out := struct {
		Rules    string         `json:"rules"`
		Commands []GuideCommand `json:"commands"`
		Examples []string       `json:"examples"`
		Text     string         `json:"text"`
	}{GuideRules(variant), GuideCommands(), guideExamples, GuideTextFor(variant)}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b) + "\n"
}
