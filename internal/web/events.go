package web

import (
	"fmt"
	"hash/fnv"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fmartingr/taskqueue/internal/config"

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

// fingerprint is what the ticker compares one tick to the next: the names,
// sizes and modification times of everything the board reads, hashed.
//
// The config file is in it as well as the task directory, because the marker
// decides colours, columns and vocabularies, and an edit to it has to reach the
// board the same way an edit to a task does.
//
// Names, sizes and modification times rather than the file contents, because
// these are read twice a second and reading them all would not scale past a
// directory this size. The known hole is a filesystem whose modification times
// are only accurate to the second: a status flipped from `todo` to `done` keeps
// the file exactly as long, so two such writes inside one second would look
// like one. Linux and macOS both keep nanoseconds, so this is a concern for
// exotic mounts; the escalation, if it ever bites, is hashing the contents.
func fingerprint(taskDir string) (string, error) {
	entries, err := os.ReadDir(taskDir)
	if err != nil {
		return "", err
	}

	lines := make([]string, 0, len(entries)+1)
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
		lines = append(lines, fmt.Sprintf("%s\x00%d\x00%d", entry.Name(), info.Size(), info.ModTime().UnixNano()))
	}

	// The config lives above the task directory, so it is stat-ed separately.
	// Its absence is a state too: writing a marker is a change worth pushing.
	if cfg, err := config.FindConfig(taskDir); err == nil && cfg != nil {
		line := cfg.File + "\x00missing"
		if info, err := os.Stat(cfg.File); err == nil {
			line = fmt.Sprintf("%s\x00%d\x00%d", cfg.File, info.Size(), info.ModTime().UnixNano())
		}
		lines = append(lines, line)
	}

	// ReadDir sorts already, but the config is appended after it.
	sort.Strings(lines)

	sum := fnv.New64a()
	for _, line := range lines {
		_, _ = sum.Write([]byte(line))
		_, _ = sum.Write([]byte("\n"))
	}
	return fmt.Sprintf("%016x", sum.Sum64()), nil
}

// event is what a subscriber is handed: the name SSE puts on the frame, and
// the data that goes with it.
type event struct {
	name string
	data string
}

// hub notices changes and tells the connected boards about them.
//
// One scan serves every board, which is the point: the alternative is each
// browser polling, so a second board doubled the disk reads. Nothing is cached
// between ticks except the fingerprint of the last one — the boards still read
// their tasks through the same REST endpoint and the same store as before.
type hub struct {
	st       *store.Store
	interval time.Duration

	mu          sync.Mutex
	subscribers map[chan event]struct{}
	last        string
	lastErr     string
	scans       int

	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newHub(st *store.Store, interval time.Duration) *hub {
	return &hub{
		st:          st,
		interval:    interval,
		subscribers: make(map[chan event]struct{}),
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

// tick reads the directory once and pushes if anything moved. It does nothing
// at all while no board is connected: with nobody to tell, a scan is pure cost.
func (h *hub) tick() {
	h.mu.Lock()
	idle := len(h.subscribers) == 0
	h.mu.Unlock()
	if idle {
		return
	}

	current, err := h.scan()
	h.mu.Lock()
	defer h.mu.Unlock()

	if err != nil {
		// Reported once per spell of failure rather than every tick, so a
		// directory that has gone away does not fill the stream.
		if message := err.Error(); message != h.lastErr {
			h.lastErr = message
			// Not called "error": EventSource dispatches its own connection
			// failures to a listener of that name, so a frame called `error`
			// would arrive indistinguishable from the stream dropping.
			h.broadcastLocked(event{name: "scan-failed", data: message})
		}
		return
	}
	h.lastErr = ""
	if current != h.last {
		h.last = current
		h.broadcastLocked(event{name: "tasks", data: current})
	}
}

func (h *hub) scan() (string, error) {
	h.mu.Lock()
	h.scans++
	h.mu.Unlock()
	return fingerprint(h.st.Dir)
}

// subscribe returns a stream of events and the function that ends it. The
// caller must call release, or the hub keeps writing to a channel nobody reads.
func (h *hub) subscribe() (<-chan event, func()) {
	// Buffered, and the send is non-blocking: an event is a signal to refetch,
	// not a record, so a board that is behind wants the latest one rather than
	// all of them. That is what keeps a slow reader from either growing memory
	// or holding up the tick for everybody else.
	ch := make(chan event, 1)

	// A subscriber arriving after the hub stopped gets a channel that is
	// already closed, so its handler returns at once. Left open it would wait
	// for a broadcast that can never come, and hold the server's shutdown until
	// the timeout expired — which is the stall closing the streams first exists
	// to avoid, reintroduced through the reconnect the board does every 500ms.
	select {
	case <-h.done:
		close(ch)
		return ch, func() {}
	default:
	}

	// Nothing was scanned while nobody was listening, so the first subscriber
	// starts from what is on disk now: without it the next tick compares
	// against a stale reading and looks like a change.
	//
	// Read before taking the lock, and kept only if this really is the first —
	// two boards connecting at once would otherwise race to assign, and the
	// later assignment could install the older reading.
	fresh, err := fingerprint(h.st.Dir)

	h.mu.Lock()
	if len(h.subscribers) == 0 && err == nil {
		h.last = fresh
	}
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if _, ok := h.subscribers[ch]; ok {
				delete(h.subscribers, ch)
				close(ch)
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

// current is the fingerprint as the hub last read it, for the frame a stream
// opens with.
func (h *hub) current() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.last
}

func (h *hub) broadcastLocked(e event) {
	for ch := range h.subscribers {
		select {
		case ch <- e:
		default:
			// One is already waiting. The board refetches everything either
			// way, so the pending signal covers this one too.
		}
	}
}

func (h *hub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subscribers {
		delete(h.subscribers, ch)
		close(ch)
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

	stream, release := s.events.subscribe()
	defer release()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	// Nothing here goes through a proxy today, but a buffering one would defeat
	// the whole feature, and this is the header that asks it not to.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Say where this stream starts. A board that reconnected after missing a
	// change refetches on this rather than trusting what it already had.
	writeEvent(w, flusher, event{name: "tasks", data: s.events.current()})

	keepAlive := time.NewTicker(keepAliveInterval)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			// The board went away. Returning releases the subscription and
			// this goroutine with it.
			return
		case e, ok := <-stream:
			if !ok {
				return // the hub is shutting down
			}
			writeEvent(w, flusher, e)
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
