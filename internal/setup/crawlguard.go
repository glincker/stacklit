package setup

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/glincker/stacklit/internal/config"
	"github.com/glincker/stacklit/internal/git"
	"github.com/glincker/stacklit/internal/graph"
	"github.com/glincker/stacklit/internal/session"
)

// Crawl guard modes.
const (
	CrawlOff      = "off"
	CrawlNudge    = "nudge"
	CrawlRedirect = "redirect"
)

// hookCrawlCmd is the installed command prefix; the mode is its argument.
const hookCrawlCmd = "stacklit hook crawl"

// CrawlModes lists the valid modes.
func CrawlModes() []string { return []string{CrawlOff, CrawlNudge, CrawlRedirect} }

func validCrawlMode(m string) bool {
	for _, v := range CrawlModes() {
		if m == v {
			return true
		}
	}
	return false
}

func identOf(pat string) string { return graph.PatternIdent(pat) }

// DetectBash judges a shell command line. A cd into a subdirectory ends the
// scan, since later relative paths no longer mean the repo root.
func DetectBash(cmd, root string) (Crawl, bool) {
	for _, seg := range splitShell(cmd) {
		toks := stripPrefix(seg.toks)
		if len(toks) > 0 {
			switch lowerBase(toks[0]) {
			case "cd", "pushd", "set-location", "chdir":
				if len(toks) > 1 && !rootish(toks[1], root) {
					return Crawl{}, false
				}
				continue
			}
		}
		if c, ok := detectSegment(seg, root); ok {
			return c, true
		}
	}
	return Crawl{}, false
}

var globAll = regexp.MustCompile(`^\*(\.(\*|\w+|\{[\w,]+\}))?$`)

// DetectGlob flags a Glob that lists every file or every file of an
// extension from the repo root. Patterns naming a file or a subdirectory pass.
func DetectGlob(pattern, dir, root string) (Crawl, bool) {
	if !rootish(dir, root) {
		return Crawl{}, false
	}
	p := strings.ReplaceAll(strings.TrimSpace(pattern), `\`, "/")
	p = strings.TrimPrefix(p, "./")
	if p != "**" && !strings.HasPrefix(p, "**/") {
		return Crawl{}, false
	}
	rest := strings.TrimPrefix(strings.TrimPrefix(p, "**"), "/")
	if rest != "" && rest != "**" && !globAll.MatchString(rest) {
		return Crawl{}, false
	}
	c := Crawl{Kind: "glob"}
	commandFor(&c, "", "")
	return c, true
}

var (
	subagentPhrase = regexp.MustCompile(`(?i)\b(explore|understand|survey|investigate|map out|get familiar with|get an overview of|overview of|structure of|layout of|architecture of|list all (the )?files in|find all (the )?files in)\b[^.\n]{0,40}\b(code ?base|repo|repository|project|source tree|whole (code|repo|project)|entire (code|repo|project))\b`)
	subagentHow    = regexp.MustCompile(`(?i)how is (the|this) (code ?base|repo|repository|project) (structured|organi[sz]ed|laid out)`)
	codeish        = regexp.MustCompile("`|[A-Za-z0-9_]/[A-Za-z0-9_.-]+|\\b\\w+\\.(go|ts|tsx|js|jsx|py|rs|java|rb|php|c|cc|cpp|h|cs|kt|swift|md|json|ya?ml)\\b|\\b[a-z]+[A-Z]\\w*\\b|\\b\\w+_\\w+\\b")
)

// maxGenericPrompt is the longest prompt still judged generic; longer ones
// carry specific instructions. STACKLIT_CRAWL_PROMPT_CHARS overrides it.
func maxGenericPrompt() int {
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_CRAWL_PROMPT_CHARS")); err == nil && n > 0 {
		return n
	}
	return 600
}

// DetectSubagent flags a subagent prompt that asks to explore the whole repo
// with no identifier, path or file named.
func DetectSubagent(prompt string) (Crawl, bool) {
	if len(prompt) > maxGenericPrompt() || codeish.MatchString(prompt) {
		return Crawl{}, false
	}
	if !subagentPhrase.MatchString(prompt) && !subagentHow.MatchString(prompt) {
		return Crawl{}, false
	}
	c := Crawl{Kind: "subagent", Command: genericCmd()}
	return c, true
}

// DetectRead flags a Read whose target is a directory.
func DetectRead(file string) (Crawl, bool) {
	if file == "" {
		return Crawl{}, false
	}
	st, err := os.Stat(file)
	if err != nil || !st.IsDir() {
		return Crawl{}, false
	}
	c := Crawl{Kind: "read-dir"}
	commandFor(&c, "", queryWords(filepath.Base(file)))
	return c, true
}

// DetectCrawl classifies one PreToolUse payload.
func DetectCrawl(in hookInput, root string) (Crawl, bool) {
	cwd := in.Cwd
	if cwd == "" {
		cwd = root
	}
	switch in.ToolName {
	case "Bash":
		return DetectBash(in.ToolInput.Command, cwd)
	case "Glob":
		return DetectGlob(in.ToolInput.Pattern, in.ToolInput.Path, cwd)
	case "Read":
		return DetectRead(in.ToolInput.FilePath)
	case "Agent", "Task":
		return DetectSubagent(in.ToolInput.Prompt)
	}
	return Crawl{}, false
}

type crawlOutput struct {
	Spec struct {
		Event    string `json:"hookEventName"`
		Decision string `json:"permissionDecision,omitempty"`
		Reason   string `json:"permissionDecisionReason,omitempty"`
		Context  string `json:"additionalContext,omitempty"`
	} `json:"hookSpecificOutput"`
}

func crawlJSON(decision, reason, ctx string) string {
	var o crawlOutput
	o.Spec.Event = "PreToolUse"
	o.Spec.Decision, o.Spec.Reason, o.Spec.Context = decision, reason, ctx
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(o)
	return buf.String()
}

// CrawlMode resolves the mode: STACKLIT_CRAWL_GUARD, then the crawl_guard
// config key, then the mode the installed hook carries. Unknown means off.
func CrawlMode(root, installed string) string {
	for _, m := range []string{os.Getenv("STACKLIT_CRAWL_GUARD"), config.Load(root).CrawlGuard, installed} {
		m = strings.ToLower(strings.TrimSpace(m))
		if m == "" {
			continue
		}
		if validCrawlMode(m) {
			return m
		}
		return CrawlOff
	}
	return CrawlOff
}

// redirectLimit is how many consecutive redirects a session gets before the
// guard stands down. STACKLIT_CRAWL_REDIRECTS overrides it.
func redirectLimit() int {
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_CRAWL_REDIRECTS")); err == nil && n >= 0 {
		return n
	}
	return 2
}

// freshTTL is how long a staleness verdict is reused within a session.
// STACKLIT_CRAWL_FRESH_TTL (seconds) overrides it.
func freshTTL() int64 {
	if n, err := strconv.ParseInt(os.Getenv("STACKLIT_CRAWL_FRESH_TTL"), 10, 64); err == nil && n >= 0 {
		return n
	}
	return 10
}

type crawlState struct {
	N        int   `json:"n"`
	Released bool  `json:"released,omitempty"`
	FreshAt  int64 `json:"fresh_at,omitempty"` // unix seconds of the last staleness check
	Fresh    bool  `json:"fresh,omitempty"`
}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func statePath(dir, sid string) string {
	if sid == "" {
		return ""
	}
	sid = unsafeID.ReplaceAllString(sid, "_")
	if len(sid) > 64 {
		sid = sid[:64]
	}
	return filepath.Join(dir, "crawl-"+sid+".json")
}

func readState(p string) crawlState {
	var s crawlState
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func writeState(p string, s crawlState) {
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	b, _ := json.Marshal(s)
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, p)
	}
}

// pruneState drops state files of long finished sessions.
func pruneState(dir string) {
	days := 7
	if n, err := strconv.Atoi(os.Getenv("STACKLIT_CRAWL_STATE_DAYS")); err == nil && n > 0 {
		days = n
	}
	ents, _ := filepath.Glob(filepath.Join(dir, "crawl-*.json"))
	for _, f := range ents {
		if st, err := os.Stat(f); err == nil && time.Since(st.ModTime()) > time.Duration(days)*24*time.Hour {
			_ = os.Remove(f)
		}
	}
}

// indexReady says whether an index and graph exist and whether the index
// matches the working tree (only computed when needFresh). Unknown staleness counts as not fresh.
func indexReady(root string, needFresh bool) (exists, fresh bool) {
	st, err := os.Stat(filepath.Join(root, "stacklit.json"))
	if err != nil {
		return false, false
	}
	if _, err := os.Stat(filepath.Join(root, graphFile)); err != nil {
		return false, false
	}
	if !needFresh {
		// The staleness check runs several git commands, so only redirect pays for it.
		return true, true
	}
	s := git.CheckStale(root, "", st.ModTime(), func(p string) bool { return graph.LangOf(p) != "" })
	return true, s.InRepo && len(s.Changed) == 0
}

// pointerMaxBytes caps the graph size loaded to build an exact pointer.
func pointerMaxBytes() int64 {
	if n, err := strconv.ParseInt(os.Getenv("STACKLIT_CRAWL_POINTER_MAX_BYTES"), 10, 64); err == nil && n >= 0 {
		return n
	}
	return 8 << 20
}

// Representative hook texts, used for sizing in `setup --report`.
const (
	crawlNudgeSample = "stacklit: this looks like a repo-wide crawl (grep). The index answers structure questions in far fewer tokens: `stacklit query \"login handler\" --budget 800`. The index may be stale: `stacklit refresh --files <paths>` updates it."
	crawlDenySample  = "Blocked a repo-wide crawl (grep): the stacklit index is current. Run `stacklit query \"login handler\" --budget 800` instead. If it lacks what you need, repeat this call and it will be allowed."
)

func nudgeText(c Crawl, root string, fresh bool) string {
	text := fmt.Sprintf("stacklit: this looks like a repo-wide crawl (%s). The index answers structure questions in far fewer tokens: %s.", c.Kind, "`"+c.Command+"`")
	if c.Ident != "" {
		gp := filepath.Join(root, graphFile)
		if st, err := os.Stat(gp); err == nil && st.Size() <= pointerMaxBytes() {
			if g, err := graph.Load(gp); err == nil {
				if p := g.Pointer(c.Ident); p != "" {
					text = p
				}
			}
		}
	}
	if !fresh {
		text += " The index may be stale: `stacklit refresh --files <paths>` updates it."
	}
	return text
}

// CrawlGuardHook answers a PreToolUse payload. Any failure, a missing index
// or a narrow call produces no output, which Claude Code reads as allow.
func CrawlGuardHook(stdin io.Reader, installed string) (out string) {
	defer func() {
		if recover() != nil {
			out = ""
		}
	}()
	in := readHookInput(stdin)
	root := hookRoot(in)
	mode := CrawlMode(root, installed)
	if mode == CrawlOff || in.ToolName == "" {
		return ""
	}
	dir := session.StateDir(root)
	sp := statePath(dir, in.SessionID)
	crawl, ok := DetectCrawl(in, root)
	if !ok {
		if sp != "" {
			// A call that is not a crawl ends the run of consecutive redirects.
			if st := readState(sp); st.N > 0 && !st.Released {
				writeState(sp, crawlState{})
			}
		}
		return ""
	}
	st := crawlState{}
	if sp != "" {
		st = readState(sp)
	}
	redirecting := mode == CrawlRedirect && sp != "" && !st.Released
	exists, fresh := indexReady(root, false)
	if !exists {
		return ""
	}
	if redirecting {
		// The git staleness check is the slow part, so reuse a recent verdict.
		if now := time.Now().Unix(); st.FreshAt > 0 && now-st.FreshAt < freshTTL() {
			fresh = st.Fresh
		} else {
			_, fresh = indexReady(root, true)
			st.FreshAt, st.Fresh = now, fresh
		}
		if fresh && st.N < redirectLimit() {
			st.N++
			writeState(sp, st)
			pruneState(dir)
			reason := fmt.Sprintf("Blocked a repo-wide crawl (%s): the stacklit index is current. Run `%s` instead. If it lacks what you need, repeat this call and it will be allowed.", crawl.Kind, crawl.Command)
			return crawlJSON("deny", reason, "")
		}
		if fresh {
			st.Released = true
		}
		writeState(sp, st)
	} else if mode == CrawlRedirect {
		fresh = true
	}
	return crawlJSON("", "", nudgeText(crawl, root, fresh))
}
