//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

// apiError is the envelope every failure returns. A board or a script branches
// on these fields, so their shape is a contract.
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

// request performs a call and returns the status and the raw body, so a test
// can assert on either.
func (s *server) request(t *testing.T, method, path, body string) (int, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, s.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, string(raw)
}

// The three read-only endpoints the board uses to orient itself.
func TestAPIReadOnlyEndpoints(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")
	srv := p.serve(t)

	t.Run("one task by id", func(t *testing.T) {
		var got taskJSON
		srv.get(t, "/api/tasks/TQ-0001", &got)
		if got.ID != "TQ-0001" || got.Title != "a task" {
			t.Errorf("task = %+v", got)
		}
	})

	t.Run("status", func(t *testing.T) {
		var status struct {
			OK        bool   `json:"ok"`
			TaskCount int    `json:"task_count"`
			TaskDir   string `json:"task_dir"`
			Version   string `json:"version"`
		}
		srv.get(t, "/api/status", &status)
		if !status.OK || status.TaskCount != 1 {
			t.Errorf("status = %+v", status)
		}
		if !strings.HasSuffix(status.TaskDir, ".tasks") {
			t.Errorf("task_dir = %q", status.TaskDir)
		}
	})

	t.Run("version", func(t *testing.T) {
		var v map[string]string
		srv.get(t, "/api/version", &v)
		if v["version"] == "" {
			t.Error("version is empty")
		}
	})

	t.Run("config", func(t *testing.T) {
		var cfg struct {
			Version int    `json:"version"`
			Path    string `json:"path"`
			TaskDir string `json:"task_dir"`
			File    string `json:"file"`
		}
		srv.get(t, "/api/config", &cfg)
		if cfg.Version != 1 || cfg.Path != ".tasks" {
			t.Errorf("config = %+v", cfg)
		}
		if !strings.HasSuffix(cfg.File, ".taskqueue.yaml") {
			t.Errorf("file = %q, want the marker this project carries", cfg.File)
		}
	})
}

// The CLI writing while the server reads is the workflow this project is for,
// and a retitle moves the file underneath the request. GET /api/tasks used to
// answer 404 for the whole collection (TQ-0011) and then to answer 200 with a
// task missing (TQ-0012); now every response is the whole queue.
func TestAPIListStaysWholeWhileTheCLIRenamesTasks(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	const tasks = 12
	ids := make([]string, 0, tasks)
	for i := 1; i <= tasks; i++ {
		p.mustRun(t, "add", fmt.Sprintf("task number %d", i), "--status", "todo")
		ids = append(ids, fmt.Sprintf("TQ-%04d", i))
	}
	srv := p.serve(t)

	writer := p.renameTasks(t, ids)
	const requests = 60
	for i := 0; i < requests; i++ {
		code, body := srv.request(t, http.MethodGet, "/api/tasks", "")
		if code != http.StatusOK {
			t.Fatalf("GET /api/tasks = %d during a rename: body %s", code, body)
		}
		var listed []taskJSON
		if err := json.Unmarshal([]byte(body), &listed); err != nil {
			t.Fatalf("GET /api/tasks is not an array of tasks: %v\n%s", err, body)
		}
		// Every task, once each: a retitle in flight must cost the board
		// neither a card nor a duplicate of one.
		seen := map[string]int{}
		for _, task := range listed {
			seen[task.ID]++
		}
		for _, id := range ids {
			switch seen[id] {
			case 1:
			case 0:
				t.Fatalf("GET /api/tasks is missing %s (%d of %d tasks)", id, len(listed), tasks)
			default:
				t.Fatalf("GET /api/tasks holds %s %d times (%d of %d tasks)", id, seen[id], len(listed), tasks)
			}
		}
	}
	writer.stopWhenDone(t)
}

// A running server keeps serving the queue around a file it cannot read: the
// listing is still an array of the healthy tasks, and /api/status names what
// was skipped so the board can say so (TQ-0011).
func TestAPISurvivesAnUnreadableFile(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "healthy")
	srv := p.serve(t)

	const broken = "TQ-0002-broken.md"
	if err := os.WriteFile(p.path(".tasks", broken), []byte("<<<<<<< HEAD\nnot a task\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, body := srv.request(t, http.MethodGet, "/api/tasks", "")
	if code != http.StatusOK {
		t.Fatalf("GET /api/tasks = %d, want 200: body %s", code, body)
	}
	// Still an array, and parsed as one: the board reads it that way, so a
	// warning cannot be wrapped around the listing.
	var listed []taskJSON
	if err := json.Unmarshal([]byte(body), &listed); err != nil {
		t.Fatalf("GET /api/tasks is not an array of tasks: %v\n%s", err, body)
	}
	if len(listed) != 1 || listed[0].ID != "TQ-0001" {
		t.Errorf("tasks = %+v, want the healthy one", listed)
	}

	var status struct {
		TaskCount  int `json:"task_count"`
		Unreadable []struct {
			File   string `json:"file"`
			Reason string `json:"reason"`
		} `json:"unreadable"`
	}
	srv.get(t, "/api/status", &status)
	if status.TaskCount != 1 {
		t.Errorf("task_count = %d, want the healthy task still counted", status.TaskCount)
	}
	if len(status.Unreadable) != 1 || status.Unreadable[0].File != broken {
		t.Fatalf("unreadable = %+v, want it to name %s", status.Unreadable, broken)
	}
	if status.Unreadable[0].Reason == "" {
		t.Error("unreadable carries no reason, so the board cannot say what to fix")
	}
}

// An ID two files claim reaches a running server as a task missing from the
// listing, and /api/status is where the board learns why (TQ-0040).
func TestAPIWithholdsAnIDTwoFilesClaim(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "doubled")
	p.mustRun(t, "add", "healthy")
	srv := p.serve(t)

	const stale = "TQ-0001-stale.md"
	content := "---\nid: TQ-0001\ntitle: doubled\nstatus: todo\npriority: normal\n" +
		"created: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n"
	if err := os.WriteFile(p.path(".tasks", stale), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	code, body := srv.request(t, http.MethodGet, "/api/tasks", "")
	if code != http.StatusOK {
		t.Fatalf("GET /api/tasks = %d, want 200: body %s", code, body)
	}
	// Still an array, and parsed as one: the board reads it that way, so a
	// warning cannot be wrapped around the listing.
	var listed []taskJSON
	if err := json.Unmarshal([]byte(body), &listed); err != nil {
		t.Fatalf("GET /api/tasks is not an array of tasks: %v\n%s", err, body)
	}
	if len(listed) != 1 || listed[0].ID != "TQ-0002" {
		t.Errorf("tasks = %+v, want only the healthy one", listed)
	}

	var status struct {
		TaskCount  int `json:"task_count"`
		Duplicated []struct {
			ID     string   `json:"id"`
			Files  []string `json:"files"`
			Reason string   `json:"reason"`
		} `json:"duplicated"`
	}
	srv.get(t, "/api/status", &status)
	if status.TaskCount != 1 {
		t.Errorf("task_count = %d, want only the healthy task counted", status.TaskCount)
	}
	if len(status.Duplicated) != 1 || status.Duplicated[0].ID != "TQ-0001" {
		t.Fatalf("duplicated = %+v, want it to name TQ-0001", status.Duplicated)
	}
	if len(status.Duplicated[0].Files) != 2 || !strings.Contains(status.Duplicated[0].Reason, stale) {
		t.Errorf("duplicated = %+v, want both files named: choosing between them is the fix", status.Duplicated[0])
	}
}

// Notes over HTTP, which the board's detail panel uses.
func TestAPIAddNote(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")
	srv := p.serve(t)

	code, body := srv.request(t, http.MethodPost, "/api/tasks/TQ-0001/notes", `{"text":"from the board"}`)
	if code != http.StatusOK {
		t.Fatalf("POST notes = %d: %s", code, body)
	}

	var got taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &got)
	if !strings.Contains(got.Body, "from the board") {
		t.Errorf("the CLI does not see the note: %q", got.Body)
	}
}

// The error envelope, end to end. Ten writeError calls in the server and none
// was checked through the binary until now.
func TestAPIErrorEnvelope(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")
	srv := p.serve(t)

	for _, tc := range []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"unknown task", http.MethodGet, "/api/tasks/TQ-9999", "", http.StatusNotFound, "task_not_found"},
		{"unknown task on patch", http.MethodPatch, "/api/tasks/TQ-9999", `{"status":"done"}`, http.StatusNotFound, "task_not_found"},
		{"unknown task on notes", http.MethodPost, "/api/tasks/TQ-9999/notes", `{"text":"x"}`, http.StatusNotFound, "task_not_found"},
		{"malformed id", http.MethodGet, "/api/tasks/nope", "", http.StatusBadRequest, "validation_error"},
		{"empty title", http.MethodPost, "/api/tasks", `{"title":"   "}`, http.StatusBadRequest, "validation_error"},
		{"bad status", http.MethodPatch, "/api/tasks/TQ-0001", `{"status":"nope"}`, http.StatusBadRequest, "validation_error"},
		{"empty note", http.MethodPost, "/api/tasks/TQ-0001/notes", `{"text":"  "}`, http.StatusBadRequest, "validation_error"},
		{"patch with no fields", http.MethodPatch, "/api/tasks/TQ-0001", `{}`, http.StatusBadRequest, "validation_error"},
		{"malformed json", http.MethodPost, "/api/tasks", `{"title":`, http.StatusBadRequest, "invalid_json"},
		{"bad ready filter", http.MethodGet, "/api/tasks?ready=maybe", "", http.StatusBadRequest, "validation_error"},
		{"unknown endpoint", http.MethodGet, "/api/nope", "", http.StatusNotFound, "not_found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, body := srv.request(t, tc.method, tc.path, tc.body)
			if status != tc.wantStatus {
				t.Errorf("%s %s = %d, want %d\nbody: %s", tc.method, tc.path, status, tc.wantStatus, body)
			}
			var e apiError
			if err := json.Unmarshal([]byte(body), &e); err != nil {
				t.Fatalf("error body is not JSON: %v\nbody: %s", err, body)
			}
			if e.Code != tc.wantCode {
				t.Errorf("code = %q, want %q\nbody: %s", e.Code, tc.wantCode, body)
			}
			if strings.TrimSpace(e.Message) == "" {
				t.Errorf("message is empty\nbody: %s", body)
			}
		})
	}
}

// The filters the board passes as query parameters.
func TestAPIListFilters(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "backend work", "--status", "todo", "--label", "backend", "--priority", "high", "--assignee", "agent-api")
	p.mustRun(t, "add", "frontend work", "--status", "todo", "--label", "frontend")
	p.mustRun(t, "move", "TQ-0002", "in-progress")
	srv := p.serve(t)

	for _, tc := range []struct {
		query string
		want  int
	}{
		{"", 2},
		{"?label=backend", 1},
		{"?priority=high", 1},
		{"?assignee=agent-api", 1},
		{"?status=in-progress", 1},
		{"?label=backend&priority=high", 1},
		{"?label=backend&priority=low", 0},
		{"?ready=true", 1},
		{"?ready=false", 2},
	} {
		t.Run("filter"+tc.query, func(t *testing.T) {
			var listed []taskJSON
			srv.get(t, "/api/tasks"+tc.query, &listed)
			if len(listed) != tc.want {
				t.Errorf("GET /api/tasks%s = %d tasks, want %d", tc.query, len(listed), tc.want)
			}
		})
	}
}

// A broken build must not pass as a working one: the page has to reference the
// assets that are actually served.
func TestServedIndexReferencesItsAssets(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	srv := p.serve(t)

	var page strings.Builder
	fetchInto(t, srv, "/", &page)
	for _, asset := range []string{"app.js", "style.css", "favicon.png"} {
		if !strings.Contains(page.String(), asset) {
			t.Errorf("the served page does not reference %s", asset)
		}
	}
}
