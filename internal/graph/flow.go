package graph

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

// A query whose answer is a set of files (a flow or a chain) is easy to
// misread as a single-file answer, because the top result looks authoritative.
// These helpers decide when to add one line naming the next candidate files.
// The word list is an input, not a tuning knob: STACKLIT_FLOW_WORDS replaces it.
const defaultFlowWords = "trace,flow,through,path,calls,caller,callers,wake,lifecycle,chain,pipeline,sequence,end to end,who calls,takes part,take part"

// Defaults for the spread test. All overridable by environment:
// STACKLIT_QUERY_NEXT (0 turns the line off), STACKLIT_QUERY_SPREAD_DIRS,
// STACKLIT_QUERY_SPREAD_TOP and STACKLIT_QUERY_FLATNESS.
const (
	defaultNextFiles   = 4
	defaultSpreadDirs  = 4
	defaultSpreadTop   = 5
	defaultFlatness    = 0.5
	maxNextFilesCapped = 5
)

// envNonNeg reads a non-negative integer; unlike envInt it accepts 0, which
// switches a feature off.
func envNonNeg(name string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name))); err == nil && v >= 0 {
		return v
	}
	return def
}

// nextFilesWanted is how many candidate files the line lists; 0 means off.
func nextFilesWanted() int {
	n := envNonNeg("STACKLIT_QUERY_NEXT", defaultNextFiles)
	if n > maxNextFilesCapped {
		n = maxNextFilesCapped
	}
	return n
}

func flowWords() []string {
	list := defaultFlowWords
	if v := os.Getenv("STACKLIT_FLOW_WORDS"); strings.TrimSpace(v) != "" {
		list = v
	}
	var out []string
	for _, w := range strings.Split(list, ",") {
		if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// questionSuggestsFlow reports whether the wording asks for a chain of files.
// Matching is on whole words so "workflow" does not count as "flow".
func questionSuggestsFlow(q string) bool {
	padded := " " + strings.Join(words(q), " ") + " "
	for _, w := range flowWords() {
		if strings.Contains(padded, " "+strings.Join(strings.Fields(w), " ")+" ") {
			return true
		}
	}
	return false
}

// fileScore is the best symbol score of one file in a ranked candidate list.
type fileScore struct {
	file  string
	score float64
}

// spreadsDirs reports whether the top files lie in several directories and
// their scores are flat (the last of them close to the first), the shape of a
// multi-file answer. A clear winner is a single answer, however scattered the tail.
func spreadsDirs(top []fileScore) bool {
	n := envNonNeg("STACKLIT_QUERY_SPREAD_TOP", defaultSpreadTop)
	if n < 2 || len(top) < 2 {
		return false
	}
	if len(top) > n {
		top = top[:n]
	}
	if top[len(top)-1].score < envFloat("STACKLIT_QUERY_FLATNESS", defaultFlatness)*top[0].score {
		return false
	}
	dirs := map[string]bool{}
	for _, f := range top {
		dirs[path.Dir(f.file)] = true
	}
	return len(dirs) >= envNonNeg("STACKLIT_QUERY_SPREAD_DIRS", defaultSpreadDirs)
}

// rankFiles orders files by their best item score, ties by path.
func (g *SymGraph) rankFiles(ids []int, score func(int) float64) []fileScore {
	best := map[string]float64{}
	for _, id := range ids {
		f := g.Nodes[id].File
		if s := score(id); s > best[f] {
			best[f] = s
		}
	}
	out := make([]fileScore, 0, len(best))
	for f, s := range best {
		out = append(out, fileScore{f, s})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].score != out[b].score {
			return out[a].score > out[b].score
		}
		return out[a].file < out[b].file
	})
	return out
}

// nextCandidates lists up to n files that rank just below the k shown items,
// and returns the text line that names them and the commands to expand.
func (g *SymGraph) nextCandidates(items []item, k int, shown map[string]bool, n int) (files []string, line string) {
	if n <= 0 || k >= len(items) {
		return nil, ""
	}
	seen := map[string]bool{}
	var firstSym string
	for _, it := range items[k:] {
		f := g.Nodes[it.node].File
		if shown[f] || seen[f] {
			continue
		}
		seen[f] = true
		if firstSym == "" {
			firstSym = g.Nodes[it.node].Name
		}
		files = append(files, f)
		if len(files) == n {
			break
		}
	}
	if len(files) == 0 {
		return nil, ""
	}
	top := g.Nodes[items[0].node].Name
	line = fmt.Sprintf("several files may take part; also ranked: %s. Expand: stacklit explain %s | stacklit path %s %s",
		strings.Join(files, ", "), top, top, firstSym)
	return files, line
}
