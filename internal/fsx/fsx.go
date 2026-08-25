// Package fsx holds the filesystem helpers shared by packages that write small
// generated files. It knows nothing about tasks.
package fsx

import (
	"os"
	"path/filepath"
)

// WriteAtomic replaces a file in one step, the way the store does for task
// files. The callers write documents tq did not author — a project's marker,
// a guide beside its tasks — so a truncating write that failed halfway would
// destroy what was already there.
func WriteAtomic(path string, content []byte) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tq-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil { // leave nothing behind when the write failed
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(content); err != nil {
		return err
	}
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
