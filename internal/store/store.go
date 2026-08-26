package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"
)

var (
	ErrTaskNotFound    = errors.New("task not found")
	ErrProjectNotFound = errors.New("no " + config.TaskDirName + " directory found")
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

	// ConfigWritten is the marker this call wrote, when it wrote one. Empty
	// when the project already had a config, which is the common case.
	ConfigWritten string

	// duringScan runs inside a listing's read window — after the directory has
	// been read, before the files are — and is nil everywhere but a test. It
	// is how the race List retries for is driven on purpose: a test that
	// renames a file from another goroutine and hopes to hit that window
	// instead is a test that passes or fails by timing.
	duringScan func()
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
	// A queue without its marker is the ambiguity the marker exists to remove,
	// so making one writes both.
	config, err := config.WriteConfigIfMissing(root, dir)
	if err != nil {
		return nil, err
	}
	return &Store{Dir: dir, Created: true, ConfigWritten: config}, nil
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
	if override := os.Getenv(config.EnvTaskDir); override != "" {
		return filepath.Abs(override)
	}

	cfg, err := config.FindConfig(startDir)
	if err != nil {
		return "", err
	}
	if cfg != nil {
		return cfg.TaskDir(), nil
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	if root, ok := config.RepositoryRoot(dir); ok {
		dir = root
	}
	return filepath.Join(dir, config.TaskDirName), nil
}

// ShadowedProjectMarker reports a project marker that discovery deliberately
// walked past — a .taskqueue.yaml above the enclosing repository, which the
// bounded search will not adopt. Creating a fresh queue while that exists is
// when a caller's tasks appear to vanish, so the CLI names it.
//
// The marker is the whole question, because the marker is what discovery looks
// for (TQ-0029). Looking for a directory named .tasks was wrong in both
// directions: a bare one above is not a queue at all, so the note was a false
// positive, and a real project above whose path is named anything else went
// unreported.
func ShadowedProjectMarker(startDir string) (string, bool) {
	if os.Getenv(config.EnvTaskDir) != "" || os.Getenv(config.EnvWalkForever) == "true" {
		return "", false // nothing was excluded: the search was not bounded
	}
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	root, ok := config.RepositoryRoot(abs)
	if !ok {
		return "", false
	}

	// Discovery already covered startDir up to root and found nothing, so the
	// shadowed marker is the nearest one above it — and the walk runs to the
	// filesystem root, since that is how far TQ_WALK_FOREVER would have gone.
	for dir := filepath.Dir(root); ; {
		candidate := filepath.Join(dir, config.ConfigFileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
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
	if override := os.Getenv(config.EnvTaskDir); override != "" {
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return "", fmt.Errorf("%w: %s=%s does not exist yet", ErrProjectNotFound, config.EnvTaskDir, override)
		case err != nil:
			return "", fmt.Errorf("%s=%s: %w", config.EnvTaskDir, override, err)
		case !info.IsDir():
			return "", fmt.Errorf("%s=%s is not a directory", config.EnvTaskDir, override)
		}
		return abs, nil
	}

	// The marker decides, when there is one: one file to find, and it says
	// where the tasks live.
	cfg, err := config.FindConfig(startDir)
	if err != nil {
		return "", err
	}
	if cfg != nil {
		declared := cfg.TaskDir()
		if info, err := os.Stat(declared); err == nil && info.IsDir() {
			return declared, nil
		}
		// The project is configured; the directory just is not there yet.
		// Creating it is the caller's business, at the declared location.
		return "", fmt.Errorf("%w (%s says the task directory is %s)", ErrProjectNotFound, cfg.File, declared)
	}

	// No marker, so there is no project here. tq does not go looking for a
	// directory that happens to be called .tasks: guessing at names on the way
	// up is what the marker replaces. The caller creates one, which writes the
	// marker with it.
	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	if stopAt := config.WalkBoundary(abs); stopAt != "" {
		// Say where the search stopped. The project the caller means may be
		// one directory further up and plainly visible to them.
		return "", fmt.Errorf("%w (no %s in %s up to the repository root %s; set %s=true to look past it)",
			ErrProjectNotFound, config.ConfigFileName, startDir, stopAt, config.EnvWalkForever)
	}
	return "", fmt.Errorf("%w (no %s in %s or any parent directory)", ErrProjectNotFound, config.ConfigFileName, startDir)
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
func TaskFileName(t task.Task) string {
	if slug := task.Slugify(t.Title); slug != "" {
		return t.ID + "-" + slug + ".md"
	}
	return t.ID + ".md"
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
	names, err := s.taskFileNames()
	if err != nil {
		return "", err
	}

	var matches []string
	for _, name := range names {
		if fileID, _ := taskFileID(name); fileID == id {
			matches = append(matches, name)
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
		return "", fmt.Errorf("%w: %s", task.ErrInvalidTaskFile, duplicateClaim(id, matches))
	}
}

// duplicateClaim is the one sentence tq says about an ID more than one file
// claims. A write refuses with it and a listing reports it, and those are two
// views of one finding: whoever meets it on the board and then on the command
// line must not be told two different things about the same pair of files
// (TQ-0040).
//
// The files are named in the sorted order the directory is read in, so the
// sentence is the same whichever surface composed it.
func duplicateClaim(id string, files []string) string {
	return fmt.Sprintf("%s is claimed by %d files (%s); keep the one you want",
		id, len(files), strings.Join(files, ", "))
}

// Priorities is the vocabulary this queue is filed under: the values a write
// may use, their ranking, and the default.
//
// It is read from the project's config on every call rather than held on the
// Store. The config sits beside the tasks and is a source of truth the same way
// they are, so an edit to it reaches a running server on its next request —
// exactly as an edit to a task file does, and for the same reason.
func (s *Store) Priorities() (task.Priorities, error) {
	cfg, err := config.FindConfig(s.Dir)
	if err != nil {
		return task.Priorities{}, err
	}
	return cfg.Vocabulary(), nil
}

// Columns is the board this queue is filed on: the statuses a write may use,
// their order, which of them offer work, and which count a dependency as met.
// Read from the config on every call, for the same reason Priorities is.
func (s *Store) Columns() (task.Columns, error) {
	cfg, err := config.FindConfig(s.Dir)
	if err != nil {
		return task.Columns{}, err
	}
	return cfg.Board(), nil
}

// UnreadableFile is a task file a scan had to skip, and why. The reason is the
// error's message without the file name it already carried, because every
// caller prints the name itself.
type UnreadableFile struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// DuplicatedID is an ID more than one task file claims, and every file
// claiming it. Both copies are withheld from the listing: the ID is what
// identifies a task, so with two files answering to one there is no saying
// which of them a reader means, and showing either would be a guess that hides
// the other's edits — which is the call locate already makes for a write
// (TQ-0040).
//
// Reason is the sentence to show, composed once here so a listing and a write
// say the same thing about the same pair.
type DuplicatedID struct {
	ID     string   `json:"id"`
	Files  []string `json:"files"`
	Reason string   `json:"reason"`
}

// Listing is what one scan of the task directory found: the tasks it could
// read, in the default order, the files it could not, and the IDs it could not
// tell apart.
//
// They travel together because a caller must be able to show all three. A
// broken file is a real problem — a merge conflict, a hand-edited key — and
// staying quiet about it would trade one loud failure for a queue that is
// quietly a task short.
type Listing struct {
	Tasks      []task.Task
	Unreadable []UnreadableFile

	// Duplicated is the IDs the directory holds more than one file for. They
	// are not in Tasks: an ID appears at most once in a listing, which is what
	// lets every caller index by it (TQ-0040).
	Duplicated []DuplicatedID

	// Incomplete reports that the directory would not hold still long enough
	// to be read consistently: every attempt found it changed underneath, so
	// this listing may not match what is on disk. It may be missing a task
	// that exists — one renamed out from under the scan, or created after the
	// scan had already read the directory.
	//
	// It is a warning, not a failure. The tasks above are still every task the
	// scan could account for, and the caller shows them; what it must not do
	// is present them as the whole queue (TQ-0012).
	Incomplete bool
}

// listAttempts bounds the rescan when the directory changes mid-scan. Each
// attempt is one full pass, so a writer has to land inside the read window
// three times running to exhaust them, and the window is one directory read
// plus one read per file. This is a guard against a directory under sustained
// rewriting, not an expected path: a single concurrent `tq update --title`
// settles on the second attempt.
const listAttempts = 3

// List returns every task in the directory in the default order: status,
// priority, creation time, ID.
//
// A file that cannot be read is skipped and reported rather than failing the
// scan: two agents each running `tq add` on their own branch is the ordinary
// way a task file ends up with conflict markers in it, and one such file must
// not hide every other task from both surfaces (TQ-0011). The error return is
// for what makes the whole directory unreadable — the directory itself, or the
// project config the order depends on.
//
// The scan is checked against the directory before it is returned, because
// reading the names and then reading the files is a TOCTOU: a rename that
// lands in between leaves the task under a name this pass never looked at, and
// a task created in between is not in the names at all. Either way the pass is
// a task short and cannot tell. So the directory is read again afterwards and
// compared with the reading the pass started from; a difference means the
// snapshot is not of any one moment, and the pass is redone (TQ-0012).
//
// An ID claimed by two files is the same signal from the other side. A retitle
// writes the new file before retiring the old one, so for an instant one task
// has two files, and a pass whose two directory readings both fall inside that
// instant sees no change at all — it just holds the task twice. So that is a
// reason to redo the pass too.
//
// What tells that instant from a queue that really does have two files for one
// ID is looking again: the pair is redone, and a pair found by two passes the
// directory did not move during is not a retitle in flight. It is reported as
// what it is, both copies withheld, and the retries stop there rather than
// being spent chasing a race that is not happening (TQ-0040).
//
// A pair only ever seen while the directory was moving is not condemned on
// that evidence: it is withheld all the same, since an ID belongs in a listing
// once or not at all, but what the listing says about it is Incomplete — that
// it may be a task short — and not that there are two files to choose between.
//
// A broken file does not move the directory, so it is reported once and never
// retried — the retry is for a directory that changed, not for a file that
// cannot be parsed.
func (s *Store) List() (Listing, error) {
	// The ranking and the board are the project's, so sorting needs both. Read
	// once, ahead of the scans: they do not change with an attempt, and a
	// failure here is a failure of the whole listing rather than of one pass.
	priorities, err := s.Priorities()
	if err != nil {
		return Listing{}, err
	}
	columns, err := s.Columns()
	if err != nil {
		return Listing{}, err
	}

	var listing Listing
	var doubled, previous []DuplicatedID
	confirmed := false
	for attempt := 1; ; attempt++ {
		before, err := s.taskFileNames()
		if err != nil {
			return Listing{}, err
		}
		listing = s.scan(before)
		after, err := s.taskFileNames()
		if err != nil {
			return Listing{}, err
		}
		doubled = duplicatedIDs(before)
		heldStill := slices.Equal(before, after)
		switch {
		case heldStill && len(doubled) == 0:
			// The directory the scan read is the directory that is there.
		case heldStill && sameClaims(doubled, previous):
			// The same pair after two passes the directory did not move
			// during: not a retitle in flight, so there is nothing to wait for.
			confirmed = true
		case attempt == listAttempts:
			listing.Incomplete = true
		default:
			// Only a pass nothing moved during is evidence. A pair seen while
			// the directory was moving is exactly what a retitle looks like,
			// so it cannot be half of the two readings that condemn an ID.
			previous = nil
			if heldStill {
				previous = doubled
			}
			continue
		}
		break
	}

	// An ID two files claim does not go out in a listing whatever stopped the
	// loop: callers index tasks by ID, so one arriving twice is what puts two
	// cards on a single key and 500s the edit of either (TQ-0040).
	//
	// Only a confirmed pair is named, though. One the attempts ran out on may
	// still be a retitle nobody finished in time, and telling someone to
	// delete one of two files when nothing is wrong is worse than saying what
	// Incomplete already says — that the listing may be missing a task.
	if confirmed {
		listing.Duplicated = doubled
	}
	listing.Tasks = withhold(listing.Tasks, doubled)

	// A task filed under a value the project has since dropped keeps it and
	// sorts last, rather than being refused by the listing that would show it.
	for i := range listing.Tasks {
		listing.Tasks[i].Status = columns.Normalize(listing.Tasks[i].Status)
	}
	task.SortTasks(listing.Tasks, priorities, columns)
	return listing, nil
}

// taskFileNames is the directory reading a scan starts from and is checked
// against: the names of the files that hold tasks, in the sorted order
// os.ReadDir returns them, with everything else left out — subdirectories, the
// generated guide, and the temporary file an atomic write leaves beside the
// tasks for an instant, which is not a task appearing and must not send a scan
// round again.
func (s *Store) taskFileNames() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := taskFileID(entry.Name()); !ok {
			continue
		}
		names = append(names, entry.Name())
	}
	return names, nil
}

// duplicatedIDs reports the IDs the given file names claim more than once,
// sorted by ID.
//
// The names decide it, not the tasks that were read, so that this agrees with
// locate: a second file for an ID stops that task being addressable whether or
// not it parses, and a listing that showed the task anyway would put a card on
// the board that 500s the moment it is edited. It is also what makes a
// retitle's instant visible from inside a pass that saw no other change.
func duplicatedIDs(names []string) []DuplicatedID {
	claims := make(map[string][]string, len(names))
	for _, name := range names {
		id, ok := taskFileID(name)
		if !ok {
			continue
		}
		claims[id] = append(claims[id], name)
	}

	var doubled []DuplicatedID
	for id, files := range claims {
		if len(files) < 2 {
			continue
		}
		doubled = append(doubled, DuplicatedID{ID: id, Files: files, Reason: duplicateClaim(id, files)})
	}
	slices.SortFunc(doubled, func(a, b DuplicatedID) int { return strings.Compare(a.ID, b.ID) })
	return doubled
}

// sameClaims reports two passes finding the same IDs under the same files,
// which is how List tells a queue that has two files for one ID from a retitle
// caught between writing the new file and retiring the old.
func sameClaims(a, b []DuplicatedID) bool {
	return slices.EqualFunc(a, b, func(x, y DuplicatedID) bool {
		return x.ID == y.ID && slices.Equal(x.Files, y.Files)
	})
}

// withhold drops every copy of an ambiguous ID from the tasks a pass read.
func withhold(tasks []task.Task, doubled []DuplicatedID) []task.Task {
	if len(doubled) == 0 {
		return tasks
	}
	ambiguous := make(map[string]struct{}, len(doubled))
	for _, d := range doubled {
		ambiguous[d.ID] = struct{}{}
	}
	kept := tasks[:0]
	for _, t := range tasks {
		if _, dup := ambiguous[t.ID]; dup {
			continue
		}
		kept = append(kept, t)
	}
	return kept
}

// scan reads the named files. It is one attempt at a listing: List is what
// decides whether the result describes a directory that stood still.
func (s *Store) scan(names []string) Listing {
	if s.duringScan != nil {
		s.duringScan()
	}

	listing := Listing{Tasks: make([]task.Task, 0, len(names))}
	for _, name := range names {
		t, err := s.readFile(name)
		switch {
		case errors.Is(err, ErrTaskNotFound):
			// The file was there when the directory was read and is gone now,
			// which is what a concurrent write looks like: update writes the
			// new name and only then retires the old one, and delete unlinks.
			// Nothing is broken, so there is nothing to report — reporting it
			// would put a red toast on the board for an ordinary retitle. The
			// directory moved, though, and the check in List is what catches
			// the task this pass may have missed because of it.
			continue
		case err != nil:
			listing.Unreadable = append(listing.Unreadable, UnreadableFile{File: name, Reason: skipReason(name, err)})
			continue
		}
		listing.Tasks = append(listing.Tasks, t)
	}
	return listing
}

// skipReason is why a file was skipped, with the file name taken off the front.
// readFile's errors name the file — that is what makes them actionable on their
// own — and the callers that render a skipped file put the name in a column or
// a field of its own, where repeating it reads as a stutter.
//
// One line, too: the YAML decoder's errors run to several, and both callers put
// this where a line is a unit — a warning on stderr, a toast on the board.
func skipReason(name string, err error) string {
	msg := err.Error()
	if _, rest, found := strings.Cut(msg, name+": "); found {
		msg = rest
	}
	return strings.Join(strings.Fields(msg), " ")
}

// Get returns a single task by ID, with its status resolved against the board:
// an alias becomes the column it names, and a column the project has removed
// becomes the first one, which is where the task is shown.
//
// Resolved in memory only. Reads never write — a listing must not rewrite the
// directory it is listing — so the corrected value reaches the file the next
// time the task is saved for some other reason.
func (s *Store) Get(id string) (task.Task, error) {
	if !task.ValidID(id) {
		return task.Task{}, fmt.Errorf("invalid task id %q (must match TQ-<number>)", id)
	}
	name, err := s.locate(id)
	if err != nil {
		return task.Task{}, err
	}
	t, err := s.readFile(name)
	if err != nil {
		return task.Task{}, err
	}
	columns, err := s.Columns()
	if err != nil {
		return task.Task{}, err
	}
	t.Status = columns.Normalize(t.Status)
	return t, nil
}

func (s *Store) readFile(name string) (task.Task, error) {
	data, err := os.ReadFile(filepath.Join(s.Dir, name))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return task.Task{}, fmt.Errorf("%w: %s", ErrTaskNotFound, name)
		}
		return task.Task{}, err
	}

	t, err := task.ParseTask(name, data)
	if err != nil {
		return task.Task{}, err
	}
	// The ID in the frontmatter is authoritative; a stale title suffix is
	// harmless and gets fixed on the next write.
	if fileID, _ := taskFileID(name); t.ID != fileID {
		return task.Task{}, fmt.Errorf("%w: %s: id %q does not match the filename", task.ErrInvalidTaskFile, name, t.ID)
	}
	return t, nil
}

// Create allocates the next ID and writes a new task file.
func (s *Store) Create(in CreateTaskInput) (task.Task, error) {
	// Allocating a number and claiming it must happen together, or a
	// concurrent caller allocates the same one. A task sharing its ID with
	// another is unreachable: locate refuses to guess between them.
	s.mu.Lock()
	defer s.mu.Unlock()

	priorities, err := s.Priorities()
	if err != nil {
		return task.Task{}, err
	}
	if err := priorities.Check(in.Priority); err != nil {
		return task.Task{}, err
	}
	columns, err := s.Columns()
	if err != nil {
		return task.Task{}, err
	}
	if err := columns.Check(in.Status); err != nil {
		return task.Task{}, err
	}

	now := time.Now().Truncate(time.Second)
	t := task.Task{
		Title:     strings.TrimSpace(in.Title),
		Status:    columns.Normalize(orDefault(in.Status, columns.Default())),
		Priority:  orDefault(in.Priority, priorities.Default()),
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
			return task.Task{}, err
		}
		t.ID = id
		if err := t.ValidateForWrite(); err != nil {
			return task.Task{}, err
		}
		if _, err := s.writeNew(t); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return task.Task{}, err
		}
		return t, nil
	}
	return task.Task{}, fmt.Errorf("could not claim a task ID after %d attempts", createAttempts)
}

// Update rewrites an existing task and refreshes its updated timestamp. The
// returned task is exactly what was written to disk.
// Update saves a task the caller has already read and changed. It is
// last-write-wins by nature: two callers that read the same version both write
// their whole copy. Use Mutate when the change depends on what is there.
//
// The priority is not checked here, because there is nothing to check it
// against: the task came out of this store, so its priority is whatever the
// file already held. Create and Patch are where a caller supplies one.
func (s *Store) Update(t task.Task) (task.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.update(t)
}

// Mutate reads a task, applies a change and saves it, holding the lock across
// all three. A read-modify-write split across Get and Update loses everything
// the other caller did; an append like a note loses information nobody can
// reconstruct.
func (s *Store) Mutate(id string, apply func(*task.Task) error) (task.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, err := s.Get(id)
	if err != nil {
		return task.Task{}, err
	}
	if err := apply(&t); err != nil {
		return task.Task{}, err
	}
	return s.update(t)
}

// Patch applies a change to a task and saves it in one step. Adding a label or
// a dependency is an append, so a caller doing this through Get and Update
// loses whatever another caller added in between.
//
// A patch is checked against the vocabulary only where it would *change* the
// priority. Filing a task under a value the project does not have is a mistake
// worth naming; restating the value it already carries changes nothing, and
// refusing that would leave every task filed under a dropped value unable to be
// moved, retitled or closed — the board's dialog sends all its fields at once,
// so an untouched priority still arrives in the patch.
func (s *Store) Patch(id string, patch task.TaskPatch) (task.Task, error) {
	return s.Mutate(id, func(t *task.Task) error {
		if patch.Status != nil && *patch.Status != t.Status {
			columns, err := s.Columns()
			if err != nil {
				return err
			}
			if err := columns.Check(*patch.Status); err != nil {
				return err
			}
			resolved := columns.Normalize(*patch.Status)
			patch.Status = &resolved
		}
		if patch.Priority != nil && *patch.Priority != t.Priority {
			priorities, err := s.Priorities()
			if err != nil {
				return err
			}
			if err := priorities.Check(*patch.Priority); err != nil {
				return err
			}
		}
		*t = task.ApplyPatch(*t, patch)
		return nil
	})
}

// Note appends a timestamped note to a task and saves it in one step.
func (s *Store) Note(id, text string) (task.Task, error) {
	return s.Mutate(id, func(t *task.Task) error {
		t.Body = task.AppendNote(t.Body, text, time.Now().Truncate(time.Second))
		return nil
	})
}

func (s *Store) update(t task.Task) (task.Task, error) {
	current, err := s.locate(t.ID)
	if err != nil {
		return task.Task{}, err
	}
	t.Title = strings.TrimSpace(t.Title)
	t.Labels = normalizeList(t.Labels)
	t.DependsOn = normalizeList(t.DependsOn)
	t.Body = strings.Trim(t.Body, "\n")
	t.Updated = time.Now().Truncate(time.Second)

	columns, err := s.Columns()
	if err != nil {
		return task.Task{}, err
	}
	t.Status = columns.Normalize(t.Status)

	if err := t.ValidateForWrite(); err != nil {
		return task.Task{}, err
	}

	written, err := s.write(t)
	if err != nil {
		return task.Task{}, err
	}
	// A retitled task moves to a new filename; the task itself is now safely
	// on disk either way, and only the file it used to live in is left.
	if written != current {
		if err := retireOldFile(filepath.Join(s.Dir, current), filepath.Join(s.Dir, written)); err != nil {
			return task.Task{}, fmt.Errorf("saved %s but could not retire %s: %w", written, current, err)
		}
	}
	return t, nil
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
	if !task.ValidID(id) {
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
func (s *Store) stage(t task.Task) (path string, err error) {
	data, err := task.RenderTask(t)
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
func (s *Store) write(t task.Task) (string, error) {
	tmpName, err := s.stage(t)
	if err != nil {
		return "", err
	}
	name := TaskFileName(t)
	if err := os.Rename(tmpName, filepath.Join(s.Dir, name)); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	return name, nil
}

// writeNew is write for a task that must not exist yet. Linking fails when the
// name is taken, where renaming would replace the file — and the file it would
// replace is another task nobody asked to lose.
func (s *Store) writeNew(t task.Task) (string, error) {
	tmpName, err := s.stage(t)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmpName) }()

	name := TaskFileName(t)
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
