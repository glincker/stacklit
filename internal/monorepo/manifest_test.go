package monorepo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadManifest(t *testing.T) {
	cases := []struct {
		file, body string
		want       Manifest
	}{
		{"package.json", `{"name":"@a/b"}`, Manifest{"@a/b", "npm"}},
		{"go.mod", "module example.com/x/y\n\ngo 1.22\n", Manifest{"example.com/x/y", "go"}},
		{"Cargo.toml", "[workspace]\nmembers=[]\n\n[package]\nname = \"my-crate\"\n", Manifest{"my-crate", "cargo"}},
		{"pyproject.toml", "[project]\nname = \"pylib\"\n", Manifest{"pylib", "python"}},
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, c.file), []byte(c.body), 0o644); err != nil {
			t.Fatal(err)
		}
		got, ok := ReadManifest(dir)
		if !ok || got != c.want {
			t.Errorf("%s: got %+v ok=%v, want %+v", c.file, got, ok, c.want)
		}
	}
	if _, ok := ReadManifest(t.TempDir()); ok {
		t.Error("empty dir must not yield a manifest")
	}
}

func TestManifestMatches(t *testing.T) {
	npm := Manifest{"@a/ui", "npm"}
	if !npm.Matches("@a/ui") || !npm.Matches("@a/ui/button") || npm.Matches("@a/ui-kit") || npm.Matches("react") {
		t.Error("npm matching wrong")
	}
	cargo := Manifest{"my-crate", "cargo"}
	if !cargo.Matches("my_crate::thing") || cargo.Matches("other") {
		t.Error("cargo matching wrong")
	}
	if (Manifest{}).Matches("x") {
		t.Error("empty manifest must match nothing")
	}
}
