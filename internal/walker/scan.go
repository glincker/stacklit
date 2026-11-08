package walker

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Options tunes WalkWith.
type Options struct {
	ExtraIgnore []string // gitignore-style patterns skipped in addition to ignore files
	Include     []string // when non-empty, only files matching one of these globs are kept
	Roots       []string // scan only these subdirectories (relative, slash-separated)
	MaxFileSize int64    // files larger than this many bytes are skipped; 0 disables
}

// Skip records one path the walker deliberately did not index.
type Skip struct {
	Path   string
	Reason string
}

// Skip reasons reported in Result.Skipped.
const (
	ReasonLarge         = "file exceeds max_file_size"
	ReasonBrokenSymlink = "broken symlink"
	ReasonSymlinkEscape = "symlink resolves outside the root"
	ReasonSymlinkDir    = "symlinked directory not followed"
	ReasonNotRegular    = "not a regular file"
	ReasonUnreadable    = "unreadable"
	ReasonMissingRoot   = "configured root not found"
	ReasonRootEscape    = "configured root resolves outside the repo"
)

// Result is the outcome of a walk.
type Result struct {
	Files   []string // sorted, slash-separated, relative to the walk root
	Skipped []Skip   // sorted by path
}

type ignoreLevel struct {
	base string // slash-separated dir the file lives in, "" for the root
	gi   *gitignore.GitIgnore
}

// WalkWith is the full-featured walker. It never follows directory symlinks,
// rejects file symlinks that are broken or leave the root, skips oversized
// files, and honours .gitignore (nested too) plus .stacklitignore.
func WalkWith(root string, o Options) (*Result, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", root, err)
	}
	st, err := os.Stat(realRoot)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var extraGi, includeGi *gitignore.GitIgnore
	if len(o.ExtraIgnore) > 0 {
		extraGi = gitignore.CompileIgnoreLines(o.ExtraIgnore...)
	}
	if len(o.Include) > 0 {
		includeGi = gitignore.CompileIgnoreLines(o.Include...)
	}

	res := &Result{}
	seen := map[string]bool{}

	starts, escapes, whole := normalizeRoots(o.Roots)
	for _, e := range escapes {
		res.Skipped = append(res.Skipped, Skip{e, ReasonRootEscape})
	}
	if whole {
		starts = []string{""}
	}
	for _, start := range starts {
		abs := root
		if start != "" {
			abs = filepath.Join(root, filepath.FromSlash(start))
			real, err := filepath.EvalSymlinks(abs)
			if err != nil {
				res.Skipped = append(res.Skipped, Skip{start, ReasonMissingRoot})
				continue
			}
			if !within(realRoot, real) {
				res.Skipped = append(res.Skipped, Skip{start, ReasonRootEscape})
				continue
			}
		}
		w := &walk{root: root, realRoot: realRoot, o: o, extraGi: extraGi, includeGi: includeGi, res: res, seen: seen}
		// Ignore files in ancestors of a custom root still apply.
		w.loadAncestors(start)
		if err := w.run(abs, start); err != nil {
			return nil, err
		}
	}

	sort.Strings(res.Files)
	sort.Slice(res.Skipped, func(i, j int) bool { return res.Skipped[i].Path < res.Skipped[j].Path })
	return res, nil
}

type walk struct {
	root, realRoot     string
	o                  Options
	extraGi, includeGi *gitignore.GitIgnore
	res                *Result
	seen               map[string]bool
	levels             []ignoreLevel
}

func (w *walk) loadAncestors(start string) {
	w.loadIgnoreFiles("")
	if start == "" {
		return
	}
	parts := strings.Split(start, "/")
	for i := 1; i < len(parts); i++ {
		w.loadIgnoreFiles(strings.Join(parts[:i], "/"))
	}
}

func (w *walk) loadIgnoreFiles(base string) {
	dir := filepath.Join(w.root, filepath.FromSlash(base))
	for _, name := range []string{".gitignore", ".stacklitignore"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if gi, err := gitignore.CompileIgnoreFile(p); err == nil && gi != nil {
			w.levels = append(w.levels, ignoreLevel{base: base, gi: gi})
		}
	}
}

func (w *walk) ignored(rel string, isDir bool) bool {
	if w.extraGi != nil {
		m := rel
		if isDir {
			m += "/"
		}
		if w.extraGi.MatchesPath(m) {
			return true
		}
	}
	for _, lv := range w.levels {
		sub := rel
		if lv.base != "" {
			if !strings.HasPrefix(rel, lv.base+"/") {
				continue
			}
			sub = strings.TrimPrefix(rel, lv.base+"/")
		}
		if isDir {
			sub += "/"
		}
		if lv.gi.MatchesPath(sub) {
			return true
		}
	}
	return false
}

func (w *walk) run(startAbs, _ string) error {
	// Nested ignore files are loaded as directories are entered; levels added
	// below a directory are dropped again when its subtree is finished.
	type frame struct {
		rel string
		n   int
	}
	var frames []frame

	return filepath.WalkDir(startAbs, func(p string, d fs.DirEntry, err error) error {
		rel, relErr := filepath.Rel(w.root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)

		// Pop levels from finished directories.
		for len(frames) > 0 && !(rel == frames[len(frames)-1].rel || strings.HasPrefix(rel, frames[len(frames)-1].rel+"/")) {
			w.levels = w.levels[:len(w.levels)-frames[len(frames)-1].n]
			frames = frames[:len(frames)-1]
		}

		if err != nil {
			// An unreadable entry must not abort the whole scan, except the start.
			if p == startAbs {
				return err
			}
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonUnreadable})
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == "." {
			return nil
		}
		name := d.Name()

		if d.IsDir() {
			if alwaysIgnoreDirs[name] || w.ignored(rel, true) {
				return filepath.SkipDir
			}
			before := len(w.levels)
			w.loadIgnoreFiles(rel)
			frames = append(frames, frame{rel: rel, n: len(w.levels) - before})
			return nil
		}

		if w.ignored(rel, false) {
			return nil
		}
		if !sourceExtensions[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if w.includeGi != nil && !w.includeGi.MatchesPath(rel) {
			return nil
		}

		size, ok := w.checkFile(p, rel, d)
		if !ok {
			return nil
		}
		if w.o.MaxFileSize > 0 && size > w.o.MaxFileSize {
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonLarge})
			return nil
		}
		if !w.seen[rel] {
			w.seen[rel] = true
			w.res.Files = append(w.res.Files, rel)
		}
		return nil
	})
}

// checkFile validates symlinks and special files and returns the byte size of
// the real target.
func (w *walk) checkFile(p, rel string, d fs.DirEntry) (int64, bool) {
	if d.Type()&fs.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(p)
		if err != nil {
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonBrokenSymlink})
			return 0, false
		}
		if !within(w.realRoot, real) {
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonSymlinkEscape})
			return 0, false
		}
		info, err := os.Stat(real)
		if err != nil {
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonBrokenSymlink})
			return 0, false
		}
		if info.IsDir() {
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonSymlinkDir})
			return 0, false
		}
		if !info.Mode().IsRegular() {
			w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonNotRegular})
			return 0, false
		}
		return info.Size(), true
	}
	if !d.Type().IsRegular() {
		w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonNotRegular})
		return 0, false
	}
	info, err := d.Info()
	if err != nil {
		w.res.Skipped = append(w.res.Skipped, Skip{rel, ReasonUnreadable})
		return 0, false
	}
	return info.Size(), true
}

// within reports whether target is root or lies below it.
func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

// normalizeRoots cleans, dedupes and drops roots nested under another root.
func normalizeRoots(roots []string) (out, escapes []string, whole bool) {
	var cleaned []string
	for _, r := range roots {
		r = path.Clean(strings.ReplaceAll(r, "\\", "/"))
		if r == "." || r == "" {
			return nil, nil, true
		}
		if r == ".." || strings.HasPrefix(r, "../") || path.IsAbs(r) {
			// Never resolve outside the repo; the caller reports it via Skipped.
			escapes = append(escapes, r)
			continue
		}
		cleaned = append(cleaned, r)
	}
	sort.Strings(cleaned)
	for _, r := range cleaned {
		if len(out) > 0 && (r == out[len(out)-1] || strings.HasPrefix(r, out[len(out)-1]+"/")) {
			continue
		}
		out = append(out, r)
	}
	return out, escapes, len(roots) == 0
}
