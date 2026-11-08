package git

import (
	"os"
	"path/filepath"
	"strings"
)

// RefreshHooks are the hooks that keep the index fresh after history moves
// without a commit: branch switches, pulls and rebases or amends.
var RefreshHooks = []string{"post-checkout", "post-merge", "post-rewrite"}

const (
	refreshStart = "# >>> stacklit refresh"
	refreshEnd   = "# <<< stacklit refresh"
)

// refreshBlock regenerates in the background so git never waits on it. The
// cache makes the run incremental; the graph is refreshed only if one exists.
// post-checkout fires for file checkouts too, so only branch switches ($3=1) count.
func refreshBlock(name string) string {
	guard := ""
	if name == "post-checkout" {
		guard = "[ \"$3\" = \"1\" ] || exit 0\n"
	}
	return refreshStart + "\n" + guard +
		"command -v stacklit >/dev/null 2>&1 || exit 0\n" +
		"g=\"\"; [ -f stacklit.graph.json ] && g=\"--graph\"\n" +
		"(stacklit generate --quiet $g >/dev/null 2>&1 &)\n" +
		refreshEnd + "\n"
}

// InstallRefreshHooks adds the refresh block to each hook in RefreshHooks,
// keeping any hook content already there. It returns the hooks it changed.
func InstallRefreshHooks(root string) ([]string, error) {
	dir := filepath.Join(root, ".git", "hooks")
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}
	var changed []string
	for _, name := range RefreshHooks {
		path := filepath.Join(dir, name)
		old, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return changed, err
		}
		if strings.Contains(string(old), refreshStart) {
			continue
		}
		body := string(old)
		if body == "" {
			body = "#!/bin/sh\n"
		} else if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		if err := os.WriteFile(path, []byte(body+refreshBlock(name)), 0o755); err != nil {
			return changed, err
		}
		changed = append(changed, name)
	}
	return changed, nil
}

// RemoveRefreshHooks strips the refresh block and deletes hooks left empty.
func RemoveRefreshHooks(root string) ([]string, error) {
	var changed []string
	for _, name := range RefreshHooks {
		path := filepath.Join(root, ".git", "hooks", name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		s := string(b)
		i, j := strings.Index(s, refreshStart), strings.Index(s, refreshEnd)
		if i < 0 || j < i {
			continue
		}
		rest := s[:i] + strings.TrimPrefix(s[j+len(refreshEnd):], "\n")
		if t := strings.TrimSpace(rest); t == "" || t == "#!/bin/sh" {
			if err := os.Remove(path); err != nil {
				return changed, err
			}
		} else if err := os.WriteFile(path, []byte(rest), 0o755); err != nil {
			return changed, err
		}
		changed = append(changed, name)
	}
	return changed, nil
}
