// Package session holds the per-worktree plumbing shared by the session
// hooks: root resolution, a state directory, an advisory lock, a pending
// queue for debounced refreshes and the structure hash.
package session

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Root finds the worktree root for cwd without spawning a process: the
// nearest ancestor holding stacklit.json, else the nearest holding .git (a
// directory, or a file in a linked worktree). With neither, cwd itself is
// used, so a repo with no git still works.
func Root(cwd string) string {
	if cwd == "" {
		cwd = "."
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return cwd
	}
	gitDir := ""
	for d := abs; ; {
		if _, err := os.Stat(filepath.Join(d, "stacklit.json")); err == nil {
			return d
		}
		if gitDir == "" {
			if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
				gitDir = d
			}
		}
		p := filepath.Dir(d)
		if p == d {
			break
		}
		d = p
	}
	if gitDir != "" {
		return gitDir
	}
	return abs
}

// GitToplevel asks git for the worktree root; "" outside a repo. Slower than
// Root, used where latency is not critical and to cross-check Root in tests.
func GitToplevel(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(string(out))
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	return filepath.FromSlash(p)
}

// StateDir is the per-worktree state location. It is keyed by the absolute
// root, so every linked worktree and every clone gets its own files.
func StateDir(root string) string {
	dir := os.Getenv("STACKLIT_CACHE_DIR")
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil {
			base = os.TempDir()
		}
		dir = filepath.Join(base, "stacklit")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(dir, "sessions", fmt.Sprintf("%x", sum[:8]))
}
