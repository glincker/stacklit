package session

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LockStale is how old a lock file must be before it is treated as left by a
// crashed process. STACKLIT_LOCK_STALE_SEC overrides it.
func LockStale() time.Duration {
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_LOCK_STALE_SEC")); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return 30 * time.Second
}

// Lock is an advisory lock held by creating a file with O_EXCL, which is
// atomic on every platform stacklit builds for.
type Lock struct{ path string }

// TryLock returns (nil, nil) when another live process holds the lock.
func TryLock(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dir, "refresh.lock")
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return &Lock{path: p}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		st, serr := os.Stat(p)
		if serr != nil {
			continue
		}
		if time.Since(st.ModTime()) < LockStale() {
			return nil, nil
		}
		_ = os.Remove(p)
	}
	return nil, nil
}

// Touch refreshes the lock mtime so a long run is not mistaken for a crash.
func (l *Lock) Touch() { now := time.Now(); _ = os.Chtimes(l.path, now, now) }

// Unlock releases the lock.
func (l *Lock) Unlock() { _ = os.Remove(l.path) }

// Enqueue appends paths to the pending file. One small O_APPEND write per
// call keeps concurrent writers from interleaving.
func Enqueue(dir string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "pending"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strings.Join(paths, "\n") + "\n")
	return err
}

// Drain takes every queued path, deduplicated and sorted, and empties the
// queue. The file is renamed first so late writers start a fresh queue.
func Drain(dir string) []string {
	src := filepath.Join(dir, "pending")
	tmp := filepath.Join(dir, fmt.Sprintf("pending.%d", os.Getpid()))
	if err := os.Rename(src, tmp); err != nil {
		return nil
	}
	defer os.Remove(tmp)
	f, err := os.Open(tmp)
	if err != nil {
		return nil
	}
	defer f.Close()
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := strings.TrimSpace(sc.Text()); l != "" {
			seen[l] = true
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// HasPending reports whether the queue holds anything.
func HasPending(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "pending"))
	return err == nil && st.Size() > 0
}
