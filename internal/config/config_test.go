package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefault(t *testing.T) {
	cfg := Load(t.TempDir())
	if cfg.MaxDepth != 4 {
		t.Errorf("expected default max_depth=4, got %d", cfg.MaxDepth)
	}
	if cfg.MaxModules != 200 {
		t.Errorf("expected default max_modules=200, got %d", cfg.MaxModules)
	}
	if cfg.MaxExports != 10 {
		t.Errorf("expected default max_exports=10, got %d", cfg.MaxExports)
	}
	if cfg.Output.JSON != "stacklit.json" {
		t.Errorf("expected default output.json=stacklit.json, got %q", cfg.Output.JSON)
	}
	if cfg.Output.Mermaid != "DEPENDENCIES.md" {
		t.Errorf("expected default output.mermaid=DEPENDENCIES.md, got %q", cfg.Output.Mermaid)
	}
	if cfg.Output.HTML != "stacklit.html" {
		t.Errorf("expected default output.html=stacklit.html, got %q", cfg.Output.HTML)
	}
}

func TestLoadCustom(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".stacklitrc.json"),
		[]byte(`{"max_depth": 6, "ignore": ["custom/"]}`), 0644)
	cfg := Load(dir)
	if cfg.MaxDepth != 6 {
		t.Errorf("expected max_depth=6, got %d", cfg.MaxDepth)
	}
	if len(cfg.Ignore) != 1 {
		t.Errorf("expected 1 ignore pattern, got %d: %v", len(cfg.Ignore), cfg.Ignore)
	}
	// Unset fields should still use defaults.
	if cfg.MaxModules != 200 {
		t.Errorf("expected default max_modules=200, got %d", cfg.MaxModules)
	}
}

func TestLoadMalformed(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), []byte(`not json`), 0644)
	cfg := Load(dir)
	// Should fall back to defaults without panicking.
	if cfg.MaxDepth != 4 {
		t.Errorf("expected default max_depth=4 after malformed file, got %d", cfg.MaxDepth)
	}
}

func TestScanIgnoreIncludesOutputs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Ignore = []string{"custom/", "vendor\\libs"}
	cfg.Output.JSON = "out\\stacklit.json"
	cfg.Output.Mermaid = "docs\\DEPENDENCIES.md"
	cfg.Output.HTML = "stacklit.html"

	got := cfg.ScanIgnore()
	want := []string{"custom/", "vendor/libs", "out/stacklit.json", "docs/DEPENDENCIES.md", "stacklit.html"}
	if len(got) != len(want) {
		t.Fatalf("expected %d ignore patterns, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected ignore[%d]=%q, got %q (all=%v)", i, want[i], got[i], got)
		}
	}
}

func TestModuleCapsDefaultsRCAndEnv(t *testing.T) {
	dir := t.TempDir()
	cfg := Load(dir)
	if cfg.MaxModuleFiles != DefaultMaxModuleFiles || cfg.ModuleSplitFiles != DefaultModuleSplitFiles {
		t.Fatalf("defaults wrong: %d %d", cfg.MaxModuleFiles, cfg.ModuleSplitFiles)
	}
	rc := `{"max_module_files": 77, "module_split_files": 12, "max_exports": 31}`
	if err := os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), []byte(rc), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg = Load(dir)
	if cfg.MaxModuleFiles != 77 || cfg.ModuleSplitFiles != 12 || cfg.MaxExports != 31 {
		t.Fatalf("rc not applied: %+v", cfg)
	}
	t.Setenv("STACKLIT_MAX_MODULE_FILES", "5")
	t.Setenv("STACKLIT_MAX_EXPORTS", "3")
	t.Setenv("STACKLIT_MODULE_SPLIT_FILES", "-1")
	cfg = Load(dir)
	if cfg.MaxModuleFiles != 5 || cfg.MaxExports != 3 || cfg.ModuleSplitFiles != -1 {
		t.Fatalf("env not applied: %+v", cfg)
	}
}
