//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
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
	p.mustRun(t, "add", "backend work", "--label", "backend", "--priority", "high", "--assignee", "agent-api")
	p.mustRun(t, "add", "frontend work", "--label", "frontend")
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
