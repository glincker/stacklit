package main

import (
	"encoding/json"
	"os"
	"path"
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

// RunStacklit answers a task from the derived map plus targeted index
// lookups. Localization: modules are ranked on the hand-written terms, then
// within each of the top modules the agent opens files whose name overlaps the
// terms (else the first non-test file listed).
func RunStacklit(arm string, iv *IndexView, mapCost Cost, task Task, topN int, sizes RepoSizes, readCap, maxExtra int) ArmResult {
	res := ArmResult{Arm: arm, Lookup: mapCost}

	if task.Kind == "dependents" {
		mod, ok := iv.Modules[task.Target]
		if !ok {
			gradeDependents(&res, task, maxExtra)
			return res
		}
		res.Lookup = res.Lookup.Add(iv.entryCost(task.Target))
		if list, ok := mod["depended_by"].([]any); ok {
			for _, x := range list {
				if s, ok := x.(string); ok {
					res.Modules = append(res.Modules, s)
				}
			}
		}
		gradeDependents(&res, task, maxExtra)
		return res
	}

	ranked := iv.RankModules(task.Terms)
	if len(ranked) > topN {
		ranked = ranked[:topN]
	}
	for _, m := range ranked {
		res.Lookup = res.Lookup.Add(iv.entryCost(m))
		res.Ranked = append(res.Ranked, filesToOpen(iv, m, task.Terms)...)
		res.Shown = append(res.Shown, listedFiles(iv, m)...)
	}
	finishLocalization(&res, task, sizes, readCap)
	return res
}

// filesToOpen lists the files of one module an agent would open first.
func filesToOpen(iv *IndexView, module string, terms []string) []string {
	list, _ := iv.Modules[module]["file_list"].([]any)
	var cands []string
	for _, x := range list {
		s, _ := x.(string)
		if s == "" || strings.Contains(s, "_test.") || strings.Contains(s, ".test.") || strings.HasPrefix(s, "test_") {
			continue
		}
		cands = append(cands, s)
	}
	dir := module
	if module == "root" {
		dir = ""
	}
	var picked []string
	for _, c := range cands {
		lc := strings.ToLower(strings.TrimSuffix(c, path.Ext(c)))
		for _, t := range terms {
			if strings.Contains(lc, strings.ToLower(t)) {
				picked = append(picked, path.Join(dir, c))
				break
			}
		}
	}
	if len(picked) == 0 && len(cands) > 0 {
		picked = append(picked, path.Join(dir, cands[0]))
	}
	return picked
}

// listedFiles are the file names the module entry prints (the index keeps at
// most max_files names per module, so large modules show only a prefix).
func listedFiles(iv *IndexView, module string) []string {
	list, _ := iv.Modules[module]["file_list"].([]any)
	dir := module
	if module == "root" {
		dir = ""
	}
	out := []string{}
	for _, x := range list {
		if s, _ := x.(string); s != "" {
			out = append(out, path.Join(dir, s))
		}
	}
	return out
}
