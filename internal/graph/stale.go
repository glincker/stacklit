package graph

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/glincker/stacklit/internal/git"
)

// StaleCheck compares a saved graph with the working tree. Only files the
// graph can index count as drift. fileTime (the graph file's mtime) is the
// fallback for graphs written before commits were recorded; zero skips it.
func StaleCheck(root string, g *SymGraph, fileTime time.Time) git.Staleness {
	if g.Commit == "" && fileTime.IsZero() {
		return git.Staleness{}
	}
	return git.CheckStale(root, g.Commit, fileTime, func(p string) bool { return LangOf(p) != "" })
}

// StaleLine is the one-line warning appended to query output, or "" when the
// graph is current, unstamped or STACKLIT_NO_STALE is set.
func StaleLine(root string, g *SymGraph, fileTime time.Time) string {
	if os.Getenv("STACKLIT_NO_STALE") != "" {
		return ""
	}
	s := StaleCheck(root, g, fileTime)
	if len(s.Changed) < envInt("STACKLIT_STALE_MIN_FILES", 1) {
		return ""
	}
	show := s.Changed
	more := ""
	if limit := envInt("STACKLIT_STALE_SHOW", 3); len(show) > limit {
		more = fmt.Sprintf(", +%d", len(show)-limit)
		show = show[:limit]
	}
	return fmt.Sprintf("[stale: %d source files changed since graph build (%s%s); run: stacklit generate --graph]\n",
		len(s.Changed), strings.Join(show, ", "), more)
}

// Stamp records the commit the graph was built at.
func (g *SymGraph) Stamp(root string) { g.Commit = git.Head(root) }
