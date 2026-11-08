package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/schema"
	"github.com/glincker/stacklit/internal/session"
)

func sessIdx() *schema.Index {
	return &schema.Index{
		Project: schema.Project{Name: "demo"},
		Tech:    schema.Tech{PrimaryLanguage: "Go"},
		Modules: map[string]schema.ModuleInfo{
			"core":  {Purpose: "Core logic", Exports: []string{"Run", "Stop"}, Lines: 100, Files: 3, DependedBy: []string{"cli"}},
			"cli":   {Purpose: "Commands", Exports: []string{"Execute"}, Lines: 50, Files: 2, DependsOn: []string{"core"}},
			"store": {Purpose: "Storage", Exports: []string{"Open"}, Lines: 70, Files: 2},
		},
		Dependencies: schema.Dependencies{Edges: [][2]string{{"cli", "core"}}, MostDepended: []string{"core"}},
	}
}

func writeIdx(t *testing.T, root string, idx *schema.Index) {
	t.Helper()
	b, err := json.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "stacklit.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func payload(t *testing.T, root, source, id string) *strings.Reader {
	t.Helper()
	b, _ := json.Marshal(map[string]string{"hook_event_name": "SessionStart", "source": source, "session_id": id, "cwd": root})
	return strings.NewReader(string(b))
}

func ctxOf(t *testing.T, out, event string) string {
	t.Helper()
	var o hookOutput
	if err := json.Unmarshal([]byte(out), &o); err != nil {
		t.Fatalf("not valid hook JSON: %v\n%q", err, out)
	}
	if o.Spec.Event != event {
		t.Fatalf("hookEventName %q, want %q", o.Spec.Event, event)
	}
	return o.Spec.Context
}

func TestSessionHookAllSourcesInjectDigestWithoutRules(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	writeIdx(t, root, sessIdx())
	for _, src := range []string{"startup", "resume", "clear", "compact", "fork"} {
		out := SessionHook(payload(t, root, src, "s-"+src))
		c := ctxOf(t, out, "SessionStart")
		for _, want := range []string{"Codebase map", "core", "stacklit slice <symbol>", "stacklit map "} {
			if !strings.Contains(c, want) {
				t.Fatalf("%s: missing %q in %q", src, want, c)
			}
		}
		if len(c) > 10000 {
			t.Fatalf("over the 10,000 character hook cap: %d", len(c))
		}
		if n := Tokens(c); n > digestBudget()+120 {
			t.Fatalf("%s: %d tokens", src, n)
		}
	}
	rec := session.ReadInjected(session.StateDir(session.Root(root)))
	if rec.For("s-compact") != session.StructureHash(sessIdx()) {
		t.Fatalf("injection not recorded: %+v", rec)
	}
}

func TestSessionHookSkipsDigestWhenRulesCarryIt(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	idx := sessIdx()
	writeIdx(t, root, idx)
	block := wrap(BodyFor(idx, DefaultSteering), StyleMarkdown)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("# mine\n\n"+block), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, src := range []string{"startup", "compact"} {
		c := ctxOf(t, SessionHook(payload(t, root, src, "s1")), "SessionStart")
		if strings.Contains(c, "Codebase map") || Tokens(c) > 80 {
			t.Fatalf("%s: CLAUDE.md already holds the digest, got %d tokens: %q", src, Tokens(c), c)
		}
	}
	// Structure moves on while CLAUDE.md is old: the fresh digest is injected.
	idx.Modules["extra"] = schema.ModuleInfo{Purpose: "New", Exports: []string{"X"}}
	writeIdx(t, root, idx)
	c := ctxOf(t, SessionHook(payload(t, root, "startup", "s2")), "SessionStart")
	if !strings.Contains(c, "extra") {
		t.Fatalf("stale CLAUDE.md block must be superseded: %q", c)
	}
}

func TestSessionHookNoIndexIsSilentAndHintsOnce(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	if out := SessionHook(payload(t, root, "resume", "a")); out != "" {
		t.Fatalf("resume without index must be silent: %q", out)
	}
	c := ctxOf(t, SessionHook(payload(t, root, "startup", "a")), "SessionStart")
	if !strings.Contains(c, "stacklit init") {
		t.Fatalf("hint missing: %q", c)
	}
	if out := SessionHook(payload(t, root, "startup", "b")); out != "" {
		t.Fatalf("hint must show once, got %q", out)
	}
}

func TestDigestByteStableAcrossNonStructuralEdits(t *testing.T) {
	base := BodyFor(sessIdx(), "a")
	b := sessIdx()
	for n, m := range b.Modules {
		m.Lines += 500
		m.Activity = "high"
		b.Modules[n] = m
	}
	b.GeneratedAt = "2030-01-01T00:00:00Z"
	b.Structure.TotalLines += 9999
	if got := BodyFor(b, "a"); got != base {
		t.Fatalf("digest changed on a non-structural edit:\n%s\nvs\n%s", base, got)
	}
	c := sessIdx()
	c.Modules["newmod"] = schema.ModuleInfo{Purpose: "New", Exports: []string{"N"}}
	if BodyFor(c, "a") == base {
		t.Fatal("digest must change when a module is added")
	}
	d := sessIdx()
	m := d.Modules["core"]
	m.Exports = append(m.Exports, "Fresh")
	d.Modules["core"] = m
	if BodyFor(d, "a") == base {
		t.Fatal("digest must change when an export is added")
	}
}

func TestSteeringVariantsStableAndSmall(t *testing.T) {
	idx := sessIdx()
	seen := map[string]bool{}
	for _, v := range SteeringVariants() {
		a, b := BodyFor(idx, v), BodyFor(idx, v)
		if a != b {
			t.Fatalf("variant %s not byte-stable", v)
		}
		if Tokens(a) > digestBudget() {
			t.Fatalf("variant %s is %d tokens, over the cap %d", v, Tokens(a), digestBudget())
		}
		for _, want := range []string{"stacklit query", "stacklit slice", "stacklit impact"} {
			if !strings.Contains(a, want) {
				t.Fatalf("variant %s lacks %q", v, want)
			}
		}
		if strings.ContainsRune(a, rune(0x2013)) || strings.ContainsRune(a, rune(0x2014)) {
			t.Fatalf("variant %s has a dash character", v)
		}
		seen[a] = true
	}
	if len(seen) != len(SteeringVariants()) {
		t.Fatal("variants must differ")
	}
	if BodyFor(idx, "nope") != BodyFor(idx, DefaultSteering) {
		t.Fatal("unknown variant must fall back to the default")
	}
	t.Setenv("STACKLIT_STEERING", "c")
	if SteeringFor(t.TempDir()) != "c" {
		t.Fatal("env override")
	}
}

func TestSubagentHookInjectsOnlyForAgentsWithoutClaudeMd(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	writeIdx(t, root, sessIdx())
	call := func(agent string) string {
		b, _ := json.Marshal(map[string]string{"hook_event_name": "SubagentStart", "agent_type": agent, "cwd": root})
		return SubagentHook(strings.NewReader(string(b)))
	}
	for _, a := range []string{"Explore", "Plan"} {
		c := ctxOf(t, call(a), "SubagentStart")
		if !strings.Contains(c, "core") || Tokens(c) > digestBudget()+60 {
			t.Fatalf("%s: %d tokens: %q", a, Tokens(c), c)
		}
	}
	for _, a := range []string{"general-purpose", "reviewer", ""} {
		if out := call(a); out != "" {
			t.Fatalf("%q loads CLAUDE.md already, got %q", a, out)
		}
	}
	t.Setenv("STACKLIT_SUBAGENT_TYPES", "reviewer")
	if call("reviewer") == "" || call("Explore") != "" {
		t.Fatal("env override of subagent types")
	}
	if SubagentHook(strings.NewReader("{")) != "" {
		t.Fatal("garbage must be silent")
	}
}

func TestClaudeSettingsHooksInstallAndRemove(t *testing.T) {
	dir := t.TempDir()
	tl, ok := Lookup("claude")
	if !ok {
		t.Fatal("claude tool")
	}
	o := Options{Tools: []Tool{tl}, NoMCP: true, Claude: ClaudeOptions{SessionHook: true, RefreshHook: true}}
	run(t, dir, o, false)
	b, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Command string `json:"command"`
			Async   bool   `json:"async"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	h := m["hooks"]
	if g := h["SessionStart"]; len(g) != 1 || g[0].Matcher != "startup|resume|clear|compact" || g[0].Hooks[0].Command != hookSessionCmd {
		t.Fatalf("SessionStart: %+v", g)
	}
	if g := h["SubagentStart"]; len(g) != 1 || g[0].Matcher != "Explore|Plan" {
		t.Fatalf("SubagentStart: %+v", g)
	}
	if g := h["PostToolUse"]; len(g) != 1 || g[0].Matcher != "Edit|Write|MultiEdit|NotebookEdit" || !g[0].Hooks[0].Async {
		t.Fatalf("PostToolUse must be async: %+v", g)
	}
	run(t, dir, o, false)
	o.Uninstall = true
	run(t, dir, o, false)
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); err == nil {
		t.Fatal("settings.json should be removed when empty")
	}
}
