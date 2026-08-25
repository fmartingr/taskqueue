// Package tqtest holds the fixtures every package's tests need. It exists
// because t.TempDir() is not an isolation barrier here: discovery walks up out
// of it, and the environment can point tq somewhere else entirely, so a careless
// test writes into a developer's own queue.
package tqtest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fmartingr/taskqueue/internal/config"
)

// Isolate removes the configuration that could send a test outside its own
// temporary directory. Call it from TestMain: TQ_DIR is the documented way to
// point tq at a queue, so a developer may well have it exported, and without
// this every test would operate on their real one.
func Isolate() {
	for _, name := range []string{config.EnvTaskDir, config.EnvWalkForever} {
		_ = os.Unsetenv(name)
	}
}

// Root returns a temporary directory that discovery cannot climb out of.
// t.TempDir() alone is not enough: it honours TMPDIR, and the walk up looks for
// .git in every parent, so with TMPDIR inside a repository the fixtures would
// bind to that repository. Marking the root as a repository stops the walk here.
func Root(t *testing.T) string {
	t.Helper()
	// Belt and braces: Isolate cleared the ambient values, this clears anything
	// a test set before reaching for a fixture.
	t.Setenv(config.EnvTaskDir, "")
	t.Setenv(config.EnvWalkForever, "")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// WriteConfig plants a project marker in dir and returns its path.
func WriteConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, config.ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
