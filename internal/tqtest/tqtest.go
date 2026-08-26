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

// isolationEnv is the configuration that can send a test outside its own
// temporary directory: the two variables discovery reads before anything else.
var isolationEnv = []string{config.EnvTaskDir, config.EnvWalkForever}

// ambient is what those variables held when the test binary started, taken
// before Isolate clears them. It is what lets RequireIsolated tell a suite that
// was isolated from one that simply ran in a clean shell.
var ambient = map[string]string{}

// isolated records that Isolate ran. Without it the guard could only ever
// observe a clean environment, which is also what a developer with nothing
// exported sees — so removing the call would keep the suite green.
var isolated bool

func init() {
	for _, name := range isolationEnv {
		ambient[name] = os.Getenv(name)
	}
}

// Isolate removes the configuration that could send a test outside its own
// temporary directory. Call it from TestMain: TQ_DIR is the documented way to
// point tq at a queue, so a developer may well have it exported, and without
// this every test would operate on their real one.
func Isolate() {
	for _, name := range isolationEnv {
		_ = os.Unsetenv(name)
	}
	isolated = true
}

// RequireIsolated fails unless this package's TestMain called Isolate and the
// ambient configuration is gone. It is the pin on Isolate itself: every other
// fixture clears the variables again on the way past, so without this guard the
// call in TestMain could be deleted and nothing would notice — while a suite run
// with TQ_DIR exported wrote into a developer's real queue.
func RequireIsolated(t *testing.T) {
	t.Helper()
	if !isolated {
		t.Fatalf("this package's TestMain must call tqtest.Isolate(): without it an exported %s or %s points the whole suite at a real queue",
			config.EnvTaskDir, config.EnvWalkForever)
	}
	for _, name := range isolationEnv {
		got := os.Getenv(name)
		if got == "" {
			continue
		}
		if got == ambient[name] {
			t.Errorf("%s = %q, the value the shell exported: Isolate did not clear it", name, got)
			continue
		}
		t.Errorf("%s = %q, want it cleared", name, got)
	}
}

// ClearEnv clears the isolation variables for one test. Isolate handles the
// ambient values once per binary; this handles anything a test set before
// reaching for a fixture, and anything a fixture is asked to build inside a
// directory it did not create itself.
//
// Empty rather than unset, because t.Setenv is what restores the value when the
// test ends and there is no t.Unsetenv. tq reads both through os.Getenv and acts
// on the empty string, so the two are the same to everything under test — and it
// is a test that must not call t.Parallel, which t.Setenv already enforces.
func ClearEnv(t *testing.T) {
	t.Helper()
	for _, name := range isolationEnv {
		t.Setenv(name, "")
	}
}

// Root returns a temporary directory that discovery cannot climb out of.
// t.TempDir() alone is not enough: it honours TMPDIR, and the walk up looks for
// the marker in every parent, so with TMPDIR inside a project the fixtures would
// bind to that project's queue. The marker is what stops the walk here, because
// the marker is what discovery looks for (TQ-0029).
func Root(t *testing.T) string {
	t.Helper()
	root := bareRoot(t)
	WriteConfig(t, root, "version: 1\npath: "+config.TaskDirName+"\n")
	return root
}

// RootWithGit returns a temporary directory anchored by .git and holding no
// marker. It is for the tests whose premise is a directory that is not a project
// yet — `tq init` writing the first marker, discovery reporting that there is
// nothing to find, a queue created at the repository root — where the marker
// cannot be the barrier because its absence is the thing under test. The
// repository bound is then the only thing that can stop the walk.
func RootWithGit(t *testing.T) string {
	t.Helper()
	root := bareRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// bareRoot is a temporary directory with the environment cleared and no anchor
// of its own. Unexported: an unanchored root is what TQ-0021 and TQ-0053 are
// about, so a test reaches for one of the anchored roots above.
func bareRoot(t *testing.T) string {
	t.Helper()
	// Belt and braces: Isolate cleared the ambient values, this clears anything
	// a test set before reaching for a fixture.
	ClearEnv(t)
	return t.TempDir()
}

// AboveFixtures is the directory this test's fixture roots sit in. It is the
// one place above a fixture where a test can safely plant a queue: t.TempDir
// hands out numbered directories inside a parent of its own, and removes that
// parent when the test ends. A test that needs to prove a fixture cannot climb
// out of itself needs somewhere to climb to, and this is it.
func AboveFixtures(t *testing.T) string {
	t.Helper()
	above := filepath.Dir(t.TempDir())
	if above == filepath.Clean(os.TempDir()) {
		t.Fatalf("t.TempDir() no longer nests inside a per-test parent: %s is the shared temporary directory, and this test must not write a queue into it", above)
	}
	return above
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
// root.
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
