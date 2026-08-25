package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testCLI struct {
	*cli
	t      *testing.T
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	root   string
}

// newTestCLI returns a CLI rooted in a temporary project that already has a
// task directory.
// requireNoQueueAbove skips a test whose premise is that nothing was excluded,
// when the machine it runs on says otherwise: TMPDIR may sit inside a project
// that has its own queue, and the notice under test would then be correct.
func requireNoQueueAbove(t *testing.T, dir string) {
	t.Helper()
	for cur := filepath.Dir(dir); ; {
		if info, err := os.Stat(filepath.Join(cur, TaskDirName)); err == nil && info.IsDir() {
			t.Skipf("%s sits above the fixture, so a notice about it is correct", filepath.Join(cur, TaskDirName))
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return
		}
		cur = parent
	}
}

// anchorProject marks a directory as a repository root, so task directory
// discovery stops there. Without it a fixture walks out of testRoot(t) and can
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
	t.Setenv(EnvTaskDir, "")
	t.Setenv(EnvWalkForever, "")
	root := testRoot(t)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	return &testCLI{
		cli:    &cli{stdout: stdout, stderr: stderr, dir: root},
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
	dir := filepath.Join(tc.root, TaskDirName)
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

	var task Task
	tc.mustRunJSON(&task, "add", "Build board", "--priority", "high",
		"--label", "frontend", "--label", "ui", "--assignee", "agent-ui",
		"--depends-on", "TQ-0001", "--body", "Kanban board.", "--json")

	if task.ID != "TQ-0002" || task.Title != "Build board" {
		t.Errorf("task = %+v", task)
	}
	if task.Priority != PriorityHigh || task.Assignee != "agent-ui" {
		t.Errorf("task = %+v", task)
	}
	if strings.Join(task.Labels, ",") != "frontend,ui" {
		t.Errorf("Labels = %v", task.Labels)
	}
	if strings.Join(task.DependsOn, ",") != "TQ-0001" {
		t.Errorf("DependsOn = %v", task.DependsOn)
	}
	if task.Body != "Kanban board." {
		t.Errorf("Body = %q", task.Body)
	}
	if task.Status != StatusTodo {
		t.Errorf("Status = %q, want %q", task.Status, StatusTodo)
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

	var tasks []Task
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
	for _, want := range []string{"TQ-0001", "Implement REST API", "todo", "high", "backend", "Some description."} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}

	var task Task
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if task.ID != "TQ-0001" || task.Body != "Some description." {
		t.Errorf("show --json = %+v", task)
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
	for _, want := range []string{"TQ-0001 (done)", "TQ-0002 (todo, blocking)", "TQ-0404 (missing)"} {
		if !strings.Contains(out, want) {
			t.Errorf("show output missing %q:\n%s", want, out)
		}
	}
}

func TestCLIShowSurvivesUnreadableSiblingTask(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Readable", "--depends-on", "TQ-0002")
	if err := os.WriteFile(filepath.Join(tc.root, TaskDirName, "TQ-0002-broken.md"), []byte("not a task"), 0o644); err != nil {
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
	if want := "TQ-0001: todo -> in-progress"; !strings.Contains(out, want) {
		t.Errorf("move output = %q, want it to contain %q", out, want)
	}

	out = tc.mustRun("done", "TQ-0001")
	if want := "TQ-0001: in-progress -> done"; !strings.Contains(out, want) {
		t.Errorf("done output = %q, want it to contain %q", out, want)
	}

	var task Task
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if task.Status != StatusDone {
		t.Errorf("status = %q, want done", task.Status)
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

	var task Task
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if task.Title != "New title" || task.Priority != PriorityUrgent || task.Assignee != "agent-api" {
		t.Errorf("task = %+v", task)
	}
	if strings.Join(task.Labels, ",") != "auth" {
		t.Errorf("Labels = %v, want [auth]", task.Labels)
	}
	if strings.Join(task.DependsOn, ",") != "TQ-0002" {
		t.Errorf("DependsOn = %v", task.DependsOn)
	}

	tc.mustRun("update", "TQ-0001", "--remove-dependency", "TQ-0002")
	var afterRemoval Task // a fresh value: omitted JSON fields do not clear a reused struct
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
	dir := filepath.Join(tc.root, TaskDirName)
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
	var task Task
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if task.Title != "A better title" {
		t.Errorf("task = %+v", task)
	}
}

func TestCLINote(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Implement REST API", "--body", "Description.")

	out := tc.mustRun("note", "TQ-0001", "CRUD endpoints implemented; tests remain.")
	if !strings.Contains(out, "TQ-0001") {
		t.Errorf("note output = %q", out)
	}

	var task Task
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if !strings.Contains(task.Body, "## Notes") {
		t.Errorf("body should have a Notes section:\n%s", task.Body)
	}
	if !strings.Contains(task.Body, "CRUD endpoints implemented; tests remain.") {
		t.Errorf("body should contain the note:\n%s", task.Body)
	}
	if !strings.HasPrefix(task.Body, "Description.") {
		t.Errorf("the original body should be preserved:\n%s", task.Body)
	}

	tc.mustRun("note", "TQ-0001", "Second note.")
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if strings.Count(task.Body, "## Notes") != 1 {
		t.Errorf("a second note should reuse the section:\n%s", task.Body)
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

	var task Task
	tc.mustRunJSON(&task, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(task.Body, body) {
		t.Fatalf("the original body should be untouched:\n%s", task.Body)
	}
	want := body + "\n\n---\n\n## Notes\n\n- "
	if !strings.HasPrefix(task.Body, want) {
		t.Errorf("the note should start a new section at the end:\n%s", task.Body)
	}
	if !strings.HasSuffix(task.Body, " — The real note.") {
		t.Errorf("the note should be the last line:\n%s", task.Body)
	}
}

func TestCLIReady(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Implement REST API", "--priority", "high", "--label", "backend")
	tc.mustRun("add", "Build Kanban board", "--label", "frontend", "--depends-on", "TQ-0001")

	out := tc.mustRun("ready")
	if !strings.Contains(out, "TQ-0001") || strings.Contains(out, "TQ-0002") {
		t.Errorf("only the unblocked task should be ready:\n%s", out)
	}

	tc.mustRun("move", "TQ-0001", "in-progress")
	var tasks []Task
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
	dir := filepath.Join(tc.root, TaskDirName)
	if _, err := os.Stat(filepath.Join(dir, "TQ-0001-first-task.md")); err != nil {
		t.Fatalf("task file not written: %v", err)
	}
	if !strings.Contains(tc.stderr.String(), dir) {
		t.Errorf("stderr should note the created directory, got %q", tc.stderr)
	}

	// Reading commands work the same way, and say nothing once it exists.
	var tasks []Task
	tc.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 1 {
		t.Errorf("list returned %d tasks, want 1", len(tasks))
	}
}

func TestCLIReadCommandsCreateAnEmptyQueue(t *testing.T) {
	tc := newBareCLI(t)

	var tasks []Task
	tc.mustRunJSON(&tasks, "ready", "--json")
	if len(tasks) != 0 {
		t.Errorf("ready = %+v, want an empty list", tasks)
	}
	if _, err := os.Stat(filepath.Join(tc.root, TaskDirName)); err != nil {
		t.Errorf("%s should have been created: %v", TaskDirName, err)
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
		if !strings.Contains(tc.stderr.String(), TaskDirName) {
			t.Errorf("stderr should mention %s, got %q", TaskDirName, tc.stderr)
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
	if !strings.Contains(out, version) {
		t.Errorf("version output = %q, want it to contain %q", out, version)
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
	tc.mustRun("add", "Task in the project")

	elsewhere := newBareCLI(t)
	elsewhere.t.Setenv(EnvTaskDir, filepath.Join(tc.root, TaskDirName))

	var tasks []Task
	elsewhere.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 1 || tasks[0].Title != "Task in the project" {
		t.Errorf("%s override ignored, got %+v", EnvTaskDir, tasks)
	}
}

// Creating a local queue while one sits above the repository is the moment a
// developer's tasks appear to vanish, so that is where tq should name the
// variable that would have found them.
func TestCLINamesAQueueTheBoundExcluded(t *testing.T) {
	outer := testRoot(t)
	if _, err := InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: repo}, t: t, stdout: stdout, stderr: stderr, root: repo}
	tc.mustRun("list")

	notice := stderr.String()
	for _, want := range []string{filepath.Join(outer, TaskDirName), EnvWalkForever} {
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

	if strings.Contains(tc.stderr.String(), EnvWalkForever) {
		t.Errorf("stderr = %q, want no mention of %s when nothing was excluded", tc.stderr, EnvWalkForever)
	}
}

// The reason this fix was reverted once: with init discovering, an unanchored
// fixture walks out of testRoot(t). TQ-0017's bound does not help here, since
// a bare temp directory has no repository root to stop at, so the fixtures
// carry their own anchor.
func TestCLIFixturesCannotReachAQueueAboveTempDir(t *testing.T) {
	outer := testRoot(t)
	if _, err := InitStore(outer); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadDir(filepath.Join(outer, TaskDirName))
	if err != nil {
		t.Fatal(err)
	}

	project := filepath.Join(outer, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: project}, t: t, stdout: stdout, stderr: stderr, root: project}
	anchorProject(t, project)

	tc.mustRun("init")
	tc.mustRun("add", "fixture task")

	after, err := os.ReadDir(filepath.Join(outer, TaskDirName))
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
	outer := testRoot(t)
	anchorProject(t, outer)

	project := filepath.Join(outer, "project")
	nested := filepath.Join(project, "backend")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// Directly, not through InitStore: the enclosing anchor would send it to
	// the repository root, and the point here is a queue the project owns.
	if err := os.MkdirAll(filepath.Join(project, TaskDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	seedOut, seedErr := &bytes.Buffer{}, &bytes.Buffer{}
	seed := &testCLI{cli: &cli{stdout: seedOut, stderr: seedErr, dir: project}, t: t, stdout: seedOut, stderr: seedErr, root: project}
	seed.mustRun("add", "existing work")

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	sub := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: nested}, t: t, stdout: stdout, stderr: stderr, root: nested}

	var out struct {
		TaskDir string `json:"task_dir"`
		Created bool   `json:"created"`
	}
	sub.mustRunJSON(&out, "init", "--json")

	if want := filepath.Join(project, TaskDirName); out.TaskDir != want {
		t.Errorf("task_dir = %q, want the project's queue %q", out.TaskDir, want)
	}
	if out.Created {
		t.Error("created = true, want false: the queue already existed")
	}
	if _, err := os.Stat(filepath.Join(nested, TaskDirName)); !os.IsNotExist(err) {
		t.Error("init forked a second queue in the subdirectory")
	}
	if _, err := os.Stat(filepath.Join(outer, TaskDirName)); !os.IsNotExist(err) {
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
	outer := testRoot(t)
	if _, err := InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: repo}, t: t, stdout: stdout, stderr: stderr, root: repo}

	var out struct {
		TaskDir string `json:"task_dir"`
		Created bool   `json:"created"`
	}
	tc.mustRunJSON(&out, "init", "--json")

	if want := filepath.Join(repo, TaskDirName); out.TaskDir != want {
		t.Errorf("task_dir = %q, want the repository's own queue %q", out.TaskDir, want)
	}
	if !out.Created {
		t.Error("created = false, want true: the repository had no queue")
	}
}

// The CLI fixtures build their own store, so they need the same isolation the
// store fixtures have.
func TestCLIFixturesIgnoreAnAmbientTaskDirOverride(t *testing.T) {
	outside := filepath.Join(testRoot(t), "real", TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvTaskDir, outside)

	tc := newTestCLI(t)
	tc.mustRun("add", "fixture task")

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the CLI fixture wrote into the directory %s names: %d entries", EnvTaskDir, len(entries))
	}
}

// tq init must not write a guide into a task directory belonging to another
// project. The fixture is deliberately unanchored, because a project with no
// repository root is the shape where discovery has no bound; a queue at the
// temp root stops the walk inside the fixture, so nothing escapes it.
func TestCLIInitDoesNotWriteTheGuideOutsideTheInvokedTree(t *testing.T) {
	t.Setenv(EnvTaskDir, "")
	t.Setenv(EnvWalkForever, "")
	root := t.TempDir()

	outside := filepath.Join(root, TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "projects", "foo")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	tc := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: deep}, t: t, stdout: stdout, stderr: stderr, root: deep}
	tc.mustRun("init")

	if _, err := os.Stat(filepath.Join(outside, AgentsFileName)); !os.IsNotExist(err) {
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
	guide := filepath.Join(tc.root, TaskDirName, AgentsFileName)
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
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	sub := &testCLI{cli: &cli{stdout: stdout, stderr: stderr, dir: deep}, t: t, stdout: stdout, stderr: stderr, root: tc.root}
	sub.mustRun("init")

	if _, err := os.Stat(guide); err != nil {
		t.Errorf("init inside the repository should still write the guide: %v", err)
	}
}
