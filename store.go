package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

	// Created reports that this call made the task directory, so the CLI can
	// say where a queue appeared instead of creating one silently.
	Created bool
}

// InitStore makes sure the task directory exists and returns a store for it:
// the TQ_DIR override when set, otherwise root/.tasks. An existing directory is
// left alone, so running it twice is harmless.
func InitStore(root string) (*Store, error) {
	dir, err := taskDirTarget(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(dir)
	switch {
	case err == nil && info.IsDir():
		return &Store{Dir: dir}, nil
	case err == nil:
		return nil, fmt.Errorf("%s exists and is not a directory", dir)
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: dir, Created: true}, nil
}

// OpenStore returns the store for startDir, creating the task directory when
// there is none: `tq add` in a fresh repository should just work rather than
// stopping to demand `tq init`.
func OpenStore(startDir string) (*Store, error) {
	dir, err := DiscoverTaskDir(startDir)
	switch {
	case err == nil:
		return &Store{Dir: dir}, nil
	case errors.Is(err, ErrProjectNotFound):
		store, createErr := InitStore(startDir)
		if createErr != nil {
			// Still "no usable task directory", which is exit code 3.
			return nil, fmt.Errorf("%w: %v", ErrProjectNotFound, createErr)
		}
		return store, nil
	default:
		return nil, err
	}
}

// taskDirTarget is where a new task directory belongs: TQ_DIR when set,
// otherwise .tasks at the root of the enclosing Git repository, falling back to
// startDir itself. Preferring the repository root keeps an agent working in a
// subdirectory from scattering task directories around the tree.
func taskDirTarget(startDir string) (string, error) {
	if override := os.Getenv(EnvTaskDir); override != "" {
		return filepath.Abs(override)
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	if root, ok := repositoryRoot(dir); ok {
		dir = root
	}
	return filepath.Join(dir, TaskDirName), nil
}

// repositoryRoot returns the nearest directory at or above dir that holds .git.
func repositoryRoot(dir string) (string, bool) {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// DiscoverTaskDir returns the existing task directory to use: the TQ_DIR
// override when set, otherwise the nearest .tasks directory at or above
// startDir. Walking up lets an agent run tq from any subdirectory of a
// repository. It reports ErrProjectNotFound when there is nothing yet, which is
// what makes OpenStore create one.
func DiscoverTaskDir(startDir string) (string, error) {
	if override := os.Getenv(EnvTaskDir); override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return "", fmt.Errorf("%w: %s=%s does not exist yet", ErrProjectNotFound, EnvTaskDir, override)
		case err != nil:
			return "", fmt.Errorf("%s=%s: %w", EnvTaskDir, override, err)
		case !info.IsDir():
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
			return "", fmt.Errorf("%w (looked in %s and every parent directory)", ErrProjectNotFound, startDir)
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

// taskFilePattern matches the two shapes a task file can have: the ID alone,
// or the ID followed by a slug of the title.
var taskFilePattern = regexp.MustCompile(`^(TQ-[0-9]+)(?:-[^/]*)?\.md$`)

// TaskFileName is the file a task belongs in: its ID, suffixed with a slug of
// the title so the directory is browsable and greppable by name. The ID stays
// first, so files sort and glob by ID.
func TaskFileName(task Task) string {
	if slug := Slugify(task.Title); slug != "" {
		return task.ID + "-" + slug + ".md"
	}
	return task.ID + ".md"
}

// taskFileID reports which task a filename holds, ignoring the title suffix.
func taskFileID(name string) (string, bool) {
	match := taskFilePattern.FindStringSubmatch(name)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// locate finds the file holding a task, whatever title suffix it carries.
func (s *Store) locate(id string) (string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return "", err
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if fileID, ok := taskFileID(entry.Name()); ok && fileID == id {
			matches = append(matches, entry.Name())
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%w: %s", ErrTaskNotFound, id)
	case 1:
		return matches[0], nil
	default:
		// Two files claiming one ID is ambiguous: renaming by hand or a
		// half-finished rename can cause it, and guessing would lose an edit.
		sort.Strings(matches)
		return "", fmt.Errorf("%w: %s is claimed by %d files (%s); keep the one you want",
			ErrInvalidTaskFile, id, len(matches), strings.Join(matches, ", "))
	}
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
		if _, ok := taskFileID(name); !ok {
			continue
		}
		task, err := s.readFile(name)
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
	name, err := s.locate(id)
	if err != nil {
		return Task{}, err
	}
	return s.readFile(name)
}

func (s *Store) readFile(name string) (Task, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Task{}, fmt.Errorf("%w: %s", ErrTaskNotFound, name)
		}
		return Task{}, err
	}

	task, err := ParseTask(name, data)
	if err != nil {
		return Task{}, err
	}
	// The ID in the frontmatter is authoritative; a stale title suffix is
	// harmless and gets fixed on the next write.
	if fileID, _ := taskFileID(name); task.ID != fileID {
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
	if _, err := s.write(task); err != nil {
		return Task{}, err
	}
	return task, nil
}

// Update rewrites an existing task and refreshes its updated timestamp. The
// returned task is exactly what was written to disk.
func (s *Store) Update(task Task) (Task, error) {
	current, err := s.locate(task.ID)
	if err != nil {
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

	written, err := s.write(task)
	if err != nil {
		return Task{}, err
	}
	// A retitled task moves to a new filename; drop the old one only once the
	// new file is safely on disk.
	if written != current {
		if err := os.Remove(filepath.Join(s.Dir, current)); err != nil {
			return Task{}, fmt.Errorf("renaming %s to %s: %w", current, written, err)
		}
	}
	return task, nil
}

// Delete removes a task file.
func (s *Store) Delete(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid task id %q (must match TQ-<number>)", id)
	}
	name, err := s.locate(id)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(s.Dir, name))
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
		id, ok := taskFileID(entry.Name())
		if !ok {
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
func (s *Store) write(task Task) (name string, err error) {
	data, err := RenderTask(task)
	if err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(s.Dir, ".tq-*.tmp")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil { // leave nothing behind when the write failed
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		return "", err
	}
	if err = tmp.Chmod(0o644); err != nil {
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}

	name = TaskFileName(task)
	if err = os.Rename(tmpName, filepath.Join(s.Dir, name)); err != nil {
		return "", err
	}
	return name, nil
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
