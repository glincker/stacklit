package setup

import (
	"os"
	"path/filepath"
	"strings"
)

// removeGitHook strips the stacklit lines from .git/hooks/post-commit.
func (r *Runner) removeGitHook() error {
	path := filepath.Join(".git", "hooks", "post-commit")
	old, existed, err := r.read(path)
	if err != nil || !existed || !strings.Contains(old, "stacklit generate") {
		return err
	}
	out := stripHookLines(old)
	if t := strings.TrimSpace(out); t == "" || t == "#!/bin/sh" {
		return r.remove(path)
	}
	return r.write(path, old, existed, out)
}

// gitHookInstalled reports whether the hook already runs stacklit.
func (r *Runner) gitHookInstalled() bool {
	b, err := os.ReadFile(r.abs(filepath.Join(".git", "hooks", "post-commit")))
	return err == nil && strings.Contains(string(b), "stacklit")
}

// stripHookLines drops the generate line and the git add line after it,
// whichever output paths the add line names.
func stripHookLines(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "stacklit generate --quiet") {
			if i+1 < len(lines) && strings.HasPrefix(lines[i+1], "git add ") {
				i++
			}
			continue
		}
		kept = append(kept, lines[i])
	}
	return strings.Join(kept, "\n")
}
