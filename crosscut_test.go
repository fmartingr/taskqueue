package taskqueue

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/config"
	"github.com/fmartingr/taskqueue/internal/tqtest"
	"github.com/fmartingr/taskqueue/internal/web"
)

// The HTTP API and the CLI must produce the same Markdown, since they share the
// same store. This is the one test that spans both surfaces, so it sits above
// them rather than inside either, and it drives them the way a user does:
// through Main and through the router, using nothing unexported. TQ-0038 will
// give it a real harness against the built binary; until then this is the
// safety net the split has.
func TestHTTPAndCLIProduceTheSameFile(t *testing.T) {
	viaCLI := tqtest.Root(t)
	t.Chdir(viaCLI)
	if code := Main([]string{"add", "Implement REST API", "--priority", "high", "--label", "backend"}); code != 0 {
		t.Fatalf("tq add = %d", code)
	}
	if code := Main([]string{"move", "TQ-0001", "in-progress"}); code != 0 {
		t.Fatalf("tq move = %d", code)
	}
	cliFile, err := os.ReadFile(filepath.Join(viaCLI, config.TaskDirName, "TQ-0001-implement-rest-api.md"))
	if err != nil {
		t.Fatal(err)
	}

	st := tqtest.NewStore(t)
	handler, err := web.NewRouter(st, false, "test")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	post(t, srv, http.MethodPost, "/api/tasks", `{"title": "Implement REST API", "priority": "high", "labels": ["backend"]}`)
	post(t, srv, http.MethodPatch, "/api/tasks/TQ-0001", `{"status": "in-progress"}`)
	httpFile, err := os.ReadFile(filepath.Join(st.Dir, "TQ-0001-implement-rest-api.md"))
	if err != nil {
		t.Fatal(err)
	}

	if stripTimestamps(cliFile) != stripTimestamps(httpFile) {
		t.Errorf("CLI and HTTP produced different files:\n%s\n---\n%s", cliFile, httpFile)
	}
}

func post(t *testing.T, srv *httptest.Server, method, path, body string) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		t.Fatalf("%s %s = %d", method, path, resp.StatusCode)
	}
}

// stripTimestamps drops the two fields that cannot match: the files are written
// at different instants.
func stripTimestamps(content []byte) string {
	var kept []string
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "created:") || strings.HasPrefix(line, "updated:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
