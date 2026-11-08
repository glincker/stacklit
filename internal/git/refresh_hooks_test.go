package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRefreshHooksInstallIdempotentAndRemovable(t *testing.T) {
	root := t.TempDir()
	hooks := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	mine := "#!/bin/sh\necho mine\n"
	if err := os.WriteFile(filepath.Join(hooks, "post-merge"), []byte(mine), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := InstallRefreshHooks(root)
	if err != nil || len(got) != 3 {
		t.Fatalf("install: %v %v", got, err)
	}
	if again, _ := InstallRefreshHooks(root); len(again) != 0 {
		t.Fatalf("second install changed %v", again)
	}
	pc, _ := os.ReadFile(filepath.Join(hooks, "post-checkout"))
	if !strings.Contains(string(pc), `"$3" = "1"`) || !strings.Contains(string(pc), "generate --quiet") {
		t.Fatalf("post-checkout body: %s", pc)
	}
	if st, _ := os.Stat(filepath.Join(hooks, "post-rewrite")); st == nil || st.Mode()&0o100 == 0 {
		t.Fatal("hook not executable")
	}
	if _, err := RemoveRefreshHooks(root); err != nil {
		t.Fatal(err)
	}
	pm, _ := os.ReadFile(filepath.Join(hooks, "post-merge"))
	if string(pm) != mine {
		t.Fatalf("user hook not restored: %q", pm)
	}
	if _, err := os.Stat(filepath.Join(hooks, "post-checkout")); err == nil {
		t.Fatal("empty hook not removed")
	}
}
