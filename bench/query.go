package main

import (
	"encoding/json"
	"os/exec"
	"sort"
)

// RunQuery is the graph query arm: one `stacklit query --budget` call per
// task with the question text verbatim, no derive map, no follow-up lookups.
// Files are read in the order the tool returns them.
func RunQuery(arm, binary, dir string, task Task, modules []string, budget string, sizes RepoSizes, readCap, maxExtra int) ArmResult {
	res := ArmResult{Arm: arm}
	run := func(extra ...string) []byte {
		args := append([]string{"query", task.Question, "--budget", budget}, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, _ := cmd.Output() // no match still exits cleanly; stderr notes are not read by the agent
		return out
	}
	res.Lookup = CostOf(string(run()))

	var doc struct {
		Files []struct {
			File string `json:"file"`
		} `json:"files"`
	}
	if err := json.Unmarshal(run("--json"), &doc); err != nil {
		if task.Kind == "dependents" {
			gradeDependents(&res, task, maxExtra)
		} else {
			finishLocalization(&res, task, sizes, readCap)
		}
		return res
	}
	seen := map[string]bool{}
	for _, f := range doc.Files {
		if !seen[f.File] {
			seen[f.File] = true
			res.Ranked = append(res.Ranked, f.File)
		}
	}
	if task.Kind == "dependents" {
		mods := map[string]bool{}
		for _, f := range res.Ranked {
			mods[moduleOfFile(f, modules)] = true
		}
		for m := range mods {
			res.Modules = append(res.Modules, m)
		}
		sort.Strings(res.Modules)
		res.Ranked = nil
		gradeDependents(&res, task, maxExtra)
		return res
	}
	finishLocalization(&res, task, sizes, readCap)
	return res
}
