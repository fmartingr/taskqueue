package web

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fmartingr/taskqueue/internal/store"
)

// eventInterval is how often the server looks for a change. It is the whole
// latency budget of the feature: a task written in a terminal reaches an open
// board within one tick, so this is the worst case a user ever waits.
//
// Half a second rather than a whole one, because a whole one makes the worst
// case exactly the second the ticket asked to stay well under. The scan it
// doubles is a readdir and a stat per file, and it only runs while a board is
// actually connected.
const eventInterval = 500 * time.Millisecond

// keepAliveInterval keeps an idle stream from being closed by anything between
// the board and the server. A comment frame is the cheapest thing SSE has.
const keepAliveInterval = 25 * time.Second

// taskFingerprint is what the ticker compares one tick to the next: the names,
// sizes and modification times of every file in the task directory, hashed.
//
// Names, sizes and modification times rather than the file contents, because
// these are read twice a second and reading them all would not scale past a
// directory this size. The known hole is a filesystem whose modification times
// are only accurate to the second: a status flipped from `todo` to `done` keeps
// the file exactly as long, so two such writes inside one second would look
// like one. Linux and macOS both keep nanoseconds, so this is a concern for
// exotic mounts; the escalation, if it ever bites, is hashing the contents.
func taskFingerprint(taskDir string) (string, error) {
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return "", err
	}

	sum := fnv.New64a()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// A file that vanished between the listing and the stat is a write
			// in progress, which the next tick will see settled.
			continue
		}
		// ReadDir sorts, so the order this hashes in is already stable.
		fmt.Fprintf(sum, "%s\x00%d\x00%d\n", entry.Name(), info.Size(), info.ModTime().UnixNano())
	}
	return fmt.Sprintf("%016x", sum.Sum64()), nil
}

// configFingerprint is the same reading for the project marker, which decides
// colours, columns and vocabularies and so has to reach the board the way a
// task does — but as its own value, because it means something different: the
// board refetches GET /api/config for it rather than the listing (TQ-0034).
//
// It reads the marker the queue was resolved through, by path. It does not go
// looking for one from the task directory: that walk finds another project's
// marker, or none, whenever `path:` puts the tasks outside the marker's own
// directory (TQ-0087).
//
// It never fails. Every state that marker can be in is a fingerprint: present,
// gone, or unreachable behind a permission error. A board wants to hear about
// all three, and the transitions between them are exactly the changes worth
// pushing — a marker deleted and put back included, since the path is what is
// watched rather than the file.
//
// A store with no marker at all fingerprints as one constant. `tq serve` cannot
// produce one — every queue is resolved through a marker — so this is the hand
// assembled Store a test builds, and there is nothing on disk to watch.
//
// It also deliberately does not parse. A file being saved is briefly invalid,
// and that half-second is the case the whole feature exists for: parsing here
// would make every mid-save state look identical, and the board would not be
// told when the file settled.
func configFingerprint(marker string) string {
	if marker == "" {
		return "no-marker"
	}
	info, err := os.Stat(marker)
	switch {
	case errors.Is(err, os.ErrNotExist):
		// Deleted, or caught mid-write by a saver that unlinks first.
		return "missing\x00" + marker
	case err != nil:
		// Not a fingerprint of the file but of the complaint, which is what
		// changes when the situation does.
		return "unreadable\x00" + err.Error()
	}
	return fmt.Sprintf("%s\x00%d\x00%d", marker, info.Size(), info.ModTime().UnixNano())
}

// event is what a subscriber is handed: the name SSE puts on the frame, and
// the data that goes with it.
type event struct {
	name string
	data string
}

// The three things the hub has to say. `tasks` and `config` are both signals to
// refetch, and they are separate because they send the board to different
// endpoints: the listing, and the project's vocabularies (TQ-0034).
//
// scanFailedEvent is not called "error": EventSource dispatches its own
// connection failures to a listener of that name, so a frame called `error`
// would arrive indistinguishable from the stream dropping.
const (
	tasksEvent      = "tasks"
	configEvent     = "config"
	scanFailedEvent = "scan-failed"
)

// subscriber is one connected board: the frames waiting to go out, and the
// channel that wakes its handler to write them.
//
// Coalesced by name rather than queued, because a frame is a signal to refetch
// and not a record — ten writes between two renders are one refetch. Two
// *different* signals are not interchangeable, though, which is what a single
// buffered slot could not express: a `config` arriving behind a `tasks` has to
// wait its turn rather than be dropped as a repeat.
type subscriber struct {
	wake    chan struct{}
	pending []event
}

// queue adds a frame, replacing one of the same name that has not gone out yet.
func (s *subscriber) queue(e event) {
	for i, waiting := range s.pending {
		if waiting.name == e.name {
			s.pending[i] = e
			return
		}
	}
	s.pending = append(s.pending, e)
}

// hub notices changes and tells the connected boards about them.
//
// One scan serves every board, which is the point: the alternative is each
// browser polling, so a second board doubled the disk reads. Nothing is cached
// between ticks except the fingerprints of the last one — the boards still read
// their tasks and their config through the same REST endpoints and the same
// store as before.
type hub struct {
	st       *store.Store
	interval time.Duration

	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	// The two readings of the last scan, kept apart because they are pushed
	// apart: an edit to the marker must not look like an edit to a task.
	lastTasks  string
	lastConfig string
	lastErr    string
	scans      int

	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newHub(st *store.Store, interval time.Duration) *hub {
	return &hub{
		st:          st,
		interval:    interval,
		subscribers: make(map[*subscriber]struct{}),
		done:        make(chan struct{}),
	}
}

func (h *hub) start() {
	h.wg.Add(1)
	go h.run()
}

// stop ends the ticker and closes every open stream. Safe to call twice, which
// matters because the CLI closes the router and the tests close it again.
func (h *hub) stop() {
	h.stopOnce.Do(func() { close(h.done) })
	h.wg.Wait()
}

// scanCount is how many times the directory has actually been read, for the
// test that nothing happens while nobody is listening.
func (h *hub) scanCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.scans
}

func (h *hub) run() {
	defer h.wg.Done()
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.done:
			h.closeAll()
			return
		case <-ticker.C:
			h.tick()
		}
	}
}

// tick reads the directory and the marker once and pushes whatever moved, as
// its own frame. It does nothing at all while no board is connected: with
// nobody to tell, a scan is pure cost.
func (h *hub) tick() {
	h.mu.Lock()
	idle := len(h.subscribers) == 0
	h.mu.Unlock()
	if idle {
		return
	}

	tasks, err := h.scan()
	cfg := configFingerprint(h.st.Marker)

	h.mu.Lock()
	defer h.mu.Unlock()

	// The config is reported even when the task directory could not be read.
	// The two are separate files: a queue that has gone away says nothing about
	// the marker, and a board is still able to use one without the other.
	if cfg != h.lastConfig {
		h.lastConfig = cfg
		h.broadcastLocked(event{name: configEvent, data: cfg})
	}

	if err != nil {
		// Reported once per spell of failure rather than every tick, so a
		// directory that has gone away does not fill the stream.
		if message := err.Error(); message != h.lastErr {
			h.lastErr = message
			h.broadcastLocked(event{name: scanFailedEvent, data: message})
		}
		return
	}
	h.lastErr = ""
	if tasks != h.lastTasks {
		h.lastTasks = tasks
		h.broadcastLocked(event{name: tasksEvent, data: tasks})
	}
}

func (h *hub) scan() (string, error) {
	h.mu.Lock()
	h.scans++
	h.mu.Unlock()
	return taskFingerprint(h.st.Dir)
}

// subscribe returns a subscriber and the function that ends it. The caller must
// call release, or the hub keeps queueing frames for a board that has gone.
func (h *hub) subscribe() (*subscriber, func()) {
	// The wake channel carries no data and holds one slot: the frames live in
	// the pending set, so a board that is behind is woken once for however many
	// arrived rather than once each. That is what keeps a slow reader from
	// either growing memory or holding up the tick for everybody else.
	sub := &subscriber{wake: make(chan struct{}, 1)}

	// A subscriber arriving after the hub stopped gets a channel that is
	// already closed, so its handler returns at once. Left open it would wait
	// for a broadcast that can never come, and hold the server's shutdown until
	// the timeout expired — which is the stall closing the streams first exists
	// to avoid, reintroduced through the reconnect the board does every 500ms.
	select {
	case <-h.done:
		close(sub.wake)
		return sub, func() {}
	default:
	}

	// Nothing was scanned while nobody was listening, so the first subscriber
	// starts from what is on disk now: without it the next tick compares
	// against a stale reading and looks like a change.
	//
	// Read before taking the lock, and kept only if this really is the first —
	// two boards connecting at once would otherwise race to assign, and the
	// later assignment could install the older reading.
	freshTasks, err := taskFingerprint(h.st.Dir)
	freshConfig := configFingerprint(h.st.Marker)

	h.mu.Lock()
	if len(h.subscribers) == 0 {
		if err == nil {
			h.lastTasks = freshTasks
		}
		h.lastConfig = freshConfig
	}
	h.subscribers[sub] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return sub, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, ok := h.subscribers[sub]; ok {
				delete(h.subscribers, sub)
				close(sub.wake)
			}
			// The next board to connect has not been told about a failure that
			// is still going on, so let it be reported again rather than
			// swallowed as a repeat.
			if len(h.subscribers) == 0 {
				h.lastErr = ""
			}
		})
	}
}

// current is the pair of fingerprints as the hub last read them, for the frames
// a stream opens with.
func (h *hub) current() (tasks, cfg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastTasks, h.lastConfig
}

// drain takes everything waiting for one board, for its handler to write out.
func (h *hub) drain(sub *subscriber) []event {
	h.mu.Lock()
	defer h.mu.Unlock()
	waiting := sub.pending
	sub.pending = nil
	return waiting
}

func (h *hub) broadcastLocked(e event) {
	for sub := range h.subscribers {
		sub.queue(e)
		select {
		case sub.wake <- struct{}{}:
		default:
			// Already awake, and it will take this frame with the one that
			// woke it.
		}
	}
}

func (h *hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subscribers {
		delete(h.subscribers, sub)
		close(sub.wake)
	}
}

// handleEvents streams changes to one board.
//
// Server-sent events rather than a WebSocket: nothing here needs the board to
// say anything back — every write already goes through REST — and SSE is
// `text/event-stream` plus http.Flusher, which is the standard library, where a
// WebSocket would be a dependency or hand-rolled framing.
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "this server cannot stream")
		return
	}

	sub, release := s.events.subscribe()
	defer release()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	// Nothing here goes through a proxy today, but a buffering one would defeat
	// the whole feature, and this is the header that asks it not to.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Say where this stream starts, on both counts. A board that reconnected
	// after missing a change refetches on these rather than trusting what it
	// already had — and the marker can have changed while it was away as
	// easily as a task can.
	tasks, cfg := s.events.current()
	writeEvent(w, flusher, event{name: tasksEvent, data: tasks})
	writeEvent(w, flusher, event{name: configEvent, data: cfg})

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			// The board went away. Returning releases the subscription and
			// this goroutine with it.
			return
		case _, ok := <-sub.wake:
			if !ok {
				return // the hub is shutting down
			}
			for _, e := range s.events.drain(sub) {
				writeEvent(w, flusher, e)
			}
		case <-keepAlive.C:
			// A named event rather than an SSE comment. A comment keeps the
			// connection alive but EventSource discards it without dispatching
			// anything, so the board could not use it to notice a connection
			// that has gone half-open — no error, nothing flowing. This is what
			// its watchdog listens for.
			writeEvent(w, flusher, event{name: "ping", data: "1"})
		}
	}
}

func writeEvent(w http.ResponseWriter, flusher http.Flusher, e event) {
	// A newline in the data would end the field and split the frame in two.
	// scan-failed carries err.Error(), which carries a path, and a directory
	// name is allowed to contain almost anything.
	data := strings.ReplaceAll(strings.ReplaceAll(e.data, "\r", " "), "\n", " ")
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, data)
	flusher.Flush()
}
