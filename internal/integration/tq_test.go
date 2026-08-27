//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The session the guide documents, start to finish, through the real binary.
func TestDocumentedSession(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	p.mustRun(t, "init")
	p.mustRun(t, "add", "Implement the API", "--priority", "high", "--label", "backend")
	p.mustRun(t, "move", "TQ-0001", "todo")

	var ready []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	p.mustRun(t, "ready", "--json").JSON(t, &ready)
	if len(ready) != 1 || ready[0].ID != "TQ-0001" || ready[0].Status != "todo" {
		t.Fatalf("ready = %+v, want one todo task", ready)
	}

	if out := p.mustRun(t, "show", "TQ-0001").Stdout; !strings.Contains(out, "Implement the API") {
		t.Errorf("show = %q", out)
	}

	p.mustRun(t, "move", "TQ-0001", "in-progress")
	p.mustRun(t, "note", "TQ-0001", "handlers done, tests remain")
	p.mustRun(t, "done", "TQ-0001")

	var task struct {
		Status string `json:"status"`
		Body   string `json:"body"`
	}
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &task)
	if task.Status != "done" {
		t.Errorf("status = %q, want done", task.Status)
	}
	if !strings.Contains(task.Body, "handlers done, tests remain") {
		t.Errorf("body lost the note: %q", task.Body)
	}

	// A done task is no longer ready.
	var after []any
	p.mustRun(t, "ready", "--json").JSON(t, &after)
	if len(after) != 0 {
		t.Errorf("ready = %v, want nothing once the task is done", after)
	}
}

// The exit codes are the agent-facing contract, and only the real binary can
// prove them: everything else asserts a return value that never reached os.Exit.
func TestExitCodes(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")

	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"success", []string{"list"}, 0},
		{"validation error", []string{"move", "TQ-0001", "not-a-status"}, 1},
		{"unknown command", []string{"nope"}, 1},
		{"task not found", []string{"show", "TQ-9999"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.run(t, tc.args...).Code; got != tc.want {
				t.Errorf("tq %s = %d, want %d", strings.Join(tc.args, " "), got, tc.want)
			}
		})
	}

	// 3 is "no usable task directory": a marker pointing at a path that cannot
	// be created.
	blocked := newProject(t)
	if err := os.WriteFile(blocked.path("not-a-directory"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocked.path(".taskqueue.yaml"), []byte("version: 1\npath: not-a-directory/sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := blocked.run(t, "list"); r.Code != 3 {
		t.Errorf("uncreatable task directory = %d, want 3\nstderr: %s", r.Code, r.Stderr)
	}
}

// --json promises stdout carries data and nothing else. A note or a warning on
// stdout would break every agent parsing it, and only a real process shows the
// two streams apart.
func TestJSONStdoutIsDataOnly(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	// A successful --json command puts the data on stdout and nothing anywhere
	// else.
	first := p.mustRun(t, "add", "first task", "--json")
	var created map[string]any
	first.JSON(t, &created)
	if created["id"] != "TQ-0001" {
		t.Errorf("id = %v", created["id"])
	}
	if first.Stderr != "" {
		t.Errorf("stderr = %q, want nothing beside the data", first.Stderr)
	}

	// An error goes to stderr too, leaving stdout empty rather than half-written.
	failed := p.run(t, "show", "TQ-9999", "--json")
	if strings.TrimSpace(failed.Stdout) != "" {
		t.Errorf("stdout = %q, want nothing when the command failed", failed.Stdout)
	}
	if !strings.Contains(failed.Stderr, "TQ-9999") {
		t.Errorf("stderr = %q, want it to name the task", failed.Stderr)
	}
}

// The product claim: the CLI and a running board see each other's writes,
// because both read the files every time. Two live surfaces, one queue.
func TestCLIAndRunningServerSeeEachOther(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "from the shell")

	srv := p.serve(t)

	var listed []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	srv.get(t, "/api/tasks", &listed)
	if len(listed) != 1 || listed[0].Title != "from the shell" {
		t.Fatalf("server did not see the CLI's task: %+v", listed)
	}

	// A write through the CLI, while the server is up, is visible next request.
	p.mustRun(t, "add", "second from the shell")
	srv.get(t, "/api/tasks", &listed)
	if len(listed) != 2 {
		t.Errorf("server saw %d tasks, want 2", len(listed))
	}

	// And a write through the board is visible to the next CLI read.
	if code := srv.send(t, "PATCH", "/api/tasks/TQ-0001", `{"status":"in-progress"}`); code != 200 {
		t.Fatalf("PATCH = %d", code)
	}
	var task struct {
		Status string `json:"status"`
	}
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &task)
	if task.Status != "in-progress" {
		t.Errorf("CLI read %q, want the board's in-progress", task.Status)
	}
}

// The binary ships the frontend inside itself; nothing needs to be on disk.
func TestServesTheEmbeddedFrontend(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	srv := p.serve(t)

	for _, path := range []string{"/", "/app.js", "/style.css", "/favicon.png"} {
		if code := srv.status(t, path); code != 200 {
			t.Errorf("GET %s = %d, want 200", path, code)
		}
	}
}

// DEV=1 serves the same files from disk instead, so a frontend edit shows up
// without rebuilding the binary.
func TestDevModeServesFromDisk(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	// The dev path is relative to the working directory, so the project needs
	// the built frontend where the server expects to find it.
	devDir := p.path("internal", "web", "public")
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := "<!-- served from disk -->"
	if err := os.WriteFile(filepath.Join(devDir, "index.html"), []byte(marker), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := p.serve(t, "DEV=1")
	var body strings.Builder
	fetchInto(t, srv, "/", &body)
	if !strings.Contains(body.String(), marker) {
		t.Errorf("DEV=1 served %q, want the file on disk", body.String())
	}
}

// Shutting the server down must leave nothing half-written behind.
func TestShutdownLeavesNoTempFiles(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")

	srv := p.serve(t)
	if code := srv.send(t, "POST", "/api/tasks", `{"title":"from the board"}`); code != 201 {
		t.Fatalf("POST = %d", code)
	}

	if srv.cmd.Process != nil {
		_ = srv.cmd.Process.Kill()
	}
	_ = srv.cmd.Wait()

	entries, err := os.ReadDir(p.path(".tasks"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tq-") || strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("left a temp file behind: %s", e.Name())
		}
	}
	// Two task files and nothing else: the guide is written by tq init, which
	// this test never runs.
	var tasks int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			tasks++
		}
	}
	if tasks != 2 || len(entries) != tasks {
		t.Errorf("task directory holds %d entries, %d of them tasks; want 2 and only those", len(entries), tasks)
	}
}
