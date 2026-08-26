package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/store"
)

// server exposes the same store the CLI uses. Every request reads from disk, so
// tasks created or edited by an agent show up without any synchronization.
type server struct {
	st *store.Store

	// version is reported by /api/version and /api/status. It is passed in
	// rather than read from a package variable, because the build stamps it on
	// the binary and this package is not the binary.
	version string

	// events is what /api/events streams from: one scan of the task directory
	// and the project marker, serving every connected board.
	events *hub
}

// Router is the handler plus the background work behind /api/events. It is an
// http.Handler, so callers use it as one; Close ends the streams and the ticker
// and must be called, or `tq serve` leaves a goroutine behind on shutdown.
type Router struct {
	http.Handler
	events *hub
}

// Close stops the event hub. Safe to call more than once.
func (r *Router) Close() error {
	r.events.stop()
	return nil
}

// newAPIRouter registers the REST API only. The frontend is added separately by
// newRouter so tests can exercise the API without the embedded assets.
func newAPIRouter(st *store.Store, version string, interval time.Duration) *Router {
	events := newHub(st, interval)
	events.start()

	s := &server{st: st, version: version, events: events}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", s.handlePatchTask)
	mux.HandleFunc("POST /api/tasks/{id}/notes", s.handleAddNote)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("GET /api/events", s.handleEvents)

	// Unknown /api/ paths must stay JSON instead of falling through to the
	// frontend handler and returning index.html.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("no such endpoint: %s %s", r.Method, r.URL.Path))
	})

	return &Router{Handler: mux, events: events}
}

// NewRouter is the full handler: REST API plus the Kanban frontend. The caller
// owns the returned Router and must Close it.
func NewRouter(st *store.Store, dev bool, version string) (*Router, error) {
	router := newAPIRouter(st, version, eventInterval)
	frontend, err := frontendHandler(dev)
	if err != nil {
		_ = router.Close()
		return nil, err
	}
	router.Handler.(*http.ServeMux).Handle("/", frontend)
	return router, nil
}

// frontendHandler serves public/ from disk in development (so Bun rebuilds are
// visible without rebuilding Go) and from the embedded copy in production.
func frontendHandler(dev bool) (http.Handler, error) {
	if dev {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			http.FileServer(http.Dir(DevDir)).ServeHTTP(w, r)
		}), nil
	}
	sub, err := fs.Sub(publicFS, embeddedDir)
	if err != nil {
		return nil, fmt.Errorf("embedded frontend: %w", err)
	}
	return http.FileServerFS(sub), nil
}

// ── Handlers ────────────────────────────────────────────────────

func (s *server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := task.Filter{
		Status:   query.Get("status"),
		Priority: query.Get("priority"),
		Label:    query.Get("label"),
		Assignee: query.Get("assignee"),
	}
	// An unparsable ready value is rejected rather than ignored: silently
	// returning every task would look like a successful "ready" query.
	if raw := query.Get("ready"); raw != "" {
		ready, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("invalid ready value %q (want true or false)", raw))
			return
		}
		filter.Ready = ready
	}
	priorities, err := s.st.Priorities()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	columns, err := s.st.Columns()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if err := filter.Validate(priorities, columns); err != nil {
		writeStoreError(w, err)
		return
	}

	// A file the scan could not read is skipped rather than failing the
	// listing, and named by GET /api/status: the response here is an array of
	// tasks and stays one, because that is what the board and every other
	// client parse (TQ-0011).
	listing, err := s.st.List()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task.FilterTasks(listing.Tasks, filter, columns))
}

func (s *server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title     string   `json:"title"`
		Status    string   `json:"status"`
		Priority  string   `json:"priority"`
		Assignee  string   `json:"assignee"`
		Labels    []string `json:"labels"`
		DependsOn []string `json:"depends_on"`
		Body      string   `json:"body"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}

	t, err := s.st.Create(store.CreateTaskInput{
		Title:     in.Title,
		Status:    in.Status,
		Priority:  in.Priority,
		Assignee:  in.Assignee,
		Labels:    in.Labels,
		DependsOn: in.DependsOn,
		Body:      in.Body,
	})
	if err != nil {
		writeStoreError(w, err)
		return
	}

	w.Header().Set("Location", "/api/tasks/"+t.ID)
	writeJSON(w, http.StatusCreated, t)
}

func (s *server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.st.Get(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *server) handlePatchTask(w http.ResponseWriter, r *http.Request) {
	var patch task.TaskPatch
	if !decodeJSON(w, r, &patch) {
		return
	}
	if patch.IsEmpty() {
		writeError(w, http.StatusBadRequest, "validation_error", "no fields to update")
		return
	}

	t, err := s.st.Patch(r.PathValue("id"), patch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *server) handleAddNote(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Text string `json:"text"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	text := strings.TrimSpace(in.Text)
	if text == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "note text cannot be empty")
		return
	}

	t, err := s.st.Note(r.PathValue("id"), text)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

// handleConfig reports the project configuration the board is looking at. The
// effective values are returned whether or not a config file exists, so the
// board never has to know the defaults; file is empty when there is none.
func (s *server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	cfg, err := config.FindConfig(s.st.Dir)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := map[string]any{
		"version":  config.ConfigVersion,
		"path":     config.TaskDirName,
		"task_dir": s.st.Dir,
		"file":     "",
		// The vocabularies are what let the board colour its chips and badges,
		// group the label filter and fill the priority selects. Both read
		// through a nil config, so these are the built-in sets when the project
		// has no file of its own.
		"labels": cfg.LabelSet(),
		// A list, not a map: priorities are ordered, most severe first, and
		// that order is the board's sort and the order of its options.
		"priorities": cfg.PrioritySet(),
		// The board itself, left to right, with the flags that say which
		// columns offer work and which count a dependency as met.
		"columns": cfg.ColumnSet(),
	}
	if cfg != nil {
		out["version"] = cfg.Version
		out["path"] = cfg.Path
		out["file"] = cfg.File
	}
	writeJSON(w, http.StatusOK, out)
}

// handleStatus reports what the server can see of the queue, including the
// files it cannot read. This is where a broken file surfaces: GET /api/tasks is
// an array of tasks with nowhere to put a warning, and the board already asks
// here (TQ-0011).
func (s *server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	listing, err := s.st.List()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	// Always an array, never null: a client can then say "none" without having
	// to tell the two apart.
	unreadable := listing.Unreadable
	if unreadable == nil {
		unreadable = []store.UnreadableFile{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"task_count": len(listing.Tasks),
		"task_dir":   s.st.Dir,
		"version":    s.version,
		"unreadable": unreadable,
	})
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": s.version})
}

// ── Responses ───────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

// writeStoreError maps domain errors onto the documented HTTP contract.
// Anything that is not a recognised domain error and not a filesystem failure
// is a validation error, because that is all the store can otherwise return.
func writeStoreError(w http.ResponseWriter, err error) {
	var pathErr *fs.PathError
	var linkErr *os.LinkError

	switch {
	case errors.Is(err, store.ErrTaskNotFound):
		writeError(w, http.StatusNotFound, "task_not_found", err.Error())
	case errors.Is(err, task.ErrInvalidTaskFile):
		writeError(w, http.StatusInternalServerError, "invalid_task_file", err.Error())
	case errors.Is(err, config.ErrConfig):
		// A marker the server cannot read is a file on this machine, not
		// anything the client sent — the same class as a task file that will
		// not parse, and reported the same way. It happens routinely for half a
		// second while an editor saves .taskqueue.yaml, which is why the board
		// keeps the configuration it already had rather than blanking on this.
		writeError(w, http.StatusInternalServerError, "invalid_config", err.Error())
	case errors.Is(err, store.ErrProjectNotFound),
		errors.As(err, &pathErr), errors.As(err, &linkErr),
		errors.Is(err, os.ErrNotExist), errors.Is(err, os.ErrPermission):
		// A full disk or a read-only task directory is our problem, not the
		// client's: reporting it as 400 would invite a retry loop.
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
	}
}

// decodeJSON reads a JSON request body, rejecting unknown fields so client
// typos fail loudly instead of being silently ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return false
	}
	return true
}

// ── tq serve ────────────────────────────────────────────────────

// requestLogger writes one line per request to stderr, keeping stdout clean.
func RequestLogger(out io.Writer, next http.Handler) http.Handler {
	logger := log.New(out, "", log.LstdFlags)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Printf("%s %s %d", r.Method, r.URL.RequestURI(), recorder.status)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Flush forwards to the writer underneath. Embedding http.ResponseWriter does
// not carry http.Flusher across, so without this the event stream asks whether
// it can flush, is told no, and /api/events fails with "this server cannot
// stream" — but only behind the logger, which is why the API tests, which do
// not wrap the handler, could not see it.
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
