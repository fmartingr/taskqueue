//go:build integration

package integration

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// The stream's ticker is 500ms in a real binary, so these wait in seconds, not
// milliseconds — and long enough that a loaded machine is not a failure.
const withinAFewTicks = 10 * time.Second

// marker rewrites the project's `.taskqueue.yaml`, which is what an editor
// saving it does — and the only way to change a project's configuration, since
// tq never writes this file itself.
func (p *project) marker(t *testing.T, body string) {
	t.Helper()
	if err := os.WriteFile(p.path(".taskqueue.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const (
	plainMarker = "version: 1\npath: .tasks\n"
	spicyMarker = plainMarker + "labels:\n  spicy:\n    color: \"#ff0000\"\n    display_name: Spicy\n"
	// A mapping where a mapping of labels is expected: what half of a save
	// looks like on disk, and what tq refuses to guess at.
	brokenMarker = plainMarker + "labels: [not, a, mapping]\n"
)

// events opens /api/events against a running server and returns the names of
// the frames as they arrive.
//
// A goroutine pumps into a buffered channel rather than each read blocking with
// a deadline: the connection stays open for the whole test, and a frame that
// arrives while nothing is waiting must not be lost.
func (s *server) events(t *testing.T) <-chan string {
	t.Helper()
	resp, err := http.Get(s.URL + "/api/events")
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		t.Fatalf("GET /api/events = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		_ = resp.Body.Close()
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	names := make(chan string, 64)
	go func() {
		defer close(names)
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return // the body was closed, or the server went away
			}
			if name, ok := strings.CutPrefix(strings.TrimSpace(line), "event: "); ok {
				names <- name
			}
		}
	}()
	return names
}

// awaitEvent reads until the named frame arrives, and says what it saw instead.
func awaitEvent(t *testing.T, names <-chan string, want string, within time.Duration) {
	t.Helper()
	var seen []string
	deadline := time.After(within)
	for {
		select {
		case name, ok := <-names:
			if !ok {
				t.Fatalf("the stream ended before a %s frame; saw %v", want, seen)
			}
			if name == want {
				return
			}
			seen = append(seen, name)
		case <-deadline:
			t.Fatalf("no %s frame within %s; saw %v", want, within, seen)
		}
	}
}

// The stream, end to end through the real thing: a real listener, the request
// logger `tq serve` wraps the router in, and a real HTTP client reading frames
// off the wire.
//
// That last part is the reason this test is here rather than only in the
// package's own tests. A frame only arrives before the handler returns if the
// response is actually flushed, and the handler never returns — so if flushing
// broke, every read below would block until the deadline. It has broken
// exactly once, and only behind the logger, which is a wrapper the package's
// tests do not assemble the way the binary does (TQ-0033).
func TestEventsStreamFromARealServer(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	srv := p.serve(t)
	names := srv.events(t)

	// A stream says where it starts, on both counts, so a board that
	// reconnected after missing a change refetches rather than trusting what it
	// already had.
	awaitEvent(t, names, "tasks", withinAFewTicks)
	awaitEvent(t, names, "config", withinAFewTicks)

	// The CLI and the server reading each other's writes, which is what this
	// layer is for: nothing told the server about either of these.
	p.mustRun(t, "add", "written by an agent")
	awaitEvent(t, names, "tasks", withinAFewTicks)

	p.marker(t, spicyMarker)
	awaitEvent(t, names, "config", withinAFewTicks)
}

// The marker is a separate file from the queue, and it is pushed as its own
// kind of change: a board refetches its configuration for one and its listing
// for the other, so an edit to `.taskqueue.yaml` must not arrive as `tasks`.
func TestEditingTheMarkerDoesNotPushATasksFrame(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "already here")
	srv := p.serve(t)
	names := srv.events(t)

	awaitEvent(t, names, "tasks", withinAFewTicks)
	awaitEvent(t, names, "config", withinAFewTicks)

	p.marker(t, spicyMarker)
	awaitEvent(t, names, "config", withinAFewTicks)

	// Nothing in the task directory moved, so nothing more should arrive. A
	// `tasks` frame here would send a board to refetch a listing that cannot
	// have changed.
	select {
	case name := <-names:
		if name == "tasks" {
			t.Error("editing the marker pushed a tasks frame, which sends the board to the wrong endpoint")
		}
	case <-time.After(2 * time.Second):
		// Several ticks with nothing to say, which is the point.
	}
}

// A running server serves what is on disk now, for the marker as much as for a
// task: `tq serve` is not holding a configuration it read at start-up, and no
// restart is needed to pick an edit up.
func TestConfigChangesReachARunningServer(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	srv := p.serve(t)

	var before struct {
		Labels map[string]struct {
			DisplayName string `json:"display_name"`
		} `json:"labels"`
	}
	srv.get(t, "/api/config", &before)
	if _, ok := before.Labels["spicy"]; ok {
		t.Fatal("the fixture already declares spicy")
	}

	p.marker(t, spicyMarker)

	after := before
	srv.get(t, "/api/config", &after)
	if after.Labels["spicy"].DisplayName != "Spicy" {
		t.Errorf("labels = %+v, want the edited marker's own, with no restart", after.Labels)
	}

	// And the CLI, a separate process, reads the same file the same way. Both
	// surfaces going through one store is what keeps them from drifting.
	var listed []struct {
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
	}
	p.mustRun(t, "label", "list", "--json").JSON(t, &listed)
	if len(listed) != 1 || listed[0].Name != "spicy" || listed[0].DisplayName != "Spicy" {
		t.Errorf("tq label list = %+v, want the same vocabulary the server reports", listed)
	}
}

// The half-saved file, at the level only a process can show: the server has to
// survive it. An editor leaves the marker unparsable for a moment on every
// save, so a `tq serve` that fell over — or that held the failure afterwards —
// would break on ordinary use.
func TestAServerSurvivesAMarkerItCannotParse(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "already here")
	srv := p.serve(t)
	names := srv.events(t)
	awaitEvent(t, names, "config", withinAFewTicks)

	p.marker(t, brokenMarker)

	// The stream keeps running and reports the change, so a board hears that
	// something happened rather than going quiet.
	awaitEvent(t, names, "config", withinAFewTicks)

	// What it gets when it refetches: the standard envelope, and a code of its
	// own so a client can tell a broken project from a bad request.
	status, body := srv.request(t, http.MethodGet, "/api/config", "")
	if status != http.StatusInternalServerError {
		t.Fatalf("GET /api/config = %d, want %d\nbody: %s", status, http.StatusInternalServerError, body)
	}
	var failure apiError
	if err := json.Unmarshal([]byte(body), &failure); err != nil {
		t.Fatalf("error body is not JSON: %v\nbody: %s", err, body)
	}
	if failure.Code != "invalid_config" || !strings.Contains(failure.Message, ".taskqueue.yaml") {
		t.Errorf("error = %+v, want invalid_config naming the file", failure)
	}

	// What needs the file fails; what does not, does not. A board can still
	// tell it is talking to a server rather than to nothing.
	if got := srv.status(t, "/api/version"); got != http.StatusOK {
		t.Errorf("GET /api/version = %d while the marker was broken, want %d", got, http.StatusOK)
	}

	// And the editor finishes its save. The same process, never restarted,
	// serves the project again — the failure was in the file, not in the
	// server, and nothing kept a memory of it.
	p.marker(t, spicyMarker)
	awaitEvent(t, names, "config", withinAFewTicks)

	var recovered struct {
		Labels map[string]json.RawMessage `json:"labels"`
	}
	srv.get(t, "/api/config", &recovered)
	if _, ok := recovered.Labels["spicy"]; !ok {
		t.Errorf("labels = %+v after the marker was fixed, want the edited vocabulary", recovered.Labels)
	}
	if got := srv.status(t, "/api/tasks"); got != http.StatusOK {
		t.Errorf("GET /api/tasks = %d after the marker was fixed, want %d", got, http.StatusOK)
	}
}
