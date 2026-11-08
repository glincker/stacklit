package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Runner applies file changes, or only reports them in dry-run mode.
type Runner struct {
	Root   string
	DryRun bool
	Out    io.Writer
	// Changed counts files that were (or would be) created, updated or removed.
	Changed int
}

func (r *Runner) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(r.Root, p)
}

func (r *Runner) say(verb, path string) {
	prefix := ""
	if r.DryRun {
		prefix = "would "
	}
	fmt.Fprintf(r.Out, "  %s%s %s\n", prefix, verb, path)
}

func (r *Runner) read(path string) (string, bool, error) {
	b, err := os.ReadFile(r.abs(path))
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

// write stores content at path when it differs from what is there.
func (r *Runner) write(path, old string, existed bool, content string) error {
	if existed && old == content {
		fmt.Fprintf(r.Out, "  unchanged %s\n", path)
		return nil
	}
	verb := "update"
	if !existed {
		verb = "create"
	}
	r.say(verb, path)
	r.Changed++
	if r.DryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(r.abs(path)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.abs(path), []byte(content), 0o644)
}

func (r *Runner) remove(path string) error {
	r.say("remove", path)
	r.Changed++
	if r.DryRun {
		return nil
	}
	if err := os.Remove(r.abs(path)); err != nil {
		return err
	}
	// Prune directories we emptied, but never the repo root itself.
	for d := filepath.Dir(r.abs(path)); d != filepath.Clean(r.Root) && strings.HasPrefix(d, filepath.Clean(r.Root)); d = filepath.Dir(d) {
		if os.Remove(d) != nil {
			break
		}
	}
	return nil
}
