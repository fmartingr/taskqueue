package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/store"

	"github.com/fmartingr/taskqueue/internal/tqtest"

	"github.com/fmartingr/taskqueue/internal/guide"
	"net/http"
	"syscall"
	"time"
)

// testVersion stands in for the string the build stamps on the binary.
const testVersion = "test-version"

type testCLI struct {
	*cli
	t      *testing.T
	stdout *syncBuffer
	stderr *syncBuffer
	root   string
}

// syncBuffer is a bytes.Buffer a test can read while a command is still
// writing to it. `tq serve` runs until it is signalled, so the tests that drive
// it poll its output from one goroutine while the command writes from another —
// which on a bare bytes.Buffer is a data race, and `go test -race` says so.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *syncBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func (b *syncBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

// newTestCLI returns a CLI rooted in a temporary project that already has a
// task directory.
// requireNoQueueAbove skips a test whose premise is that nothing was excluded,
// when the machine it runs on says otherwise: TMPDIR may sit inside a project
// that has its own queue, and the notice under test would then be correct.
func requireNoQueueAbove(t *testing.T, dir string) {
	t.Helper()
	for cur := filepath.Dir(dir); ; {
		if info, err := os.Stat(filepath.Join(cur, config.TaskDirName)); err == nil && info.IsDir() {
			t.Skipf("%s sits above the fixture, so a notice about it is correct", filepath.Join(cur, config.TaskDirName))
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return
		}
		cur = parent
	}
}

// anchorProject marks a directory as a repository root, so task directory
// discovery stops there. Without it a fixture walks out of tqtest.Root(t) and can
// reach — and write into — a developer's own queue (TQ-0053).
func anchorProject(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func newTestCLI(t *testing.T) *testCLI {
	t.Helper()
	tc := newBareCLI(t)
	if code := tc.run("init"); code != exitOK {
		t.Fatalf("init failed: %d %s", code, tc.stderr)
	}
	tc.reset()
	return tc
}

// newBareCLI returns a CLI rooted in a temporary directory with no project.
func newBareCLI(t *testing.T) *testCLI {
	t.Helper()
	// Same isolation the store fixtures take: never reach a real queue.
	t.Setenv(config.EnvTaskDir, "")
	t.Setenv(config.EnvWalkForever, "")
	root := tqtest.Root(t)
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	return &testCLI{
		cli:    &cli{stdout: stdout, stderr: stderr, dir: root, version: testVersion},
		t:      t,
		stdout: stdout,
		stderr: stderr,
		root:   root,
	}
}

func (tc *testCLI) run(args ...string) int {
	tc.t.Helper()
	tc.reset()
	return runCLI(tc.cli, args)
}

func (tc *testCLI) reset() {
	tc.stdout.Reset()
	tc.stderr.Reset()
}

// mustRun runs a command that is expected to succeed and returns stdout.
func (tc *testCLI) mustRun(args ...string) string {
	tc.t.Helper()
	if code := tc.run(args...); code != exitOK {
		tc.t.Fatalf("tq %s = exit %d, stderr: %s", strings.Join(args, " "), code, tc.stderr)
	}
	return tc.stdout.String()
}

func (tc *testCLI) mustRunJSON(target any, args ...string) {
	tc.t.Helper()
	out := tc.mustRun(args...)
	if err := json.Unmarshal([]byte(out), target); err != nil {
		tc.t.Fatalf("tq %s produced invalid JSON: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestCLIInit(t *testing.T) {
	tc := newBareCLI(t)

	out := tc.mustRun("init")
	dir := filepath.Join(tc.root, config.TaskDirName)
	if !strings.Contains(out, dir) {
		t.Errorf("init output %q should mention %q", out, dir)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("init did not create %s: %v", dir, err)
	}

	// Initialising twice is not an error: every command creates the directory
	// on demand anyway.
	out = tc.mustRun("init")
	if !strings.Contains(out, "already initialized") {
		t.Errorf("second init output = %q, want it to say the queue already exists", out)
	}
}

func TestCLIAdd(t *testing.T) {
	tc := newTestCLI(t)

	out := tc.mustRun("add", "Implement authentication")
	if want := "Created TQ-0001: Implement authentication"; !strings.Contains(out, want) {
		t.Errorf("add output = %q, want it to contain %q", out, want)
	}

	var tk task.Task
	tc.mustRunJSON(&tk, "add", "Build board", "--priority", "high",
		"--label", "frontend", "--label", "ui", "--assignee", "agent-ui",
		"--depends-on", "TQ-0001", "--body", "Kanban board.", "--json")

	if tk.ID != "TQ-0002" || tk.Title != "Build board" {
		t.Errorf("task = %+v", tk)
	}
	if tk.Priority != task.PriorityHigh || tk.Assignee != "agent-ui" {
		t.Errorf("task = %+v", tk)
	}
	if strings.Join(tk.Labels, ",") != "frontend,ui" {
		t.Errorf("Labels = %v", tk.Labels)
	}
	if strings.Join(tk.DependsOn, ",") != "TQ-0001" {
		t.Errorf("DependsOn = %v", tk.DependsOn)
	}
	if tk.Body != "Kanban board." {
		t.Errorf("Body = %q", tk.Body)
	}
	if tk.Status != task.StatusInbox {
		t.Errorf("Status = %q, want %q", tk.Status, task.StatusInbox)
	}
}

func TestCLIAddValidation(t *testing.T) {
	tc := newTestCLI(t)

	if code := tc.run("add"); code != exitError {
		t.Errorf("add without a title = exit %d, want %d", code, exitError)
	}
	if code := tc.run("add", "x", "--priority", "whenever"); code != exitError {
		t.Errorf("add with a bad priority = exit %d, want %d", code, exitError)
	}
	if tc.stdout.Len() != 0 {
		t.Errorf("failed commands must not write to stdout, got %q", tc.stdout)
	}
}

func TestCLIListAndFilters(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Add task API", "--priority", "high", "--label", "backend", "--assignee", "agent-api")
	tc.mustRun("add", "Build board", "--label", "frontend", "--assignee", "agent-ui")
	tc.mustRun("move", "TQ-0002", "in-progress")

	out := tc.mustRun("list")
	if !strings.HasPrefix(out, "ID") || !strings.Contains(out, "STATUS") || !strings.Contains(out, "TITLE") {
		t.Errorf("list should print a header row, got:\n%s", out)
	}
	if !strings.Contains(out, "TQ-0001") || !strings.Contains(out, "TQ-0002") {
		t.Errorf("list should show both tasks:\n%s", out)
	}

	out = tc.mustRun("list", "--status", "in-progress")
	if strings.Contains(out, "TQ-0001") || !strings.Contains(out, "TQ-0002") {
		t.Errorf("--status filter failed:\n%s", out)
	}

	for _, filter := range [][]string{
		{"--priority", "high"},
		{"--label", "backend"},
		{"--assignee", "agent-api"},
	} {
		out = tc.mustRun(append([]string{"list"}, filter...)...)
		if !strings.Contains(out, "TQ-0001") || strings.Contains(out, "TQ-0002") {
			t.Errorf("filter %v failed:\n%s", filter, out)
		}
	}

	var tasks []task.Task
	tc.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 2 {
		t.Fatalf("list --json returned %d tasks, want 2", len(tasks))
	}
	if tc.stderr.Len() != 0 {
		t.Errorf("a successful JSON command should print nothing on stderr, got %q", tc.stderr)
	}

	tc.mustRunJSON(&tasks, "list", "--status", "done", "--json")
	if len(tasks) != 0 {
		t.Errorf("an empty result should still be a JSON array, got %d tasks", len(tasks))
	}

	if code := tc.run("list", "--status", "nope"); code != exitError {
		t.Errorf("list with an invalid status = exit %d, want %d", code, exitError)
	}
}

func TestCLIShow(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Implement REST API", "--priority", "high", "--label", "backend", "--body", "Some description.")

	out := tc.mustRun("show", "TQ-0001")
	for _, want := range []string{"TQ-0001", "Implement REST API", "inbox", "high", "backend", "Some description."} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if tk.ID != "TQ-0001" || tk.Body != "Some description." {
		t.Errorf("show --json = %+v", tk)
	}

	if code := tc.run("show", "TQ-4242"); code != exitTaskNotFound {
		t.Errorf("show of a missing task = exit %d, want %d", code, exitTaskNotFound)
	}
	if tc.stdout.Len() != 0 {
		t.Errorf("stdout should stay empty on error, got %q", tc.stdout)
	}
	if !strings.Contains(tc.stderr.String(), "TQ-4242") {
		t.Errorf("stderr should name the missing task, got %q", tc.stderr)
	}

	if code := tc.run("show", "TQ-4242", "--json"); code != exitTaskNotFound {
		t.Errorf("show --json of a missing task = exit %d, want %d", code, exitTaskNotFound)
	}
	if tc.stdout.Len() != 0 {
		t.Errorf("JSON mode must not print errors to stdout, got %q", tc.stdout)
	}
}

func TestCLIShowDescribesDependencies(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Dependency that is finished")
	tc.mustRun("add", "Dependency still in flight")
	tc.mustRun("done", "TQ-0001")
	tc.mustRun("add", "Dependent task",
		"--depends-on", "TQ-0001", "--depends-on", "TQ-0002", "--depends-on", "TQ-0404")

	out := tc.mustRun("show", "TQ-0003")
	for _, want := range []string{"TQ-0001 (done)", "TQ-0002 (inbox, blocking)", "TQ-0404 (missing)"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}
}

func TestCLIShowSurvivesUnreadableSiblingTask(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Readable", "--depends-on", "TQ-0002")
	if err := os.WriteFile(filepath.Join(tc.root, config.TaskDirName, "TQ-0002-broken.md"), []byte("not a task"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The dependency status cannot be resolved, but the task itself still shows.
	out := tc.mustRun("show", "TQ-0001")
	if !strings.Contains(out, "TQ-0001") || !strings.Contains(out, "TQ-0002") {
		t.Errorf("show output:\n%s", out)
	}
	if !strings.Contains(tc.stderr.String(), "warning") {
		t.Errorf("stderr should warn about the unreadable task, got %q", tc.stderr)
	}
}

func TestCLIMoveAndDone(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Implement REST API")

	out := tc.mustRun("move", "TQ-0001", "in-progress")
	if want := "TQ-0001: inbox -> in-progress"; !strings.Contains(out, want) {
		t.Errorf("move output = %q, want it to contain %q", out, want)
	}

	out = tc.mustRun("done", "TQ-0001")
	if want := "TQ-0001: in-progress -> done"; !strings.Contains(out, want) {
		t.Errorf("done output = %q, want it to contain %q", out, want)
	}

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if tk.Status != task.StatusDone {
		t.Errorf("status = %q, want done", tk.Status)
	}

	if code := tc.run("move", "TQ-0001", "shipped"); code != exitError {
		t.Errorf("move to an unknown status = exit %d, want %d", code, exitError)
	}
	if code := tc.run("move", "TQ-4242", "done"); code != exitTaskNotFound {
		t.Errorf("move of a missing task = exit %d, want %d", code, exitTaskNotFound)
	}
	if code := tc.run("move", "TQ-0001"); code != exitError {
		t.Errorf("move without a status = exit %d, want %d", code, exitError)
	}
}

func TestCLIUpdate(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Original", "--label", "backend")
	tc.mustRun("add", "Dependency")

	tc.mustRun("update", "TQ-0001",
		"--title", "New title",
		"--priority", "urgent",
		"--assignee", "agent-api",
		"--add-label", "auth",
		"--remove-label", "backend",
		"--add-dependency", "TQ-0002")

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if tk.Title != "New title" || tk.Priority != task.PriorityUrgent || tk.Assignee != "agent-api" {
		t.Errorf("task = %+v", tk)
	}
	if strings.Join(tk.Labels, ",") != "auth" {
		t.Errorf("Labels = %v, want [auth]", tk.Labels)
	}
	if strings.Join(tk.DependsOn, ",") != "TQ-0002" {
		t.Errorf("DependsOn = %v", tk.DependsOn)
	}

	tc.mustRun("update", "TQ-0001", "--remove-dependency", "TQ-0002")
	var afterRemoval task.Task // a fresh value: omitted JSON fields do not clear a reused struct
	tc.mustRunJSON(&afterRemoval, "show", "TQ-0001", "--json")
	if len(afterRemoval.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty", afterRemoval.DependsOn)
	}

	if code := tc.run("update", "TQ-0001"); code != exitError {
		t.Errorf("update without any flag = exit %d, want %d", code, exitError)
	}
	if code := tc.run("update", "TQ-0001", "--priority", "whenever"); code != exitError {
		t.Errorf("update with a bad priority = exit %d, want %d", code, exitError)
	}
}

func TestCLIUpdateRenamesTheFile(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Original title")
	dir := filepath.Join(tc.root, config.TaskDirName)
	if _, err := os.Stat(filepath.Join(dir, "TQ-0001-original-title.md")); err != nil {
		t.Fatalf("add should name the file after the title: %v", err)
	}

	tc.mustRun("update", "TQ-0001", "--title", "A better title")

	if _, err := os.Stat(filepath.Join(dir, "TQ-0001-a-better-title.md")); err != nil {
		t.Errorf("the file should follow the new title: %v", err)
	}
	var taskFiles []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "TQ-") {
			taskFiles = append(taskFiles, entry.Name())
		}
	}
	if len(taskFiles) != 1 {
		t.Errorf("directory contains %v, want only the renamed file", taskFiles)
	}

	// The ID still addresses the task after the rename.
	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if tk.Title != "A better title" {
		t.Errorf("task = %+v", tk)
	}
}

func TestCLINote(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Implement REST API", "--body", "Description.")

	out := tc.mustRun("note", "TQ-0001", "CRUD endpoints implemented; tests remain.")
	if !strings.Contains(out, "TQ-0001") {
		t.Errorf("note output = %q", out)
	}

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.Contains(tk.Body, "## Notes") {
		t.Errorf("body should have a Notes section:\n%s", tk.Body)
	}
	if !strings.Contains(tk.Body, "CRUD endpoints implemented; tests remain.") {
		t.Errorf("body should contain the note:\n%s", tk.Body)
	}
	if !strings.HasPrefix(tk.Body, "Description.") {
		t.Errorf("the original body should be preserved:\n%s", tk.Body)
	}

	tc.mustRun("note", "TQ-0001", "Second note.")
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if strings.Count(tk.Body, "## Notes") != 1 {
		t.Errorf("a second note should reuse the section:\n%s", tk.Body)
	}

	if code := tc.run("note", "TQ-0001"); code != exitError {
		t.Errorf("note without text = exit %d, want %d", code, exitError)
	}
	if code := tc.run("note", "TQ-4242", "text"); code != exitTaskNotFound {
		t.Errorf("note on a missing task = exit %d, want %d", code, exitTaskNotFound)
	}
}

func TestCLINoteLeavesAContentNotesSectionAlone(t *testing.T) {
	tc := newTestCLI(t)
	body := "Description.\n\n## Notes\n\nProse that belongs to the task.\n\n## Acceptance criteria\n\n- something"
	tc.mustRun("add", "A task documenting its own notes", "--body", body)

	tc.mustRun("note", "TQ-0001", "The real note.")

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(tk.Body, body) {
		t.Fatalf("the original body should be untouched:\n%s", tk.Body)
	}
	want := body + "\n\n---\n\n## Notes\n\n- "
	if !strings.HasPrefix(tk.Body, want) {
		t.Errorf("the note should start a new section at the end:\n%s", tk.Body)
	}
	if !strings.HasSuffix(tk.Body, " — The real note.") {
		t.Errorf("the note should be the last line:\n%s", tk.Body)
	}
}

func TestCLIReady(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Implement REST API", "--status", "todo", "--priority", "high", "--label", "backend")
	tc.mustRun("add", "Build Kanban board", "--status", "todo", "--label", "frontend", "--depends-on", "TQ-0001")

	out := tc.mustRun("ready")
	if !strings.Contains(out, "TQ-0001") || strings.Contains(out, "TQ-0002") {
		t.Errorf("only the unblocked task should be ready:\n%s", out)
	}

	tc.mustRun("move", "TQ-0001", "in-progress")
	var tasks []task.Task
	tc.mustRunJSON(&tasks, "ready", "--json")
	if len(tasks) != 0 {
		t.Errorf("a claimed task and its blocked dependant are not ready, got %+v", tasks)
	}

	tc.mustRun("done", "TQ-0001")
	tc.mustRunJSON(&tasks, "ready", "--json")
	if len(tasks) != 1 || tasks[0].ID != "TQ-0002" {
		t.Errorf("finishing the dependency should unblock TQ-0002, got %+v", tasks)
	}

	tc.mustRunJSON(&tasks, "ready", "--label", "backend", "--json")
	if len(tasks) != 0 {
		t.Errorf("ready --label should filter, got %+v", tasks)
	}
}

func TestCLICreatesTaskDirOnDemand(t *testing.T) {
	tc := newBareCLI(t)

	// No `tq init` first: the directory appears when it is needed.
	out := tc.mustRun("add", "First task")
	if !strings.Contains(out, "Created TQ-0001") {
		t.Errorf("add output = %q", out)
	}
	dir := filepath.Join(tc.root, config.TaskDirName)
	if _, err := os.Stat(filepath.Join(dir, "TQ-0001-first-task.md")); err != nil {
		t.Fatalf("task file not written: %v", err)
	}
	if !strings.Contains(tc.stderr.String(), dir) {
		t.Errorf("stderr should note the created directory, got %q", tc.stderr)
	}

	// Reading commands work the same way, and say nothing once it exists.
	var tasks []task.Task
	tc.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 1 {
		t.Errorf("list returned %d tasks, want 1", len(tasks))
	}
}

func TestCLIReadCommandsCreateAnEmptyQueue(t *testing.T) {
	tc := newBareCLI(t)

	var tasks []task.Task
	tc.mustRunJSON(&tasks, "ready", "--json")
	if len(tasks) != 0 {
		t.Errorf("ready = %+v, want an empty list", tasks)
	}
	if _, err := os.Stat(filepath.Join(tc.root, config.TaskDirName)); err != nil {
		t.Errorf("%s should have been created: %v", config.TaskDirName, err)
	}
}

func TestCLIReportsUncreatableTaskDir(t *testing.T) {
	tc := newBareCLI(t)
	file := filepath.Join(tc.root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc.dir = filepath.Join(file, "sub")

	// Fixtures are anchored, so creation now falls back to the repository
	// root: make that unwritable too, or there is nothing uncreatable left to
	// report. Restored before t.TempDir's own cleanup, which runs after this.
	if err := os.Chmod(tc.root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(tc.root, 0o755) })

	for _, args := range [][]string{{"list"}, {"add", "x"}, {"ready"}} {
		if code := tc.run(args...); code != exitProjectNotFound {
			t.Errorf("tq %s = exit %d, want %d", strings.Join(args, " "), code, exitProjectNotFound)
		}
		if !strings.Contains(tc.stderr.String(), config.TaskDirName) {
			t.Errorf("stderr should mention %s, got %q", config.TaskDirName, tc.stderr)
		}
	}
}

func TestCLIUsageAndVersion(t *testing.T) {
	tc := newBareCLI(t)

	if code := tc.run(); code != exitError {
		t.Errorf("no arguments = exit %d, want %d", code, exitError)
	}
	if !strings.Contains(tc.stderr.String(), "Usage") {
		t.Errorf("stderr should show usage, got %q", tc.stderr)
	}

	if code := tc.run("frobnicate"); code != exitError {
		t.Errorf("unknown command = exit %d, want %d", code, exitError)
	}
	if !strings.Contains(tc.stderr.String(), "frobnicate") {
		t.Errorf("stderr should name the unknown command, got %q", tc.stderr)
	}

	out := tc.mustRun("version")
	if !strings.Contains(out, testVersion) {
		t.Errorf("version output = %q, want it to contain %q", out, testVersion)
	}

	out = tc.mustRun("help")
	if !strings.Contains(out, "Usage") {
		t.Errorf("help should print usage to stdout, got %q", out)
	}
}

func TestCLIHelpFlagStopsCleanly(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "A task")

	// -h prints usage and exits 0 without running the command, even for
	// commands that require positional arguments.
	for _, args := range [][]string{
		{"add", "-h"},
		{"list", "-h"},
		{"show", "-h"},
		{"move", "-h"},
		{"done", "-h"},
		{"update", "-h"},
		{"note", "-h"},
		{"ready", "-h"},
		{"serve", "-h"},
		{"version", "-h"},
	} {
		if code := tc.run(args...); code != exitOK {
			t.Errorf("tq %s = exit %d, want %d", strings.Join(args, " "), code, exitOK)
		}
		if !strings.Contains(tc.stderr.String(), "Usage of tq "+args[0]) {
			t.Errorf("tq %s should print usage, got %q", strings.Join(args, " "), tc.stderr)
		}
	}
}

func TestCLIEnvTaskDirOverride(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "task.Task in the project")

	elsewhere := newBareCLI(t)
	elsewhere.t.Setenv(config.EnvTaskDir, filepath.Join(tc.root, config.TaskDirName))

	var tasks []task.Task
	elsewhere.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 1 || tasks[0].Title != "task.Task in the project" {
		t.Errorf("%s override ignored, got %+v", config.EnvTaskDir, tasks)
	}
}

// Creating a local queue while one sits above the repository is the moment a
// developer's tasks appear to vanish, so that is where tq should name the
// variable that would have found them.
func TestCLINamesAQueueTheBoundExcluded(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: repo, version: testVersion}, t: t, stdout: stdout, stderr: stderr, root: repo}
	tc.mustRun("list")

	notice := stderr.String()
	for _, want := range []string{filepath.Join(outer, config.TaskDirName), config.EnvWalkForever} {
		if !strings.Contains(notice, want) {
			t.Errorf("stderr = %q, want it to mention %q", notice, want)
		}
	}
}

// With nothing above it, the notice stays a single line.
func TestCLIDoesNotInventAnExcludedQueue(t *testing.T) {
	tc := newBareCLI(t)
	requireNoQueueAbove(t, tc.root)
	tc.mustRun("list")

	if strings.Contains(tc.stderr.String(), config.EnvWalkForever) {
		t.Errorf("stderr = %q, want no mention of %s when nothing was excluded", tc.stderr, config.EnvWalkForever)
	}
}

// The reason this fix was reverted once: with init discovering, an unanchored
// fixture walks out of tqtest.Root(t). TQ-0017's bound does not help here, since
// a bare temp directory has no repository root to stop at, so the fixtures
// carry their own anchor.
func TestCLIFixturesCannotReachAQueueAboveTempDir(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(filepath.Join(outer, config.TaskDirName))
	if err != nil {
		t.Fatal(err)
	}

	project := filepath.Join(outer, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: project, version: testVersion}, t: t, stdout: stdout, stderr: stderr, root: project}
	anchorProject(t, project)

	tc.mustRun("init")
	tc.mustRun("add", "fixture task")

	after, err := os.ReadDir(filepath.Join(outer, config.TaskDirName))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("the fixture wrote into the queue above it: %d entries, was %d", len(after), len(before))
	}
}

// tq init from a subdirectory must adopt the project's queue, not fork one.
// The project here is deliberately not a Git repository, which is the shape
// that still breaks: when there is a repository root, taskDirTarget already
// stops at it and init lands in the right place by accident. The enclosing
// temp directory carries the .git anchor so the walk cannot escape it.
func TestCLIInitFindsTheQueueAbove(t *testing.T) {
	outer := tqtest.Root(t)
	anchorProject(t, outer)

	project := filepath.Join(outer, "project")
	nested := filepath.Join(project, "backend")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// Directly, not through store.InitStore: the enclosing anchor would send it to
	// the repository root, and the point here is a queue the project owns.
	// The marker is what makes it the project's queue rather than a directory
	// that happens to be named .tasks.
	if err := os.MkdirAll(filepath.Join(project, config.TaskDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, config.ConfigFileName), []byte("version: 1\npath: "+config.TaskDirName+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	seedOut, seedErr := &syncBuffer{}, &syncBuffer{}
	seed := &testCLI{cli: &cli{stdout: seedOut, stderr: seedErr, dir: project, version: testVersion}, t: t, stdout: seedOut, stderr: seedErr, root: project}
	seed.mustRun("add", "existing work")

	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	sub := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: nested, version: testVersion}, t: t, stdout: stdout, stderr: stderr, root: nested}

	var out struct {
		TaskDir string `json:"task_dir"`
		Created bool   `json:"created"`
	}
	sub.mustRunJSON(&out, "init", "--json")

	if want := filepath.Join(project, config.TaskDirName); out.TaskDir != want {
		t.Errorf("task_dir = %q, want the project's queue %q", out.TaskDir, want)
	}
	if out.Created {
		t.Error("created = true, want false: the queue already existed")
	}
	if _, err := os.Stat(filepath.Join(nested, config.TaskDirName)); !os.IsNotExist(err) {
		t.Error("init forked a second queue in the subdirectory")
	}
	if _, err := os.Stat(filepath.Join(outer, config.TaskDirName)); !os.IsNotExist(err) {
		t.Error("init created a queue at the enclosing repository root")
	}
	sub.reset()
	if listing := sub.mustRun("list"); !strings.Contains(listing, "existing work") {
		t.Errorf("the subdirectory lost sight of the project's work: %q", listing)
	}
}

// The bound TQ-0017 added is what makes the above safe: discovery must not
// reach a queue outside the repository, or init adopts it and creates nothing.
func TestCLIInitDoesNotAdoptAQueueOutsideTheRepository(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: repo, version: testVersion}, t: t, stdout: stdout, stderr: stderr, root: repo}

	var out struct {
		TaskDir string `json:"task_dir"`
		Created bool   `json:"created"`
	}
	tc.mustRunJSON(&out, "init", "--json")

	if want := filepath.Join(repo, config.TaskDirName); out.TaskDir != want {
		t.Errorf("task_dir = %q, want the repository's own queue %q", out.TaskDir, want)
	}
	if !out.Created {
		t.Error("created = false, want true: the repository had no queue")
	}
}

// The CLI fixtures build their own store, so they need the same isolation the
// store fixtures have.
func TestCLIFixturesIgnoreAnAmbientTaskDirOverride(t *testing.T) {
	outside := filepath.Join(tqtest.Root(t), "real", config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvTaskDir, outside)

	tc := newTestCLI(t)
	tc.mustRun("add", "fixture task")

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the CLI fixture wrote into the directory %s names: %d entries", config.EnvTaskDir, len(entries))
	}
}

// tq init must not write a guide into a task directory belonging to another
// project. The fixture is deliberately unanchored, because a project with no
// repository root is the shape where discovery has no bound; a queue at the
// temp root stops the walk inside the fixture, so nothing escapes it.
func TestCLIInitDoesNotWriteTheGuideOutsideTheInvokedTree(t *testing.T) {
	t.Setenv(config.EnvTaskDir, "")
	t.Setenv(config.EnvWalkForever, "")
	root := t.TempDir()

	outside := filepath.Join(root, config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	// With a marker above it, the deep directory discovers this queue — which
	// is the situation the guide must not be written into.
	if err := os.WriteFile(filepath.Join(root, config.ConfigFileName), []byte("version: 1\npath: "+config.TaskDirName+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "projects", "foo")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: deep, version: testVersion}, t: t, stdout: stdout, stderr: stderr, root: deep}
	tc.mustRun("init")

	if _, err := os.Stat(filepath.Join(outside, guide.AgentsFileName)); !os.IsNotExist(err) {
		t.Errorf("init wrote a guide into %s, which belongs to another project", outside)
	}
	if strings.Contains(stdout.String(), "Wrote ") {
		t.Errorf("init reported a write it must not make: %q", stdout)
	}
	if !strings.Contains(stderr.String(), outside) {
		t.Errorf("stderr should say which directory was left alone, got %q", stderr)
	}
}

// The ordinary cases must keep their guide: init at the root of a project, and
// init anywhere inside a repository.
func TestCLIInitWritesTheGuideInsideTheInvokedTree(t *testing.T) {
	tc := newTestCLI(t)
	guide := filepath.Join(tc.root, config.TaskDirName, guide.AgentsFileName)
	if _, err := os.Stat(guide); err != nil {
		t.Fatalf("guide not written at the project root: %v", err)
	}

	deep := filepath.Join(tc.root, "src", "deep")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(guide); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	sub := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: deep, version: testVersion}, t: t, stdout: stdout, stderr: stderr, root: tc.root}
	sub.mustRun("init")

	if _, err := os.Stat(guide); err != nil {
		t.Errorf("init inside the repository should still write the guide: %v", err)
	}
}

// tq must refuse a title that would render as a block scalar, and refuse it
// before touching the file: the whole point is that the task survives.
func TestCLIUpdateRejectsAMultiLineTitle(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Fix the parser")
	tc.reset()

	if code := tc.run("update", "TQ-0001", "--title", "line1\n---\nline2"); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(tc.stderr.String(), "single line") {
		t.Errorf("stderr = %q, want it to say why", tc.stderr)
	}
	tc.reset()

	// The directory must still be readable, and the task unchanged.
	listing := tc.mustRun("list")
	if !strings.Contains(listing, "Fix the parser") {
		t.Errorf("the task should be untouched, got %q", listing)
	}
}

// tq init leaves the marker, so the next command finds the queue by the file
// rather than by guessing at directory names.
func TestCLIInitWritesTheConfigMarker(t *testing.T) {
	tc := newBareCLI(t)
	out := tc.mustRun("init")

	path := filepath.Join(tc.root, config.ConfigFileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s not written: %v", config.ConfigFileName, err)
	}
	if !strings.Contains(out, path) {
		t.Errorf("init should report the config it wrote, got %q", out)
	}

	cfg, err := config.FindConfig(tc.root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TaskDir() != filepath.Join(tc.root, config.TaskDirName) {
		t.Errorf("config points at %q", cfg.TaskDir())
	}

	// Re-running writes nothing: the file is the user's.
	tc.reset()
	if out := tc.mustRun("init"); strings.Contains(out, "Wrote "+path) {
		t.Errorf("a second init rewrote the config: %q", out)
	}
}

// A config naming a path is what moves the queue, from any subdirectory.
func TestCLIFollowsTheConfigPath(t *testing.T) {
	tc := newBareCLI(t)
	if err := os.WriteFile(filepath.Join(tc.root, config.ConfigFileName), []byte("version: 1\npath: docs/queue\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tc.mustRun("add", "configured")

	if _, err := os.Stat(filepath.Join(tc.root, "docs", "queue")); err != nil {
		t.Fatalf("the task directory should follow the config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tc.root, config.TaskDirName)); !os.IsNotExist(err) {
		t.Error("the default directory should not have been created")
	}
}

// A broken config is reported as a config problem, not as a missing queue.
func TestCLIReportsABrokenConfig(t *testing.T) {
	tc := newBareCLI(t)
	if err := os.WriteFile(filepath.Join(tc.root, config.ConfigFileName), []byte("version: 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := tc.run("list"); code == exitOK {
		t.Error("exit = 0, want a failure for a config tq cannot use")
	}
	if !strings.Contains(tc.stderr.String(), "newer tq") {
		t.Errorf("stderr = %q, want it to say the file needs a newer tq", tc.stderr)
	}
}

func TestCLIInitWritesTheGuideAndNothingElse(t *testing.T) {
	tc := newBareCLI(t)

	out := tc.mustRun("init")
	guidePath := filepath.Join(tc.root, config.TaskDirName, guide.AgentsFileName)
	if !strings.Contains(out, guidePath) {
		t.Errorf("init should report the guidePath it wrote, got %q", out)
	}
	if _, err := os.Stat(guidePath); err != nil {
		t.Fatalf("guidePath not written: %v", err)
	}

	// tq no longer manages the repository's own agent instructions: it says
	// what to add instead of writing the file.
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(tc.root, name)); !os.IsNotExist(err) {
			t.Errorf("init created %s; it must leave those files to the user", name)
		}
	}
	if !strings.Contains(out, "@"+config.TaskDirName+"/"+guide.AgentsFileName) {
		t.Errorf("init should print the line to add, got %q", out)
	}

	// Re-running refreshes without reporting spurious writes, but still says
	// what to add — the file it names may not exist yet.
	out = tc.mustRun("init")
	if strings.Contains(out, "Wrote ") {
		t.Errorf("nothing should be rewritten on a second init, got %q", out)
	}
	if !strings.Contains(out, "@"+config.TaskDirName+"/"+guide.AgentsFileName) {
		t.Errorf("the second init should still print the line to add, got %q", out)
	}
}

// --port 0 asks the OS for a free port, which is how anything driving the real
// binary avoids racing for one. The banner has to say which port it got, or the
// caller has no way to find out.
func TestCLIServePrintsTheAddressItActuallyGot(t *testing.T) {
	tc := newTestCLI(t)

	done := make(chan int, 1)
	go func() { done <- tc.run("serve", "--port", "0") }()

	var line string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := tc.stdout.String(); strings.Contains(s, "http://") {
			line = strings.SplitN(s, "\n", 2)[0]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if line == "" {
		t.Fatalf("no banner within the deadline; stderr = %q", tc.stderr)
	}
	if strings.Contains(line, ":0") {
		t.Errorf("banner = %q, want the port the listener got, not the one requested", line)
	}

	_, addr, found := strings.Cut(line, "http://")
	if !found {
		t.Fatalf("banner = %q, want an address", line)
	}
	resp, err := http.Get("http://" + strings.TrimSpace(addr) + "/api/status")
	if err != nil {
		t.Fatalf("the printed address should be reachable: %v", err)
	}
	_ = resp.Body.Close()

	if p, err := os.FindProcess(os.Getpid()); err == nil {
		_ = p.Signal(syscall.SIGTERM)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not shut down")
	}
}

// The terminator's guarantee has to hold for every argument after it, not just
// the first: the parse loop used to re-feed the rest through flag.Parse.
func TestCLITerminatorProtectsEveryArgumentAfterIt(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "a task")

	tc.reset()
	if code := tc.run("note", "--", "TQ-0001", "-1 test still failing"); code != exitOK {
		t.Errorf("exit = %d, want 0: both arguments are after the terminator\nstderr: %s", code, tc.stderr)
	}

	// And the inverse: a flag written after "--" is an argument, so this is a
	// second positional and the command must refuse it.
	tc.reset()
	if code := tc.run("add", "--", "-weird title", "--json"); code == exitOK {
		t.Errorf("exit = 0, want a failure: --json after -- is an argument, not a flag")
	}
}

// ── The project's priority vocabulary ────────────────────────────

const customPriorities = `version: 1
path: .tasks
priorities:
  - name: p0
    color: "#b60205"
    display_name: Critical
  - name: p1
    color: "#c2410c"
  - name: p2
    color: "#4b5563"
    default: true
`

// cliWithPriorities returns a CLI over a project that declares p0..p2.
func cliWithPriorities(t *testing.T) *testCLI {
	t.Helper()
	tc := newBareCLI(t)
	tqtest.WriteConfig(t, tc.root, customPriorities)
	if code := tc.run("init"); code != exitOK {
		t.Fatalf("init failed: %d %s", code, tc.stderr)
	}
	return tc
}

// The round trip the vocabulary has to survive: create under a configured
// value, default to the configured default, filter by one, and sort by the
// order the file gives.
func TestCLIRoundTripsACustomVocabulary(t *testing.T) {
	tc := cliWithPriorities(t)

	var created task.Task
	tc.mustRunJSON(&created, "add", "Critical work", "--priority", "p0", "--json")
	if created.Priority != "p0" {
		t.Errorf("Priority = %q, want p0", created.Priority)
	}

	var defaulted task.Task
	tc.mustRunJSON(&defaulted, "add", "Ordinary work", "--json")
	if defaulted.Priority != "p2" {
		t.Errorf("Priority = %q, want p2 (the configured default)", defaulted.Priority)
	}

	tc.mustRun("add", "Middling work", "--priority", "p1")

	var listed []task.Task
	tc.mustRunJSON(&listed, "list", "--json")
	got := make([]string, 0, len(listed))
	for _, tk := range listed {
		got = append(got, tk.Priority)
	}
	if want := "p0,p1,p2"; strings.Join(got, ",") != want {
		t.Errorf("list order = %q, want %q", strings.Join(got, ","), want)
	}

	var filtered []task.Task
	tc.mustRunJSON(&filtered, "list", "--priority", "p0", "--json")
	if len(filtered) != 1 || filtered[0].ID != created.ID {
		t.Errorf("list --priority p0 = %+v, want just %s", filtered, created.ID)
	}

	var updated task.Task
	tc.mustRunJSON(&updated, "update", created.ID, "--priority", "p2", "--json")
	if updated.Priority != "p2" {
		t.Errorf("Priority after update = %q, want p2", updated.Priority)
	}
}

// A value from the built-in set is not special: once a project declares its
// own, "urgent" is as invalid as anything else, and the message says what is.
func TestCLIRejectsAPriorityOutsideTheVocabulary(t *testing.T) {
	tc := cliWithPriorities(t)
	tc.mustRun("add", "Something")

	for _, args := range [][]string{
		{"add", "Nope", "--priority", "urgent"},
		{"update", "TQ-0001", "--priority", "urgent"},
		{"list", "--priority", "urgent"},
		{"ready", "--priority", "urgent"},
	} {
		if code := tc.run(args...); code != exitError {
			t.Errorf("tq %s = exit %d, want %d", strings.Join(args, " "), code, exitError)
		}
		if !strings.Contains(tc.stderr.String(), "p0, p1, p2") {
			t.Errorf("tq %s stderr = %q, want it to list the valid values", strings.Join(args, " "), tc.stderr)
		}
	}
}

// Help that names the built-in set while the store refuses it would be worse
// than no help at all, so both the usage and the flag help read the config.
func TestCLIHelpNamesTheConfiguredVocabulary(t *testing.T) {
	tc := cliWithPriorities(t)

	usage := tc.mustRun("help")
	if !strings.Contains(usage, "Priorities: p0, p1, p2") {
		t.Errorf("usage does not name the project's priorities:\n%s", usage)
	}
	if !strings.Contains(usage, "default: p2") {
		t.Errorf("usage does not name the project's default:\n%s", usage)
	}

	// -h prints the flag help to stderr and exits 0.
	tc.run("add", "-h")
	if flags := tc.stderr.String(); !strings.Contains(flags, "p0, p1, p2") {
		t.Errorf("add flag help does not name the project's priorities:\n%s", flags)
	}
}

// ── The project's board ──────────────────────────────────────────

const customBoard = `version: 1
path: .tasks
columns:
  - name: spotted
    display_name: Spotted
    consider_ready: true
  - name: doing
    display_name: Doing
    default: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

func cliWithBoard(t *testing.T) *testCLI {
	t.Helper()
	tc := newBareCLI(t)
	tqtest.WriteConfig(t, tc.root, customBoard)
	if code := tc.run("init"); code != exitOK {
		t.Fatalf("init failed: %d %s", code, tc.stderr)
	}
	return tc
}

// Custom board: create, move, ready, sort, and done follow column flags.
func TestCLIRoundTripsACustomBoard(t *testing.T) {
	tc := cliWithBoard(t)

	var defaulted task.Task
	tc.mustRunJSON(&defaulted, "add", "Filed with no column", "--json")
	if defaulted.Status != "doing" {
		t.Errorf("Status = %q, want the column marked default", defaulted.Status)
	}

	var spotted task.Task
	tc.mustRunJSON(&spotted, "add", "Something to pick up", "--status", "spotted", "--json")

	var ready []task.Task
	tc.mustRunJSON(&ready, "ready", "--json")
	if len(ready) != 1 || ready[0].ID != spotted.ID {
		t.Errorf("ready = %+v, want only the task in the column that offers work", ready)
	}

	var finished task.Task
	tc.mustRunJSON(&finished, "done", spotted.ID, "--json")
	if finished.Status != "shipped" {
		t.Errorf("done moved to %q, want shipped", finished.Status)
	}

	var blocked task.Task
	tc.mustRunJSON(&blocked, "add", "Waits on it", "--status", "spotted", "--depends-on", spotted.ID, "--json")
	tc.mustRunJSON(&ready, "ready", "--json")
	found := false
	for _, tk := range ready {
		if tk.ID == blocked.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("ready = %+v, want the task whose dependency reached the satisfying column", ready)
	}

	var listed []task.Task
	tc.mustRunJSON(&listed, "list", "--json")
	got := make([]string, 0, len(listed))
	for _, tk := range listed {
		got = append(got, tk.Status)
	}
	if want := "spotted,doing,shipped"; strings.Join(got, ",") != want {
		t.Errorf("list order = %q, want %q", strings.Join(got, ","), want)
	}
}

func TestCLIRejectsAStatusTheBoardHasNoColumnFor(t *testing.T) {
	tc := cliWithBoard(t)
	tc.mustRun("add", "Something", "--status", "spotted")

	for _, args := range [][]string{
		{"add", "Nope", "--status", "todo"},
		{"move", "TQ-0001", "todo"},
		{"update", "TQ-0001", "--status", "todo"},
		{"list", "--status", "todo"},
	} {
		if code := tc.run(args...); code != exitError {
			t.Errorf("tq %s = exit %d, want %d", strings.Join(args, " "), code, exitError)
		}
		if !strings.Contains(tc.stderr.String(), "spotted, doing, shipped") {
			t.Errorf("tq %s stderr = %q, want it to list the columns", strings.Join(args, " "), tc.stderr)
		}
	}
}

// A board with no column claiming finished work cannot answer `tq done`, and
// has to say which way round it is rather than guessing.
func TestCLIDoneSaysWhenNoColumnClaimsIt(t *testing.T) {
	tc := newBareCLI(t)
	tqtest.WriteConfig(t, tc.root, "version: 1\npath: .tasks\ncolumns:\n  - {name: a}\n  - {name: b}\n")
	tc.mustRun("add", "Something")

	if code := tc.run("done", "TQ-0001"); code != exitError {
		t.Errorf("tq done = exit %d, want %d", code, exitError)
	}
	if !strings.Contains(tc.stderr.String(), "no column is marked") {
		t.Errorf("stderr = %q, want it to say no column claims finished work", tc.stderr)
	}
}

func TestCLIHelpNamesTheConfiguredBoard(t *testing.T) {
	tc := cliWithBoard(t)
	if usage := tc.mustRun("help"); !strings.Contains(usage, "Statuses:   spotted, doing, shipped") {
		t.Errorf("usage does not name the project's board:\n%s", usage)
	}
}

// backlog resolves to inbox via the built-in alias.
func TestCLIAcceptsBacklogAsInbox(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Something")

	var moved task.Task
	tc.mustRunJSON(&moved, "move", "TQ-0001", "backlog", "--json")
	if moved.Status != task.StatusInbox {
		t.Errorf("move to backlog put the task in %q, want inbox", moved.Status)
	}
}
