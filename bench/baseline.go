package main

import (
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
}

// DefaultBaseline is deliberately generous to the baseline: test files are
// excluded from the search, the pattern is hand written per task, and files
// are ranked by definition-like matches.
var DefaultBaseline = BaselineParams{MaxGrepLines: 50, MaxLineChars: 200}

// SourceFile is a searchable file of a checked-out repo.
type SourceFile struct {
	Rel   string
	Lines []string
	Size  int
}

// skipDir lists directories no arm looks into.
func skipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".venv", "dist", "target":
		return true
	}
	return false
}

// LoadSources reads the searchable (non-test) source files of a repo and the
// size of every code file including tests. Run it before generating so the
// stacklit outputs never enter any arm.
func LoadSources(root string, repo RepoSpec) ([]SourceFile, RepoSizes, error) {
	var out []SourceFile
	sizes := RepoSizes{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !hasExt(rel, repo.Exts) {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		sizes[rel] = CostOf(string(raw))
		slashed := "/" + rel
		for _, m := range repo.TestMarkers {
			if strings.Contains(slashed, m) {
				return nil
			}
		}
		if len(raw) > 1<<20 {
			return nil
		}
		out = append(out, SourceFile{Rel: rel, Lines: strings.Split(string(raw), "\n"), Size: len(raw)})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, sizes, err
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

// RunBaseline simulates `grep -rn PATTERN`. The agent sees only the first
// MaxGrepLines output lines, ranks the files visible there (definition-like
// lines first, then match count) and reads them in that order.
func RunBaseline(arm string, files []SourceFile, task Task, modules []string, p BaselineParams, sizes RepoSizes, readCap, maxExtra int) (ArmResult, error) {
	pat := task.Pattern
	if task.Word {
		pat = `\b(?:` + pat + `)\b`
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return ArmResult{}, fmt.Errorf("task %s pattern: %w", task.ID, err)
	}
	defRe := regexp.MustCompile(`\b(func|def|class|type|interface|const|let|var|function|struct|enum|trait|fn|module|impl)\b.*(?:` + task.Pattern + `)`)

	res := ArmResult{Arm: arm}
	var out strings.Builder
	score := map[string]int{}
	seen := map[string]bool{}
	var visible []string
	n := 0
scan:
	for _, f := range files {
		for i, line := range f.Lines {
			if !re.MatchString(line) {
				continue
			}
			if n >= p.MaxGrepLines {
				break scan
			}
			n++
			text := strings.TrimSpace(line)
			if len(text) > p.MaxLineChars {
				text = text[:p.MaxLineChars]
			}
			fmt.Fprintf(&out, "%s:%d:%s\n", f.Rel, i+1, text)
			if !seen[f.Rel] {
				seen[f.Rel] = true
				visible = append(visible, f.Rel)
			}
			score[f.Rel]++
			if defRe.MatchString(line) {
				score[f.Rel] += 100
			}
		}
	}
	res.Lookup = CostOf(out.String())

	if task.Kind == "dependents" {
		inferred := map[string]bool{}
		for _, hf := range visible {
			inferred[moduleOfFile(hf, modules)] = true
		}
		for m := range inferred {
			res.Modules = append(res.Modules, m)
		}
		sort.Strings(res.Modules)
		gradeDependents(&res, task, maxExtra)
		return res, nil
	}

	sort.SliceStable(visible, func(i, j int) bool {
		if score[visible[i]] != score[visible[j]] {
			return score[visible[i]] > score[visible[j]]
		}
		return visible[i] < visible[j]
	})
	res.Ranked = visible
	finishLocalization(&res, task, sizes, readCap)
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
