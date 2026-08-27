// Package tqtest holds the fixtures every package's tests need. It exists
// because t.TempDir() is not an isolation barrier here: discovery walks up out
// of it, and the environment can point tq somewhere else entirely, so a careless
// test writes into a developer's own queue.
package tqtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/config"
	"github.com/fmartingr/taskqueue/internal/store"
	"github.com/fmartingr/taskqueue/internal/task"
)

// isolationEnv is the configuration that can send a test outside its own
// temporary directory: TQ_CONFIG_PATH stands in for the walk entirely, so an
// exported one points the whole suite at whatever project it names.
var isolationEnv = []string{config.EnvConfigPath}

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
// temporary directory. Call it from TestMain: TQ_CONFIG_PATH is the documented
// way to point tq at a project, so a developer may well have it exported, and
// without this every test would operate on their real one.
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
// with TQ_CONFIG_PATH exported wrote into a developer's real queue.
func RequireIsolated(t *testing.T) {
	t.Helper()
	if !isolated {
		t.Fatalf("this package's TestMain must call tqtest.Isolate(): without it an exported %s points the whole suite at a real queue",
			config.EnvConfigPath)
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

// Root returns a temporary project directory that discovery cannot climb out
// of. t.TempDir() alone is not enough: it honours TMPDIR, and the walk up looks
// for the marker in every parent, so with TMPDIR inside a project the fixtures
// would bind to that project's queue. The marker is what stops the walk here,
// because the marker is what discovery looks for (TQ-0029).
//
// It writes the marker and nothing else. The task directory the marker names
// does not exist yet, which is a project no command but `tq init` can work in
// (TQ-0085): a fixture that needs the queue too calls NewStore, or runs init.
func Root(t *testing.T) string {
	t.Helper()
	root := bareRoot(t)
	WriteConfig(t, root, "version: 1\npath: "+config.TaskDirName+"\n")
	return root
}

// RootWithoutMarker returns a temporary directory that is not a project: no
// marker in it, and — asserted, not assumed — none anywhere the walk could
// reach from it. It is for the tests whose premise is that there is nothing to
// find: `tq init` writing the first marker, discovery reporting no project,
// a config loader answering "none".
//
// The assertion is the whole fixture. Since TQ-0085 the walk stops at the home
// directory, and a temporary directory outside it — macOS hands out
// /var/folders/… — runs to the filesystem root instead, so nothing structural
// keeps this root isolated. Asking discovery's own question is what does: a
// marker above would put the code under test on a different branch entirely,
// and the test would pass or fail for a reason that has nothing to do with it
// (TQ-0064).
//
// Furnish the directory freely; what it must not be handed to is anything that
// would climb out of it, and the assertion is what proves nothing can.
func RootWithoutMarker(t *testing.T) string {
	t.Helper()
	root := bareRoot(t)
	// A near-miss name or an unreadable directory anywhere up the walk comes
	// back as an error rather than "no marker", and either one means the answer
	// below cannot be trusted.
	path, err := config.ConfigPath(root)
	if err != nil {
		t.Fatalf("looking for a marker above %s: %v", root, err)
	}
	if path != "" {
		t.Fatalf("%s sits above the fixture, so this test's premise — a directory with no marker anywhere above it — does not hold here", path)
	}
	return root
}

// bareRoot is a temporary directory with the environment cleared and no marker
// of its own. Unexported: an unguarded root is what TQ-0021 and TQ-0053 are
// about, so a test reaches for Root, or for the guarded RootWithoutMarker when
// an absent marker is the premise.
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

// DecoyName is the column, the priority and the label the decoy marker in
// EscapedQueue declares. Nothing in tq produces it, so a value seen anywhere
// says the configuration was re-derived from the task directory.
const DecoyName = "decoy"

// decoyConfig is the project the escaped queue's task directory happens to sit
// in. Its board, its vocabulary and its label set share not one name with the
// built-in sets or with anything a test declares, so whichever of the three a
// caller ends up with is unmistakable.
const decoyConfig = `version: 1
path: queue
columns:
  - name: ` + DecoyName + `
    display_name: Decoy
    default: true
priorities:
  - name: ` + DecoyName + `
    color: "#000000"
    default: true
labels:
  ` + DecoyName + `:
    color: "#000000"
    display_name: Decoy
`

// EscapedQueue plants the project shape TQ-0087 is about and returns its root
// and the task directory that root declares. The queue is created, so the
// caller opens it with InitStore or OpenStore — or runs the CLI in root — as it
// needs.
//
// The shape is a marker whose `path` names a task directory *outside* the
// marker's own directory, with a second, decoy marker sitting directly above
// that directory. It is documented and ordinary — `path` is resolved against
// the marker, so it may point anywhere — and it is what TQ_CONFIG_PATH makes
// routine, since the marker it hands over need be nowhere near the tasks.
// Reading the
// marker the queue was resolved through gives the project's own configuration;
// walking up from the task directory instead lands on the decoy, and every
// value that comes back is then visibly the wrong one.
//
// extra is appended to the project's marker: its columns, its priorities, its
// labels, whatever the test is about.
func EscapedQueue(t *testing.T, extra string) (root, queue string) {
	t.Helper()
	above := AboveFixtures(t)
	root = Root(t)
	queue = filepath.Join(above, "queue")
	if err := os.MkdirAll(queue, 0o755); err != nil {
		t.Fatal(err)
	}

	rel, err := filepath.Rel(root, queue)
	if err != nil {
		t.Fatal(err)
	}
	// The premise of the fixture: the path leaves the marker's own directory.
	// Without it the two markers below would be the same file.
	if !strings.HasPrefix(rel, "..") {
		t.Fatalf("path %q does not leave %s, so this fixture is not the shape it is for", rel, root)
	}

	WriteConfig(t, above, decoyConfig)
	WriteConfig(t, root, "version: 1\npath: "+filepath.ToSlash(rel)+"\n"+extra)
	return root, queue
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
