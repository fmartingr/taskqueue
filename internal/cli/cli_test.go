package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/store"

	"github.com/fmartingr/taskqueue/internal/tqtest"

	"github.com/fmartingr/taskqueue/internal/guide"
	"net/http"
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
func newTestCLI(t *testing.T) *testCLI {
	t.Helper()
	tc := newBareCLI(t)
	if code := tc.run("init"); code != exitOK {
		t.Fatalf("init failed: %d %s", code, tc.stderr)
	}
	tc.reset()
	return tc
}

// newBareCLI returns a CLI rooted in a fixture directory that is not a project
// yet — the shape `tq init` is meant to turn into one. The fixture asserts that
// no marker sits above it either, since an absent marker is the premise and a
// walk that reached one would put every command here on a different queue
// (TQ-0053).
func newBareCLI(t *testing.T) *testCLI {
	t.Helper()
	return newCLIIn(t, tqtest.RootWithoutMarker(t))
}

// newCLIIn returns a CLI running in dir. It is the only place a testCLI is
// built: a test that assembled its own would skip the isolation here, and the
// fixture guards below would have nothing to fail on.
func newCLIIn(t *testing.T, dir string) *testCLI {
	t.Helper()
	// The CLI fixtures build their own store, so they take the same isolation
	// the store fixtures do: never reach a real queue.
	tqtest.ClearEnv(t)
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	return &testCLI{
		cli:    &cli{stdin: strings.NewReader(""), stdout: stdout, stderr: stderr, dir: dir, version: testVersion},
		t:      t,
		stdout: stdout,
		stderr: stderr,
		root:   dir,
	}
}

// feed puts text on the CLI's standard input, for the commands that read a
// value from it. The reader is consumed by the next command that asks for it.
func (tc *testCLI) feed(text string) *testCLI {
	tc.stdin = strings.NewReader(text)
	return tc
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

	// Initialising twice is not an error: the directory is already there and
	// the marker is left exactly as it was.
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

// One file nobody can parse — a merge conflict, a hand-added key — used to take
// the whole queue down. It now costs only itself: the healthy tasks print, the
// file is named on stderr, and the exit code stays 0 (TQ-0011).
func TestCLIListAndReadySkipAnUnreadableFile(t *testing.T) {
	broken := "TQ-0003-broken.md"
	newProject := func(t *testing.T) *testCLI {
		t.Helper()
		tc := newTestCLI(t)
		tc.mustRun("add", "Healthy and ready", "--status", "todo")
		tc.mustRun("add", "Healthy too", "--status", "todo")
		content := "---\nid: TQ-0003\ntitle: broken\nstatus: todo\nepic: platform\n---\n"
		if err := os.WriteFile(filepath.Join(tc.root, config.TaskDirName, broken), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return tc
	}

	for _, command := range []string{"list", "ready"} {
		t.Run(command, func(t *testing.T) {
			tc := newProject(t)

			out := tc.mustRun(command)
			for _, id := range []string{"TQ-0001", "TQ-0002"} {
				if !strings.Contains(out, id) {
					t.Errorf("%s output is missing %s:\n%s", command, id, out)
				}
			}
			if !strings.Contains(tc.stderr.String(), broken) {
				t.Errorf("stderr should name %s, got %q", broken, tc.stderr)
			}
		})

		t.Run(command+" --json", func(t *testing.T) {
			tc := newProject(t)

			var tasks []task.Task
			tc.mustRunJSON(&tasks, command, "--json")
			if len(tasks) != 2 {
				t.Errorf("%s --json returned %d tasks, want the 2 healthy ones", command, len(tasks))
			}
			// The whole point of the split: stdout parsed above, so the warning
			// went where it cannot corrupt what an agent reads.
			if !strings.Contains(tc.stderr.String(), broken) {
				t.Errorf("stderr should name %s, got %q", broken, tc.stderr)
			}
		})
	}
}

// A stale copy of a task file is worse than a broken one: it parses, so it was
// listed, and a copy left at todo went on being offered as work long after the
// real task was done. Neither copy is listed now, and both surfaces name the
// two files and say the same thing about them (TQ-0040).
func TestCLIListAndReadyWithholdAnIDTwoFilesClaim(t *testing.T) {
	const stale = "TQ-0001-stale.md"
	newProject := func(t *testing.T) *testCLI {
		t.Helper()
		tc := newTestCLI(t)
		tc.mustRun("add", "Doubled", "--status", "todo")
		tc.mustRun("add", "Healthy and ready", "--status", "todo")
		// What an interrupted retitle leaves behind: a second file for TQ-0001,
		// left at the status it had when the first copy was written.
		content := "---\nid: TQ-0001\ntitle: doubled\nstatus: done\npriority: normal\n" +
			"created: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n"
		if err := os.WriteFile(filepath.Join(tc.root, config.TaskDirName, stale), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return tc
	}

	for _, command := range []string{"list", "ready"} {
		t.Run(command, func(t *testing.T) {
			tc := newProject(t)

			out := tc.mustRun(command)
			if strings.Contains(out, "TQ-0001") {
				t.Errorf("%s offers TQ-0001, which two files claim:\n%s", command, out)
			}
			if !strings.Contains(out, "TQ-0002") {
				t.Errorf("%s output is missing TQ-0002:\n%s", command, out)
			}
			warning := tc.stderr.String()
			if !strings.Contains(warning, stale) || !strings.Contains(warning, "TQ-0001-doubled.md") {
				t.Errorf("stderr should name both files, got %q", warning)
			}
		})

		t.Run(command+" --json", func(t *testing.T) {
			tc := newProject(t)

			var tasks []task.Task
			tc.mustRunJSON(&tasks, command, "--json")
			if len(tasks) != 1 || tasks[0].ID != "TQ-0002" {
				t.Errorf("%s --json returned %+v, want only TQ-0002", command, tasks)
			}
			// The whole point of the split: stdout parsed above, so the warning
			// went where it cannot corrupt what an agent reads.
			if !strings.Contains(tc.stderr.String(), stale) {
				t.Errorf("stderr should name %s, got %q", stale, tc.stderr)
			}
		})
	}

	// And the sentence is the one a lookup of that ID is refused with, so the
	// two surfaces cannot tell a different story about the same two files.
	t.Run("agrees with a lookup", func(t *testing.T) {
		tc := newProject(t)
		tc.mustRun("list")
		listed := tc.stderr.String()

		if code := tc.run("show", "TQ-0001"); code != exitError {
			t.Fatalf("show TQ-0001 = exit %d, want %d", code, exitError)
		}
		claim, _, _ := strings.Cut(strings.TrimPrefix(tc.stderr.String(), "error: invalid task file: "), "\n")
		if claim == "" || !strings.Contains(listed, claim) {
			t.Errorf("show says %q, the listing said %q; they are the same finding", claim, listed)
		}
	})
}

// `.md`, lowercase, is the only extension a task file may have, and a file
// spelled otherwise is not one — tq does not read it, adopt it or rename it.
// What it does is say so: a queue that looked empty with nothing on stderr was
// how a second file came to claim an ID in silence (TQ-0039).
func TestCLIListNamesAFileSpelledMD(t *testing.T) {
	const foreign = "TQ-0002-from-a-windows-checkout.MD"
	newProject := func(t *testing.T) *testCLI {
		t.Helper()
		tc := newTestCLI(t)
		tc.mustRun("add", "Healthy and ready", "--status", "todo")
		content := "---\nid: TQ-0002\ntitle: from a windows checkout\nstatus: todo\npriority: normal\n" +
			"created: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n"
		if err := os.WriteFile(filepath.Join(tc.root, config.TaskDirName, foreign), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return tc
	}

	t.Run("list", func(t *testing.T) {
		tc := newProject(t)

		out := tc.mustRun("list")
		if strings.Contains(out, "TQ-0002") {
			t.Errorf("list holds TQ-0002, which no task file claims:\n%s", out)
		}
		if warning := tc.stderr.String(); !strings.Contains(warning, foreign) || !strings.Contains(warning, ".md") {
			t.Errorf("stderr should name %s and the rule it breaks, got %q", foreign, warning)
		}
	})

	// A warning, not a failure: the tasks that did read are still the answer,
	// and they go to stdout alone so an agent's parse is untouched.
	t.Run("--json", func(t *testing.T) {
		tc := newProject(t)

		var tasks []task.Task
		tc.mustRunJSON(&tasks, "list", "--json")
		if len(tasks) != 1 || tasks[0].ID != "TQ-0001" {
			t.Errorf("list --json returned %+v, want only TQ-0001", tasks)
		}
		if !strings.Contains(tc.stderr.String(), foreign) {
			t.Errorf("stderr should name %s, got %q", foreign, tc.stderr)
		}
	})

	// And a lookup of the ID it claims says where it went, rather than leaving
	// the reader to wonder about a file that is plainly in the directory.
	t.Run("show", func(t *testing.T) {
		tc := newProject(t)

		if code := tc.run("show", "TQ-0002"); code != exitTaskNotFound {
			t.Fatalf("show TQ-0002 = exit %d, want %d", code, exitTaskNotFound)
		}
		if warning := tc.stderr.String(); !strings.Contains(warning, foreign) {
			t.Errorf("show says %q, want it to name %s", warning, foreign)
		}
	})
}

// A listing the store could not square with the directory says so, and says it
// on stderr: the listing itself is what an agent parses, and a warning on
// stdout would break --json. It is a warning and not a failure — the tasks it
// did read are still the answer (TQ-0012).
func TestCLIWarnsAboutAListingItCouldNotComplete(t *testing.T) {
	tc := newTestCLI(t)

	tc.warnListing(store.Listing{Incomplete: true})
	if got := tc.stderr.String(); !strings.Contains(got, "may be missing a task") {
		t.Errorf("stderr = %q, want it to say the listing may be short", got)
	}
	if got := tc.stdout.String(); got != "" {
		t.Errorf("stdout = %q, want nothing: --json parses this stream", got)
	}

	// Nothing to say about a listing that squared with the directory.
	tc.reset()
	tc.warnListing(store.Listing{})
	if got := tc.stderr.String(); got != "" {
		t.Errorf("stderr = %q, want nothing for an ordinary listing", got)
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

// `tq update --body` is how a body is revised, and keeping the notes is the
// contract: they are appended one at a time and reconstructible from nothing,
// so an edit that dropped them would lose what nobody can write again (TQ-0044).
func TestCLIUpdateBodyKeepsTheNotes(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Ticket", "--body", "## Finding\n\nThe old finding.")
	tc.mustRun("note", "TQ-0001", "Something worth remembering.")

	out := tc.mustRun("update", "TQ-0001", "--body", "## Finding\n\nThe corrected finding.")
	if !strings.Contains(out, "Updated TQ-0001") {
		t.Errorf("update output = %q", out)
	}

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(tk.Body, "## Finding\n\nThe corrected finding.") {
		t.Errorf("the content should be the new one:\n%s", tk.Body)
	}
	if strings.Contains(tk.Body, "The old finding.") {
		t.Errorf("the old content should be gone:\n%s", tk.Body)
	}
	if !strings.Contains(tk.Body, "Something worth remembering.") {
		t.Errorf("the note should have survived the edit:\n%s", tk.Body)
	}
	if strings.Count(tk.Body, "## Notes") != 1 {
		t.Errorf("the notes section should be there exactly once:\n%s", tk.Body)
	}

	// A second edit appends nothing and duplicates nothing.
	tc.mustRun("update", "TQ-0001", "--body", "Third draft.")
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if want := "Third draft.\n\n---\n\n## Notes\n\n"; !strings.HasPrefix(tk.Body, want) {
		t.Errorf("body:\ngot:  %q\nwant it to start with %q", tk.Body, want)
	}
	if strings.Count(tk.Body, "Something worth remembering.") != 1 {
		t.Errorf("the note should appear once:\n%s", tk.Body)
	}
}

func TestCLIUpdateBodyOnATaskWithoutNotes(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Ticket", "--body", "The old finding.")

	tc.mustRun("update", "TQ-0001", "--body", "The new finding.")

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if tk.Body != "The new finding." {
		t.Errorf("body = %q, want just the new content", tk.Body)
	}
	if strings.Contains(tk.Body, "## Notes") {
		t.Errorf("a task with no notes should not gain a section:\n%s", tk.Body)
	}
}

// Clearing the content is a body edit like any other. It empties the document
// and leaves the record, which is the case a wholesale replacement would eat.
func TestCLIUpdateBodyEmptyKeepsTheNotes(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Ticket", "--body", "Wrong from the start.")
	tc.mustRun("note", "TQ-0001", "Filed against the wrong component.")

	tc.mustRun("update", "TQ-0001", "--body", "")

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if strings.Contains(tk.Body, "Wrong from the start.") {
		t.Errorf("the content should be gone:\n%s", tk.Body)
	}
	if !strings.HasPrefix(tk.Body, "## Notes") {
		t.Errorf("the body should be the notes section alone:\n%s", tk.Body)
	}
	if !strings.Contains(tk.Body, "Filed against the wrong component.") {
		t.Errorf("the note should have survived:\n%s", tk.Body)
	}

	// And the same on a task that has no notes to keep.
	tc.mustRun("add", "Second")
	tc.mustRun("update", "TQ-0002", "--body", "")
	var empty task.Task
	tc.mustRunJSON(&empty, "show", "TQ-0002", "--json")
	if empty.Body != "" {
		t.Errorf("body = %q, want it empty", empty.Body)
	}
}

// A body may document how this project writes notes. A "## Notes" heading with
// another section after it is prose, and a "---" without a blank line above it
// underlines a setext heading — neither is the record, going in or coming out.
func TestCLIUpdateBodyDoesNotMistakeProseForNotes(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Ticket", "--body", "Placeholder.")
	tc.mustRun("note", "TQ-0001", "The real note.")

	content := "Finding\n---\n\n## Notes\n\nHow we write them.\n\n## Acceptance\n\n- ships"
	tc.mustRun("update", "TQ-0001", "--body", content)

	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(tk.Body, content) {
		t.Fatalf("the prose should be the content, verbatim:\n%s", tk.Body)
	}
	if !strings.HasSuffix(tk.Body, " — The real note.") {
		t.Errorf("the record should still end the body:\n%s", tk.Body)
	}

	// The proof is the next note: it lands in the record, not in the prose.
	tc.mustRun("note", "TQ-0001", "The second note.")
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(tk.Body, content) {
		t.Errorf("a note should not have been appended to the prose:\n%s", tk.Body)
	}
	if !strings.HasSuffix(tk.Body, " — The second note.") {
		t.Errorf("the note should be the last line:\n%s", tk.Body)
	}
}

// The loop the guide describes: `tq show --json` hands out the whole body,
// notes included, so an agent that reads it, edits it and hands it back passes
// the record straight back in. Appending the task's notes to a text that
// already carries them turned one revision into two copies of every note.
func TestCLIUpdateBodyRoundTripDoesNotDoubleTheNotes(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Ticket", "--body", "## Finding\n\nThe old finding.")
	tc.mustRun("note", "TQ-0001", "The record.")

	for pass := range 3 {
		var read task.Task
		tc.mustRunJSON(&read, "show", "TQ-0001", "--json")
		// Edited in place, the way an agent would: the whole body goes back.
		tc.feed(strings.Replace(read.Body, "old finding", "corrected finding", 1)).
			mustRun("update", "TQ-0001", "--body", "-")

		var written task.Task
		tc.mustRunJSON(&written, "show", "TQ-0001", "--json")
		if got := strings.Count(written.Body, "## Notes"); got != 1 {
			t.Fatalf("pass %d: %d notes sections, want 1:\n%s", pass+1, got, written.Body)
		}
		if got := strings.Count(written.Body, "The record."); got != 1 {
			t.Fatalf("pass %d: the note appears %d times:\n%s", pass+1, got, written.Body)
		}
	}

	var final task.Task
	tc.mustRunJSON(&final, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(final.Body, "## Finding\n\nThe corrected finding.") {
		t.Errorf("the edit should have landed:\n%s", final.Body)
	}
}

// A body is a document, and a shell has better ways to hand one over than an
// argument that has to survive its own quoting.
func TestCLIBodyFromStdin(t *testing.T) {
	tc := newTestCLI(t)

	tc.feed("## Finding\n\nPiped in.\n").mustRun("add", "Ticket", "--body", "-")
	var tk task.Task
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if tk.Body != "## Finding\n\nPiped in." {
		t.Errorf("add --body -: body = %q", tk.Body)
	}

	tc.mustRun("note", "TQ-0001", "A note to keep.")
	tc.feed("## Finding\n\nPiped in again.\n").mustRun("update", "TQ-0001", "--body", "-")
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(tk.Body, "## Finding\n\nPiped in again.") {
		t.Errorf("update --body -: body = %q", tk.Body)
	}
	if !strings.Contains(tk.Body, "A note to keep.") {
		t.Errorf("update --body - should keep the notes:\n%s", tk.Body)
	}

	// An empty stdin is a value like any other: it is how `--body ""` is
	// spelled through a pipe.
	tc.feed("").mustRun("update", "TQ-0001", "--body", "-")
	tc.mustRunJSON(&tk, "show", "TQ-0001", "--json")
	if !strings.HasPrefix(tk.Body, "## Notes") {
		t.Errorf("an empty stdin should clear the content and keep the notes:\n%s", tk.Body)
	}
}

// The --json contract: data on stdout and nothing else, everything else on
// stderr. Agents parse the first and would choke on a warning in it.
func TestCLIUpdateBodyJSONIsTheOnlyThingOnStdout(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Ticket", "--body", "Old.")
	tc.mustRun("note", "TQ-0001", "Kept.")

	var tk task.Task
	tc.mustRunJSON(&tk, "update", "TQ-0001", "--body", "New.", "--json")
	if !strings.HasPrefix(tk.Body, "New.") || !strings.Contains(tk.Body, "Kept.") {
		t.Errorf("the JSON should carry the saved body: %q", tk.Body)
	}
	if tc.stderr.Len() != 0 {
		t.Errorf("stderr = %q, want it empty", tc.stderr)
	}

	// A missing task keeps stdout clean and exits 2.
	if code := tc.run("update", "TQ-4242", "--body", "New.", "--json"); code != exitTaskNotFound {
		t.Errorf("update on a missing task = exit %d, want %d", code, exitTaskNotFound)
	}
	if tc.stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing on a failure", tc.stdout)
	}
	if tc.stderr.Len() == 0 {
		t.Error("the failure should be reported on stderr")
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

// Once init has made the queue, every command works from it and says nothing
// about creating anything: there is nothing left for them to create.
func TestCLIWorksOnTheQueueInitMade(t *testing.T) {
	tc := newTestCLI(t)

	out := tc.mustRun("add", "First task")
	if !strings.Contains(out, "Created TQ-0001") {
		t.Errorf("add output = %q", out)
	}
	dir := filepath.Join(tc.root, config.TaskDirName)
	if _, err := os.Stat(filepath.Join(dir, "TQ-0001-first-task.md")); err != nil {
		t.Fatalf("task file not written: %v", err)
	}
	if tc.stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing: the queue was already there", tc.stderr)
	}

	var tasks []task.Task
	tc.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 1 {
		t.Errorf("list returned %d tasks, want 1", len(tasks))
	}
}

// A queue that cannot be made is init's problem to report, and it is a plain
// error rather than "no project found": the project is right here.
func TestCLIInitReportsAnUncreatableTaskDir(t *testing.T) {
	tc := newBareCLI(t)
	// A regular file where the task directory belongs. A permission bit would
	// not do — uid 0 ignores it, and CI runs as root in a container — but no
	// privilege makes a directory out of a file.
	if err := os.WriteFile(filepath.Join(tc.root, config.TaskDirName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := tc.run("init"); code != exitError {
		t.Errorf("tq init = exit %d, want %d", code, exitError)
	}
	if !strings.Contains(tc.stderr.String(), config.TaskDirName) {
		t.Errorf("stderr should mention %s, got %q", config.TaskDirName, tc.stderr)
	}
	if tc.stdout.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", tc.stdout)
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

// TQ_CONFIG_PATH hands the command a marker, so it works on that project from
// a directory that is not one.
func TestCLIEnvConfigPathOverride(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "task.Task in the project")

	elsewhere := newBareCLI(t)
	elsewhere.t.Setenv(config.EnvConfigPath, filepath.Join(tc.root, config.ConfigFileName))

	var tasks []task.Task
	elsewhere.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 1 || tasks[0].Title != "task.Task in the project" {
		t.Errorf("%s override ignored, got %+v", config.EnvConfigPath, tasks)
	}
}

// A marker the variable names but tq cannot use stops the command rather than
// sending it to whatever the walk would have found.
func TestCLIEnvConfigPathRefusesWhatItCannotRead(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "task.Task in the project")

	for _, tc2 := range []struct {
		name string
		path func(root string) string
	}{
		{"missing", func(root string) string { return filepath.Join(root, "nowhere.yaml") }},
		{"a directory", func(root string) string { return root }},
	} {
		t.Run(tc2.name, func(t *testing.T) {
			elsewhere := newBareCLI(t)
			elsewhere.t.Setenv(config.EnvConfigPath, tc2.path(tc.root))
			if code := elsewhere.run("list"); code == exitOK {
				t.Errorf("exit = 0, want a failure for a %s the variable names", tc2.name)
			}
			if !strings.Contains(elsewhere.stderr.String(), config.EnvConfigPath) {
				t.Errorf("stderr = %q, want it to name %s", elsewhere.stderr, config.EnvConfigPath)
			}
		})
	}
}

// tq init makes the queue in the directory it is run in, and nowhere else: a
// project above does not adopt it, and a repository root does not relocate it.
// Forking in a subdirectory is the specified behaviour now, not the bug TQ-0047
// filed (TQ-0085).
func TestCLIInitCreatesTheQueueWhereItIsRun(t *testing.T) {
	outer := newTestCLI(t)
	outer.mustRun("add", "the parent's work")

	nested := filepath.Join(outer.root, "src", "deep")
	if err := os.MkdirAll(filepath.Join(nested, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := newCLIIn(t, nested)

	var out struct {
		TaskDir string   `json:"task_dir"`
		Created bool     `json:"created"`
		Written []string `json:"written"`
	}
	sub.mustRunJSON(&out, "init", "--json")

	if want := filepath.Join(nested, config.TaskDirName); out.TaskDir != want {
		t.Errorf("task_dir = %q, want the queue in the directory init was run in, %q", out.TaskDir, want)
	}
	if !out.Created {
		t.Error("created = false, want true")
	}
	marker := filepath.Join(nested, config.ConfigFileName)
	if !slices.Contains(out.Written, marker) {
		t.Errorf("written = %v, want it to include %q", out.Written, marker)
	}
	if _, err := os.Stat(filepath.Join(nested, config.TaskDirName, guide.AgentsFileName)); err != nil {
		t.Errorf("the guide belongs beside the queue init made: %v", err)
	}

	// And the commands below it use the nearer marker, not the parent's.
	sub.reset()
	if listing := sub.mustRun("list"); strings.Contains(listing, "the parent's work") {
		t.Errorf("the subdirectory still reads the parent's queue: %q", listing)
	}
}

// Running init twice changes nothing the second time: the marker is the user's
// file and the queue is already there.
func TestCLIInitTwiceChangesNothing(t *testing.T) {
	tc := newBareCLI(t)
	tc.mustRun("init")

	marker := filepath.Join(tc.root, config.ConfigFileName)
	before, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	guidePath := filepath.Join(tc.root, config.TaskDirName, guide.AgentsFileName)
	beforeGuide, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatal(err)
	}

	tc.reset()
	var out struct {
		TaskDir string   `json:"task_dir"`
		Created bool     `json:"created"`
		Written []string `json:"written"`
	}
	tc.mustRunJSON(&out, "init", "--json")

	if out.Created {
		t.Error("created = true on the second init")
	}
	if len(out.Written) != 0 {
		t.Errorf("written = %v, want nothing rewritten", out.Written)
	}
	if after, err := os.ReadFile(marker); err != nil || string(after) != string(before) {
		t.Errorf("the marker changed: %v\n%s", err, after)
	}
	if after, err := os.ReadFile(guidePath); err != nil || string(after) != string(beforeGuide) {
		t.Errorf("the guide changed: %v", err)
	}
}

// Every command but init needs a project to exist, and says which command makes
// one. Nothing is created on the way to that message.
func TestCLIRefusesToWorkWithoutAProject(t *testing.T) {
	for _, args := range [][]string{
		{"list"},
		{"ready"},
		{"add", "something"},
		{"show", "TQ-0001"},
		{"note", "TQ-0001", "text"},
	} {
		t.Run(args[0], func(t *testing.T) {
			tc := newBareCLI(t)

			if code := tc.run(args...); code != exitProjectNotFound {
				t.Errorf("tq %s = exit %d, want %d\nstderr: %s", strings.Join(args, " "), code, exitProjectNotFound, tc.stderr)
			}
			for _, want := range []string{config.ConfigFileName, "tq init"} {
				if !strings.Contains(tc.stderr.String(), want) {
					t.Errorf("stderr = %q, want it to mention %q", tc.stderr, want)
				}
			}
			if tc.stdout.Len() != 0 {
				t.Errorf("stdout = %q, want nothing but the error on stderr", tc.stdout)
			}
			entries, err := os.ReadDir(tc.root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("the command created %d entries, want none", len(entries))
			}
		})
	}
}

// The nearest marker wins, and a .git in between bounds nothing (TQ-0059).
func TestCLIUsesTheNearestMarker(t *testing.T) {
	outer := newTestCLI(t)
	outer.mustRun("add", "the parent's work")

	// A submodule-shaped directory: its own .git, no marker of its own.
	submodule := filepath.Join(outer.root, "vendor", "dep")
	if err := os.MkdirAll(filepath.Join(submodule, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if listing := newCLIIn(t, submodule).mustRun("list"); !strings.Contains(listing, "the parent's work") {
		t.Errorf("a directory with its own .git lost the project's queue: %q", listing)
	}
	if _, err := os.Stat(filepath.Join(submodule, config.TaskDirName)); !os.IsNotExist(err) {
		t.Error("a command forked a queue inside the submodule")
	}

	// A marker of its own is what takes it off the parent's queue.
	service := filepath.Join(outer.root, "service")
	if err := os.MkdirAll(service, 0o755); err != nil {
		t.Fatal(err)
	}
	inner := newCLIIn(t, service)
	inner.mustRun("init")
	inner.mustRun("add", "the service's work")
	inner.reset()

	deep := filepath.Join(service, "src")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	listing := newCLIIn(t, deep).mustRun("list")
	if !strings.Contains(listing, "the service's work") || strings.Contains(listing, "the parent's work") {
		t.Errorf("the nearest marker did not win: %q", listing)
	}
}

// The fixture must not be able to reach a queue above its temporary directory.
// This drives the shared fixture rather than building one: give newBareCLI a
// bare t.TempDir() back and the guard inside it is what fails.
func TestCLIFixturesCannotReachAQueueAboveTempDir(t *testing.T) {
	tc := newBareCLI(t)

	// A real project one level above the fixture root — exactly where a walk
	// that escaped the fixture would arrive, and where a developer's own queue
	// sits when TMPDIR is inside their repository.
	above := tqtest.AboveFixtures(t)
	outside := filepath.Join(above, config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	tqtest.WriteConfig(t, above, "version: 1\npath: "+config.TaskDirName+"\n")

	tc.mustRun("init")
	tc.mustRun("add", "fixture task")

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the fixture wrote into the queue above it: %d entries", len(entries))
	}
	if _, err := os.Stat(filepath.Join(tc.root, config.TaskDirName)); err != nil {
		t.Errorf("the fixture should have made its own queue instead: %v", err)
	}
}

// The CLI fixtures build their own store, so they need the same isolation the
// store fixtures have.
func TestCLIFixturesIgnoreAnAmbientConfigPathOverride(t *testing.T) {
	real := tqtest.Root(t)
	outside := filepath.Join(real, config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvConfigPath, filepath.Join(real, config.ConfigFileName))

	tc := newTestCLI(t)
	tc.mustRun("add", "fixture task")

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the CLI fixture wrote into the queue %s names: %d entries", config.EnvConfigPath, len(entries))
	}
}

// A project above the directory init is run in keeps its queue and its guide:
// init writes into the queue it just made, and touches nothing else.
func TestCLIInitLeavesTheProjectAboveAlone(t *testing.T) {
	outer := newTestCLI(t)
	// Removed so a guide found there afterwards can only have been written by
	// the init below.
	outerGuide := filepath.Join(outer.root, config.TaskDirName, guide.AgentsFileName)
	if err := os.Remove(outerGuide); err != nil {
		t.Fatal(err)
	}

	deep := filepath.Join(outer.root, "projects", "foo")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	newCLIIn(t, deep).mustRun("init")

	if _, err := os.Stat(outerGuide); !os.IsNotExist(err) {
		t.Errorf("init wrote a guide into %s, which belongs to the project above: %v", outer.root, err)
	}
	if _, err := os.Stat(filepath.Join(deep, config.TaskDirName, guide.AgentsFileName)); err != nil {
		t.Errorf("init did not write the guide beside the queue it made: %v", err)
	}
}

// The guide is rewritten wherever init runs, so a project whose guide went
// missing gets it back by running init at its root.
func TestCLIInitRestoresTheGuide(t *testing.T) {
	tc := newTestCLI(t)
	guidePath := filepath.Join(tc.root, config.TaskDirName, guide.AgentsFileName)
	if _, err := os.Stat(guidePath); err != nil {
		t.Fatalf("guide not written at the project root: %v", err)
	}
	if err := os.Remove(guidePath); err != nil {
		t.Fatal(err)
	}

	tc.mustRun("init")
	if _, err := os.Stat(guidePath); err != nil {
		t.Errorf("init should have written the guide back: %v", err)
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
	tc.mustRun("init")
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
	// It names the guide by its absolute path, so the line holds wherever the
	// user pastes it and whatever directory they ran init from (TQ-0061).
	pointer := "\nThe agent guide is at:\n\n    " + guidePath + "\n"
	if !strings.Contains(out, pointer) {
		t.Errorf("init should name the guide, got %q", out)
	}
	if !strings.Contains(out, "Include it in your preferred agent context file") {
		t.Errorf("init should say what to do with the guide, got %q", out)
	}
	if !strings.Contains(out, "\nStart the board with:\n\n    tq serve\n") {
		t.Errorf("init should say how to start the board, got %q", out)
	}
	if want := "http://" + defaultHost + ":" + defaultPort; !strings.Contains(out, want) {
		t.Errorf("init should name the default board address %q, got %q", want, out)
	}

	// Re-running refreshes without reporting spurious writes, but still names
	// the guide — the file that references it may not exist yet — and still
	// says how to start the board.
	out = tc.mustRun("init")
	if strings.Contains(out, "Wrote ") {
		t.Errorf("nothing should be rewritten on a second init, got %q", out)
	}
	if !strings.Contains(out, pointer) {
		t.Errorf("the second init should still name the guide, got %q", out)
	}
	if !strings.Contains(out, "\nStart the board with:\n\n    tq serve\n") {
		t.Errorf("the second init should still say how to start the board, got %q", out)
	}
}

// --port 0 asks the OS for a free port, which is how anything driving the real
// binary avoids racing for one. The banner has to say which port it got, or the
// caller has no way to find out.
func TestCLIServePrintsTheAddressItActuallyGot(t *testing.T) {
	tc := newTestCLI(t)

	done := make(chan int, 1)
	go func() { done <- tc.run("serve", "--port", "0") }()

	line := awaitBanner(t, tc)
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

	stopServe(t, done)
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
	tc.mustRun("init")
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

// The commands an interrupt is held through are the ones that put a file on
// disk, and they have to be commands the CLI actually dispatches: a name that
// has been renamed or dropped leaves a save running unprotected under a set
// that still reads correctly. serve is deliberately not among them — it
// installs a handler of its own, because it is the one command whose whole job
// happens after the signal arrives.
func TestWriteCommandsAreCommandsTheCLIHas(t *testing.T) {
	if writeCommands["serve"] {
		t.Error("serve handles its own signals; holding them for it would take its graceful shutdown away")
	}
	for name := range writeCommands {
		t.Run(name, func(t *testing.T) {
			tc := newBareCLI(t)
			tc.run(name)
			if strings.Contains(tc.stderr.String(), "unknown command") {
				t.Errorf("%q is held through an interrupt but the CLI does not dispatch it: %s", name, tc.stderr)
			}
		})
	}
}

// Holding a signal is all holdSignals does: with none to report it is the
// command it ran, exit code and streams alike. A save that finished is a save
// that landed, so the code it returns has to be the command's own — saying
// otherwise is the report for a landed write that TQ-0015 is about.
func TestHoldSignalsPassesTheCommandThrough(t *testing.T) {
	var stderr bytes.Buffer
	if code := holdSignals(&stderr, func() int { return exitTaskNotFound }); code != exitTaskNotFound {
		t.Errorf("holdSignals = %d, want the command's own %d", code, exitTaskNotFound)
	}
	if stderr.Len() != 0 {
		t.Errorf("holdSignals wrote %q with no signal to report", stderr.String())
	}
}

// ── The marker is the source of truth (TQ-0087) ─────────────────

// escapedProject is the board and vocabularies of the project the CLI tests
// below run in. No default and no decoy shares a name with any of them.
const escapedProject = `columns:
  - name: backlog
    display_name: Backlog
    default: true
  - name: doing
    display_name: Doing
    consider_ready: true
  - name: shipped
    display_name: Shipped
    consider_done: true
priorities:
  - name: blocker
    color: "#b42318"
  - name: routine
    color: "#4b5563"
    default: true
labels:
  billing:
    color: "#00ff00"
    display_name: Billing
`

// newEscapedCLI runs the CLI in a project whose `path:` leaves the marker's own
// directory, with a decoy marker above the queue for anything that walks up
// from it to find.
func newEscapedCLI(t *testing.T) *testCLI {
	t.Helper()
	root, _ := tqtest.EscapedQueue(t, escapedProject)
	tc := newCLIIn(t, root)
	tc.mustRun("init")
	tc.reset()
	return tc
}

// Every command has to keep the project's board, vocabulary and labels, not
// just the one that reads them last (TQ-0087).
func TestCLIKeepsTheProjectsConfigWhenThePathLeavesTheMarkersDirectory(t *testing.T) {
	tc := newEscapedCLI(t)

	var created task.Task
	tc.mustRunJSON(&created, "add", "Real work", "--json")
	if created.Status != "backlog" {
		t.Errorf("status = %q, want the project's default column backlog", created.Status)
	}
	if created.Priority != "routine" {
		t.Errorf("priority = %q, want the project's default priority routine", created.Priority)
	}

	// A column the project declares, which the built-in board does not.
	if out := tc.mustRun("move", created.ID, "doing"); !strings.Contains(out, "doing") {
		t.Errorf("move said %q, want it to name the column", out)
	}

	// A priority the project declares.
	tc.mustRun("update", created.ID, "--priority", "blocker")

	var listed []labelRow
	tc.mustRunJSON(&listed, "label", "list", "--json")
	if len(listed) != 1 || listed[0].Name != "billing" || !listed[0].Configured {
		t.Errorf("label list = %+v, want the project's own vocabulary", listed)
	}

	var shown task.Task
	tc.mustRunJSON(&shown, "show", created.ID, "--json")
	if shown.Status != "doing" || shown.Priority != "blocker" {
		t.Errorf("task = %+v, want status doing and priority blocker", shown)
	}
}

// The data-loss case: an edit that says nothing about status must not move the
// task. It did, silently and with exit 0, whenever the board a write was
// validated against was not the project's (TQ-0087).
func TestCLIUnrelatedEditsLeaveTheStatusAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"assignee", []string{"update", "TQ-0001", "--assignee", "alice"}},
		{"title", []string{"update", "TQ-0001", "--title", "Renamed"}},
		{"label", []string{"update", "TQ-0001", "--add-label", "billing"}},
		{"note", []string{"note", "TQ-0001", "something happened"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cli := newEscapedCLI(t)
			cli.mustRun("add", "Real work")
			cli.mustRun("move", "TQ-0001", "doing")
			cli.mustRun(tc.args...)

			var shown task.Task
			cli.mustRunJSON(&shown, "show", "TQ-0001", "--json")
			if shown.Status != "doing" {
				t.Errorf("status = %q after `tq %s`, want doing left alone", shown.Status, strings.Join(tc.args, " "))
			}
		})
	}
}

// Help and the filters read the same marker the writes are validated against,
// so a project whose queue sits outside the marker's directory sees its own
// vocabulary offered rather than the built-in one.
func TestCLIHelpOffersTheProjectsVocabulary(t *testing.T) {
	tc := newEscapedCLI(t)
	out := tc.mustRun("--help")
	for _, want := range []string{"blocker", "routine"} {
		if !strings.Contains(out, want) {
			t.Errorf("help does not offer %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, tqtest.DecoyName) {
		t.Errorf("help offers the decoy's vocabulary, so the config was re-derived from the task directory:\n%s", out)
	}
}

// A marker tq cannot read must not have a command tell the project its own
// columns do not exist. `tq move` validates against the board the store
// resolved, so it reports the broken file the way every other command does
// (TQ-0087).
func TestCLIMoveReportsABrokenMarkerRatherThanTheBuiltInBoard(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Real work")
	tqtest.WriteConfig(t, tc.root, "version: 99\n")

	for _, args := range [][]string{
		{"move", "TQ-0001", "doing"},
		{"done", "TQ-0001"},
	} {
		tc.reset()
		if code := tc.run(args...); code == exitOK {
			t.Errorf("tq %s = 0, want a failure for a config tq cannot use", strings.Join(args, " "))
		}
		if !strings.Contains(tc.stderr.String(), "newer tq") {
			t.Errorf("tq %s said %q, want it to name the broken file rather than the built-in board",
				strings.Join(args, " "), tc.stderr)
		}
	}
}

// ── A column the project removed (TQ-0088) ──────────────────────
//
// Editing `.taskqueue.yaml` is a documented thing to do, and deleting a column
// from it strands every task still filed there. The queue moves them all to the
// default column, and the command that noticed says which — on stderr, so
// `--json` stdout stays exactly what an agent parses.

// removedColumnBoard is the report's project: three columns, the default first.
const removedColumnBoard = `version: 1
path: .tasks
columns:
  - name: backlog
    display_name: Backlog
    default: true
  - name: review
    display_name: Review
    consider_ready: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

// withoutReviewBoard is the same board after the user deletes `review`.
const withoutReviewBoard = `version: 1
path: .tasks
columns:
  - name: backlog
    display_name: Backlog
    default: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

// strandedCLI is a project with three tasks in a column the config no longer
// declares — the state the report reproduces, reached the way a user reaches
// it: by editing their own file.
func strandedCLI(t *testing.T) *testCLI {
	t.Helper()
	tc := newBareCLI(t)
	tqtest.WriteConfig(t, tc.root, removedColumnBoard)
	tc.mustRun("init")
	for _, title := range []string{"One", "Two", "Three"} {
		tc.mustRun("add", title, "--status", "review")
	}
	tqtest.WriteConfig(t, tc.root, withoutReviewBoard)
	return tc
}

// statusesOnDisk is what the files themselves say, keyed by ID. Every defect
// here was a difference between a file and what some surface showed for it, so
// this is the only reading that settles one.
func statusesOnDisk(t *testing.T, tc *testCLI) map[string]string {
	t.Helper()
	dir := filepath.Join(tc.root, config.TaskDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "TQ-") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// TQ-0001-a-slug.md: the ID is the first two dash-separated fields.
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), ".md"), "-", 3)
		if len(parts) < 2 {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			if rest, found := strings.CutPrefix(line, "status: "); found {
				out[parts[0]+"-"+parts[1]] = rest
				break
			}
		}
	}
	return out
}

func TestCLIListReconcilesARemovedColumnAndSaysWhatMoved(t *testing.T) {
	tc := strandedCLI(t)

	out := tc.mustRun("list")
	for _, id := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		if !strings.Contains(out, id+"  backlog") {
			t.Errorf("tq list does not show %s in backlog:\n%s", id, out)
		}
	}
	// The listing is of the files, not a rendering over them.
	for id, status := range statusesOnDisk(t, tc) {
		if status != "backlog" {
			t.Errorf("%s is %q on disk, want backlog: a listing must not show a status the file does not hold", id, status)
		}
	}

	warning := tc.stderr.String()
	if !strings.Contains(warning, "review") || !strings.Contains(warning, "backlog") {
		t.Errorf("stderr = %q, want it to name the column that went and the one the tasks moved to", warning)
	}
	for _, id := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		if !strings.Contains(warning, id) {
			t.Errorf("stderr = %q, want it to name %s", warning, id)
		}
	}
	// One line for the three of them, not three.
	if got := strings.Count(strings.TrimSpace(warning), "\n"); got != 0 {
		t.Errorf("stderr is %d lines, want the tasks that came out of one column gathered into one:\n%s", got+1, warning)
	}

	// Settled now: a second listing has nothing to say.
	tc.mustRun("list")
	if tc.stderr.Len() != 0 {
		t.Errorf("stderr = %q on a settled queue, want nothing", tc.stderr)
	}
}

// The warning is a warning: the tasks are still the answer and the exit code
// stays 0, and on stdout there is nothing but the JSON an agent parses.
func TestCLIListJSONStaysPureJSONWhileReconciling(t *testing.T) {
	tc := strandedCLI(t)

	var tasks []task.Task
	tc.mustRunJSON(&tasks, "list", "--json")
	if len(tasks) != 3 {
		t.Fatalf("tq list --json returned %d tasks, want 3", len(tasks))
	}
	for _, tk := range tasks {
		if tk.Status != "backlog" {
			t.Errorf("%s = %q, want backlog", tk.ID, tk.Status)
		}
	}
	if tc.stderr.Len() == 0 {
		t.Error("stderr is empty, want the reconciliation reported somewhere")
	}
}

// The report's second half: one unrelated edit used to rewrite the status of
// the single task it touched and leave the rest of the queue where it was.
func TestCLIUpdateDoesNotChangeAStatusByItself(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Work")
	tc.mustRun("move", "TQ-0001", "in-progress")

	tc.mustRun("update", "TQ-0001", "--assignee", "bob")
	if got := statusesOnDisk(t, tc)["TQ-0001"]; got != "in-progress" {
		t.Errorf("status on disk = %q after `tq update --assignee`, want in-progress", got)
	}
	tc.mustRun("note", "TQ-0001", "still going")
	if got := statusesOnDisk(t, tc)["TQ-0001"]; got != "in-progress" {
		t.Errorf("status on disk = %q after `tq note`, want in-progress", got)
	}
}

// Whichever command gets there first moves the whole queue. A single
// `tq update --assignee` on a stranded project migrates all three tasks, not
// the one it was pointed at.
func TestCLIAnUnrelatedEditMigratesTheWholeQueueOrNoneOfIt(t *testing.T) {
	tc := strandedCLI(t)

	tc.mustRun("update", "TQ-0001", "--assignee", "bob")
	for id, status := range statusesOnDisk(t, tc) {
		if status != "backlog" {
			t.Errorf("%s is %q on disk, want backlog: the queue must never be half-migrated", id, status)
		}
	}
	if !strings.Contains(tc.stderr.String(), "TQ-0003") {
		t.Errorf("stderr = %q, want the tasks the edit moved named", tc.stderr)
	}
}

// Editing the board means running `tq init` again, and this is what makes that
// advice true rather than a formality.
func TestCLIInitReconcilesARemovedColumn(t *testing.T) {
	tc := strandedCLI(t)

	out := tc.mustRun("init")
	if !strings.Contains(out, "already initialized") {
		t.Errorf("init output = %q, want the second run to report the existing queue", out)
	}
	for id, status := range statusesOnDisk(t, tc) {
		if status != "backlog" {
			t.Errorf("%s is %q on disk, want tq init to have moved it to the default column", id, status)
		}
	}
	if !strings.Contains(tc.stderr.String(), "review") {
		t.Errorf("stderr = %q, want tq init to name the column that went", tc.stderr)
	}
}

// The board's default column is what a stranded task is filed under, whatever
// order the columns are listed in. On a board whose first column is the one
// that satisfies dependencies, the first-column answer would have marked every
// stranded task done.
func TestCLIReconciliationUsesTheDefaultColumnNotTheFirst(t *testing.T) {
	tc := newBareCLI(t)
	tqtest.WriteConfig(t, tc.root, `version: 1
path: .tasks
columns:
  - name: shipped
    display_name: Shipped
    consider_done: true
  - name: review
    display_name: Review
    consider_ready: true
  - name: backlog
    display_name: Backlog
    default: true
`)
	tc.mustRun("init")
	tc.mustRun("add", "Work", "--status", "review")
	tqtest.WriteConfig(t, tc.root, `version: 1
path: .tasks
columns:
  - name: shipped
    display_name: Shipped
    consider_done: true
  - name: backlog
    display_name: Backlog
    default: true
`)

	tc.mustRun("list")
	if got := statusesOnDisk(t, tc)["TQ-0001"]; got != "backlog" {
		t.Errorf("status on disk = %q, want backlog: the default column, not the first one", got)
	}
}

// A queue that cannot be written still lists, and says what it could not move.
// A read-only checkout — CI, a container volume, a root-owned .tasks — must not
// become a queue nobody can read because a column was removed from its config.
func TestCLIListStillWorksWhenTheReconciliationCannotWrite(t *testing.T) {
	tc := strandedCLI(t)
	dir := filepath.Join(tc.root, config.TaskDirName)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if f, err := os.CreateTemp(dir, "probe-*"); err == nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		t.Skip("this filesystem let a write through a read-only directory")
	}

	out := tc.mustRun("list")
	for _, id := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		// The column their files still hold, which is what a listing promises.
		if !strings.Contains(out, id+"  review") {
			t.Errorf("tq list does not show %s in review:\n%s", id, out)
		}
		if !strings.Contains(tc.stderr.String(), id) {
			t.Errorf("stderr = %q, want it to name %s among the tasks it could not move", tc.stderr, id)
		}
	}
	if !strings.Contains(tc.stderr.String(), "could not move") {
		t.Errorf("stderr = %q, want it to say the migration did not happen", tc.stderr)
	}
}
