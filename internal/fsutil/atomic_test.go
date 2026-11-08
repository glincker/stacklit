package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.json")
	if err := WriteFileAtomic(p, []byte("one"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("two"), 0644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "two" {
		t.Fatalf("got %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestWriteFileIfChangedKeepsMtime(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a")
	if err := WriteFileIfChanged(p, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	st1, _ := os.Stat(p)
	if err := WriteFileIfChanged(p, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	st2, _ := os.Stat(p)
	if !st1.ModTime().Equal(st2.ModTime()) {
		t.Fatal("unchanged write touched the file")
	}
}

func TestWriteFileAtomicMissingDir(t *testing.T) {
	if err := WriteFileAtomic(filepath.Join(t.TempDir(), "nope", "f"), []byte("x"), 0644); err == nil {
		t.Fatal("expected error")
	}
}
