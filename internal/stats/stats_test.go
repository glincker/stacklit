package stats

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOffByDefaultWritesNothing(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	t.Setenv("STACKLIT_STATS", "")
	root := t.TempDir()
	Record(root, "query", "x")
	if _, err := os.Stat(LogPath()); err == nil {
		t.Fatal("log written while disabled")
	}
}

func TestConfigKeyEnablesAndEnvOverrides(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	t.Setenv("STACKLIT_STATS", "")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".stacklitrc.json"), []byte(`{"stats": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Enabled(root) {
		t.Fatal("config key ignored")
	}
	t.Setenv("STACKLIT_STATS", "0")
	if Enabled(root) {
		t.Fatal("env 0 must win")
	}
}

func TestRecordAndReport(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	t.Setenv("STACKLIT_STATS", "1")
	root := t.TempDir()
	big := strings.Repeat("x", 4000)
	if err := os.WriteFile(filepath.Join(root, "big.go"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	out := "big.go:10 func Foo\nbig.go:20 func Bar\n../outside.go missing.go\n"
	Record(root, "query", out)
	Record(root, "slice", "big.go:1")
	s, err := Read(time.Time{})
	if err != nil || s.Calls != 2 {
		t.Fatalf("%+v %v", s, err)
	}
	if s.ReplacedEst != 2000 {
		t.Fatalf("replaced estimate counts each file once per call: %d", s.ReplacedEst)
	}
	if s.Served != Tokens(out)+Tokens("big.go:1") || len(s.ByCmd) != 2 {
		t.Fatalf("%+v", s)
	}
	if later, _ := Read(time.Now().Add(time.Hour)); later.Calls != 0 {
		t.Fatal("since filter")
	}
	b, _ := os.ReadFile(LogPath())
	if strings.Contains(string(b), root) {
		t.Fatal("log must not contain full paths")
	}
}

func TestReadMissingLog(t *testing.T) {
	t.Setenv("STACKLIT_CACHE_DIR", t.TempDir())
	if s, err := Read(time.Time{}); err != nil || s.Calls != 0 {
		t.Fatalf("%+v %v", s, err)
	}
}
