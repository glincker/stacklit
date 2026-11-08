// Package fsutil holds small filesystem helpers shared across stacklit.
package fsutil

import (
	"bytes"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path via a temp file in the same directory
// and a rename, so a crash or a concurrent reader never sees a partial file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// WriteFileIfChanged is WriteFileAtomic but skips the write when the file
// already holds identical bytes, which keeps mtimes and git status quiet.
func WriteFileIfChanged(path string, data []byte, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
		return nil
	}
	return WriteFileAtomic(path, data, perm)
}
