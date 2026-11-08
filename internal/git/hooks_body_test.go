package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookBodyFollowsConfiguredOutputs(t *testing.T) {
	dir := t.TempDir()
	if got := HookBody(dir); !strings.Contains(got, "git add stacklit.json DEPENDENCIES.md ") {
		t.Fatalf("default body stages wrong files: %q", got)
	}
	rc := `{"output":{"mermaid":"docs/DEPENDENCIES.md"}}`
	if err := os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), []byte(rc), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := HookBody(dir); !strings.Contains(got, "git add stacklit.json docs/DEPENDENCIES.md ") {
		t.Fatalf("custom body stages wrong files: %q", got)
	}
}
