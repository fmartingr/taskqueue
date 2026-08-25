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
	root := t.TempDir()
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("directory contains %d files, want 1 (the old name should be gone)", len(entries))
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
