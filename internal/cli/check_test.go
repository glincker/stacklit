package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func runCheck(t *testing.T, rc string, args ...string) (string, int) {
	t.Helper()
	dir := t.TempDir()
	idx := `{"modules":{"a":{"depends_on":["b"]},"b":{"depends_on":["a"]}},"dependencies":{"edges":[["a","b"],["b","a"]]}}`
	os.WriteFile(filepath.Join(dir, "stacklit.json"), []byte(idx), 0o644)
	if rc != "" {
		os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), []byte(rc), 0o644)
	}
	cmd := newCheckCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--root", dir}, args...))
	err := cmd.Execute()
	return out.String(), ExitCode(err)
}

func TestCheckExitCodes(t *testing.T) {
	if _, code := runCheck(t, ""); code != ExitOK {
		t.Fatalf("no rules: %d", code)
	}
	out, code := runCheck(t, `{"check":{"cycles":true}}`, "--json")
	if code != ExitProblems {
		t.Fatalf("cycle: %d", code)
	}
	var res struct {
		OK         bool `json:"ok"`
		Violations []struct{ Rule string }
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.OK || len(res.Violations) != 1 {
		t.Fatalf("%v %s", err, out)
	}
}
