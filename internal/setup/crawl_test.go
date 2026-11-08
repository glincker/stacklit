package setup

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testRoot = "/work/repo"

func TestDetectBash(t *testing.T) {
	cases := []struct {
		cmd   string
		crawl bool
		kind  string
	}{
		// broad crawls
		{"find . -name '*.go'", true, "find"},
		{"find . -type f", true, "find"},
		{"find /work/repo -name foo.ts", true, "find"},
		{"find -name '*.py'", true, "find"},
		{"find . -maxdepth 3 -type f", true, "find"},
		{"ls -R", true, "ls"},
		{"ls -laR .", true, "ls"},
		{"ls --recursive ./", true, "ls"},
		{"tree", true, "tree"},
		{"tree .", true, "tree"},
		{"tree -L 3", true, "tree"},
		{"grep -r handleLogin .", true, "grep"},
		{"grep -rn handleLogin", true, "grep"},
		{"grep -R --include='*.go' Foo ./", true, "grep"},
		{"rg handleLogin", true, "grep"},
		{"rg -n 'func Foo' .", true, "grep"},
		{"rg --files", true, "grep"},
		{"git grep parseToken", true, "grep"},
		{"cd /work/repo && rg token", true, "grep"},
		{"echo hi; find . -name x", true, "find"},
		{"FOO=1 sudo find . -type d", true, "find"},
		{"bash -c 'find . -name x'", true, "find"},
		// Windows style
		{`dir /s`, true, "ls"},
		{`dir /s /b .`, true, "ls"},
		{`Get-ChildItem -Recurse`, true, "ls"},
		{`gci -r .`, true, "ls"},
		{`ls -Recurse`, true, "ls"},
		{`findstr /s /i "parseToken" *.go`, true, "grep"},
		{`cmd /c dir /s`, true, "ls"},
		{`powershell -Command "Get-ChildItem -Recurse"`, true, "ls"},
		// narrow or unrelated
		{"find src -name '*.go'", false, ""},
		{"find . -maxdepth 1 -type f", false, ""},
		{"find . -name x -delete", false, ""},
		{"ls", false, ""},
		{"ls -la", false, ""},
		{"ls -R src/api", false, ""},
		{"ls -r", false, ""},
		{"tree -L 1", false, ""},
		{"tree src", false, ""},
		{"grep -r token src/auth", false, ""},
		{"grep -n token main.go", false, ""},
		{"grep token file.txt", false, ""},
		{"cat main.go | grep token", false, ""},
		{"git log | rg fix", false, ""},
		{"rg token internal/auth", false, ""},
		{"rg token cmd/ internal/", false, ""},
		{"cd internal && grep -r foo .", false, ""},
		{"git status", false, ""},
		{"go test ./...", false, ""},
		{"npm run build", false, ""},
		{"make", false, ""},
		{"echo 'find . -name x'", false, ""},
		{"git grep parseToken -- internal/auth", false, ""},
		{`dir src /s`, false, ""},
		{`dir`, false, ""},
		{`Get-ChildItem src -Recurse`, false, ""},
		{`findstr /i foo main.go`, false, ""},
		{`findstr /s foo src\*.go`, false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		got, ok := DetectBash(c.cmd, testRoot)
		if ok != c.crawl {
			t.Errorf("%q: crawl=%v want %v (%+v)", c.cmd, ok, c.crawl, got)
			continue
		}
		if ok && got.Kind != c.kind {
			t.Errorf("%q: kind %q want %q", c.cmd, got.Kind, c.kind)
		}
		if ok && !strings.HasPrefix(got.Command, "stacklit ") {
			t.Errorf("%q: command %q", c.cmd, got.Command)
		}
	}
}

func TestDetectWindowsRoot(t *testing.T) {
	root := `C:\work\repo`
	if _, ok := DetectBash(`find C:\work\repo -name x`, root); !ok {
		t.Error("absolute Windows root should count as the repo root")
	}
	if _, ok := DetectBash(`dir C:\work\repo\src /s`, root); ok {
		t.Error("subdirectory must stay narrow")
	}
	if _, ok := DetectBash(`dir .\ /s`, root); !ok {
		t.Error(`.\ is the root`)
	}
}

func TestDetectCommandSuggestion(t *testing.T) {
	cases := []struct{ cmd, want string }{
		{"grep -r parseToken .", "stacklit explain parseToken"},
		{"rg 'login handler' .", `stacklit query "login handler" --budget 800`},
		{"find . -name '*Handler*.go'", `stacklit query "Handler" --budget 800`},
		{"ls -R", `stacklit query "<question>" --budget 800`},
	}
	for _, c := range cases {
		got, ok := DetectBash(c.cmd, testRoot)
		if !ok || got.Command != c.want {
			t.Errorf("%q: %q want %q", c.cmd, got.Command, c.want)
		}
	}
}

func TestDetectGlob(t *testing.T) {
	cases := []struct {
		pat, dir string
		crawl    bool
	}{
		{"**/*", "", true},
		{"**", "", true},
		{"**/*.go", "", true},
		{"**/*.{ts,tsx}", "/work/repo", true},
		{"**/*.*", ".", true},
		{"./**/*.py", "", true},
		{"**/*.go", "src", false},
		{"src/**/*.go", "", false},
		{"**/main.go", "", false},
		{"**/*Handler*.go", "", false},
		{"*.go", "", false},
		{"cmd/*", "", false},
	}
	for _, c := range cases {
		if _, ok := DetectGlob(c.pat, c.dir, testRoot); ok != c.crawl {
			t.Errorf("glob %q dir %q: %v want %v", c.pat, c.dir, ok, c.crawl)
		}
	}
}

func TestDetectSubagent(t *testing.T) {
	cases := []struct {
		p     string
		crawl bool
	}{
		{"Explore the codebase and tell me how it works", true},
		{"Please explore the entire repo to understand the architecture.", true},
		{"Give me an overview of the project structure", true},
		{"How is the repository structured?", true},
		{"Investigate the codebase for anything related to auth", true},
		{"Find where `handleLogin` is defined in the codebase", false},
		{"Explore the codebase and look at internal/auth/token.go", false},
		{"Explore the codebase, focusing on parse_token and refresh logic", false},
		{"Write unit tests for the parser", false},
		{strings.Repeat("Explore the codebase. ", 60), false},
	}
	for _, c := range cases {
		if _, ok := DetectSubagent(c.p); ok != c.crawl {
			t.Errorf("%q: %v want %v", c.p, ok, c.crawl)
		}
	}
}

func TestDetectReadDir(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "a.go")
	if err := os.WriteFile(f, []byte("package a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := DetectRead(d); !ok {
		t.Error("directory should be a crawl")
	}
	if _, ok := DetectRead(f); ok {
		t.Error("file must not be a crawl")
	}
	if _, ok := DetectRead(filepath.Join(d, "missing")); ok {
		t.Error("missing path must not be a crawl")
	}
	if _, ok := DetectRead(""); ok {
		t.Error("empty path must not be a crawl")
	}
}

// guardRepo builds a git repo with a fresh index and graph.
func guardRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	t.Setenv("STACKLIT_CRAWL_GUARD", "")
	root := t.TempDir()
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = root
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("stacklit*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "core.hooksPath=/dev/null", "commit", "-q", "-m", "init")
	writeIdx(t, root, sessIdx())
	if err := os.WriteFile(filepath.Join(root, graphFile), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// git log works in whole seconds, so date the index after the commit.
	future := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Join(root, "stacklit.json"), future, future); err != nil {
		t.Fatal(err)
	}
	return root
}

func hookIn(root, sid, tool string, input map[string]string) *bytes.Reader {
	b, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": sid, "cwd": root, "tool_name": tool, "tool_input": input})
	return bytes.NewReader(b)
}

type guardOut struct {
	Spec struct {
		Event    string `json:"hookEventName"`
		Decision string `json:"permissionDecision"`
		Reason   string `json:"permissionDecisionReason"`
		Context  string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func parseGuard(t *testing.T, out string) guardOut {
	t.Helper()
	var g guardOut
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("bad JSON %q: %v", out, err)
	}
	if g.Spec.Event != "PreToolUse" {
		t.Fatalf("event %q", g.Spec.Event)
	}
	return g
}

func crawlCall(root, sid string) *bytes.Reader {
	return hookIn(root, sid, "Bash", map[string]string{"command": "grep -r parseToken ."})
}

func TestCrawlGuardOffIsSilent(t *testing.T) {
	root := guardRepo(t)
	for _, mode := range []string{"", "off", "bogus"} {
		if out := CrawlGuardHook(crawlCall(root, "s1"), mode); out != "" {
			t.Errorf("installed %q: want silence, got %q", mode, out)
		}
	}
}

func TestCrawlGuardNudge(t *testing.T) {
	root := guardRepo(t)
	out := CrawlGuardHook(crawlCall(root, "s1"), CrawlNudge)
	g := parseGuard(t, out)
	if g.Spec.Decision != "" || g.Spec.Reason != "" {
		t.Fatalf("nudge must not decide: %+v", g.Spec)
	}
	if !strings.Contains(g.Spec.Context, "stacklit explain parseToken") {
		t.Fatalf("context lacks the command: %q", g.Spec.Context)
	}
	if strings.Contains(g.Spec.Context, "stale") {
		t.Fatalf("fresh index must not mention stale: %q", g.Spec.Context)
	}
	if strings.ContainsAny(out, "\u2013\u2014") {
		t.Fatal("dash character in output")
	}
}

func TestCrawlGuardRedirectAndLoopBreaker(t *testing.T) {
	root := guardRepo(t)
	t.Setenv("STACKLIT_CRAWL_REDIRECTS", "")
	for i := 1; i <= 2; i++ {
		g := parseGuard(t, CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect))
		if g.Spec.Decision != "deny" {
			t.Fatalf("call %d: decision %q", i, g.Spec.Decision)
		}
		if !strings.Contains(g.Spec.Reason, "stacklit explain parseToken") {
			t.Fatalf("reason lacks command: %q", g.Spec.Reason)
		}
	}
	// Third consecutive crawl is allowed with a nudge, and stays allowed.
	for i := 0; i < 3; i++ {
		g := parseGuard(t, CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect))
		if g.Spec.Decision != "" || g.Spec.Context == "" {
			t.Fatalf("released call %d: %+v", i, g.Spec)
		}
	}
	// Another session still gets redirected.
	if g := parseGuard(t, CrawlGuardHook(crawlCall(root, "s2"), CrawlRedirect)); g.Spec.Decision != "deny" {
		t.Fatalf("new session: %+v", g.Spec)
	}
}

func TestCrawlGuardCounterResetsOnNarrowCall(t *testing.T) {
	root := guardRepo(t)
	narrow := func() *bytes.Reader {
		return hookIn(root, "s1", "Bash", map[string]string{"command": "stacklit query foo"})
	}
	CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect)
	CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect)
	if out := CrawlGuardHook(narrow(), CrawlRedirect); out != "" {
		t.Fatalf("narrow call must be silent: %q", out)
	}
	if g := parseGuard(t, CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect)); g.Spec.Decision != "deny" {
		t.Fatalf("counter should have reset: %+v", g.Spec)
	}
}

func TestCrawlGuardNeverBlocksNarrowOrOtherTools(t *testing.T) {
	root := guardRepo(t)
	calls := []struct {
		tool string
		in   map[string]string
	}{
		{"Bash", map[string]string{"command": "go test ./..."}},
		{"Bash", map[string]string{"command": "grep -r token internal/auth"}},
		{"Read", map[string]string{"file_path": filepath.Join(root, "main.go")}},
		{"Edit", map[string]string{"file_path": filepath.Join(root, "main.go")}},
		{"Write", map[string]string{"file_path": filepath.Join(root, "new.go")}},
		{"Glob", map[string]string{"pattern": "src/**/*.go"}},
		{"Agent", map[string]string{"prompt": "Fix the failing test in `parser_test.go`"}},
	}
	for _, c := range calls {
		if out := CrawlGuardHook(hookIn(root, "s1", c.tool, c.in), CrawlRedirect); out != "" {
			t.Errorf("%s %v: want silence, got %q", c.tool, c.in, out)
		}
	}
}

func TestCrawlGuardRedirectSurfaces(t *testing.T) {
	root := guardRepo(t)
	cases := []struct {
		tool string
		in   map[string]string
	}{
		{"Bash", map[string]string{"command": "find . -name '*.go'"}},
		{"Glob", map[string]string{"pattern": "**/*"}},
		{"Read", map[string]string{"file_path": root}},
		{"Agent", map[string]string{"prompt": "Explore the codebase and report how it is organised", "subagent_type": "Explore"}},
	}
	for i, c := range cases {
		sid := "surf" + string(rune('a'+i))
		g := parseGuard(t, CrawlGuardHook(hookIn(root, sid, c.tool, c.in), CrawlRedirect))
		if g.Spec.Decision != "deny" || !strings.Contains(g.Spec.Reason, "stacklit ") {
			t.Errorf("%s: %+v", c.tool, g.Spec)
		}
	}
}

func TestCrawlGuardFailOpen(t *testing.T) {
	root := guardRepo(t)
	// Garbage and empty stdin.
	for _, s := range []string{"", "not json", "{", `{"tool_name":`} {
		if out := CrawlGuardHook(strings.NewReader(s), CrawlRedirect); out != "" {
			t.Errorf("stdin %q: %q", s, out)
		}
	}
	// No index: crawl is allowed silently.
	bare := t.TempDir()
	if out := CrawlGuardHook(crawlCall(bare, "s1"), CrawlRedirect); out != "" {
		t.Errorf("no index: %q", out)
	}
	// Index without graph.
	if err := os.Remove(filepath.Join(root, graphFile)); err != nil {
		t.Fatal(err)
	}
	if out := CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect); out != "" {
		t.Errorf("no graph: %q", out)
	}
	// Missing session id: nudge only, never deny.
	root2 := guardRepo(t)
	g := parseGuard(t, CrawlGuardHook(crawlCall(root2, ""), CrawlRedirect))
	if g.Spec.Decision != "" {
		t.Errorf("no session id must not deny: %+v", g.Spec)
	}
	// Unwritable state dir: still no panic and still answers.
	t.Setenv("STACKLIT_CACHE_DIR", filepath.Join(root2, "stacklit.json", "nope"))
	_ = CrawlGuardHook(crawlCall(root2, "s9"), CrawlRedirect)
}

func TestCrawlGuardStaleIndexOnlyNudges(t *testing.T) {
	root := guardRepo(t)
	late := filepath.Join(root, "late.go")
	if err := os.WriteFile(late, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Add(2 * time.Minute)
	if err := os.Chtimes(late, after, after); err != nil {
		t.Fatal(err)
	}
	g := parseGuard(t, CrawlGuardHook(crawlCall(root, "s1"), CrawlRedirect))
	if g.Spec.Decision != "" || !strings.Contains(g.Spec.Context, "stacklit refresh --files") {
		t.Fatalf("stale index must only nudge with a refresh hint: %+v", g.Spec)
	}
}

func TestCrawlModePrecedence(t *testing.T) {
	root := t.TempDir()
	t.Setenv("STACKLIT_CRAWL_GUARD", "")
	if got := CrawlMode(root, ""); got != CrawlOff {
		t.Fatalf("default %q", got)
	}
	if got := CrawlMode(root, "nudge"); got != CrawlNudge {
		t.Fatalf("installed %q", got)
	}
	if err := os.WriteFile(filepath.Join(root, ".stacklitrc.json"), []byte(`{"crawl_guard":"redirect"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := CrawlMode(root, "nudge"); got != CrawlRedirect {
		t.Fatalf("config %q", got)
	}
	t.Setenv("STACKLIT_CRAWL_GUARD", "off")
	if got := CrawlMode(root, "nudge"); got != CrawlOff {
		t.Fatalf("env %q", got)
	}
}

func TestCrawlGuardLatency(t *testing.T) {
	root := guardRepo(t)
	measure := func(mode, sid string) time.Duration {
		const n = 10
		start := time.Now()
		for i := 0; i < n; i++ {
			CrawlGuardHook(hookIn(root, sid, "Bash", map[string]string{"command": "find . -name '*.go'"}), mode)
		}
		return time.Since(start) / n
	}
	nudge := measure(CrawlNudge, "lat1")
	// A huge limit keeps every call on the freshness path.
	t.Setenv("STACKLIT_CRAWL_REDIRECTS", "1000")
	redirect := measure(CrawlRedirect, "lat2")
	t.Logf("per call: nudge %v, redirect (with git freshness) %v", nudge, redirect)
	if nudge > 100*time.Millisecond {
		t.Fatalf("nudge latency %v exceeds 100ms", nudge)
	}
	if redirect > 4*100*time.Millisecond {
		t.Fatalf("redirect latency %v is far above budget", redirect)
	}
}

func TestApplyCrawlGuardInstallUninstall(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	r := &Runner{Root: root, Out: &buf}
	settings := filepath.Join(root, ".claude", "settings.json")
	count := func() int {
		b, err := os.ReadFile(settings)
		if err != nil {
			return 0
		}
		return strings.Count(string(b), "stacklit hook crawl")
	}
	for i := 0; i < 2; i++ { // idempotent
		if err := r.ApplyClaudeExtras(ClaudeOptions{CrawlGuard: CrawlNudge}, false); err != nil {
			t.Fatal(err)
		}
	}
	if count() != 1 {
		t.Fatalf("want one hook, got %d", count())
	}
	b, _ := os.ReadFile(settings)
	if !strings.Contains(string(b), "stacklit hook crawl nudge") || !strings.Contains(string(b), crawlMatcher) {
		t.Fatalf("settings: %s", b)
	}
	// Switching mode replaces, never duplicates.
	if err := r.ApplyClaudeExtras(ClaudeOptions{CrawlGuard: CrawlRedirect}, false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(settings)
	if count() != 1 || !strings.Contains(string(b), "crawl redirect") {
		t.Fatalf("switch: %s", b)
	}
	// "off" removes only the crawl hook.
	if err := r.ApplyClaudeExtras(ClaudeOptions{CrawlGuard: "off"}, false); err != nil {
		t.Fatal(err)
	}
	if count() != 0 {
		t.Fatal("off should remove the hook")
	}
	if err := r.ApplyClaudeExtras(ClaudeOptions{CrawlGuard: CrawlNudge}, false); err != nil {
		t.Fatal(err)
	}
	if err := r.ApplyClaudeExtras(ClaudeOptions{}, true); err != nil {
		t.Fatal(err)
	}
	if count() != 0 {
		t.Fatal("uninstall should remove the hook")
	}
}

func TestApplyCrawlGuardDryRun(t *testing.T) {
	root := t.TempDir()
	r := &Runner{Root: root, DryRun: true, Out: &bytes.Buffer{}}
	if err := r.ApplyClaudeExtras(ClaudeOptions{CrawlGuard: CrawlNudge}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude")); err == nil {
		t.Fatal("dry run wrote files")
	}
	if r.Changed != 1 {
		t.Fatalf("changed %d", r.Changed)
	}
}
