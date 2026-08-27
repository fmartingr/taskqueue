//go:build integration

// Package integration drives the compiled tq the way a user or an agent does:
// as a process, with real exit codes, a real stdout/stderr split and a real
// listening server. Everything else in the suite runs inside the process and
// cannot see any of that.
//
// Run with: make test-integration
package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// binary is built once for the whole run and reused by every test.
var binary string

// stampedVersion is what the test build passes to -ldflags, so the version
// command has something to report that the default would not produce.
const stampedVersion = "0.0.0-integration"

func TestMain(m *testing.M) {
	// The environment is neutralised here rather than per test: these are
	// separate processes, so t.Setenv would not reach them, and a developer
	// with TQ_DIR exported must not have the suite operate on their own queue.
	// TQ-0021 and TQ-0023 are this mistake made in the unit tests.
	for _, name := range []string{"TQ_DIR", "TQ_HOST", "TQ_PORT", "DEV"} {
		_ = os.Unsetenv(name)
	}

	dir, err := os.MkdirTemp("", "tq-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration: %v\n", err)
		os.Exit(1)
	}
	binary = filepath.Join(dir, "tq")

	// Built the way `make build` does, so `tq version` reports a stamped string
	// rather than the "dev" default. That contract is only visible here.
	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/fmartingr/taskqueue.version="+stampedVersion,
		"-o", binary, "../../cmd/tq")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: building tq: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// project is a directory the binary can treat as a real project: it carries the
// marker and the task directory the marker names, so discovery resolves inside
// it rather than climbing out into whatever happens to be above the temp
// directory.
//
// Both files, not just the marker: since TQ-0085 no command but `tq init`
// creates a task directory, so a marker on its own is a project every other
// command refuses. They are written directly rather than by running init,
// because a generated AGENTS.md in the queue is a file the tests that count
// what a command left behind would have to know about.
type project struct {
	dir string
}

func newProject(t *testing.T) *project {
	t.Helper()
	dir := bareDir(t)
	if err := writeFile(filepath.Join(dir, ".taskqueue.yaml"), "version: 1\npath: .tasks\n"); err != nil {
		t.Fatal(err)
	}
	if err := mkdirAll(filepath.Join(dir, ".tasks")); err != nil {
		t.Fatal(err)
	}
	return &project{dir: dir}
}

// bareDir is a temporary directory that is not a project, with the premise
// asserted rather than assumed: no marker in it, and none anywhere the walk
// could reach from it. Since TQ-0085 the walk stops at the home directory, so
// with TMPDIR inside a developer's own project a fixture built on t.TempDir
// alone would resolve to their queue (TQ-0023, TQ-0064).
//
// It walks for the marker itself rather than calling into tq: this package
// links none of tq's own code, because what it is here to check is the compiled
// binary's behaviour and not a function it could have called directly.
func bareDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	for at := dir; ; {
		if _, err := os.Stat(filepath.Join(at, ".taskqueue.yaml")); err == nil {
			t.Fatalf("%s sits above the fixture, so this test's premise — a directory with no project above it — does not hold here", at)
		}
		parent := filepath.Dir(at)
		if at == home || parent == at {
			return dir
		}
		at = parent
	}
}

// path joins a path inside the project.
func (p *project) path(elem ...string) string {
	return filepath.Join(append([]string{p.dir}, elem...)...)
}

// realPath is the path the binary reports back for one inside a temporary
// directory. A separate process resolves its own working directory through the
// kernel, and on macOS t.TempDir() hands out a path under /var that really is
// /private/var — so an exact comparison against what tq printed has to resolve
// the fixture's side too. dir must exist; rest need not.
func realPath(t *testing.T, dir string, rest ...string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolving %s: %v", dir, err)
	}
	return filepath.Join(append([]string{resolved}, rest...)...)
}

// result is one run of the binary. The streams stay apart, which is the whole
// point: the --json contract is a claim about which stream carries what.
type result struct {
	Code   int
	Stdout string
	Stderr string
}

// JSON decodes stdout, and says so loudly when stdout was not JSON alone —
// that failure is the contract this layer exists to check.
func (r result) JSON(t *testing.T, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(r.Stdout), target); err != nil {
		t.Fatalf("stdout is not JSON on its own: %v\nstdout: %q\nstderr: %q", err, r.Stdout, r.Stderr)
	}
}

// run executes the binary in the project and returns what a shell would see.
func (p *project) run(t *testing.T, args ...string) result {
	t.Helper()
	return p.runIn(t, p.dir, nil, args...)
}

// runIn executes the binary in a chosen directory, with extra environment.
func (p *project) runIn(t *testing.T, dir string, env []string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	code := 0
	if err := cmd.Run(); err != nil {
		exit := &exec.ExitError{}
		if !errorsAs(err, &exit) {
			t.Fatalf("running tq %s: %v", strings.Join(args, " "), err)
		}
		code = exit.ExitCode()
	}
	return result{Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

// mustRun fails the test when the command did not succeed, and says what the
// binary printed on both streams.
func (p *project) mustRun(t *testing.T, args ...string) result {
	t.Helper()
	r := p.run(t, args...)
	if r.Code != 0 {
		t.Fatalf("tq %s = %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.Code, r.Stdout, r.Stderr)
	}
	return r
}

// server is a running `tq serve`, reachable at URL.
type server struct {
	URL    string
	cmd    *exec.Cmd
	stderr *syncBuffer
}

// serve starts the binary on a port the OS picks, waits until it answers, and
// stops it when the test ends. Port 0 is what keeps these tests parallel-safe.
func (p *project) serve(t *testing.T, env ...string) *server {
	t.Helper()

	cmd := exec.Command(binary, "serve", "--port", "0")
	cmd.Dir = p.dir
	cmd.Env = append(os.Environ(), env...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	errOut := &syncBuffer{}
	cmd.Stderr = errOut

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	s := &server{cmd: cmd, stderr: errOut}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	// The banner carries the address the listener actually got.
	lines := bufio.NewScanner(stdout)
	deadline := time.AfterFunc(10*time.Second, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	defer deadline.Stop()

	for lines.Scan() {
		line := lines.Text()
		_, addr, found := strings.Cut(line, "http://")
		if !found {
			continue
		}
		s.URL = "http://" + strings.TrimSpace(addr)
		break
	}
	if s.URL == "" {
		t.Fatalf("serve printed no address; stderr:\n%s", errOut.String())
	}

	// Drain the rest of stdout so the process never blocks on a full pipe.
	go func() {
		for lines.Scan() {
		}
	}()

	s.waitReady(t)
	return s
}

// waitReady polls until the server answers, and reports its stderr on failure
// rather than a bare timeout.
func (s *server) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(s.URL + "/api/status")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s never became ready; stderr:\n%s", s.URL, s.stderr.String())
}

// get fetches a path and decodes the JSON body.
func (s *server) get(t *testing.T, path string, target any) {
	t.Helper()
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", path, resp.StatusCode)
	}
	if target != nil {
		if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
			t.Fatal(err)
		}
	}
}

// send performs a request with a JSON body and returns the status code.
func (s *server) send(t *testing.T, method, path, body string) int {
	t.Helper()
	req, err := http.NewRequest(method, s.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// status fetches a path and returns only the status code.
func (s *server) status(t *testing.T, path string) int {
	t.Helper()
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// renamer retitles tasks in a loop, in the background, as a separate process.
// Retitling is what moves a task's file: `tq update --title` writes the task
// under its new name and only then retires the old one, so a reader that lists
// the directory and then opens the files can catch a task under neither name,
// or under both (TQ-0012). Nothing but a real second process produces that.
type renamer struct {
	stop chan struct{}
	done chan struct{}
	once sync.Once

	mu       sync.Mutex
	rounds   int
	failures []string
}

// renameTasks starts the loop and stops it when the test ends, whichever way it
// ends: a t.Fatalf inside a test returns without running anything the test
// wrote after it, and a writer left running would go on forking processes at a
// temporary directory that cleanup is about to remove — in a parallel suite.
func (p *project) renameTasks(t *testing.T, ids []string) *renamer {
	t.Helper()
	r := &renamer{stop: make(chan struct{}), done: make(chan struct{})}
	t.Cleanup(r.halt)
	go func() {
		defer close(r.done)
		for round := 0; ; round++ {
			select {
			case <-r.stop:
				return
			default:
			}
			id := ids[round%len(ids)]
			cmd := exec.Command(binary, "update", id, "--title", fmt.Sprintf("%s round %d", id, round))
			cmd.Dir = p.dir
			cmd.Env = os.Environ()
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()

			// Reported rather than fataled: this is not the test's goroutine,
			// so it may not fail the test itself.
			r.mu.Lock()
			r.rounds++
			if err != nil {
				r.failures = append(r.failures, fmt.Sprintf("tq update %s: %v: %s", id, err, stderr.String()))
			}
			r.mu.Unlock()
		}
	}()
	return r
}

// halt ends the loop and waits for the process it may have in flight. Idempotent,
// because both the test and the cleanup call it.
func (r *renamer) halt() {
	r.once.Do(func() { close(r.stop) })
	<-r.done
}

// stopWhenDone ends the loop and fails the test if the writer itself failed or
// never got going — a test that measured a listing against a directory nothing
// was writing to would pass for the wrong reason.
func (r *renamer) stopWhenDone(t *testing.T) int {
	t.Helper()
	r.halt()

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.failures {
		t.Errorf("the rename loop failed: %s", f)
	}
	if r.rounds == 0 {
		t.Error("the rename loop never ran, so nothing was competing with the listing")
	}
	return r.rounds
}

// syncBuffer is a bytes.Buffer safe to read while a process writes to it.
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

// errorsAs is errors.As, kept local so the import list stays about the harness.
func errorsAs(err error, target **exec.ExitError) bool {
	e, ok := err.(*exec.ExitError)
	if ok {
		*target = e
	}
	return ok
}

// fetchInto reads a path's body into w.
func fetchInto(t *testing.T, s *server, path string, w *strings.Builder) {
	t.Helper()
	resp, err := http.Get(s.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	w.Write(body)
}

// readAddress reads the serve banner and returns the base URL it names.
func readAddress(t *testing.T, stdout io.Reader) string {
	t.Helper()
	lines := bufio.NewScanner(stdout)
	for lines.Scan() {
		_, addr, found := strings.Cut(lines.Text(), "http://")
		if found {
			go func() {
				for lines.Scan() {
				}
			}()
			return "http://" + strings.TrimSpace(addr)
		}
	}
	t.Fatal("serve printed no address")
	return ""
}

// mkdirAll and writeFile keep the test files free of os import noise.
func mkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }

func writeFile(path, body string) error { return os.WriteFile(path, []byte(body), 0o644) }
