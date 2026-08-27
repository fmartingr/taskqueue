package taskqueue

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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
	if code := Main([]string{"init"}); code != 0 {
		t.Fatalf("tq init = %d", code)
	}
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
	// The router owns the event hub's goroutine and ticker.
	t.Cleanup(func() { _ = handler.Close() })
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

// ── The marker is the source of truth (TQ-0087) ─────────────────

// walkFromATaskDir matches a config walk handed a task directory: the mistake
// TQ-0087 was, in the spellings it took.
//
// The marker says where the tasks live; the tasks say nothing about the marker,
// so walking up from a task directory finds another project's marker or none at
// all — and either answer silently replaced the project's board, which is what
// made `tq update --assignee` rewrite a status.
var walkFromATaskDir = regexp.MustCompile(`config\.(?:FindConfig|ConfigPath|ConfigIn|Load)\([^)\n]*(?:\.Dir\b|[Tt]askDir)`)

// theShapeItHad is what the pattern above is aimed at, kept so the pattern
// cannot quietly stop matching anything. A regexp that has been edited into
// matching nothing passes the guard below in perfect silence, which is the one
// way a tripwire fails.
//
// These are five of the six places the mistake was. The sixth, in the CLI, read
// `config.FindConfig(dir)` where dir had been assigned from DiscoverTaskDir a
// line earlier — no pattern over one line reaches that, and it is why the guard
// this test supports is behavioural.
var theShapeItHad = []string{
	`cfg, err := config.FindConfig(s.Dir)`,
	`cfg, err := config.FindConfig(st.Dir)`,
	`cfg, err := config.FindConfig(s.st.Dir)`,
	`path, err := config.ConfigPath(taskDir)`,
	`cfg, err := config.Load(h.st.Dir)`,
}

func TestTheTaskDirectoryGuardStillMatchesTheShapeItIsFor(t *testing.T) {
	for _, line := range theShapeItHad {
		if !walkFromATaskDir.MatchString(line) {
			t.Errorf("the guard no longer matches %q, so it would pass over the mistake it exists for", line)
		}
	}
	// And it is not simply matching everything: a walk from a working
	// directory is what the design asks for.
	for _, line := range []string{
		`cfg, err := config.Optional(config.FindConfig(c.dir))`,
		`marker, err := config.ConfigPath(startDir)`,
		`return config.Load(s.Marker)`,
	} {
		if walkFromATaskDir.MatchString(line) {
			t.Errorf("the guard matches %q, which is the correct shape", line)
		}
	}
}

// This is the tripwire, not the guard. The guard is behavioural and lives in
// the packages themselves: tqtest.EscapedQueue plants a decoy marker above a
// task directory that sits outside its project, and the store, CLI, web and
// integration tests all assert the decoy's board never reaches them. That is
// what would catch a re-derivation through some new helper.
//
// This adds the cheap half: the shape itself, refused in the source. It reads
// the tree because the rule is about which argument a walk is handed, and no
// type distinguishes a task directory from any other string.
func TestNoConfigIsDerivedFromATaskDirectory(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir():
			// The module's own source and nothing else: node_modules holds the
			// frontend's dependencies, and a dot directory is .git, the task
			// queue, or a checkout some tool keeps beside the working tree.
			if name := entry.Name(); name == "node_modules" || (name != "." && strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		case !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		case strings.HasPrefix(filepath.ToSlash(path), "internal/config/"):
			// The walk itself lives here, and its argument is a working
			// directory by definition.
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(source), "\n") {
			if walkFromATaskDir.MatchString(line) {
				t.Errorf("%s:%d re-derives the project config from a task directory:\n\t%s\n"+
					"The marker a queue was resolved through is the only thing that says what the project is; read it through store.Config (TQ-0087).",
					path, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading the source tree: %v", err)
	}
}

// TQ_DIR named a task directory, which is the mistake this whole change is
// about in the shape of an environment variable: it handed a command a queue
// and left it with no project to validate that queue against. TQ_CONFIG_PATH
// names a marker instead, and the old name is gone outright — no alias, the way
// TQ-0085 removed TQ_WALK_FOREVER (TQ-0087).
//
// Assembled rather than written out, so this file does not match itself.
var retiredEnv = "TQ_" + "DIR"

// The scope is what still instructs somebody: the source, the two documents
// that describe discovery, and the guide tq generates. The task files under
// .tasks are a record of what happened and name the old variable throughout,
// which is the point of them.
func TestTheRetiredEnvironmentVariableIsGone(t *testing.T) {
	if !strings.Contains(usageOfTheLiveVariable(t), config.EnvConfigPath) {
		t.Fatalf("this guard is looking for %q against a repository that does not mention %q, so it proves nothing",
			retiredEnv, config.EnvConfigPath)
	}

	check := func(path string) {
		t.Helper()
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for i, line := range strings.Split(string(source), "\n") {
			if strings.Contains(line, retiredEnv) {
				t.Errorf("%s:%d still names %s, which no longer exists:\n\t%s",
					path, i+1, retiredEnv, strings.TrimSpace(line))
			}
		}
	}

	for _, named := range []string{"README.md", "AGENTS.md", filepath.Join(".tasks", "AGENTS.md")} {
		check(named)
	}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir():
			if name := entry.Name(); name == "node_modules" || (name != "." && strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		case path == "crosscut_test.go":
			// This file names it on purpose, one line above.
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".ts", ".vue":
			check(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("reading the source tree: %v", err)
	}
}

// usageOfTheLiveVariable is the generated guide, which is where the replacement
// has to show up for an agent to ever learn about it.
func usageOfTheLiveVariable(t *testing.T) string {
	t.Helper()
	guide, err := os.ReadFile(filepath.Join(".tasks", "AGENTS.md"))
	if err != nil {
		t.Fatalf("reading the generated guide: %v", err)
	}
	return string(guide)
}
