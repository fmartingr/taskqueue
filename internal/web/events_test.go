package web

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/fmartingr/taskqueue/internal/store"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// tick is the ticker interval the tests run the hub at. Short enough that a
// test is not mostly waiting, long enough that a loaded machine still gets
// several ticks inside a deadline.
const tick = 20 * time.Millisecond

// eventuallyDifferent is how long a test waits for a change to be noticed.
const eventuallyDifferent = 5 * time.Second

func TestFingerprintFollowsTheTaskDirectory(t *testing.T) {
	st := tqtest.NewStore(t)

	first, err := fingerprint(st.Dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	// Reading twice with nothing changed must not look like a change, or the
	// board would refetch on every tick.
	again, err := fingerprint(st.Dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if first != again {
		t.Errorf("fingerprint changed with nothing to change it: %q then %q", first, again)
	}

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Something"})
	after, err := fingerprint(st.Dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if after == first {
		t.Error("fingerprint did not change when a task was added")
	}
}

// TQ-0034 builds on this stream, so an edit to the marker has to reach the
// board the same way an edit to a task does.
func TestFingerprintCoversTheConfigFile(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	before, err := fingerprint(st.Dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}

	// Written through the same helper, so the modification time really moves.
	time.Sleep(2 * time.Millisecond)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nserver:\n  port: 7412\n")

	after, err := fingerprint(st.Dir)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if after == before {
		t.Errorf("fingerprint did not change when %s was edited", config.ConfigFileName)
	}
}

// newEventServer returns a server whose hub ticks fast, and the router so a
// test can close it.
func newEventServer(t *testing.T) (*httptest.Server, *store.Store, *Router) {
	t.Helper()
	st := tqtest.NewStore(t)
	router := newAPIRouter(st, testVersion, tick)
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { _ = router.Close() })
	return srv, st, router
}

// openStream connects to /api/events and returns a reader over the frames.
func openStream(t *testing.T, srv *httptest.Server) (*bufio.Reader, func()) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	return bufio.NewReader(resp.Body), func() { _ = resp.Body.Close() }
}

// nextEvent reads frames until one names an event, and returns that name.
func nextEvent(t *testing.T, r *bufio.Reader, within time.Duration) string {
	t.Helper()
	type result struct {
		name string
		err  error
	}
	out := make(chan result, 1)
	go func() {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				out <- result{err: err}
				return
			}
			if name, ok := strings.CutPrefix(strings.TrimSpace(line), "event: "); ok {
				out <- result{name: name}
				return
			}
		}
	}()

	select {
	case got := <-out:
		if got.err != nil {
			t.Fatalf("reading the stream: %v", got.err)
		}
		return got.name
	case <-time.After(within):
		t.Fatalf("no event within %s", within)
		return ""
	}
}

// The point of the whole ticket: a task written outside the server reaches an
// open board without the board asking.
func TestEventsPushWhenATaskChanges(t *testing.T) {
	srv, st, _ := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()

	// A stream says where it starts, so a board that reconnected after missing
	// something refetches rather than trusting what it already had.
	if got := nextEvent(t, stream, eventuallyDifferent); got != "tasks" {
		t.Fatalf("first event = %q, want tasks", got)
	}

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Written by an agent"})

	if got := nextEvent(t, stream, eventuallyDifferent); got != "tasks" {
		t.Errorf("event after a write = %q, want tasks", got)
	}
}

// A board that goes away must not leave anything running behind it.
func TestEventsFreeTheirGoroutines(t *testing.T) {
	srv, _, _ := newEventServer(t)

	for i := 0; i < 5; i++ {
		stream, closeStream := openStream(t, srv)
		nextEvent(t, stream, eventuallyDifferent)
		closeStream()
	}

	settled := runtime.NumGoroutine()
	deadline := time.Now().Add(eventuallyDifferent)
	for time.Now().Before(deadline) {
		if settled = runtime.NumGoroutine(); settled < 40 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if settled >= 40 {
		t.Errorf("%d goroutines still running after five connect/disconnect rounds", settled)
	}
}

// Shutting the server down ends the streams rather than leaving browsers
// holding a connection that will never say anything again.
func TestEventsEndWhenTheRouterCloses(t *testing.T) {
	srv, _, router := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()
	nextEvent(t, stream, eventuallyDifferent)

	if err := router.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		for {
			if _, err := stream.ReadString('\n'); err != nil {
				done <- err
				return
			}
		}
	}()
	select {
	case <-done: // the stream ended, which is what Close promises
	case <-time.After(eventuallyDifferent):
		t.Error("the stream was still open after Close")
	}
}

// The hub costs nothing while nobody is listening: with no subscriber it must
// not be reading the task directory on every tick.
func TestHubDoesNotScanWithoutSubscribers(t *testing.T) {
	st := tqtest.NewStore(t)
	h := newHub(st, tick)
	h.start()
	t.Cleanup(h.stop)

	time.Sleep(10 * tick)
	if scans := h.scanCount(); scans != 0 {
		t.Errorf("%d scans with nobody subscribed, want 0", scans)
	}

	// And it starts once somebody is.
	_, release := h.subscribe()
	defer release()
	deadline := time.Now().Add(eventuallyDifferent)
	for time.Now().Before(deadline) && h.scanCount() == 0 {
		time.Sleep(tick)
	}
	if h.scanCount() == 0 {
		t.Error("no scan after a subscriber arrived")
	}
}

// A directory that cannot be read reaches the board as an error rather than
// going quiet, so the existing banner has something to show.
func TestEventsReportAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads an unreadable directory anyway")
	}
	srv, st, _ := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()
	nextEvent(t, stream, eventuallyDifferent)

	if err := os.Chmod(st.Dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(st.Dir, 0o755) })

	if got := nextEvent(t, stream, eventuallyDifferent); got != "scan-failed" {
		t.Errorf("event for an unreadable directory = %q, want scan-failed", got)
	}
}

// The logger wraps the ResponseWriter to record the status, and a wrapper that
// does not carry http.Flusher across makes streaming impossible. The API tests
// do not wrap anything, so this is the layer that would have shipped broken.
func TestEventsStreamThroughTheRequestLogger(t *testing.T) {
	st := tqtest.NewStore(t)
	router := newAPIRouter(st, testVersion, tick)
	t.Cleanup(func() { _ = router.Close() })

	srv := httptest.NewServer(RequestLogger(io.Discard, router))
	t.Cleanup(srv.Close)

	stream, closeStream := openStream(t, srv)
	defer closeStream()

	if got := nextEvent(t, stream, eventuallyDifferent); got != "tasks" {
		t.Fatalf("first event through the logger = %q, want tasks", got)
	}

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Through the logger"})
	if got := nextEvent(t, stream, eventuallyDifferent); got != "tasks" {
		t.Errorf("event after a write = %q, want tasks", got)
	}
}

// A board reconnecting into the shutdown window must not be left holding a
// stream that will never speak: the handler would sit in its select until the
// client gave up, and http.Server.Shutdown waits for handlers, so it would
// stall for the whole timeout. The board retries every 500ms, so this window
// is reached in practice rather than in theory.
func TestSubscribeAfterStopIsAlreadyClosed(t *testing.T) {
	h := newHub(tqtest.NewStore(t), tick)
	h.start()
	h.stop()

	stream, release := h.subscribe()
	defer release()

	select {
	case _, ok := <-stream:
		if ok {
			t.Error("a stream opened after stop delivered an event")
		}
	case <-time.After(time.Second):
		t.Error("a stream opened after stop stayed open, which would hold shutdown until its timeout")
	}
}

// The failure is reported once per spell so it does not fill the stream, but
// "once" must not outlive the boards that heard it: someone who reloads the
// page after the directory broke has been told nothing.
func TestAScanFailureIsReportedAgainToAFreshBoard(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads an unreadable directory anyway")
	}
	srv, st, _ := newEventServer(t)

	first, closeFirst := openStream(t, srv)
	nextEvent(t, first, eventuallyDifferent)
	if err := os.Chmod(st.Dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(st.Dir, 0o755) })
	if got := nextEvent(t, first, eventuallyDifferent); got != "scan-failed" {
		t.Fatalf("first board saw %q, want scan-failed", got)
	}
	closeFirst()

	// A second board, arriving while the directory is still unreadable.
	second, closeSecond := openStream(t, srv)
	defer closeSecond()
	for deadline := time.Now().Add(eventuallyDifferent); time.Now().Before(deadline); {
		if nextEvent(t, second, eventuallyDifferent) == "scan-failed" {
			return
		}
	}
	t.Error("a board that connected after the failure was never told about it")
}

func TestWriteEventKeepsOneFrameOnOneLine(t *testing.T) {
	rec := httptest.NewRecorder()
	writeEvent(rec, rec, event{name: "scan-failed", data: "open /tmp/a\nb/.tasks: permission denied"})

	body := rec.Body.String()
	if strings.Count(body, "\n\n") != 1 || !strings.Contains(body, "open /tmp/a b/.tasks") {
		t.Errorf("frame = %q, want the newline in the data flattened into one frame", body)
	}
}
