package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/glincker/stacklit/internal/setup"
)

func TestGuideCommand(t *testing.T) {
	out, err := execCLI(t, "", "guide")
	if err != nil {
		t.Fatal(err)
	}
	if out != setup.GuideText() {
		t.Fatal("guide output differs from the shared text source")
	}
	out, err = execCLI(t, "", "guide", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var j struct {
		Rules string `json:"rules"`
	}
	if err := json.Unmarshal([]byte(out), &j); err != nil || !strings.Contains(j.Rules, "Do not crawl") {
		t.Fatalf("bad json guide: %v %q", err, out)
	}
	// Flags persist on the shared root command, so reset for later tests.
	_, _ = execCLI(t, "", "guide", "--json=false")
}

func TestCrawlHookCommandFailsOpen(t *testing.T) {
	t.Setenv("STACKLIT_CRAWL_GUARD", "")
	for _, in := range []string{"", "garbage"} {
		out, err := execCLI(t, in, "hook", "crawl", "redirect")
		if err != nil || out != "" {
			t.Fatalf("stdin %q: out %q err %v", in, out, err)
		}
	}
}

func TestSetupRejectsBadCrawlMode(t *testing.T) {
	root := project(t)
	t.Chdir(root)
	t.Cleanup(func() {
		if c, _, err := rootCmd.Find([]string{"setup"}); err == nil {
			_ = c.Flags().Set("crawl-guard", "")
			_ = c.Flags().Set("dry-run", "false")
		}
	})
	if _, err := execCLI(t, "", "setup", "claude", "--dry-run", "--crawl-guard=sideways"); err == nil {
		t.Fatal("want an error for an unknown mode")
	}
	out, err := execCLI(t, "", "setup", "claude", "--dry-run", "--crawl-guard")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, ".claude/settings.json") {
		t.Fatalf("bare --crawl-guard should plan a settings change:\n%s", out)
	}
}
