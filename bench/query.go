package main

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
)

// queryDoc is the part of `stacklit query --json` the harness reads. The tool
// prints files grouped and sorted by path; it exposes no per-file relevance
// score, so relevance order cannot be read back from the output.
type queryDoc struct {
	Files []struct {
		File    string `json:"file"`
		Symbols []struct {
			Name string `json:"name"`
			Sig  string `json:"sig"`
			Doc  string `json:"doc"`
		} `json:"symbols"`
	} `json:"files"`
}

// RunQuery is the graph query arm: one `stacklit query --budget` call per
// task with the question text verbatim, no derive map, no follow-up lookups.
// It returns two results from the same call: the files opened in the order the
// tool prints them (path order), and the files opened in an order a reader
// picks from the printed text alone (see readerOrder).
func RunQuery(arm, readerArm, binary, dir string, task Task, modules []string, budget string, sizes RepoSizes, readCap, maxExtra int) (ArmResult, ArmResult) {
	tool := ArmResult{Arm: arm}
	reader := ArmResult{Arm: readerArm}
	run := func(extra ...string) []byte {
		args := append([]string{"query", task.Question, "--budget", budget}, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, _ := cmd.Output() // no match still exits cleanly; stderr notes are not read by the agent
		return out
	}
	tool.Lookup = CostOf(string(run()))
	reader.Lookup = tool.Lookup

	var doc queryDoc
	if err := json.Unmarshal(run("--json"), &doc); err != nil {
		doc = queryDoc{}
	}
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
		reader.NA = true
		return tool, reader
	}

	seen := map[string]bool{}
	for _, f := range doc.Files {
		if !seen[f.File] {
			seen[f.File] = true
			tool.Ranked = append(tool.Ranked, f.File)
		}
	}
	finishLocalization(&tool, task, sizes, readCap)

	reader.Ranked = readerOrder(doc, task.Question)
	finishLocalization(&reader, task, sizes, readCap)
	return tool, reader
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
