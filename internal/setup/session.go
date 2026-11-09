package setup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/session"
)

// toolsLine is the factual tool summary appended to injected context. Hook
// text avoids imperative wording on purpose: the docs warn it can read as an
// injection attempt.
const toolsLine = "Tools: stacklit query \"<question>\" --budget 800 returns ranked symbols; stacklit slice <symbol> returns one symbol's source; stacklit impact <symbol> lists dependents.\n"

// sessionStatusSample is a representative status line, used for sizing.
const sessionStatusSample = "stacklit map 0123456789abcdef is current (HEAD 1a2b3c4d, no source changes since the index)."

// noIndexHint is shown once per worktree when no index exists.
const noIndexHint = "This repo has no stacklit index. `stacklit init` builds one in a few seconds; later sessions then start with a short orientation map."

// mapRe finds the structure hash a rules file was written from.
var mapRe = regexp.MustCompile(`\| ` + StructureTag + `([0-9a-f]{16})`)

// rulesHash returns the structure hash embedded in the project's Claude Code
// rules file, or "". Project-root CLAUDE.md is re-read from disk by Claude
// Code at start and after compaction, so a matching hash means the digest is
// already in context.
func rulesHash(root string) string {
	for _, p := range []string{"CLAUDE.md", filepath.Join(".claude", "CLAUDE.md")} {
		b, err := os.ReadFile(filepath.Join(root, p))
		if err != nil {
			continue
		}
		s, e := markers(StyleMarkdown)
		si := strings.Index(string(b), s)
		if si < 0 {
			continue
		}
		blk := string(b)[si:]
		if ei := strings.Index(blk, e); ei >= 0 {
			blk = blk[:ei]
		}
		if m := mapRe.FindStringSubmatch(blk); m != nil {
			return m[1]
		}
	}
	return ""
}

func readHookInput(stdin io.Reader) hookInput {
	var in hookInput
	_ = json.NewDecoder(stdin).Decode(&in)
	return in
}

func hookRoot(in hookInput) string {
	cwd := in.Cwd
	if cwd == "" {
		cwd = os.Getenv("CLAUDE_PROJECT_DIR")
	}
	return session.Root(cwd)
}

// statusLine says whether the map matches the working tree. It is the only
// volatile part of the injection and always comes last.
func statusLine(root string, idx *schema.Index, hash string) string {
	st, err := os.Stat(filepath.Join(root, "stacklit.json"))
	if err != nil {
		return ""
	}
	head := git.Head(root)
	if head == "" {
		return fmt.Sprintf("stacklit map %s (no git, staleness unknown).", hash)
	}
	s := git.CheckStale(root, "", st.ModTime(), func(p string) bool { return graph.LangOf(p) != "" })
	if len(s.Changed) == 0 {
		return fmt.Sprintf("stacklit map %s is current (HEAD %s, no source changes since the index).", hash, head)
	}
	show := s.Changed
	if len(show) > 3 {
		show = show[:3]
	}
	return fmt.Sprintf("stacklit map %s may be stale at HEAD %s: %d source files changed since the index (%s). `stacklit refresh --files <paths>` updates it.",
		hash, head, len(s.Changed), strings.Join(show, ", "))
}

// digestPayload is the orientation block hooks inject: digest without the
// imperative steering rules, then the tools line.
func digestPayload(idx *schema.Index) string {
	body := BodyFor(idx, DefaultSteering)
	if i := strings.Index(body, "\n```\n"); i >= 0 {
		body = "## Codebase map (stacklit)" + body[i:]
	}
	return body + toolsLine
}

// SessionHook answers a SessionStart payload for every source (startup,
// resume, clear, compact, fork). A small repo (see IsSmallRepo) gets one line
// saying grep is enough, plus the status line, never the digest. Otherwise, when the project's CLAUDE.md already holds a
// digest of the current structure it adds only the status line, because Claude
// Code re-reads that file itself, including after compaction. Otherwise it
// injects the digest. Any failure or a missing index yields no output.
func SessionHook(stdin io.Reader) string {
	in := readHookInput(stdin)
	root := hookRoot(in)
	idx, err := session.LoadIndex(root)
	dir := session.StateDir(root)
	if err != nil {
		return noIndexOnce(dir, in.Source)
	}
	hash := session.StructureHash(idx)
	var parts []string
	switch {
	case IsQuietRepo(root, idx):
		session.RecordInjected(dir, in.SessionID, hash)
		return ""
	case IsSmallRepo(root, idx):
		parts = append(parts, smallRepoLine(idx))
	case rulesHash(root) != hash:
		parts = append(parts, strings.TrimRight(digestPayload(idx), "\n"))
	}
	if sl := statusLine(root, idx, hash); sl != "" {
		parts = append(parts, sl)
	}
	session.RecordInjected(dir, in.SessionID, hash)
	if len(parts) == 0 {
		return ""
	}
	return hookJSON("SessionStart", strings.Join(parts, "\n"))
}

func noIndexOnce(dir, source string) string {
	if source != "startup" && source != "" {
		return ""
	}
	marker := filepath.Join(dir, "no-index-hinted")
	if _, err := os.Stat(marker); err == nil {
		return ""
	}
	if os.MkdirAll(dir, 0o755) == nil {
		_ = os.WriteFile(marker, []byte("1\n"), 0o644)
	}
	return hookJSON("SessionStart", noIndexHint)
}

// subagentTypes lists agent types that start without CLAUDE.md and so need
// the digest injected. STACKLIT_SUBAGENT_TYPES overrides it (comma list).
func subagentTypes() map[string]bool {
	list := "Explore,Plan"
	if v := os.Getenv("STACKLIT_SUBAGENT_TYPES"); v != "" {
		list = v
	}
	m := map[string]bool{}
	for _, t := range strings.Split(list, ",") {
		if t = strings.TrimSpace(t); t != "" {
			m[t] = true
		}
	}
	return m
}

// SubagentHook answers a SubagentStart payload. Explore and Plan skip
// CLAUDE.md, so they get the digest; other agents already load it and get
// nothing, which avoids paying for the same text twice.
func SubagentHook(stdin io.Reader) string {
	in := readHookInput(stdin)
	if !subagentTypes()[in.AgentType] {
		return ""
	}
	root := hookRoot(in)
	idx, err := session.LoadIndex(root)
	if err != nil {
		return ""
	}
	if IsQuietRepo(root, idx) {
		return ""
	}
	if IsSmallRepo(root, idx) {
		return hookJSON("SubagentStart", smallRepoLine(idx))
	}
	return hookJSON("SubagentStart", strings.TrimRight(digestPayload(idx), "\n"))
}
