package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/schema"
)

func idxWithFiles(n int) *schema.Index {
	idx := sessIdx()
	idx.Tech.Languages = map[string]schema.LangStats{"go": {Files: n - n/10}, "json": {Files: n / 10}}
	return idx
}

func TestIsSmallRepoThresholdAndSwitches(t *testing.T) {
	root := t.TempDir()
	if !IsSmallRepo(root, idxWithFiles(100)) || IsSmallRepo(root, idxWithFiles(5000)) {
		t.Fatal("default threshold must split a 100 file repo from a 5000 file one")
	}
	if IsSmallRepo(root, idxWithFiles(599)) != true || IsSmallRepo(root, idxWithFiles(600)) {
		t.Fatal("the limit is exclusive: 599 is small, 600 is not")
	}
	if IsSmallRepo(root, sessIdx()) {
		t.Fatal("an index without file counts is never small")
	}
	t.Setenv("STACKLIT_SMALL_REPO_FILES", "0")
	if IsSmallRepo(root, idxWithFiles(100)) {
		t.Fatal("0 must switch small repo mode off")
	}
	t.Setenv("STACKLIT_SMALL_REPO_FILES", "50")
	if IsSmallRepo(root, idxWithFiles(100)) {
		t.Fatal("env must lower the limit")
	}
	t.Setenv("STACKLIT_SMALL_REPO_FILES", "")
	rc := `{"small_repo_files": 2000}`
	if err := os.WriteFile(filepath.Join(root, ".stacklitrc.json"), []byte(rc), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsSmallRepo(root, idxWithFiles(1500)) {
		t.Fatal("the config key must raise the limit")
	}
}

func TestSessionHookSmallRepoGetsOneLineNotTheDigest(t *testing.T) {
	t.Setenv("STACKLIT_QUIET_REPO_FILES", "0")
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	idx := idxWithFiles(100)
	writeIdx(t, root, idx)
	for _, src := range []string{"startup", "resume", "clear", "compact"} {
		c := ctxOf(t, SessionHook(payload(t, root, src, "s-"+src)), "SessionStart")
		if strings.Contains(c, "Codebase map") || strings.Contains(c, "modules:") {
			t.Fatalf("%s: a small repo must not get the digest: %q", src, c)
		}
		if !strings.Contains(c, "This repo is small (100 source files)") || !strings.Contains(c, "grep") || !strings.Contains(c, "stacklit map ") {
			t.Fatalf("%s: want the small repo line and the status line: %q", src, c)
		}
		if n := Tokens(c); n > 80 {
			t.Fatalf("%s: %d tokens for a small repo", src, n)
		}
	}
	// Even when a full digest sits in CLAUDE.md, the hook still says the repo is small.
	block := wrap(BodyFor(idx, DefaultSteering), StyleMarkdown)
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(block), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := ctxOf(t, SessionHook(payload(t, root, "startup", "x")), "SessionStart"); !strings.Contains(c, "This repo is small") {
		t.Fatalf("missing small repo line: %q", c)
	}
	// Switched off, the full digest returns.
	t.Setenv("STACKLIT_SMALL_REPO_FILES", "0")
	if err := os.Remove(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	if c := ctxOf(t, SessionHook(payload(t, root, "startup", "y")), "SessionStart"); !strings.Contains(c, "Codebase map") {
		t.Fatalf("mode off must restore the digest: %q", c)
	}
}

func TestSubagentHookSmallRepo(t *testing.T) {
	t.Setenv("STACKLIT_QUIET_REPO_FILES", "0")
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	writeIdx(t, root, idxWithFiles(100))
	b, _ := json.Marshal(map[string]string{"hook_event_name": "SubagentStart", "agent_type": "Explore", "cwd": root})
	c := ctxOf(t, SubagentHook(strings.NewReader(string(b))), "SubagentStart")
	if strings.Contains(c, "modules:") || !strings.Contains(c, "This repo is small") {
		t.Fatalf("subagent on a small repo: %q", c)
	}
}

func TestBodyForRootSmallBlockIsShortAndRecognisable(t *testing.T) {
	root := t.TempDir()
	small, full := idxWithFiles(100), idxWithFiles(5000)
	sb := BodyForRoot(root, small, "e")
	if Tokens(sb) >= Tokens(BodyFor(small, "e"))/2 {
		t.Errorf("small block %d tokens vs full %d", Tokens(sb), Tokens(BodyFor(small, "e")))
	}
	if BodyForRoot(root, full, "e") != BodyFor(full, "e") {
		t.Error("a large repo keeps the full digest")
	}
	if !noDashes(sb) || strings.Contains(sb, "modules:") {
		t.Errorf("bad small block: %q", sb)
	}
	// The structure token survives so the session hook can match the block.
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(wrap(sb, StyleMarkdown)), 0o644); err != nil {
		t.Fatal(err)
	}
	if rulesHash(root) == "" {
		t.Error("rulesHash cannot read the small block")
	}
	t.Logf("small block %d tokens, full digest %d tokens", Tokens(sb), Tokens(BodyFor(small, "e")))
}

func TestQuietRepoHooksInjectNothing(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	writeIdx(t, root, idxWithFiles(3000))
	if !IsQuietRepo(root, idxWithFiles(3000)) || IsQuietRepo(root, idxWithFiles(5000)) {
		t.Fatal("default quiet limit is exclusive at 5000")
	}
	for _, src := range []string{"startup", "resume", "compact"} {
		if out := SessionHook(payload(t, root, src, "q-"+src)); out != "" {
			t.Fatalf("%s: quiet repo must inject nothing: %q", src, out)
		}
	}
	in := strings.NewReader(`{"cwd":"` + root + `","agent_type":"Explore"}`)
	if out := SubagentHook(in); out != "" {
		t.Fatalf("quiet repo subagent must get nothing: %q", out)
	}
	t.Setenv("STACKLIT_QUIET_REPO_FILES", "0")
	if out := SessionHook(payload(t, root, "startup", "q-off")); out == "" {
		t.Fatal("quiet mode off must restore the hook text")
	}
}
