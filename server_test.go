package taskqueue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*httptest.Server, *Store) {
	t.Helper()
	store := newTestStore(t)
	srv := httptest.NewServer(newAPIRouter(store))
	t.Cleanup(srv.Close)
	return srv, store
}

func do(t *testing.T, srv *httptest.Server, method, path, body string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, string(payload)
}

func decode[T any](t *testing.T, payload string) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		t.Fatalf("invalid JSON response: %v\n%s", err, payload)
	}
	return v
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func expectError(t *testing.T, resp *http.Response, payload string, status int, code string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Errorf("status = %d, want %d (body: %s)", resp.StatusCode, status, payload)
	}
	got := decode[apiError](t, payload)
	if got.Code != code {
		t.Errorf("error code = %q, want %q (body: %s)", got.Code, code, payload)
	}
	if got.Error == "" {
		t.Errorf("error message should not be empty: %s", payload)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}
}

func TestAPIListTasks(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "Add task API", Priority: PriorityHigh, Labels: []string{"backend"}, Assignee: "agent-api"})
	mustCreate(t, store, CreateTaskInput{Title: "Build board", Labels: []string{"frontend"}, DependsOn: []string{"TQ-0001"}})

	resp, payload := do(t, srv, "GET", "/api/tasks", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	tasks := decode[[]Task](t, payload)
	if len(tasks) != 2 {
		t.Fatalf("got %d tasks, want 2", len(tasks))
	}

	for query, want := range map[string][]string{
		"?status=todo":        {"TQ-0001", "TQ-0002"},
		"?priority=high":      {"TQ-0001"},
		"?label=frontend":     {"TQ-0002"},
		"?assignee=agent-api": {"TQ-0001"},
		"?ready=true":         {"TQ-0001"},
		"?status=done":        {},
	} {
		_, payload := do(t, srv, "GET", "/api/tasks"+query, "")
		tasks := decode[[]Task](t, payload)
		var ids []string
		for _, task := range tasks {
			ids = append(ids, task.ID)
		}
		if strings.Join(ids, ",") != strings.Join(want, ",") {
			t.Errorf("GET /api/tasks%s = %v, want %v", query, ids, want)
		}
	}

	resp, payload = do(t, srv, "GET", "/api/tasks?status=nope", "")
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIReadyFilterValues(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "Unblocked"})
	mustCreate(t, store, CreateTaskInput{Title: "Blocked", DependsOn: []string{"TQ-0404"}})

	for _, value := range []string{"true", "1"} {
		_, payload := do(t, srv, "GET", "/api/tasks?ready="+value, "")
		if tasks := decode[[]Task](t, payload); len(tasks) != 1 || tasks[0].ID != "TQ-0001" {
			t.Errorf("?ready=%s returned %+v, want only TQ-0001", value, tasks)
		}
	}

	_, payload := do(t, srv, "GET", "/api/tasks?ready=false", "")
	if tasks := decode[[]Task](t, payload); len(tasks) != 2 {
		t.Errorf("?ready=false should not filter, got %d tasks", len(tasks))
	}

	// An unparsable value must fail rather than quietly return everything.
	resp, payload := do(t, srv, "GET", "/api/tasks?ready=maybe", "")
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIFilesystemFailuresAreServerErrors(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "Gone in a moment"})
	if err := os.RemoveAll(store.Dir); err != nil {
		t.Fatal(err)
	}

	// A vanished task directory is our problem, not a malformed request.
	resp, payload := do(t, srv, "GET", "/api/tasks", "")
	expectError(t, resp, payload, http.StatusInternalServerError, "internal_error")

	resp, payload = do(t, srv, "POST", "/api/tasks", `{"title": "Cannot be written"}`)
	expectError(t, resp, payload, http.StatusInternalServerError, "internal_error")
}

func TestAPICreateTask(t *testing.T) {
	srv, store := newTestServer(t)

	resp, payload := do(t, srv, "POST", "/api/tasks", `{
		"title": "Implement authentication",
		"priority": "high",
		"labels": ["backend", "auth"],
		"assignee": "agent-auth",
		"depends_on": [],
		"body": "Use the existing OIDC provider."
	}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", resp.StatusCode, payload)
	}
	task := decode[Task](t, payload)
	if task.ID != "TQ-0001" || task.Title != "Implement authentication" || task.Priority != PriorityHigh {
		t.Errorf("task = %+v", task)
	}
	if loc := resp.Header.Get("Location"); loc != "/api/tasks/TQ-0001" {
		t.Errorf("Location = %q", loc)
	}

	stored, err := store.Get("TQ-0001")
	if err != nil {
		t.Fatalf("task not persisted: %v", err)
	}
	if stored.Body != "Use the existing OIDC provider." {
		t.Errorf("stored body = %q", stored.Body)
	}

	resp, payload = do(t, srv, "POST", "/api/tasks", `{"title":`)
	expectError(t, resp, payload, http.StatusBadRequest, "invalid_json")

	resp, payload = do(t, srv, "POST", "/api/tasks", `{"nope": true}`)
	expectError(t, resp, payload, http.StatusBadRequest, "invalid_json")

	resp, payload = do(t, srv, "POST", "/api/tasks", `{"title": ""}`)
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")

	resp, payload = do(t, srv, "POST", "/api/tasks", `{"title": "x", "priority": "whenever"}`)
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIGetTask(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "Findable"})

	resp, payload := do(t, srv, "GET", "/api/tasks/TQ-0001", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if task := decode[Task](t, payload); task.Title != "Findable" {
		t.Errorf("task = %+v", task)
	}

	resp, payload = do(t, srv, "GET", "/api/tasks/TQ-4242", "")
	expectError(t, resp, payload, http.StatusNotFound, "task_not_found")

	resp, payload = do(t, srv, "GET", "/api/tasks/not-an-id", "")
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIPatchTask(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "Drag me", Body: "Body."})

	resp, payload := do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"status": "in-progress"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.StatusCode, payload)
	}
	if task := decode[Task](t, payload); task.Status != StatusInProgress {
		t.Errorf("task = %+v", task)
	}
	if stored, _ := store.Get("TQ-0001"); stored.Status != StatusInProgress || stored.Body != "Body." {
		t.Errorf("stored = %+v", stored)
	}

	// A partial update leaves the other fields alone.
	_, payload = do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"labels": ["ui"], "body": "Edited body."}`)
	task := decode[Task](t, payload)
	if strings.Join(task.Labels, ",") != "ui" || task.Body != "Edited body." || task.Status != StatusInProgress {
		t.Errorf("task = %+v", task)
	}

	resp, payload = do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"status": "shipped"}`)
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")

	resp, payload = do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{}`)
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")

	resp, payload = do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"id": "TQ-9999"}`)
	expectError(t, resp, payload, http.StatusBadRequest, "invalid_json")

	resp, payload = do(t, srv, "PATCH", "/api/tasks/TQ-4242", `{"status": "done"}`)
	expectError(t, resp, payload, http.StatusNotFound, "task_not_found")
}

func TestAPIAddNote(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "Note me", Body: "Description."})

	resp, payload := do(t, srv, "POST", "/api/tasks/TQ-0001/notes", `{"text": "API implemented; tests still failing."}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.StatusCode, payload)
	}
	task := decode[Task](t, payload)
	if !strings.Contains(task.Body, "## Notes") || !strings.Contains(task.Body, "API implemented") {
		t.Errorf("body = %q", task.Body)
	}

	stored, _ := store.Get("TQ-0001")
	if stored.Body != task.Body {
		t.Errorf("stored body differs from the response:\n%q\n%q", stored.Body, task.Body)
	}

	resp, payload = do(t, srv, "POST", "/api/tasks/TQ-0001/notes", `{"text": "   "}`)
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")

	resp, payload = do(t, srv, "POST", "/api/tasks/TQ-4242/notes", `{"text": "hello"}`)
	expectError(t, resp, payload, http.StatusNotFound, "task_not_found")
}

func TestAPIStatusAndVersion(t *testing.T) {
	srv, store := newTestServer(t)
	mustCreate(t, store, CreateTaskInput{Title: "One"})

	_, payload := do(t, srv, "GET", "/api/status", "")
	status := decode[struct {
		OK        bool   `json:"ok"`
		TaskCount int    `json:"task_count"`
		TaskDir   string `json:"task_dir"`
		Version   string `json:"version"`
	}](t, payload)
	if !status.OK || status.TaskCount != 1 || status.TaskDir != store.Dir || status.Version != version {
		t.Errorf("status = %+v", status)
	}

	_, payload = do(t, srv, "GET", "/api/version", "")
	if got := decode[map[string]string](t, payload)["version"]; got != version {
		t.Errorf("version = %q, want %q", got, version)
	}
}

func TestAPIUnknownRouteReturnsJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, payload := do(t, srv, "GET", "/api/nope", "")
	expectError(t, resp, payload, http.StatusNotFound, "not_found")
}

func TestAPIMalformedTaskFile(t *testing.T) {
	srv, store := newTestServer(t)
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-0001-broken.md"), []byte("no frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, payload := do(t, srv, "GET", "/api/tasks", "")
	expectError(t, resp, payload, http.StatusInternalServerError, "invalid_task_file")
	if !strings.Contains(decode[apiError](t, payload).Error, "TQ-0001-broken.md") {
		t.Errorf("the error should name the offending file: %s", payload)
	}
}

func TestRouterServesEmbeddedFrontend(t *testing.T) {
	store := newTestStore(t)
	handler, err := newRouter(store, false)
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	for path, want := range map[string]string{
		"/":          "<title>tq",
		"/app.js":    "/api/tasks",
		"/style.css": ".board",
	} {
		resp, payload := do(t, srv, "GET", path, "")
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, resp.StatusCode)
			continue
		}
		if !strings.Contains(payload, want) {
			t.Errorf("GET %s should contain %q", path, want)
		}
	}

	// The API still wins over the frontend handler.
	resp, _ := do(t, srv, "GET", "/api/tasks", "")
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("GET /api/tasks Content-Type = %q, want JSON", ct)
	}
}

// The HTTP API and the CLI must produce the same Markdown, since they share the
// same store.
func TestHTTPAndCLIProduceTheSameFile(t *testing.T) {
	viaCLI := newTestCLI(t)
	viaCLI.mustRun("add", "Implement REST API", "--priority", "high", "--label", "backend")
	viaCLI.mustRun("move", "TQ-0001", "in-progress")
	cliFile, err := os.ReadFile(filepath.Join(viaCLI.root, TaskDirName, "TQ-0001-implement-rest-api.md"))
	if err != nil {
		t.Fatal(err)
	}

	srv, store := newTestServer(t)
	do(t, srv, "POST", "/api/tasks", `{"title": "Implement REST API", "priority": "high", "labels": ["backend"]}`)
	do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"status": "in-progress"}`)
	httpFile, err := os.ReadFile(filepath.Join(store.Dir, "TQ-0001-implement-rest-api.md"))
	if err != nil {
		t.Fatal(err)
	}

	stripTimestamps := func(content []byte) string {
		var kept []string
		for _, line := range strings.Split(string(content), "\n") {
			if strings.HasPrefix(line, "created:") || strings.HasPrefix(line, "updated:") {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}
	if stripTimestamps(cliFile) != stripTimestamps(httpFile) {
		t.Errorf("CLI and HTTP produced different files:\n%s\n---\n%s", cliFile, httpFile)
	}
}

// The HTTP surface reaches the same store, so it must refuse the same title.
func TestAPIRejectsAMultiLineTitle(t *testing.T) {
	srv, store := newTestServer(t)
	created := mustCreate(t, store, CreateTaskInput{Title: "Fix the parser"})

	body := strings.NewReader(`{"title":"line1\n---\nline2"}`)
	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/tasks/"+created.ID, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	after, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("the task should still be readable: %v", err)
	}
	if after.Title != "Fix the parser" {
		t.Errorf("Title = %q, want it unchanged", after.Title)
	}
}

// The board reads the project's configuration through the API, the same file
// the CLI reads, resolved the same way.
func TestAPIConfig(t *testing.T) {
	srv, store := newTestServer(t)

	resp, err := srv.Client().Get(srv.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var got struct {
		Version int    `json:"version"`
		Path    string `json:"path"`
		TaskDir string `json:"task_dir"`
		File    string `json:"file"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Version != ConfigVersion {
		t.Errorf("version = %d, want %d", got.Version, ConfigVersion)
	}
	if got.TaskDir != store.Dir {
		t.Errorf("task_dir = %q, want %q", got.TaskDir, store.Dir)
	}
}
