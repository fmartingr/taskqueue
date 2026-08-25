package taskqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = "7331"
)

// server exposes the same store the CLI uses. Every request reads from disk, so
// tasks created or edited by an agent show up without any synchronization.
type server struct {
	store *Store
}

// newAPIRouter registers the REST API only. The frontend is added separately by
// newRouter so tests can exercise the API without the embedded assets.
func newAPIRouter(store *Store) *http.ServeMux {
	s := &server{store: store}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/tasks", s.handleListTasks)
	mux.HandleFunc("POST /api/tasks", s.handleCreateTask)
	mux.HandleFunc("GET /api/tasks/{id}", s.handleGetTask)
	mux.HandleFunc("PATCH /api/tasks/{id}", s.handlePatchTask)
	mux.HandleFunc("POST /api/tasks/{id}/notes", s.handleAddNote)
	mux.HandleFunc("GET /api/config", s.handleConfig)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/version", s.handleVersion)

	// Unknown /api/ paths must stay JSON instead of falling through to the
	// frontend handler and returning index.html.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("no such endpoint: %s %s", r.Method, r.URL.Path))
	})

	return mux
}

// newRouter is the full handler: REST API plus the Kanban frontend.
func newRouter(store *Store, dev bool) (http.Handler, error) {
	mux := newAPIRouter(store)
	frontend, err := frontendHandler(dev)
	if err != nil {
		return nil, err
	}
	mux.Handle("/", frontend)
	return mux, nil
}

// frontendHandler serves public/ from disk in development (so Bun rebuilds are
// visible without rebuilding Go) and from the embedded copy in production.
func frontendHandler(dev bool) (http.Handler, error) {
	if dev {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			http.FileServer(http.Dir(publicDirName)).ServeHTTP(w, r)
		}), nil
	}
	sub, err := fs.Sub(publicFS, publicDirName)
	if err != nil {
		return nil, fmt.Errorf("embedded frontend: %w", err)
	}
	return http.FileServerFS(sub), nil
}

// ── Handlers ────────────────────────────────────────────────────

func (s *server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := Filter{
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
	if err := filter.Validate(); err != nil {
		writeStoreError(w, err)
		return
	}

	tasks, err := s.store.List()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, FilterTasks(tasks, filter))
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

	task, err := s.store.Create(CreateTaskInput{
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

	w.Header().Set("Location", "/api/tasks/"+task.ID)
	writeJSON(w, http.StatusCreated, task)
}

func (s *server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	task, err := s.store.Get(r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *server) handlePatchTask(w http.ResponseWriter, r *http.Request) {
	var patch TaskPatch
	if !decodeJSON(w, r, &patch) {
		return
	}
	if patch.IsEmpty() {
		writeError(w, http.StatusBadRequest, "validation_error", "no fields to update")
		return
	}

	task, err := s.store.Patch(r.PathValue("id"), patch)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
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

	task, err := s.store.Note(r.PathValue("id"), text)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// handleConfig reports the project configuration the board is looking at. The
// effective values are returned whether or not a config file exists, so the
// board never has to know the defaults; file is empty when there is none.
func (s *server) handleConfig(w http.ResponseWriter, _ *http.Request) {
	cfg, err := FindConfig(s.store.Dir)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	out := map[string]any{
		"version":  ConfigVersion,
		"path":     TaskDirName,
		"task_dir": s.store.Dir,
		"file":     "",
	}
	if cfg != nil {
		out["version"] = cfg.Version
		out["path"] = cfg.Path
		out["file"] = cfg.File
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	tasks, err := s.store.List()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":         true,
		"task_count": len(tasks),
		"task_dir":   s.store.Dir,
		"version":    version,
	})
}

func (s *server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version})
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
	case errors.Is(err, ErrTaskNotFound):
		writeError(w, http.StatusNotFound, "task_not_found", err.Error())
	case errors.Is(err, ErrInvalidTaskFile):
		writeError(w, http.StatusInternalServerError, "invalid_task_file", err.Error())
	case errors.Is(err, ErrProjectNotFound),
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

func (c *cli) runServe(args []string) int {
	fs := c.flagSet("serve")
	host := fs.String("host", envOr("TQ_HOST", defaultHost), "host to bind to")
	port := fs.String("port", envOr("TQ_PORT", defaultPort), "port to listen on")
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}

	store, err := c.store()
	if err != nil {
		return c.fail(err)
	}

	dev := os.Getenv("DEV") != ""
	handler, err := newRouter(store, dev)
	if err != nil {
		return c.fail(err)
	}

	addr := net.JoinHostPort(*host, *port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           requestLogger(c.stderr, handler),
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return c.fail(err)
	}

	fmt.Fprintf(c.stdout, "Serving %s on http://%s\n", store.Dir, addr)
	if dev {
		fmt.Fprintf(c.stdout, "DEV mode: frontend served from ./%s\n", publicDirName)
	}

	// Ctrl-C should leave no half-served requests behind.
	shutdown := make(chan struct{})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			fmt.Fprintf(c.stderr, "shutdown: %v\n", err)
		}
		close(shutdown)
	}()

	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return c.fail(err)
	}
	<-shutdown
	return exitOK
}

// requestLogger writes one line per request to stderr, keeping stdout clean.
func requestLogger(out io.Writer, next http.Handler) http.Handler {
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

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
