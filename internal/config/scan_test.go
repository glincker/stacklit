package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRC(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScanFieldsLoad(t *testing.T) {
	dir := writeRC(t, `{"roots":["src","./pkg/"],"include":["*.go"],"exclude":["gen/"],"max_file_size":2048,"workers":3,"cache":false,"timestamps":false}`)
	c := Load(dir)
	if c.MaxFileSize != 2048 || c.Workers != 3 || c.CacheEnabled() || c.TimestampsEnabled() {
		t.Fatalf("unexpected cfg: %+v", c)
	}
	roots := c.NormalizedRoots()
	if len(roots) != 2 || roots[0] != "src" || roots[1] != "pkg" {
		t.Fatalf("roots: %v", roots)
	}
	found := false
	for _, p := range c.ScanIgnore() {
		if p == "gen/" {
			found = true
		}
	}
	if !found {
		t.Fatal("exclude not merged into ScanIgnore")
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := writeRC(t, `{"max_file_size":2048}`)
	t.Setenv("STACKLIT_MAX_FILE_SIZE", "99")
	t.Setenv("STACKLIT_WORKERS", "2")
	t.Setenv("STACKLIT_NO_CACHE", "1")
	t.Setenv("STACKLIT_ROOTS", "a, b")
	c := Load(dir)
	if c.MaxFileSize != 99 || c.Workers != 2 || c.CacheEnabled() || len(c.Roots) != 2 {
		t.Fatalf("env not applied: %+v", c)
	}
}

func TestDefaultsHaveFileSize(t *testing.T) {
	if Load(t.TempDir()).MaxFileSize != DefaultMaxFileSize {
		t.Fatal("default max file size missing")
	}
}

func TestLoadStrictErrors(t *testing.T) {
	if _, err := LoadStrict(writeRC(t, `{not json`)); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := LoadStrict(writeRC(t, `{"bogus":1}`)); err == nil {
		t.Fatal("expected unknown field error")
	}
	if _, err := LoadStrict(writeRC(t, `{"roots":["../outside"]}`)); err == nil {
		t.Fatal("expected root validation error")
	}
	if _, err := LoadStrict(t.TempDir()); err != nil {
		t.Fatalf("missing file must be fine: %v", err)
	}
}

func TestSmallRepoLimit(t *testing.T) {
	if got := Load(t.TempDir()).SmallRepoLimit(); got != DefaultSmallRepoFiles {
		t.Fatalf("default %d", got)
	}
	if got := Load(writeRC(t, `{"small_repo_files":0}`)).SmallRepoLimit(); got != 0 {
		t.Fatalf("explicit 0 must turn it off, got %d", got)
	}
	dir := writeRC(t, `{"small_repo_files":10}`)
	t.Setenv("STACKLIT_SMALL_REPO_FILES", "25")
	if got := Load(dir).SmallRepoLimit(); got != 25 {
		t.Fatalf("env must win, got %d", got)
	}
	t.Setenv("STACKLIT_SMALL_REPO_FILES", "junk")
	if got := Load(dir).SmallRepoLimit(); got != 10 {
		t.Fatalf("a bad env value is ignored, got %d", got)
	}
	if _, err := LoadStrict(writeRC(t, `{"small_repo_files":-1}`)); err == nil {
		t.Fatal("a negative limit must fail validation")
	}
}
