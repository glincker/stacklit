package setup

import (
	"os"
	"path/filepath"
	"strings"
)

const gitHookBody = "stacklit generate --quiet 2>/dev/null\ngit add stacklit.json DEPENDENCIES.md 2>/dev/null || true\n"

// removeGitHook strips the stacklit lines from .git/hooks/post-commit.
func (r *Runner) removeGitHook() error {
	path := filepath.Join(".git", "hooks", "post-commit")
	old, existed, err := r.read(path)
	if err != nil || !existed || !strings.Contains(old, "stacklit generate") {
		return err
	}
	out := strings.ReplaceAll(old, "\n"+gitHookBody, "")
	out = strings.ReplaceAll(out, gitHookBody, "")
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
