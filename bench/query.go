package main

import (
	"encoding/json"
	"os/exec"
	"strings"
)

// QueryResult is the third arm: one `stacklit query --budget` call per task,
// no derive map and no follow-up lookups.
type QueryResult struct {
	Cost    Cost
	Files   []string
	Found   bool
	Recall  float64 // dependents tasks: share of expected modules with a file in the output
	Covered []string
}

// RunQuery asks the graph query path the task's own question text and grades
// the files it returns against the hand-verified answer.
func RunQuery(binary, dir string, task Task, modules []string, budget string) QueryResult {
	var res QueryResult
	run := func(extra ...string) []byte {
		args := append([]string{"query", task.Question, "--budget", budget}, extra...)
		cmd := exec.Command(binary, args...)
		cmd.Dir = dir
		out, _ := cmd.Output() // a query with no match still exits cleanly; stderr notes are not read by the agent
		return out
	}
	res.Cost = CostOf(string(run()))

	var doc struct {
		Files []struct {
			File string `json:"file"`
		} `json:"files"`
	}
	if err := json.Unmarshal(run("--json"), &doc); err != nil {
		return res
	}
	for _, f := range doc.Files {
		res.Files = append(res.Files, f.File)
	}

	if task.Kind == "dependents" {
		seen := map[string]bool{}
		for _, f := range res.Files {
			seen[moduleOfFile(f, modules)] = true
		}
		hit := 0
		for _, m := range task.ExpectModules {
			if seen[m] {
				hit++
				res.Covered = append(res.Covered, m)
			}
		}
		if len(task.ExpectModules) > 0 {
			res.Recall = float64(hit) / float64(len(task.ExpectModules))
		}
		res.Found = len(task.ExpectModules) > 0 && hit == len(task.ExpectModules)
		return res
	}
	for _, f := range res.Files {
		for _, p := range task.ExpectPaths {
			if f == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(f, p)) {
				res.Found = true
			}
		}
	}
	return res
}
