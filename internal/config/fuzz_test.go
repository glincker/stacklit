package config

import (
	"os"
	"path/filepath"
	"testing"
)

// FuzzLoadConfig makes sure hostile .stacklitrc.json content never panics
// and always yields a usable config.
func FuzzLoadConfig(f *testing.F) {
	for _, s := range []string{
		`{}`, `{"roots":["a","../b"]}`, `{"max_file_size":-1}`, `[1,2]`, `null`, `{"workers":"x"}`, `{"cache":null}`, ``,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".stacklitrc.json"), data, 0o644); err != nil {
			t.Skip()
		}
		cfg := Load(dir)
		if cfg.MaxFileSize <= 0 || cfg.MaxDepth <= 0 {
			t.Fatalf("unusable defaults: %+v", cfg)
		}
		_ = cfg.NormalizedRoots()
		_ = cfg.ScanIgnore()
		_ = Check(dir)
		_, _ = LoadStrict(dir)
	})
}
