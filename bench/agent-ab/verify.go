package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// fileCheck says an answer file must contain text matching Regex, so a task
// cannot point at a file that has since changed meaning.
type fileCheck struct {
	Path  string `json:"path"`
	Regex string `json:"regex"`
}

// importScan recomputes an impact task's answer with grep rules: every
// tracked file with one of Include's extensions that is not a test file and
// has a line matching Pattern, minus paths matching ExcludeRegex.
type importScan struct {
	Include      []string `json:"include"`
	Pattern      string   `json:"pattern"`
	ExcludeTests bool     `json:"exclude_tests"`
	ExcludeRegex string   `json:"exclude_regex,omitempty"`
}

var testPathRe = regexp.MustCompile(`(_test\.go$|\.test\.[a-z]+$|\.spec\.[a-z]+$|(^|/)__tests__/|(^|/)tests?/|(^|/)testdata/)`)

func trackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(out)), nil
}

// scanImporters returns the sorted files of the checkout in dir that the scan selects.
func scanImporters(dir string, files []string, s importScan) ([]string, error) {
	re, err := regexp.Compile(s.Pattern)
	if err != nil {
		return nil, err
	}
	var skip *regexp.Regexp
	if s.ExcludeRegex != "" {
		if skip, err = regexp.Compile(s.ExcludeRegex); err != nil {
			return nil, err
		}
	}
	ext := map[string]bool{}
	for _, e := range s.Include {
		ext[e] = true
	}
	var hits []string
	for _, f := range files {
		if !ext[filepath.Ext(f)] || (s.ExcludeTests && testPathRe.MatchString(f)) || (skip != nil && skip.MatchString(f)) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			if t := strings.TrimSpace(line); strings.HasPrefix(t, "//") || strings.HasPrefix(t, "*") {
				continue
			}
			if re.MatchString(line) {
				hits = append(hits, f)
				break
			}
		}
	}
	sort.Strings(hits)
	return hits, nil
}

// verifyTasks checks every task of one repo against its pinned checkout and
// returns one message per problem.
func verifyTasks(dir string, tasks []task) []string {
	files, err := trackedFiles(dir)
	if err != nil {
		return []string{"git ls-files: " + err.Error()}
	}
	have := map[string]bool{}
	for _, f := range files {
		have[f] = true
	}
	exists := func(p string) bool {
		if have[p] {
			return true
		}
		if strings.HasSuffix(p, "/") {
			for _, f := range files {
				if strings.HasPrefix(f, p) {
					return true
				}
			}
		}
		return false
	}
	var bad []string
	for _, t := range tasks {
		for _, p := range append(append([]string{}, t.AllOf...), t.AnyOf...) {
			if !exists(p) {
				bad = append(bad, fmt.Sprintf("%s: %s does not exist at the pinned commit", t.ID, p))
			}
		}
		for _, c := range t.Check {
			b, err := os.ReadFile(filepath.Join(dir, c.Path))
			if err != nil {
				bad = append(bad, fmt.Sprintf("%s: cannot read %s", t.ID, c.Path))
				continue
			}
			if re, err := regexp.Compile(c.Regex); err != nil || !re.Match(b) {
				bad = append(bad, fmt.Sprintf("%s: %s does not match %q", t.ID, c.Path, c.Regex))
			}
		}
		if t.Verify != nil {
			got, err := scanImporters(dir, files, *t.Verify)
			if err != nil {
				bad = append(bad, fmt.Sprintf("%s: %v", t.ID, err))
				continue
			}
			want := append([]string{}, t.AllOf...)
			sort.Strings(want)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				bad = append(bad, fmt.Sprintf("%s: grep finds %d files, the task lists %d\ngrep: %v\ntask: %v", t.ID, len(got), len(want), got, want))
			}
		}
	}
	return bad
}
