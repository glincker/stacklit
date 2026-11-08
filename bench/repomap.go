package main

import (
	"math"
	"path"
	"regexp"
	"sort"
	"strings"
)

// This file is an APPROXIMATION of a "repo map" as popularised by aider: a
// ranked list of definitions, personalised by identifiers the user mentions,
// fitted to a token budget. Differences from the real tool: definitions come
// from per-language regular expressions instead of tree-sitter queries, the
// render shows definition lines only (no surrounding context), and files
// without any definition are not appended. The ranking follows the published
// idea: file graph, edges from referencing to defining files weighted by
// sqrt(reference count) with the same multipliers, PageRank (damping 0.85),
// rank spread over edges. It is not the real tool and numbers must not be
// quoted as such.

var defPatterns = map[string][]*regexp.Regexp{
	".go": {
		regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)`),
		regexp.MustCompile(`^type\s+([A-Za-z_]\w*)`),
		regexp.MustCompile(`^(?:var|const)\s+([A-Za-z_]\w*)`),
	},
	".py": {
		regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)`),
		regexp.MustCompile(`^\s*class\s+(\w+)`),
	},
	".ts":  tsPatterns,
	".tsx": tsPatterns,
	".rs": {
		regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:(?:async|unsafe|const|extern)\s+)*(?:fn|struct|enum|trait|type|mod|static|const)\s+(\w+)`),
	},
	".java": {
		regexp.MustCompile(`^\s*(?:(?:public|protected|private|static|final|abstract|sealed)\s+)*(?:class|interface|enum|record)\s+(\w+)`),
		regexp.MustCompile(`^\s+(?:(?:public|protected|private|static|final|abstract|synchronized)\s+)+[\w<>\[\],.? ]+?\s+(\w+)\s*\(`),
	},
	".rb": {
		regexp.MustCompile(`^\s*def\s+(?:self\.)?(\w+[?!=]?)`),
		regexp.MustCompile(`^\s*(?:class|module)\s+(?:\w+::)*(\w+)`),
	},
}

var tsPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?(?:function\*?|class|interface|type|enum|const|let)\s+(\w+)`),
	regexp.MustCompile(`^\s+(?:(?:public|private|protected|static|async|readonly)\s+)*([A-Za-z_]\w*)\s*(?:<[^>]*>)?\([^)]*\)\s*(?::\s*[^{]+)?\{\s*$`),
}

var identRe = regexp.MustCompile(`[A-Za-z_]\w*`)

var defKeywords = map[string]bool{"if": true, "for": true, "while": true, "switch": true, "catch": true, "return": true, "function": true}

type mapDef struct {
	file string
	name string
	line int
	text string
}

// RepoMap holds the per-repo tags and reference counts, built once.
type RepoMap struct {
	files []string
	defs  []mapDef
	byID  map[string][]int   // identifier -> indexes into defs
	refs  map[string][]refCt // identifier -> files referencing it
}

type refCt struct {
	file  int
	count int
}

// BuildRepoMap extracts definitions and reference counts from the sources.
func BuildRepoMap(files []SourceFile) *RepoMap {
	rm := &RepoMap{byID: map[string][]int{}, refs: map[string][]refCt{}}
	for fi, f := range files {
		rm.files = append(rm.files, f.Rel)
		pats := defPatterns[path.Ext(f.Rel)]
		counts := map[string]int{}
		for i, line := range f.Lines {
			for _, p := range pats {
				if m := p.FindStringSubmatch(line); m != nil && !defKeywords[m[1]] {
					rm.byID[m[1]] = append(rm.byID[m[1]], len(rm.defs))
					rm.defs = append(rm.defs, mapDef{file: f.Rel, name: m[1], line: i, text: clip(strings.TrimRight(line, " \t"), 120)})
					break
				}
			}
			for _, id := range identRe.FindAllString(line, -1) {
				counts[id]++
			}
		}
		for id, c := range counts {
			rm.refs[id] = append(rm.refs[id], refCt{fi, c})
		}
	}
	for _, list := range rm.refs {
		sort.Slice(list, func(i, j int) bool { return list[i].file < list[j].file })
	}
	return rm
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

var snakeOrCamel = regexp.MustCompile(`[a-z][A-Z]|_`)

// Render ranks definitions for a question and fits them to the token budget.
func (rm *RepoMap) Render(question string, budget int) (string, []string) {
	mentioned := map[string]bool{}
	for _, w := range identRe.FindAllString(question, -1) {
		mentioned[strings.ToLower(w)] = true
	}

	type edge struct {
		to int
		w  float64
		id string
	}
	n := len(rm.files)
	fileIdx := map[string]int{}
	for i, f := range rm.files {
		fileIdx[f] = i
	}
	out := make([][]edge, n)
	ids := make([]string, 0, len(rm.byID))
	for id := range rm.byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		mul := 1.0
		if mentioned[strings.ToLower(id)] {
			mul *= 10
		}
		if len(id) >= 8 && snakeOrCamel.MatchString(id) {
			mul *= 10
		}
		if strings.HasPrefix(id, "_") {
			mul *= 0.1
		}
		definers := map[int]bool{}
		for _, di := range rm.byID[id] {
			definers[fileIdx[rm.defs[di].file]] = true
		}
		if len(definers) > 5 {
			mul *= 0.1
		}
		dlist := make([]int, 0, len(definers))
		for d := range definers {
			dlist = append(dlist, d)
		}
		sort.Ints(dlist)
		for _, r := range rm.refs[id] {
			for _, d := range dlist {
				if d != r.file {
					out[r.file] = append(out[r.file], edge{d, mul * math.Sqrt(float64(r.count)), id})
				}
			}
		}
	}

	rank := make([]float64, n)
	for i := range rank {
		rank[i] = 1 / float64(n)
	}
	outW := make([]float64, n)
	for i, es := range out {
		for _, e := range es {
			outW[i] += e.w
		}
	}
	const damping = 0.85
	for it := 0; it < 50; it++ {
		next := make([]float64, n)
		var dangling float64
		for i := range rank {
			if outW[i] == 0 {
				dangling += rank[i]
				continue
			}
			for _, e := range out[i] {
				next[e.to] += rank[i] * e.w / outW[i]
			}
		}
		for i := range next {
			next[i] = (1-damping)/float64(n) + damping*(next[i]+dangling/float64(n))
		}
		rank = next
	}

	defScore := map[int]float64{}
	for i, es := range out {
		for _, e := range es {
			for _, di := range rm.byID[e.id] {
				if rm.defs[di].file == rm.files[e.to] {
					defScore[di] += rank[i] * e.w / outW[i]
				}
			}
		}
	}
	order := make([]int, 0, len(defScore))
	for di := range defScore {
		order = append(order, di)
	}
	sort.Slice(order, func(a, b int) bool {
		sa, sb := defScore[order[a]], defScore[order[b]]
		if sa != sb {
			return sa > sb
		}
		return order[a] < order[b]
	})

	render := func(k int) (string, []string) {
		byFile := map[string][]mapDef{}
		var fileOrder []string
		for _, di := range order[:k] {
			d := rm.defs[di]
			if _, ok := byFile[d.file]; !ok {
				fileOrder = append(fileOrder, d.file)
			}
			byFile[d.file] = append(byFile[d.file], d)
		}
		var b strings.Builder
		for _, f := range fileOrder {
			ds := byFile[f]
			sort.Slice(ds, func(i, j int) bool { return ds[i].line < ds[j].line })
			b.WriteString(f + ":\n")
			for _, d := range ds {
				b.WriteString("| " + d.text + "\n")
			}
			b.WriteString("\n")
		}
		return b.String(), fileOrder
	}
	// Largest prefix of the ranking that fits the budget (binary search).
	lo, hi := 0, len(order)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		txt, _ := render(mid)
		if CostOf(txt).Tokens() <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return render(lo)
}

// RunRepoMap shows the budgeted map for the task's question and lets the agent
// open the files in map order. Dependents tasks are not applicable.
func RunRepoMap(arm string, rm *RepoMap, task Task, budget int, sizes RepoSizes, readCap int) ArmResult {
	if task.Kind == "dependents" {
		return ArmResult{Arm: arm, NA: true}
	}
	txt, files := rm.Render(task.Question, budget)
	res := ArmResult{Arm: arm, Lookup: CostOf(txt), Ranked: files}
	finishLocalization(&res, task, sizes, readCap)
	return res
}
