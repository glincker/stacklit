package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// BaselineParams are the knobs of the simulated grep+read agent.
type BaselineParams struct {
	MaxGrepLines int // grep output lines the agent sees
	MaxLineChars int // each output line is cut to this length
	ReadTopFiles int // files read in full after grepping
}

// DefaultBaseline is deliberately generous to the baseline: test files are
// excluded from the search and files are ranked by definition-like matches.
var DefaultBaseline = BaselineParams{MaxGrepLines: 50, MaxLineChars: 200, ReadTopFiles: 3}

// SourceFile is a searchable file of a checked-out repo.
type SourceFile struct {
	Rel   string
	Lines []string
	Size  int
}

// BaselineResult is the outcome of the grep+read procedure for one task.
type BaselineResult struct {
	GrepCost     Cost
	ReadCost     Cost
	HitFiles     []string // files surfaced by grep (after the line cap), ranked
	ReadFiles    []string
	FoundInGrep  bool
	FoundInRead  bool
	Modules      []string // dependents tasks: modules inferred from hit files
	Precision    float64
	Recall       float64
	DependentsOK bool
}

// LoadSources reads all searchable source files of a repo, skipping tests,
// vendored code and stacklit's own outputs.
func LoadSources(root string, repo RepoSpec) ([]SourceFile, error) {
	var out []SourceFile
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", ".venv", "dist":
				return filepath.SkipDir
			}
			return nil
		}
		if !hasExt(rel, repo.Exts) {
			return nil
		}
		slashed := "/" + rel
		for _, m := range repo.TestMarkers {
			if strings.Contains(slashed, m) {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		var lines []string
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			lines = append(lines, sc.Text())
		}
		out = append(out, SourceFile{Rel: rel, Lines: lines, Size: int(info.Size())})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, err
}

func hasExt(rel string, exts []string) bool {
	for _, e := range exts {
		if strings.HasSuffix(rel, e) {
			return true
		}
	}
	return false
}

// pathMatches reports whether rel satisfies an expected path: an exact file or
// a directory prefix ending in "/".
func pathMatches(rel, expected string) bool {
	if strings.HasSuffix(expected, "/") {
		return strings.HasPrefix(rel, expected)
	}
	return rel == expected
}

// RunBaseline simulates `grep -rn PATTERN`, then reading the best-ranked files.
func RunBaseline(files []SourceFile, task Task, modules []string, p BaselineParams) (BaselineResult, error) {
	pat := task.Pattern
	if task.Word {
		pat = `\b(?:` + pat + `)\b`
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return BaselineResult{}, fmt.Errorf("task %s pattern: %w", task.ID, err)
	}
	defRe := regexp.MustCompile(`\b(func|def|class|type|interface|const|let|var|function|struct|enum|trait)\b.*(?:` + task.Pattern + `)`)

	type hit struct {
		file string
		line string
	}
	var hits []hit
	score := map[string]int{}
	for _, f := range files {
		for i, line := range f.Lines {
			if !re.MatchString(line) {
				continue
			}
			text := strings.TrimSpace(line)
			if len(text) > p.MaxLineChars {
				text = text[:p.MaxLineChars]
			}
			hits = append(hits, hit{f.Rel, fmt.Sprintf("%s:%d:%s", f.Rel, i+1, text)})
			score[f.Rel]++
			if defRe.MatchString(line) {
				score[f.Rel] += 100
			}
		}
	}

	res := BaselineResult{}
	var out strings.Builder
	seen := map[string]bool{}
	for i, h := range hits {
		if i >= p.MaxGrepLines {
			break
		}
		out.WriteString(h.line)
		out.WriteString("\n")
		if !seen[h.file] {
			seen[h.file] = true
			res.HitFiles = append(res.HitFiles, h.file)
		}
	}
	res.GrepCost = CostOf(out.String())

	if task.Kind == "dependents" {
		inferred := map[string]bool{}
		for _, hf := range res.HitFiles {
			inferred[moduleOfFile(hf, modules)] = true
		}
		for m := range inferred {
			res.Modules = append(res.Modules, m)
		}
		sort.Strings(res.Modules)
		res.Precision, res.Recall = precisionRecall(res.Modules, task.ExpectModules, false)
		res.DependentsOK = res.Recall == 1
		return res, nil
	}

	ranked := make([]string, 0, len(score))
	for f := range score {
		ranked = append(ranked, f)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if score[ranked[i]] != score[ranked[j]] {
			return score[ranked[i]] > score[ranked[j]]
		}
		return ranked[i] < ranked[j]
	})
	sizes := map[string]*SourceFile{}
	for i := range files {
		sizes[files[i].Rel] = &files[i]
	}
	for i, f := range ranked {
		if i >= p.ReadTopFiles {
			break
		}
		res.ReadFiles = append(res.ReadFiles, f)
		sf := sizes[f]
		res.ReadCost = res.ReadCost.Add(Cost{Bytes: sf.Size, Pieces: pieceCount(sf.Lines)})
	}
	for _, e := range task.ExpectPaths {
		for _, f := range res.HitFiles {
			if pathMatches(f, e) {
				res.FoundInGrep = true
			}
		}
		for _, f := range res.ReadFiles {
			if pathMatches(f, e) {
				res.FoundInRead = true
			}
		}
	}
	return res, nil
}

func pieceCount(lines []string) int {
	return CostOf(strings.Join(lines, "\n")).Pieces
}

// moduleOfFile maps a file to the longest module directory containing it.
func moduleOfFile(rel string, modules []string) string {
	best := "root"
	bestLen := -1
	for _, m := range modules {
		dir := m
		if m == "root" {
			dir = ""
		}
		if dir == "" {
			if !strings.Contains(rel, "/") && bestLen < 0 {
				best, bestLen = m, 0
			}
			continue
		}
		if strings.HasPrefix(rel, dir+"/") && len(dir) > bestLen {
			best, bestLen = m, len(dir)
		}
	}
	if bestLen < 0 {
		// Not under any known module: use the directory itself.
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			return rel[:i]
		}
	}
	return best
}

// precisionRecall compares found modules with expected ones. When loose is set,
// index entries may be short names (a trailing path segment), as stacklit emits.
func precisionRecall(found, expected []string, loose bool) (precision, recall float64) {
	matches := func(f, e string) bool {
		if f == e {
			return true
		}
		if strings.HasSuffix(e, "/") && strings.HasPrefix(f, e) {
			return true
		}
		return loose && strings.HasSuffix(e, "/"+f)
	}
	if len(found) == 0 {
		return 0, 0
	}
	tp := 0
	for _, f := range found {
		for _, e := range expected {
			if matches(f, e) {
				tp++
				break
			}
		}
	}
	hitE := 0
	for _, e := range expected {
		for _, f := range found {
			if matches(f, e) {
				hitE++
				break
			}
		}
	}
	return float64(tp) / float64(len(found)), float64(hitE) / float64(len(expected))
}
