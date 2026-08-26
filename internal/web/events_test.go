package web

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

	first, err := taskFingerprint(st.Dir)
	if err != nil {
		t.Fatalf("taskFingerprint: %v", err)
	}
	// Reading twice with nothing changed must not look like a change, or the
	// board would refetch on every tick.
	again, err := taskFingerprint(st.Dir)
	if err != nil {
		t.Fatalf("taskFingerprint: %v", err)
	}
	if first != again {
		t.Errorf("fingerprint changed with nothing to change it: %q then %q", first, again)
	}

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Something"})
	after, err := taskFingerprint(st.Dir)
	if err != nil {
		t.Fatalf("taskFingerprint: %v", err)
	}
	if after == first {
		t.Error("fingerprint did not change when a task was added")
	}
}

// An edit to the marker has to reach the board the way an edit to a task does,
// but as its own reading: the board refetches a different endpoint for it, so
// the two readings must not be able to stand in for each other (TQ-0034).
func TestConfigFingerprintFollowsTheMarkerAlone(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	before := configFingerprint(st.Dir)
	tasksBefore, err := taskFingerprint(st.Dir)
	if err != nil {
		t.Fatalf("taskFingerprint: %v", err)
	}

	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nserver:\n  port: 7412\n")

	if after := configFingerprint(st.Dir); after == before {
		t.Errorf("config fingerprint did not change when %s was edited", config.ConfigFileName)
	}
	tasksAfter, err := taskFingerprint(st.Dir)
	if err != nil {
		t.Fatalf("taskFingerprint: %v", err)
	}
	if tasksAfter != tasksBefore {
		t.Error("editing the marker moved the task fingerprint, which would push a listing nobody changed")
	}

	// And a task must not move the config's reading either.
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Something"})
	if configFingerprint(st.Dir) == before {
		t.Error("the config fingerprint is expected to still differ from before the edit")
	}
}

// A file being saved is briefly unparsable, and that is the moment the board
// most needs to hear about: the reading is a stat, so it moves anyway, and it
// moves again when the file settles.
func TestConfigFingerprintMovesForAFileItCannotParse(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	good := configFingerprint(st.Dir)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nlabels: [not, a, mapping]\n")
	broken := configFingerprint(st.Dir)
	if broken == good {
		t.Fatal("a half-saved marker looks unchanged, so the board would never be told")
	}
	if _, err := config.FindConfig(st.Dir); err == nil {
		t.Fatal("the fixture is supposed to be unparsable")
	}

	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")
	if configFingerprint(st.Dir) == broken {
		t.Error("the marker settling is a change the board has to hear about")
	}
}

// The absence of a marker is a state too: writing one is worth pushing.
func TestConfigFingerprintDistinguishesNoMarkerAtAll(t *testing.T) {
	root := tqtest.Root(t)
	taskDir := filepath.Join(root, ".tasks")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}

	missing := configFingerprint(taskDir)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")
	if configFingerprint(taskDir) == missing {
		t.Error("writing a marker where there was none did not register")
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

// openingFrames consumes the pair a stream starts with, and fails if it is not
// the pair. A board that reconnected after missing a change refetches on these
// rather than trusting what it already had, and it has to be told about both
// the queue and the marker.
func openingFrames(t *testing.T, r *bufio.Reader) {
	t.Helper()
	for _, want := range []string{tasksEvent, configEvent} {
		if got := nextEvent(t, r, eventuallyDifferent); got != want {
			t.Fatalf("opening frame = %q, want %s", got, want)
		}
	}
}

// waitForEvents reads frames until every name asked for has arrived, in any
// order, and says what it did see when one never does. Order is deliberately
// not asserted: two changes in one tick are two frames, and which goes first is
// the hub's business rather than the board's.
func waitForEvents(t *testing.T, r *bufio.Reader, within time.Duration, names ...string) {
	t.Helper()
	missing := make(map[string]struct{}, len(names))
	for _, name := range names {
		missing[name] = struct{}{}
	}

	var seen []string
	deadline := time.Now().Add(within)
	for len(missing) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no %v within %s; saw %v", names, within, seen)
		}
		got := nextEvent(t, r, within)
		seen = append(seen, got)
		delete(missing, got)
	}
}

// configFile is the marker of a store's project, for a test that rewrites it.
func configFile(st *store.Store) string {
	return filepath.Join(filepath.Dir(st.Dir), config.ConfigFileName)
}

// The point of the whole ticket: a task written outside the server reaches an
// open board without the board asking.
func TestEventsPushWhenATaskChanges(t *testing.T) {
	srv, st, _ := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()
	openingFrames(t, stream)

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Written by an agent"})

	if got := nextEvent(t, stream, eventuallyDifferent); got != tasksEvent {
		t.Errorf("event after a write = %q, want tasks", got)
	}
}

// TQ-0034: the marker is pushed as its own kind of change, because the board
// does something different with it — it refetches GET /api/config, not the
// listing — and a `tasks` frame would send it to the wrong endpoint.
func TestEventsPushWhenTheConfigChanges(t *testing.T) {
	srv, st, _ := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()
	openingFrames(t, stream)

	if err := os.WriteFile(configFile(st), []byte("version: 1\npath: .tasks\n"+
		"labels:\n  spicy:\n    color: \"#ff0000\"\n    display_name: Spicy\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := nextEvent(t, stream, eventuallyDifferent); got != configEvent {
		t.Errorf("event after editing the marker = %q, want config", got)
	}
}

// The half-saved file. An editor leaves the marker unparsable for a moment, and
// the board has to be told — then told again when it settles — rather than
// being left showing a vocabulary that no longer exists.
func TestEventsPushAConfigItCannotParseAndTheAPISaysWhy(t *testing.T) {
	srv, st, _ := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()
	openingFrames(t, stream)

	if err := os.WriteFile(configFile(st), []byte("version: 1\npath: .tasks\nlabels: [nope]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForEvents(t, stream, eventuallyDifferent, configEvent)

	// And the endpoint the board refetches on that frame reports the problem in
	// the standard shape, so the board can say what is wrong and keep the
	// configuration it already had.
	resp, payload := do(t, srv, http.MethodGet, "/api/config", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d\n%s", resp.StatusCode, http.StatusInternalServerError, payload)
	}
	failure := decode[struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}](t, payload)
	if failure.Code != "invalid_config" || !strings.Contains(failure.Error, config.ConfigFileName) {
		t.Errorf("error = %+v, want invalid_config naming %s", failure, config.ConfigFileName)
	}

	// Reverting it is a change of its own: the board gets its labels back
	// without anybody reloading the page.
	if err := os.WriteFile(configFile(st), []byte("version: 1\npath: .tasks\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForEvents(t, stream, eventuallyDifferent, configEvent)

	if resp, payload := do(t, srv, http.MethodGet, "/api/config", ""); resp.StatusCode != http.StatusOK {
		t.Errorf("status after the marker was fixed = %d, want %d\n%s", resp.StatusCode, http.StatusOK, payload)
	}
}

// Two changes in the same tick are two frames. They are coalesced by name, not
// replaced by the latest: one signal sends the board to the listing and the
// other to its configuration, so losing either leaves it wrong about something.
func TestEventsKeepBothKindsOfChangeFromOneTick(t *testing.T) {
	srv, st, _ := newEventServer(t)
	stream, closeStream := openStream(t, srv)
	defer closeStream()
	openingFrames(t, stream)

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Written with the marker"})
	if err := os.WriteFile(configFile(st), []byte("version: 1\npath: .tasks\nserver:\n  port: 7412\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	waitForEvents(t, stream, eventuallyDifferent, tasksEvent, configEvent)
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
	openingFrames(t, stream)

	if err := os.Chmod(st.Dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(st.Dir, 0o755) })

	waitForEvents(t, stream, eventuallyDifferent, scanFailedEvent)
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

	openingFrames(t, stream)

	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Through the logger"})
	if got := nextEvent(t, stream, eventuallyDifferent); got != tasksEvent {
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

	sub, release := h.subscribe()
	defer release()

	select {
	case _, ok := <-sub.wake:
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
	openingFrames(t, first)
	if err := os.Chmod(st.Dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(st.Dir, 0o755) })
	waitForEvents(t, first, eventuallyDifferent, scanFailedEvent)
	closeFirst()

	// A second board, arriving while the directory is still unreadable.
	second, closeSecond := openStream(t, srv)
	defer closeSecond()
	waitForEvents(t, second, eventuallyDifferent, scanFailedEvent)
}

func TestWriteEventKeepsOneFrameOnOneLine(t *testing.T) {
	rec := httptest.NewRecorder()
	writeEvent(rec, rec, event{name: scanFailedEvent, data: "open /tmp/a\nb/.tasks: permission denied"})

	body := rec.Body.String()
	if strings.Count(body, "\n\n") != 1 || !strings.Contains(body, "open /tmp/a b/.tasks") {
		t.Errorf("frame = %q, want the newline in the data flattened into one frame", body)
	}
}
