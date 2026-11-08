package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// IndexView is the part of stacklit.json the benchmark needs. It is read as
// generic JSON so the harness does not import stacklit's internal packages.
type IndexView struct {
	Modules map[string]map[string]any
	Raw     []byte
}

// LoadIndex reads stacklit.json from a generated repo.
func LoadIndex(file string) (*IndexView, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Modules map[string]map[string]any `json:"modules"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return &IndexView{Modules: doc.Modules, Raw: raw}, nil
}

// ModuleNames returns module keys, sorted.
func (iv *IndexView) ModuleNames() []string {
	names := make([]string, 0, len(iv.Modules))
	for n := range iv.Modules {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// entryCost is the cost of one get_module style response.
func (iv *IndexView) entryCost(name string) Cost {
	b, _ := json.Marshal(map[string]any{"name": name, "module": iv.Modules[name]})
	return CostOf(string(b))
}

func flatten(v any, out *[]string) {
	switch t := v.(type) {
	case string:
		*out = append(*out, t)
	case []any:
		for _, x := range t {
			flatten(x, out)
		}
	case map[string]any:
		for k, x := range t {
			*out = append(*out, k)
			flatten(x, out)
		}
	}
}

// RankModules scores modules against query terms: a name hit is worth 3, a
// purpose hit 2, a hit in exports, type definitions or file names 1. This
// stands in for an agent that reads the index and picks candidates; it is
// more capable than the MCP find_module tool, which matches names and purpose.
func (iv *IndexView) RankModules(terms []string) []string {
	type scored struct {
		name  string
		score int
	}
	var all []scored
	for name, mod := range iv.Modules {
		lname := strings.ToLower(name)
		purpose, _ := mod["purpose"].(string)
		lpurpose := strings.ToLower(purpose)
		var body []string
		for _, key := range []string{"exports", "type_defs", "file_list"} {
			flatten(mod[key], &body)
		}
		lbody := strings.ToLower(strings.Join(body, "\n"))
		s := 0
		for _, t := range terms {
			lt := strings.ToLower(t)
			if strings.Contains(lname, lt) {
				s += 3
			}
			if strings.Contains(lpurpose, lt) {
				s += 2
			}
			if strings.Contains(lbody, lt) {
				s++
			}
		}
		if s > 0 {
			all = append(all, scored{name, s})
		}
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].name < all[j].name
	})
	out := make([]string, len(all))
	for i, s := range all {
		out[i] = s.name
	}
	return out
}

// StacklitResult is the outcome of answering one task from the index.
type StacklitResult struct {
	MapCost     Cost
	QueryCost   Cost
	ConfirmCost Cost
	TopModules  []string
	ConfirmFile string
	Found       bool
	DependedBy  []string
	Precision   float64
	Recall      float64
}

// RunStacklit answers a task from the derived map plus targeted index lookups.
func RunStacklit(iv *IndexView, mapCost Cost, repoDir string, task Task, topN int) StacklitResult {
	res := StacklitResult{MapCost: mapCost}

	if task.Kind == "dependents" {
		mod, ok := iv.Modules[task.Target]
		if !ok {
			return res
		}
		res.QueryCost = iv.entryCost(task.Target)
		if list, ok := mod["depended_by"].([]any); ok {
			for _, x := range list {
				if s, ok := x.(string); ok {
					res.DependedBy = append(res.DependedBy, s)
				}
			}
		}
		res.Precision, res.Recall = precisionRecall(res.DependedBy, task.ExpectModules, true)
		res.Found = res.Recall == 1
		return res
	}

	ranked := iv.RankModules(task.Terms)
	if len(ranked) > topN {
		ranked = ranked[:topN]
	}
	res.TopModules = ranked
	for _, m := range ranked {
		res.QueryCost = res.QueryCost.Add(iv.entryCost(m))
		for _, e := range task.ExpectModules {
			if m == e || (strings.HasSuffix(e, "/") && strings.HasPrefix(m, e)) {
				res.Found = true
			}
		}
	}
	if len(ranked) > 0 {
		res.ConfirmFile, res.ConfirmCost = confirmRead(iv, repoDir, ranked[0], task.Terms)
	}
	return res
}

// confirmRead picks one file of the best module (name overlap with the terms,
// else the first non-test file) and measures reading it in full.
func confirmRead(iv *IndexView, repoDir, module string, terms []string) (string, Cost) {
	list, _ := iv.Modules[module]["file_list"].([]any)
	var cands []string
	for _, x := range list {
		s, _ := x.(string)
		if s == "" || strings.Contains(s, "_test.") || strings.Contains(s, ".test.") || strings.HasPrefix(s, "test_") {
			continue
		}
		cands = append(cands, s)
	}
	if len(cands) == 0 {
		return "", Cost{}
	}
	pick := cands[0]
	for _, c := range cands {
		lc := strings.ToLower(strings.TrimSuffix(c, path.Ext(c)))
		for _, t := range terms {
			if strings.Contains(lc, strings.ToLower(t)) {
				pick = c
				goto done
			}
		}
	}
done:
	dir := module
	if module == "root" {
		dir = ""
	}
	full := filepath.Join(repoDir, filepath.FromSlash(dir), pick)
	b, err := os.ReadFile(full)
	if err != nil {
		return fmt.Sprintf("%s (missing)", pick), Cost{}
	}
	return path.Join(dir, pick), CostOf(string(b))
}
