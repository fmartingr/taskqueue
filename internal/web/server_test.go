package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/store"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// testVersion is what the router under test is built with.
const testVersion = "test-version"

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st := tqtest.NewStore(t)
	router := newAPIRouter(st, testVersion, eventInterval)
	srv := httptest.NewServer(router)
	t.Cleanup(func() { _ = router.Close() })
	t.Cleanup(srv.Close)
	return srv, st
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

// serverStatus is GET /api/status as a client sees it, the board included.
type serverStatus struct {
	OK         bool                   `json:"ok"`
	TaskCount  int                    `json:"task_count"`
	TaskDir    string                 `json:"task_dir"`
	Version    string                 `json:"version"`
	Unreadable []store.UnreadableFile `json:"unreadable"`
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
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Add task API", Status: task.StatusTodo, Priority: task.PriorityHigh, Labels: []string{"backend"}, Assignee: "agent-api"})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Build board", Status: task.StatusTodo, Labels: []string{"frontend"}, DependsOn: []string{"TQ-0001"}})

	resp, payload := do(t, srv, "GET", "/api/tasks", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	tasks := decode[[]task.Task](t, payload)
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
		tasks := decode[[]task.Task](t, payload)
		var ids []string
		for _, tk := range tasks {
			ids = append(ids, tk.ID)
		}
		if strings.Join(ids, ",") != strings.Join(want, ",") {
			t.Errorf("GET /api/tasks%s = %v, want %v", query, ids, want)
		}
	}

	resp, payload = do(t, srv, "GET", "/api/tasks?status=nope", "")
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIReadyFilterValues(t *testing.T) {
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Unblocked", Status: task.StatusTodo})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Blocked", Status: task.StatusTodo, DependsOn: []string{"TQ-0404"}})

	for _, value := range []string{"true", "1"} {
		_, payload := do(t, srv, "GET", "/api/tasks?ready="+value, "")
		if tasks := decode[[]task.Task](t, payload); len(tasks) != 1 || tasks[0].ID != "TQ-0001" {
			t.Errorf("?ready=%s returned %+v, want only TQ-0001", value, tasks)
		}
	}

	_, payload := do(t, srv, "GET", "/api/tasks?ready=false", "")
	if tasks := decode[[]task.Task](t, payload); len(tasks) != 2 {
		t.Errorf("?ready=false should not filter, got %d tasks", len(tasks))
	}

	// An unparsable value must fail rather than quietly return everything.
	resp, payload := do(t, srv, "GET", "/api/tasks?ready=maybe", "")
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIFilesystemFailuresAreServerErrors(t *testing.T) {
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Gone in a moment"})
	if err := os.RemoveAll(st.Dir); err != nil {
		t.Fatal(err)
	}

	// A vanished task directory is our problem, not a malformed request.
	resp, payload := do(t, srv, "GET", "/api/tasks", "")
	expectError(t, resp, payload, http.StatusInternalServerError, "internal_error")

	resp, payload = do(t, srv, "POST", "/api/tasks", `{"title": "Cannot be written"}`)
	expectError(t, resp, payload, http.StatusInternalServerError, "internal_error")
}

func TestAPICreateTask(t *testing.T) {
	srv, st := newTestServer(t)

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
	tk := decode[task.Task](t, payload)
	if tk.ID != "TQ-0001" || tk.Title != "Implement authentication" || tk.Priority != task.PriorityHigh {
		t.Errorf("task = %+v", tk)
	}
	if loc := resp.Header.Get("Location"); loc != "/api/tasks/TQ-0001" {
		t.Errorf("Location = %q", loc)
	}

	stored, err := st.Get("TQ-0001")
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
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Findable"})

	resp, payload := do(t, srv, "GET", "/api/tasks/TQ-0001", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if tk := decode[task.Task](t, payload); tk.Title != "Findable" {
		t.Errorf("task = %+v", tk)
	}

	resp, payload = do(t, srv, "GET", "/api/tasks/TQ-4242", "")
	expectError(t, resp, payload, http.StatusNotFound, "task_not_found")

	resp, payload = do(t, srv, "GET", "/api/tasks/not-an-id", "")
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")
}

func TestAPIPatchTask(t *testing.T) {
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Drag me", Body: "Body."})

	resp, payload := do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"status": "in-progress"}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.StatusCode, payload)
	}
	if tk := decode[task.Task](t, payload); tk.Status != task.StatusInProgress {
		t.Errorf("task = %+v", tk)
	}
	if stored, _ := st.Get("TQ-0001"); stored.Status != task.StatusInProgress || stored.Body != "Body." {
		t.Errorf("stored = %+v", stored)
	}

	// A partial update leaves the other fields alone.
	_, payload = do(t, srv, "PATCH", "/api/tasks/TQ-0001", `{"labels": ["ui"], "body": "Edited body."}`)
	tk := decode[task.Task](t, payload)
	if strings.Join(tk.Labels, ",") != "ui" || tk.Body != "Edited body." || tk.Status != task.StatusInProgress {
		t.Errorf("task = %+v", tk)
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
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Note me", Body: "Description."})

	resp, payload := do(t, srv, "POST", "/api/tasks/TQ-0001/notes", `{"text": "API implemented; tests still failing."}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.StatusCode, payload)
	}
	tk := decode[task.Task](t, payload)
	if !strings.Contains(tk.Body, "## Notes") || !strings.Contains(tk.Body, "API implemented") {
		t.Errorf("body = %q", tk.Body)
	}

	stored, _ := st.Get("TQ-0001")
	if stored.Body != tk.Body {
		t.Errorf("stored body differs from the response:\n%q\n%q", stored.Body, tk.Body)
	}

	resp, payload = do(t, srv, "POST", "/api/tasks/TQ-0001/notes", `{"text": "   "}`)
	expectError(t, resp, payload, http.StatusBadRequest, "validation_error")

	resp, payload = do(t, srv, "POST", "/api/tasks/TQ-4242/notes", `{"text": "hello"}`)
	expectError(t, resp, payload, http.StatusNotFound, "task_not_found")
}

func TestAPIStatusAndVersion(t *testing.T) {
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "One"})

	_, payload := do(t, srv, "GET", "/api/status", "")
	status := decode[serverStatus](t, payload)
	if !status.OK || status.TaskCount != 1 || status.TaskDir != st.Dir || status.Version != testVersion {
		t.Errorf("status = %+v", status)
	}
	// An array, never null, so a client can say "none" without telling the two
	// apart.
	if status.Unreadable == nil || len(status.Unreadable) != 0 {
		t.Errorf("unreadable = %#v, want an empty array with nothing broken", status.Unreadable)
	}

	_, payload = do(t, srv, "GET", "/api/version", "")
	if got := decode[map[string]string](t, payload)["version"]; got != testVersion {
		t.Errorf("version = %q, want %q", got, testVersion)
	}
}

func TestAPIUnknownRouteReturnsJSON(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, payload := do(t, srv, "GET", "/api/nope", "")
	expectError(t, resp, payload, http.StatusNotFound, "not_found")
}

// One file the server cannot read costs only itself: the listing still serves
// the healthy tasks, and GET /api/status names what was skipped (TQ-0011).
func TestAPIMalformedTaskFile(t *testing.T) {
	srv, st := newTestServer(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Healthy"})
	if err := os.WriteFile(filepath.Join(st.Dir, "TQ-0002-broken.md"), []byte("no frontmatter"), 0o644); err != nil {
		t.Fatal(err)
	}

	resp, payload := do(t, srv, "GET", "/api/tasks", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, payload)
	}
	// An array, and only an array: the board and every other client parse it as
	// one, so a warning cannot be wrapped around it.
	tasks := decode[[]task.Task](t, payload)
	if len(tasks) != 1 || tasks[0].ID != "TQ-0001" {
		t.Errorf("tasks = %+v, want the healthy one", tasks)
	}

	resp, payload = do(t, srv, "GET", "/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, payload)
	}
	status := decode[serverStatus](t, payload)
	if status.TaskCount != 1 || status.TaskDir != st.Dir {
		t.Errorf("status = %+v, want it to still describe the queue", status)
	}
	if len(status.Unreadable) != 1 || status.Unreadable[0].File != "TQ-0002-broken.md" {
		t.Fatalf("unreadable = %+v, want it to name TQ-0002-broken.md", status.Unreadable)
	}
	if status.Unreadable[0].Reason == "" {
		t.Error("unreadable carries no reason, so nothing can say what to fix")
	}
}

func TestRouterServesEmbeddedFrontend(t *testing.T) {
	st := tqtest.NewStore(t)
	handler, err := NewRouter(st, false, testVersion)
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	// The router owns the event hub's goroutine and ticker.
	t.Cleanup(func() { _ = handler.Close() })
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

// The HTTP surface reaches the same store, so it must refuse the same title.
func TestAPIRejectsAMultiLineTitle(t *testing.T) {
	srv, st := newTestServer(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Fix the parser"})

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
	after, err := st.Get(created.ID)
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
	srv, st := newTestServer(t)

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
	if got.Version != config.ConfigVersion {
		t.Errorf("version = %d, want %d", got.Version, config.ConfigVersion)
	}
	if got.TaskDir != st.Dir {
		t.Errorf("task_dir = %q, want %q", got.TaskDir, st.Dir)
	}
}

// The board draws its chips from the config, so the vocabulary has to reach it.
// A project that has not changed the set — the marker as `tq init` seeds it —
// gets the base set.
func TestAPIConfigCarriesTheLabelSet(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, payload := do(t, srv, http.MethodGet, "/api/config", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decode[struct {
		Labels map[string]config.Label `json:"labels"`
	}](t, payload)

	if len(got.Labels) != len(config.DefaultLabels()) {
		t.Fatalf("labels = %d entries, want the %d defaults", len(got.Labels), len(config.DefaultLabels()))
	}
	backend, ok := got.Labels["component/backend"]
	if !ok {
		t.Fatalf("component/backend missing from %+v", got.Labels)
	}
	if backend.DisplayName != "Backend" || backend.Color == "" {
		t.Errorf("component/backend = %+v, want a display name and a colour", backend)
	}
}

func TestAPIConfigCarriesTheProjectsOwnLabels(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: "+config.TaskDirName+
		"\nlabels:\n  spicy:\n    color: \"#ff0000\"\n    display_name: Spicy\n")
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	router := newAPIRouter(st, testVersion, eventInterval)
	srv := httptest.NewServer(router)
	t.Cleanup(func() { _ = router.Close() })
	t.Cleanup(srv.Close)

	resp, payload := do(t, srv, http.MethodGet, "/api/config", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decode[struct {
		Labels map[string]config.Label `json:"labels"`
	}](t, payload)

	if len(got.Labels) != 1 || got.Labels["spicy"].DisplayName != "Spicy" {
		t.Errorf("labels = %+v, want only the project's own", got.Labels)
	}
}

// ── The project's priority vocabulary ────────────────────────────

const customPriorities = "priorities:\n" +
	"  - {name: p0, color: \"#b60205\", display_name: Critical}\n" +
	"  - {name: p1, color: \"#c2410c\"}\n" +
	"  - {name: p2, color: \"#4b5563\", default: true}\n"

// serverWithPriorities returns a server over a project that declares p0..p2.
func serverWithPriorities(t *testing.T) *httptest.Server {
	t.Helper()
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: "+config.TaskDirName+"\n"+customPriorities)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	router := newAPIRouter(st, testVersion, eventInterval)
	srv := httptest.NewServer(router)
	t.Cleanup(func() { _ = router.Close() })
	t.Cleanup(srv.Close)
	return srv
}

// The board builds its selects and colours its badges from this, so the
// vocabulary has to reach it — as a list, since the order is the ranking.
func TestAPIConfigCarriesThePrioritiesInRankOrder(t *testing.T) {
	srv := serverWithPriorities(t)

	resp, payload := do(t, srv, http.MethodGet, "/api/config", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decode[struct {
		Priorities []config.Priority `json:"priorities"`
	}](t, payload)

	if len(got.Priorities) != 3 {
		t.Fatalf("priorities = %+v, want the project's three", got.Priorities)
	}
	names := make([]string, 0, 3)
	for _, priority := range got.Priorities {
		names = append(names, priority.Name)
	}
	if want := "p0,p1,p2"; strings.Join(names, ",") != want {
		t.Errorf("priorities = %q, want %q (the order the config lists)", strings.Join(names, ","), want)
	}
	if got.Priorities[0].DisplayName != "Critical" || got.Priorities[0].Color != "#b60205" {
		t.Errorf("p0 = %+v, want the configured display name and colour", got.Priorities[0])
	}
	if !got.Priorities[2].Default {
		t.Error("p2 is not marked default, so the board has nothing to preselect")
	}
}

// A project without a config of its own still gets a vocabulary, so the board
// never has to know the built-in set.
func TestAPIConfigCarriesTheBuiltInPriorities(t *testing.T) {
	srv, _ := newTestServer(t)

	_, payload := do(t, srv, http.MethodGet, "/api/config", "")
	got := decode[struct {
		Priorities []config.Priority `json:"priorities"`
	}](t, payload)

	if len(got.Priorities) != len(config.DefaultPriorities()) {
		t.Fatalf("priorities = %d entries, want the %d built-in", len(got.Priorities), len(config.DefaultPriorities()))
	}
	if got.Priorities[0].Name != task.PriorityUrgent {
		t.Errorf("first priority = %q, want %q", got.Priorities[0].Name, task.PriorityUrgent)
	}
}

func TestAPIRejectsAPriorityOutsideTheVocabulary(t *testing.T) {
	srv := serverWithPriorities(t)

	resp, payload := do(t, srv, http.MethodPost, "/api/tasks", `{"title": "Nope", "priority": "urgent"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("POST status = %d, want %d (%s)", resp.StatusCode, http.StatusBadRequest, payload)
	}
	if !strings.Contains(payload, "p0, p1, p2") {
		t.Errorf("POST error = %s, want it to list the valid values", payload)
	}

	resp, _ = do(t, srv, http.MethodPost, "/api/tasks", `{"title": "Fine", "priority": "p0"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	resp, payload = do(t, srv, http.MethodPatch, "/api/tasks/TQ-0001", `{"priority": "high"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("PATCH status = %d, want %d (%s)", resp.StatusCode, http.StatusBadRequest, payload)
	}

	// Filtering on a value the project cannot file is refused rather than
	// answered with an empty list, which would read as an empty queue.
	resp, _ = do(t, srv, http.MethodGet, "/api/tasks?priority=urgent", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("GET ?priority=urgent status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	resp, _ = do(t, srv, http.MethodGet, "/api/tasks?priority=p0", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET ?priority=p0 status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
