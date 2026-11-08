package main

import (
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

// queryDoc is the printed `stacklit query` text, parsed back into files and
// symbol lines. The harness reads the text, not --json, because the agent sees
// the text and the two are cut to the same token budget at different sizes.
type queryDoc struct {
	Files []struct {
		File    string
		Symbols []struct{ Name, Sig, Doc string }
	}
}

var (
	rankPrefix = regexp.MustCompile(`^\d+\. `)
	symLine    = regexp.MustCompile(`^  \d+(?:-\d+)? (.*)$`)
)

// parseQueryText reads files in printed order. Unindented lines are file paths
// (optionally "N. " ranked) except the title, head and bracketed notice lines.
func parseQueryText(out []byte) queryDoc {
	var doc queryDoc
	for i, line := range strings.Split(string(out), "\n") {
		switch {
		case line == "" || i == 0:
		case strings.HasPrefix(line, "      "):
		case symLine.MatchString(line):
			if n := len(doc.Files); n > 0 {
				label, docText, _ := strings.Cut(symLine.FindStringSubmatch(line)[1], "  // ")
				doc.Files[n-1].Symbols = append(doc.Files[n-1].Symbols, struct{ Name, Sig, Doc string }{label, label, docText})
			}
		case line[0] == '[' || strings.HasPrefix(line, "best match: ") || strings.Contains(line, " seeds, ") || line[0] == ' ':
		default:
			doc.Files = append(doc.Files, struct {
				File    string
				Symbols []struct{ Name, Sig, Doc string }
			}{File: rankPrefix.ReplaceAllString(line, "")})
		}
	}
	return doc
}

// RunQuery is the graph query arm: one `stacklit query --budget` call per
// task with the question text verbatim, no derive map, no follow-up lookups.
// It returns three results: files opened in the order the tool prints them by
// default (relevance order), files opened in the order of `--order path` (the
// tool's behaviour before ranked output, kept for comparison), and files opened
// in an order a reader picks from the path-ordered text alone (see readerOrder).
func RunQuery(arm, pathArm, readerArm, binary, dir string, task Task, modules []string, budget string, sizes RepoSizes, readCap, maxExtra int) (ArmResult, ArmResult, ArmResult) {
	tool := ArmResult{Arm: arm}
	byPath := ArmResult{Arm: pathArm}
	reader := ArmResult{Arm: readerArm}
	run := func(extra ...string) []byte {
		args := append([]string{"query", task.Question, "--budget", budget}, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, _ := cmd.Output() // no match still exits cleanly; stderr notes are not read by the agent
		return out
	}
	text, pathText := run(), run("--order", "path")
	tool.Lookup, byPath.Lookup = CostOf(string(text)), CostOf(string(pathText))
	reader.Lookup = byPath.Lookup

	doc, pathDoc := parseQueryText(text), parseQueryText(pathText)
	if task.Kind == "dependents" {
		mods := map[string]bool{}
		for _, f := range doc.Files {
			mods[moduleOfFile(f.File, modules)] = true
		}
		for m := range mods {
			tool.Modules = append(tool.Modules, m)
		}
		sort.Strings(tool.Modules)
		gradeDependents(&tool, task, maxExtra)
		pm := map[string]bool{}
		for _, f := range pathDoc.Files {
			pm[moduleOfFile(f.File, modules)] = true
		}
		for m := range pm {
			byPath.Modules = append(byPath.Modules, m)
		}
		sort.Strings(byPath.Modules)
		gradeDependents(&byPath, task, maxExtra)
		reader.NA = true
		return tool, byPath, reader
	}

	ranked := func(d queryDoc) []string {
		seen := map[string]bool{}
		var out []string
		for _, f := range d.Files {
			if !seen[f.File] {
				seen[f.File] = true
				out = append(out, f.File)
			}
		}
		return out
	}
	tool.Ranked = ranked(doc)
	finishLocalization(&tool, task, sizes, readCap)
	byPath.Ranked = ranked(pathDoc)
	finishLocalization(&byPath, task, sizes, readCap)

	reader.Ranked = readerOrder(pathDoc, task.Question)
	finishLocalization(&reader, task, sizes, readCap)
	return tool, byPath, reader
}

// readerOrder ranks the printed files from the question and the printed text
// only: 3 per question word found in the path, 2 per question word found in a
// symbol name, 1 per question word found in a signature or doc line. No gold
// answer, hand-written term or file contents are used. Ties keep path order.
func readerOrder(doc queryDoc, question string) []string {
	terms := queryTokens(question)
	type scored struct {
		file  string
		score int
	}
	all := make([]scored, 0, len(doc.Files))
	for _, f := range doc.Files {
		lp := strings.ToLower(f.File)
		s := 0
		for _, t := range terms {
			if strings.Contains(lp, t) {
				s += 3
			}
		}
		for _, sym := range f.Symbols {
			ln := strings.ToLower(sym.Name)
			lt := strings.ToLower(sym.Sig + " " + sym.Doc)
			for _, t := range terms {
				if strings.Contains(ln, t) {
					s += 2
				} else if strings.Contains(lt, t) {
					s++
				}
			}
		}
		all = append(all, scored{f.File, s})
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	out := make([]string, len(all))
	for i, s := range all {
		out[i] = s.file
	}
	return out
}
