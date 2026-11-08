package graph

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/glincker/stacklit/internal/walker"
)

// WatchOptions configures Watch.
type WatchOptions struct {
	Root     string
	Out      string // graph file path, absolute or relative to Root
	Ignore   []string
	Debounce time.Duration
	// MaxWatches caps directory watches; above it Watch falls back to polling so
	// huge trees cannot exhaust inotify. 0 reads STACKLIT_WATCH_MAX (default 4096).
	MaxWatches   int
	PollInterval time.Duration // polling cadence, 0 reads STACKLIT_WATCH_POLL_MS (default 2000)
	ForcePoll    bool
	// Extract tunes parsing, including whether test files are tracked.
	Extract ExtractOptions
	Log     func(format string, args ...any)
}

type fileStamp struct {
	mod  int64
	size int64
}

type watcher struct {
	o      WatchOptions
	syms   map[string]*FileSyms
	stamps map[string]fileStamp
	eo     ExtractOptions
	mods   map[string]string
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

// Watch builds the graph once, then rebuilds incrementally on source changes
// until ctx ends. Only changed files are re-parsed; linking is one linear pass.
func Watch(ctx context.Context, o WatchOptions) error {
	if o.Debounce == 0 {
		o.Debounce = 300 * time.Millisecond
	}
	if o.MaxWatches == 0 {
		o.MaxWatches = envInt("STACKLIT_WATCH_MAX", 4096)
	}
	if o.PollInterval == 0 {
		o.PollInterval = time.Duration(envInt("STACKLIT_WATCH_POLL_MS", 2000)) * time.Millisecond
	}
	if o.Log == nil {
		o.Log = func(string, ...any) {}
	}
	if !filepath.IsAbs(o.Out) {
		o.Out = filepath.Join(o.Root, o.Out)
	}
	w := &watcher{o: o, syms: map[string]*FileSyms{}, stamps: map[string]fileStamp{}}
	files, err := walker.Walk(o.Root, o.Ignore)
	if err != nil {
		return err
	}
	w.sync(files, nil)
	if err := w.write(); err != nil {
		return err
	}
	o.Log("watching %d files", len(w.syms))

	dirs := watchDirs(files)
	if !o.ForcePoll && len(dirs) <= o.MaxWatches {
		if err := w.runFSNotify(ctx, dirs); err == nil || ctx.Err() != nil {
			return nil
		}
		o.Log("fsnotify unavailable, falling back to polling")
	} else {
		o.Log("%d directories exceed the watch limit, polling every %s", len(dirs), o.PollInterval)
	}
	return w.runPoll(ctx)
}

// watchDirs lists directories that hold sources plus their ancestors, so new
// subdirectories are noticed without watching ignored trees.
func watchDirs(files []string) []string {
	set := map[string]bool{".": true}
	for _, f := range files {
		for d := path.Dir(f); d != "." && !set[d]; d = path.Dir(d) {
			set[d] = true
		}
	}
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// sync reconciles the cache with the current file list and changed paths.
// It returns true when anything changed.
func (w *watcher) sync(files []string, dirty map[string]bool) bool {
	changed := false
	present := make(map[string]bool, len(files))
	for _, f := range files {
		if LangOf(f) == "" || (!w.o.Extract.IncludeTests && isTestFile(f)) {
			continue
		}
		present[f] = true
		st, err := os.Stat(filepath.Join(w.o.Root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		stamp := fileStamp{st.ModTime().UnixNano(), st.Size()}
		if old, ok := w.stamps[f]; ok && old == stamp && !dirty[f] {
			continue
		}
		eo := w.o.Extract
		if eo.MaxFileBytes == 0 {
			eo.MaxFileBytes = 1 << 20
		}
		fs := extractPath(w.o.Root, f, eo, nil)
		if fs == nil {
			delete(w.syms, f)
		} else {
			w.syms[f] = fs
		}
		w.stamps[f] = stamp
		changed = true
	}
	for f := range w.syms {
		if !present[f] {
			delete(w.syms, f)
			delete(w.stamps, f)
			changed = true
		}
	}
	return changed
}

func (w *watcher) write() error {
	paths := make([]string, 0, len(w.syms))
	for p := range w.syms {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	fs := make([]*FileSyms, len(paths))
	for i, p := range paths {
		fs[i] = w.syms[p]
	}
	if w.mods == nil || len(w.mods) == 0 {
		w.mods = DiscoverGoModules(w.o.Root, paths)
	}
	g := Resolve(fs, ResolveOptions{GoModules: w.mods})
	g.Root = "."
	g.attachBodies(w.o.Root)
	tmp := w.o.Out + ".tmp"
	if err := g.Save(tmp); err != nil {
		return err
	}
	return os.Rename(tmp, w.o.Out)
}

func (w *watcher) rebuild(structural bool, dirty map[string]bool) {
	files := make([]string, 0, len(w.syms))
	if structural {
		var err error
		files, err = walker.Walk(w.o.Root, w.o.Ignore)
		if err != nil {
			w.o.Log("walk failed: %v", err)
			return
		}
	} else {
		for p := range w.syms {
			files = append(files, p)
		}
		for p := range dirty {
			if _, ok := w.syms[p]; !ok {
				files = append(files, p)
			}
		}
	}
	if w.sync(files, dirty) {
		if err := w.write(); err != nil {
			w.o.Log("write failed: %v", err)
			return
		}
		w.o.Log("graph updated (%d files)", len(w.syms))
	}
}

func (w *watcher) runFSNotify(ctx context.Context, dirs []string) error {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fw.Close()
	added := map[string]bool{}
	add := func(d string) error {
		if added[d] {
			return nil
		}
		if err := fw.Add(filepath.Join(w.o.Root, filepath.FromSlash(d))); err != nil {
			return err
		}
		added[d] = true
		return nil
	}
	for _, d := range dirs {
		if err := add(d); err != nil {
			return err
		}
	}
	// Edits made after the first graph was written but before the watches existed
	// produce no event, so reconcile once now.
	w.rebuild(true, nil)
	dirty := map[string]bool{}
	structural := false
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-fw.Errors:
			if err != nil {
				return err
			}
		case ev := <-fw.Events:
			rel, rerr := filepath.Rel(w.o.Root, ev.Name)
			if rerr != nil || ev.Op == fsnotify.Chmod {
				continue
			}
			rel = filepath.ToSlash(rel)
			if abs, _ := filepath.Abs(ev.Name); abs == w.o.Out || abs == w.o.Out+".tmp" {
				continue
			}
			if ev.Has(fsnotify.Create) {
				if st, serr := os.Stat(ev.Name); serr == nil && st.IsDir() {
					structural = true
					timer.Reset(w.o.Debounce) // files may land before the watch is added
					// Watch the new directory if it is not ignored; the walk decides.
					if files, werr := walker.Walk(w.o.Root, w.o.Ignore); werr == nil {
						if len(added)+len(dirs) > w.o.MaxWatches*2 {
							return os.ErrInvalid
						}
						for _, d := range watchDirs(files) {
							if err := add(d); err != nil {
								return err
							}
						}
					}
				}
			}
			if LangOf(rel) == "" {
				continue
			}
			dirty[rel] = true
			if ev.Has(fsnotify.Create) || ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
				structural = true
			}
			timer.Reset(w.o.Debounce)
		case <-timer.C:
			w.rebuild(structural, dirty)
			dirty = map[string]bool{}
			structural = false
		}
	}
}

func (w *watcher) runPoll(ctx context.Context) error {
	t := time.NewTicker(w.o.PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			w.rebuild(true, nil)
		}
	}
}
