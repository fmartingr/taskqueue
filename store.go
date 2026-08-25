package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TaskDirName is the directory, relative to the project root, that holds one
// Markdown file per task. It is meant to be committed to Git.
const TaskDirName = ".tasks"

// EnvTaskDir overrides task directory discovery, which is useful for
// automation and tests: TQ_DIR=/repo/.tasks tq list
const EnvTaskDir = "TQ_DIR"

var (
	ErrTaskNotFound    = errors.New("task not found")
	ErrProjectNotFound = errors.New("no " + TaskDirName + " directory found")
)

// Store owns every filesystem interaction. Both the CLI and the HTTP server go
// through it, so there is exactly one implementation of validation, ID
// allocation and serialization.
//
// Reads always hit the disk: a task created by the CLI is visible to a running
// server on its next request, with no cache to invalidate.
type Store struct {
	Dir string
}

// InitStore creates the task directory: the TQ_DIR override when it is set,
// otherwise root/.tasks. Honouring the override keeps `tq init` and every other
// command pointing at the same place.
func InitStore(root string) (*Store, error) {
	dir := filepath.Join(root, TaskDirName)
	if override := os.Getenv(EnvTaskDir); override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return nil, err
		}
		dir = abs
	}
	if _, err := os.Stat(dir); err == nil {
		return nil, fmt.Errorf("%s already exists", dir)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// OpenStore discovers the task directory starting at startDir.
func OpenStore(startDir string) (*Store, error) {
	dir, err := DiscoverTaskDir(startDir)
	if err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

// DiscoverTaskDir returns the task directory to use: the TQ_DIR override when
// set, otherwise the nearest .tasks directory at or above startDir. Walking up
// lets an agent run tq from any subdirectory of a repository.
func DiscoverTaskDir(startDir string) (string, error) {
	if override := os.Getenv(EnvTaskDir); override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("%s=%s: %w", EnvTaskDir, override, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s=%s is not a directory", EnvTaskDir, override)
		}
		return abs, nil
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, TaskDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // filesystem root
			return "", fmt.Errorf("%w (looked in %s and every parent directory; run `tq init` first)", ErrProjectNotFound, startDir)
		}
		dir = parent
	}
}

// CreateTaskInput carries the fields a caller may set when creating a task.
// Everything else (ID, timestamps) is owned by the store.
type CreateTaskInput struct {
	Title     string
	Status    string
	Priority  string
	Assignee  string
	Labels    []string
	DependsOn []string
	Body      string
}

func (s *Store) path(id string) string {
	return filepath.Join(s.Dir, id+".md")
}

// List returns every task in the directory in the default order: status,
// priority, creation time, ID.
func (s *Store) List() ([]Task, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}

	tasks := make([]Task, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		id, ok := strings.CutSuffix(name, ".md")
		if !ok || !ValidID(id) {
			continue
		}
		task, err := s.read(id)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	SortTasks(tasks)
	return tasks, nil
}

// Get returns a single task by ID.
func (s *Store) Get(id string) (Task, error) {
	if !ValidID(id) {
		return Task{}, fmt.Errorf("invalid task id %q (must match TQ-<number>)", id)
	}
	return s.read(id)
}

func (s *Store) read(id string) (Task, error) {
	name := id + ".md"
	data, err := os.ReadFile(filepath.Join(s.Dir, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Task{}, fmt.Errorf("%w: %s", ErrTaskNotFound, id)
		}
		return Task{}, err
	}
	task, err := ParseTask(name, data)
	if err != nil {
		return Task{}, err
	}
	if task.ID != id {
		return Task{}, fmt.Errorf("%w: %s: id %q does not match the filename", ErrInvalidTaskFile, name, task.ID)
	}
	return task, nil
}

// Create allocates the next ID and writes a new task file.
func (s *Store) Create(in CreateTaskInput) (Task, error) {
	id, err := s.NextID()
	if err != nil {
		return Task{}, err
	}

	now := time.Now().Truncate(time.Second)
	task := Task{
		ID:        id,
		Title:     strings.TrimSpace(in.Title),
		Status:    orDefault(in.Status, StatusTodo),
		Priority:  orDefault(in.Priority, PriorityNormal),
		Assignee:  in.Assignee,
		Labels:    normalizeList(in.Labels),
		DependsOn: normalizeList(in.DependsOn),
		Created:   now,
		Updated:   now,
		Body:      strings.Trim(in.Body, "\n"),
	}
	if err := task.Validate(); err != nil {
		return Task{}, err
	}
	if err := s.write(task); err != nil {
		return Task{}, err
	}
	return task, nil
}

// Update rewrites an existing task and refreshes its updated timestamp. The
// returned task is exactly what was written to disk.
func (s *Store) Update(task Task) (Task, error) {
	if _, err := s.Get(task.ID); err != nil {
		return Task{}, err
	}
	task.Title = strings.TrimSpace(task.Title)
	task.Labels = normalizeList(task.Labels)
	task.DependsOn = normalizeList(task.DependsOn)
	task.Body = strings.Trim(task.Body, "\n")
	task.Updated = time.Now().Truncate(time.Second)

	if err := task.Validate(); err != nil {
		return Task{}, err
	}
	if err := s.write(task); err != nil {
		return Task{}, err
	}
	return task, nil
}

// Delete removes a task file.
func (s *Store) Delete(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid task id %q (must match TQ-<number>)", id)
	}
	if err := os.Remove(s.path(id)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrTaskNotFound, id)
		}
		return err
	}
	return nil
}

// NextID returns the next sequential task ID. Two processes creating a task at
// exactly the same time can race for the same ID; that is a documented PoC
// limitation rather than an oversight.
func (s *Store) NextID() (string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return "", err
	}

	highest := 0
	for _, entry := range entries {
		id, ok := strings.CutSuffix(entry.Name(), ".md")
		if !ok || !ValidID(id) {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(id, "TQ-"))
		if err == nil && n > highest {
			highest = n
		}
	}
	return fmt.Sprintf("TQ-%04d", highest+1), nil
}

// write renders the task and replaces the destination file atomically, so a
// crash mid-write can never leave a half-written task on disk.
func (s *Store) write(task Task) (err error) {
	data, err := RenderTask(task)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(s.Dir, ".tq-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil { // leave nothing behind when the write failed
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	err = os.Rename(tmpName, s.path(task.ID))
	return err
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// normalizeList trims entries and drops empty ones, keeping YAML lists tidy.
func normalizeList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
