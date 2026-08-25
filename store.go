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
	"sync"
	"time"
)

// TaskDirName is the directory, relative to the project root, that holds one
// Markdown file per task. It is meant to be committed to Git.
const TaskDirName = ".tasks"

// EnvTaskDir overrides task directory discovery, which is useful for
// automation and tests: TQ_DIR=/repo/.tasks tq list
const EnvTaskDir = "TQ_DIR"

// EnvWalkForever lifts the bound on the search: set to "true", discovery walks
// past the repository root to the filesystem root, for a queue deliberately
// kept above several repositories.
const EnvWalkForever = "TQ_WALK_FOREVER"

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

	// mu serialises ID allocation. The HTTP server shares one Store across
	// handlers, so without it two requests scan the directory, see the same
	// highest number, and both claim it.
	mu sync.Mutex

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

// ShadowedTaskDir reports a task directory that discovery deliberately walked
// past — one above the enclosing repository. Creating a fresh queue while that
// exists is when a caller's tasks appear to vanish, so the CLI names it.
func ShadowedTaskDir(startDir string) (string, bool) {
	if os.Getenv(EnvTaskDir) != "" || os.Getenv(EnvWalkForever) == "true" {
		return "", false // nothing was excluded: the search was not bounded
	}
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	root, ok := repositoryRoot(abs)
	if !ok {
		return "", false
	}

	for dir := filepath.Dir(root); ; {
		candidate := filepath.Join(dir, TaskDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
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

	// The search stops at the repository root, so a queue above a project
	// cannot capture it — creation has always stopped there (taskDirTarget),
	// and finding one should agree.
	stopAt := ""
	if os.Getenv(EnvWalkForever) != "true" {
		if root, ok := repositoryRoot(dir); ok {
			stopAt = root
		}
	}

	for {
		candidate := filepath.Join(dir, TaskDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if dir == stopAt {
			// Say where the search stopped: the queue the caller means may be
			// one directory further up, and plainly visible to them.
			return "", fmt.Errorf("%w (looked in %s up to the repository root %s; set %s=true to search past it)",
				ErrProjectNotFound, startDir, stopAt, EnvWalkForever)
		}
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
	// Allocating a number and claiming it must happen together, or a
	// concurrent caller allocates the same one. A task sharing its ID with
	// another is unreachable: locate refuses to guess between them.
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Truncate(time.Second)
	task := Task{
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
	// Another process shares no mutex with this one, so the claim can still
	// fail. Take the next free number rather than replacing its file.
	for attempt := 0; attempt < createAttempts; attempt++ {
		id, err := s.NextID()
		if err != nil {
			return Task{}, err
		}
		task.ID = id
		if err := task.ValidateForWrite(); err != nil {
			return Task{}, err
		}
		if _, err := s.writeNew(task); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return Task{}, err
		}
		return task, nil
	}
	return Task{}, fmt.Errorf("could not claim a task ID after %d attempts", createAttempts)
}

// Update rewrites an existing task and refreshes its updated timestamp. The
// returned task is exactly what was written to disk.
// Update saves a task the caller has already read and changed. It is
// last-write-wins by nature: two callers that read the same version both write
// their whole copy. Use Mutate when the change depends on what is there.
func (s *Store) Update(task Task) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.update(task)
}

// Mutate reads a task, applies a change and saves it, holding the lock across
// all three. A read-modify-write split across Get and Update loses everything
// the other caller did; an append like a note loses information nobody can
// reconstruct.
func (s *Store) Mutate(id string, apply func(*Task) error) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, err := s.Get(id)
	if err != nil {
		return Task{}, err
	}
	if err := apply(&task); err != nil {
		return Task{}, err
	}
	return s.update(task)
}

// Patch applies a change to a task and saves it in one step. Adding a label or
// a dependency is an append, so a caller doing this through Get and Update
// loses whatever another caller added in between.
func (s *Store) Patch(id string, patch TaskPatch) (Task, error) {
	return s.Mutate(id, func(task *Task) error {
		*task = ApplyPatch(*task, patch)
		return nil
	})
}

// Note appends a timestamped note to a task and saves it in one step.
func (s *Store) Note(id, text string) (Task, error) {
	return s.Mutate(id, func(task *Task) error {
		task.Body = AppendNote(task.Body, text, time.Now().Truncate(time.Second))
		return nil
	})
}

func (s *Store) update(task Task) (Task, error) {
	current, err := s.locate(task.ID)
	if err != nil {
		return Task{}, err
	}
	task.Title = strings.TrimSpace(task.Title)
	task.Labels = normalizeList(task.Labels)
	task.DependsOn = normalizeList(task.DependsOn)
	task.Body = strings.Trim(task.Body, "\n")
	task.Updated = time.Now().Truncate(time.Second)

	if err := task.ValidateForWrite(); err != nil {
		return Task{}, err
	}

	written, err := s.write(task)
	if err != nil {
		return Task{}, err
	}
	// A retitled task moves to a new filename; the task itself is now safely
	// on disk either way, and only the file it used to live in is left.
	if written != current {
		if err := retireOldFile(filepath.Join(s.Dir, current), filepath.Join(s.Dir, written)); err != nil {
			return Task{}, fmt.Errorf("saved %s but could not retire %s: %w", written, current, err)
		}
	}
	return task, nil
}

// retireOldFile disposes of the file a task used to live in, now that it has
// been written under a new name. Comparing the two names would not do: on a
// case-insensitive or normalizing filesystem they can be one directory entry,
// and removing it would delete the task that was just written. Such an entry
// is renamed into the canonical spelling instead, so a hand-renamed file
// converges like any other stale name.
func retireOldFile(oldPath, newPath string) error {
	// Lstat rather than Stat, to match os.Remove: both act on the directory
	// entry itself, so a symlink is compared as a symlink and a dangling one
	// still gets unlinked.
	oldInfo, err := os.Lstat(oldPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // another writer already removed it
		}
		return err
	}
	newInfo, err := os.Lstat(newPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.Remove(oldPath) // the new file is gone; the old one is stale
	}
	if os.SameFile(oldInfo, newInfo) {
		return os.Rename(oldPath, newPath)
	}
	return os.Remove(oldPath)
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
// createAttempts bounds the retry when another process claims the ID first.
// Each attempt rescans, so the number only rises; this is a guard against a
// pathological loop, not an expected path.
const createAttempts = 10

// stage renders a task into a temporary file beside the tasks and returns its
// path. The caller moves it into place, or removes it.
func (s *Store) stage(task Task) (path string, err error) {
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

	return tmpName, nil
}

// write puts a task on disk, replacing whatever was at its filename.
func (s *Store) write(task Task) (string, error) {
	tmpName, err := s.stage(task)
	if err != nil {
		return "", err
	}
	name := TaskFileName(task)
	if err := os.Rename(tmpName, filepath.Join(s.Dir, name)); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	return name, nil
}

// writeNew is write for a task that must not exist yet. Linking fails when the
// name is taken, where renaming would replace the file — and the file it would
// replace is another task nobody asked to lose.
func (s *Store) writeNew(task Task) (string, error) {
	tmpName, err := s.stage(task)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpName) }()

	name := TaskFileName(task)
	if err := os.Link(tmpName, filepath.Join(s.Dir, name)); err != nil {
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
