package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// BM25 parameters are the textbook defaults.
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

var (
	wordRe  = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*`)
	camelRe = regexp.MustCompile(`[A-Z]+[a-z0-9]*|[a-z0-9]+`)
)

// questionStop holds words that carry no signal in a "where is X" question.
var questionStop = map[string]bool{
	"where": true, "is": true, "the": true, "a": true, "an": true, "of": true, "in": true, "to": true,
	"what": true, "which": true, "for": true, "and": true, "how": true, "would": true, "i": true,
	"add": true, "implemented": true, "defined": true, "depends": true, "on": true, "are": true,
	"new": true, "such": true, "as": true, "or": true, "by": true, "that": true, "with": true,
	"from": true, "so": true, "it": true, "does": true, "do": true, "define": true, "register": true,
}

// tokenize lowercases identifiers and keeps each whole identifier plus its
// camelCase or snake_case parts, so "getValue" yields getvalue, get, value.
func tokenize(s string) []string {
	var out []string
	for _, w := range wordRe.FindAllString(s, -1) {
		lw := strings.ToLower(w)
		parts := camelRe.FindAllString(w, -1)
		if len(parts) > 1 {
			if len(lw) > 1 {
				out = append(out, lw)
			}
			for _, p := range parts {
				if len(p) > 1 {
					out = append(out, strings.ToLower(p))
				}
			}
		} else if len(lw) > 1 {
			out = append(out, lw)
		}
	}
	return out
}

func queryTokens(question string) []string {
	var out []string
	for _, t := range tokenize(question) {
		if !questionStop[t] {
			out = append(out, t)
		}
	}
	return out
}

// BM25Index is a plain in-memory BM25 over the searchable source files.
type BM25Index struct {
	files  []string
	tf     []map[string]int
	length []int
	df     map[string]int
	avgLen float64
}

// BuildBM25 indexes file paths and contents together.
func BuildBM25(files []SourceFile) *BM25Index {
	ix := &BM25Index{df: map[string]int{}}
	var total int
	for _, f := range files {
		toks := tokenize(f.Rel + "\n" + strings.Join(f.Lines, "\n"))
		tf := map[string]int{}
		for _, t := range toks {
			tf[t]++
		}
		for t := range tf {
			ix.df[t]++
		}
		ix.files = append(ix.files, f.Rel)
		ix.tf = append(ix.tf, tf)
		ix.length = append(ix.length, len(toks))
		total += len(toks)
	}
	if len(files) > 0 {
		ix.avgLen = float64(total) / float64(len(files))
	}
	return ix
}

// Rank returns the top k files for the query, ties broken by path.
func (ix *BM25Index) Rank(q []string, k int) []string {
	n := float64(len(ix.files))
	score := make([]float64, len(ix.files))
	for _, t := range q {
		df := float64(ix.df[t])
		if df == 0 {
			continue
		}
		idf := math.Log(1 + (n-df+0.5)/(df+0.5))
		for i, tf := range ix.tf {
			f := float64(tf[t])
			if f == 0 {
				continue
			}
			norm := f * (bm25K1 + 1) / (f + bm25K1*(1-bm25B+bm25B*float64(ix.length[i])/ix.avgLen))
			score[i] += idf * norm
		}
	}
	idx := make([]int, 0, len(score))
	for i, s := range score {
		if s > 0 {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if score[idx[a]] != score[idx[b]] {
			return score[idx[a]] > score[idx[b]]
		}
		return ix.files[idx[a]] < ix.files[idx[b]]
	})
	if len(idx) > k {
		idx = idx[:k]
	}
	out := make([]string, len(idx))
	for i, j := range idx {
		out[i] = ix.files[j]
	}
	return out
}

// RunBM25 retrieves files for the question text (no hand-written terms). The
// tool prints its top listed paths; the agent then opens them in rank order.
func RunBM25(arm string, ix *BM25Index, task Task, listed, ranked int, sizes RepoSizes, readCap int) ArmResult {
	if task.Kind == "dependents" {
		return ArmResult{Arm: arm, NA: true}
	}
	files := ix.Rank(queryTokens(task.Question), ranked)
	res := ArmResult{Arm: arm, Ranked: files}
	if listed > len(files) {
		listed = len(files)
	}
	res.Shown = files[:listed]
	res.Lookup = CostOf(strings.Join(files[:listed], "\n") + "\n")
	finishLocalization(&res, task, sizes, readCap)
	return res
}
