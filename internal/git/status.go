package git

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Staleness describes how far the working tree has moved since an index was built.
type Staleness struct {
	InRepo        bool
	Head          string
	CommitsBehind int
	Changed       []string
}

func run(root string, args ...string) (string, bool) {
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return strings.TrimSpace(out.String()), true
}

// Head returns the abbreviated HEAD commit of root, or "" outside a repo.
func Head(root string) string {
	h, _ := run(root, "rev-parse", "--short", "HEAD")
	return h
}

// generated lists outputs stacklit rewrites itself; they never count as drift.
var generated = map[string]bool{
	"stacklit.json": true, "stacklit.graph.json": true, "stacklit.html": true,
	"stacklit.mmd": true, "DEPENDENCIES.md": true, "stacklit-multi.json": true,
}

// CheckStale reports files changed since builtCommit (preferred) or, when the
// commit is unknown, since builtAt. keep filters which paths count.
func CheckStale(root, builtCommit string, builtAt time.Time, keep func(string) bool) Staleness {
	s := Staleness{Head: Head(root)}
	if s.Head == "" {
		return s
	}
	s.InRepo = true
	set := map[string]bool{}
	add := func(out string) {
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(l)
			if l != "" && !generated[filepath.Base(l)] && (keep == nil || keep(l)) {
				set[l] = true
			}
		}
	}
	// Dirty files only count when touched after the build, since a graph built
	// from an unclean tree already contains them.
	dirty := func() {
		for _, args := range [][]string{{"diff", "--relative", "--name-only", "HEAD"}, {"ls-files", "--others", "--exclude-standard"}} {
			out, _ := run(root, args...)
			for _, f := range strings.Split(out, "\n") {
				if f = strings.TrimSpace(f); f == "" {
					continue
				}
				if builtAt.IsZero() {
					add(f)
				} else if st, err := os.Stat(filepath.Join(root, f)); err == nil && st.ModTime().After(builtAt) {
					add(f)
				}
			}
		}
	}
	if builtCommit != "" {
		if _, ok := run(root, "cat-file", "-e", builtCommit+"^{commit}"); ok {
			if n, ok := run(root, "rev-list", "--count", builtCommit+"..HEAD"); ok {
				s.CommitsBehind, _ = strconv.Atoi(n)
			}
			if d, ok := run(root, "diff", "--relative", "--name-only", builtCommit, "HEAD"); ok {
				add(d)
			}
			dirty()
			return s.finish(set)
		}
	}
	if !builtAt.IsZero() {
		since := "--since=" + strconv.FormatInt(builtAt.Unix(), 10)
		if l, ok := run(root, "log", since, "--relative", "--name-only", "--pretty=format:"); ok {
			add(l)
		}
		if n, ok := run(root, "rev-list", "--count", since, "HEAD"); ok {
			s.CommitsBehind, _ = strconv.Atoi(n)
		}
		dirty()
	}
	return s.finish(set)
}

func (s Staleness) finish(set map[string]bool) Staleness {
	for f := range set {
		s.Changed = append(s.Changed, f)
	}
	sort.Strings(s.Changed)
	return s
}
