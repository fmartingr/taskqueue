//go:build integration

package integration

import (
	"net/http"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TQ_HOST and TQ_PORT are the documented defaults behind --host and --port, and
// only a process can show that the environment reached the listener.
func TestServeHostAndPortFromTheEnvironment(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	cmd := exec.Command(binary, "serve")
	cmd.Dir = p.dir
	// Port 0 through the environment: the banner still has to name the real one.
	cmd.Env = append(cmd.Environ(), "TQ_HOST=127.0.0.1", "TQ_PORT=0")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	url := readAddress(t, stdout)
	if !strings.Contains(url, "127.0.0.1:") || strings.HasSuffix(url, ":0") {
		t.Errorf("banner = %q, want the host from the environment and a real port", url)
	}
	resp, err := http.Get(url + "/api/status")
	if err != nil {
		t.Fatalf("the address from the environment is not reachable: %v", err)
	}
	_ = resp.Body.Close()
}

// Requests are logged on stderr, which is the only place that behaviour shows.
func TestServeLogsRequestsToStderr(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	srv := p.serve(t)

	if code := srv.status(t, "/api/tasks"); code != http.StatusOK {
		t.Fatalf("GET = %d", code)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if logged := srv.stderr.String(); strings.Contains(logged, "/api/tasks") {
			if !strings.Contains(logged, "200") {
				t.Errorf("the log line has no status: %q", logged)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("no request logged on stderr within the deadline: %q", srv.stderr.String())
}

// SIGTERM is the signal a supervisor sends. The server has to stop on it and
// exit cleanly, rather than be killed.
func TestServeShutsDownOnSIGTERM(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	cmd := exec.Command(binary, "serve", "--port", "0")
	cmd.Dir = p.dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	url := readAddress(t, stdout)

	// It is serving before the signal.
	resp, err := http.Get(url + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("exited with %v, want a clean shutdown", err)
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("SIGTERM did not stop the server")
	}

	// And it is no longer listening.
	if _, err := http.Get(url + "/api/status"); err == nil {
		t.Error("the server still answers after shutting down")
	}
}

// DEV=1 serves from disk; without it the embedded copy is what arrives, even
// when a different file is sitting where DEV would have looked.
func TestEmbeddedCopyWinsWithoutDev(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	devDir := p.path("internal", "web", "public")
	if err := mkdirAll(devDir); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(devDir+"/index.html", "<!-- from disk -->"); err != nil {
		t.Fatal(err)
	}

	srv := p.serve(t)
	var page strings.Builder
	fetchInto(t, srv, "/", &page)
	if strings.Contains(page.String(), "from disk") {
		t.Error("served the file on disk without DEV=1")
	}
	if !strings.Contains(page.String(), "app.js") {
		t.Errorf("served neither copy: %q", page.String())
	}
}
