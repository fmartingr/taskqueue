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
	"github.com/fmartingr/taskqueue/internal/store"
	"github.com/fmartingr/taskqueue/internal/task"
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

// NewStore returns a store backed by a fresh task directory inside an isolated
// root. The store package keeps its own copy of this: a package whose tests
// reach its internals cannot import a helper that builds one of its own values.
func NewStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.InitStore(Root(t))
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return s
}

// MustCreate adds a task or fails the test.
func MustCreate(t *testing.T, s *store.Store, in store.CreateTaskInput) task.Task {
	t.Helper()
	created, err := s.Create(in)
	if err != nil {
		t.Fatalf("Create(%+v): %v", in, err)
	}
	return created
}
