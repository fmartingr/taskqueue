package taskqueue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestMain clears the environment the whole suite runs under. TQ_DIR is the
// documented way to point tq at a queue, so a developer may well have it
// exported; without this every test would operate on their real one, and one
// of them deletes the directory it is given.
func TestMain(m *testing.M) {
	isolate()
	os.Exit(m.Run())
}

// isolate removes the configuration that could send a test outside its own
// temp directory. Individual tests still set these with t.Setenv when that is
// what they are testing.
func isolate() {
	for _, name := range []string{EnvTaskDir, EnvWalkForever} {
		_ = os.Unsetenv(name)
	}
}

// testRoot returns a temp directory that discovery cannot climb out of.
// testRoot(t) alone is not an isolation barrier: it honours TMPDIR, and the
// walk up looks for .git and .tasks in every parent, so with TMPDIR inside a
// repository the fixtures would bind to that repository's committed queue.
// Marking the root as a repository root stops the walk here.
func testRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// newTestStore returns a store backed by a fresh .tasks directory.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	// Belt and braces: TestMain cleared the ambient values, this clears
	// anything a test set before reaching for a fixture.
	t.Setenv(EnvTaskDir, "")
	t.Setenv(EnvWalkForever, "")
	root := testRoot(t)
	store, err := InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return store
}

func mustCreate(t *testing.T, s *Store, in CreateTaskInput) Task {
	t.Helper()
	task, err := s.Create(in)
	if err != nil {
		t.Fatalf("Create(%q): %v", in.Title, err)
	}
	return task
}

func TestInitStore(t *testing.T) {
	root := testRoot(t)
	store, err := InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(root, TaskDirName); store.Dir != want {
		t.Errorf("Dir = %q, want %q", store.Dir, want)
	}
	if info, err := os.Stat(store.Dir); err != nil || !info.IsDir() {
		t.Fatalf("task directory not created: %v", err)
	}
	if !store.Created {
		t.Error("Created should be true the first time")
	}

	// Initialising again is harmless and reports that nothing was created.
	again, err := InitStore(root)
	if err != nil {
		t.Fatalf("second InitStore: %v", err)
	}
	if again.Dir != store.Dir || again.Created {
		t.Errorf("second InitStore = %+v, want the same directory with Created=false", again)
	}
}

func TestOpenStoreCreatesTaskDirOnDemand(t *testing.T) {
	root := testRoot(t)

	store, err := OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if want := filepath.Join(root, TaskDirName); store.Dir != want {
		t.Errorf("Dir = %q, want %q", store.Dir, want)
	}
	if !store.Created {
		t.Error("Created should report that OpenStore made the directory")
	}
	if info, err := os.Stat(store.Dir); err != nil || !info.IsDir() {
		t.Fatalf("task directory not created: %v", err)
	}

	// Opening it again finds the existing directory instead of recreating it.
	again, err := OpenStore(root)
	if err != nil {
		t.Fatalf("second OpenStore: %v", err)
	}
	if again.Dir != store.Dir || again.Created {
		t.Errorf("second OpenStore = %+v, want the same directory with Created=false", again)
	}
}

func TestOpenStoreCreatesAtTheRepositoryRoot(t *testing.T) {
	root := testRoot(t)
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	// A new task directory belongs next to .git, not in whichever
	// subdirectory the agent happened to be standing in.
	store, err := OpenStore(nested)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if want := filepath.Join(root, TaskDirName); store.Dir != want {
		t.Errorf("Dir = %q, want %q", store.Dir, want)
	}
	if _, err := os.Stat(filepath.Join(nested, TaskDirName)); err == nil {
		t.Errorf("no %s should have been created in the subdirectory", TaskDirName)
	}
}

func TestOpenStoreReportsUncreatableDir(t *testing.T) {
	root := testRoot(t)
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The root is anchored, so creation would otherwise fall back to it:
	// make that unwritable too, leaving nothing creatable to report on.
	if err := os.Chmod(root, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	// Nothing can be created below a regular file, so this is still "no usable
	// task directory" rather than a filesystem error nobody can act on.
	_, err := OpenStore(filepath.Join(file, "sub"))
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestCreateWritesMarkdownFile(t *testing.T) {
	store := newTestStore(t)
	task := mustCreate(t, store, CreateTaskInput{
		Title:    "Implement REST API",
		Priority: PriorityHigh,
		Labels:   []string{"backend"},
		Body:     "Some description.",
	})

	if task.ID != "TQ-0001" {
		t.Errorf("ID = %q, want TQ-0001", task.ID)
	}
	if task.Status != StatusTodo {
		t.Errorf("Status = %q, want the default %q", task.Status, StatusTodo)
	}
	if task.Created.IsZero() || task.Updated.IsZero() {
		t.Error("timestamps should be set on create")
	}

	// The filename carries a slug of the title so the directory is browsable.
	data, err := os.ReadFile(filepath.Join(store.Dir, "TQ-0001-implement-rest-api.md"))
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}
	if !strings.HasPrefix(string(data), "---\nid: TQ-0001\ntitle: Implement REST API\nstatus: todo\n") {
		t.Errorf("unexpected file contents:\n%s", data)
	}
	if !strings.HasSuffix(string(data), "\nSome description.\n") {
		t.Errorf("body missing from file:\n%s", data)
	}
}

func TestCreateDefaultsPriorityToNormal(t *testing.T) {
	store := newTestStore(t)
	task := mustCreate(t, store, CreateTaskInput{Title: "No priority given"})
	if task.Priority != PriorityNormal {
		t.Errorf("Priority = %q, want %q", task.Priority, PriorityNormal)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	store := newTestStore(t)
	for _, in := range []CreateTaskInput{
		{Title: ""},
		{Title: "x", Priority: "whenever"},
		{Title: "x", Status: "shipped"},
		{Title: "x", DependsOn: []string{"nope"}},
	} {
		if _, err := store.Create(in); err == nil {
			t.Errorf("Create(%+v) should have failed", in)
		}
	}
	entries, _ := os.ReadDir(store.Dir)
	if len(entries) != 0 {
		t.Errorf("no files should be written for invalid input, found %d", len(entries))
	}
}

func TestNextIDIsSequential(t *testing.T) {
	store := newTestStore(t)
	for _, want := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		got := mustCreate(t, store, CreateTaskInput{Title: "task"})
		if got.ID != want {
			t.Fatalf("ID = %q, want %q", got.ID, want)
		}
	}

	// IDs continue past four digits without renumbering existing tasks, and the
	// title suffix does not confuse the scan.
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-9999-nearly-out-of-digits.md"), []byte("---\nid: TQ-9999\ntitle: x\nstatus: todo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := store.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if next != "TQ-10000" {
		t.Errorf("NextID = %q, want TQ-10000", next)
	}
}

func TestGet(t *testing.T) {
	store := newTestStore(t)
	created := mustCreate(t, store, CreateTaskInput{Title: "Findable", Body: "Body text."})

	got, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Findable" || got.Body != "Body text." {
		t.Errorf("Get returned %+v", got)
	}

	if _, err := store.Get("TQ-4242"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Get(missing) = %v, want ErrTaskNotFound", err)
	}
	if _, err := store.Get("not-an-id"); err == nil {
		t.Error("Get should reject malformed IDs")
	}
}

func TestListSortsAndReportsBadFiles(t *testing.T) {
	store := newTestStore(t)
	mustCreate(t, store, CreateTaskInput{Title: "first"})
	second := mustCreate(t, store, CreateTaskInput{Title: "second", Priority: PriorityUrgent})

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 || tasks[0].ID != second.ID {
		t.Errorf("List should sort urgent first, got %+v", tasks)
	}

	// Files that are not tasks are ignored.
	if err := os.WriteFile(filepath.Join(store.Dir, "README.md"), []byte("not a task"), 0o644); err != nil {
		t.Fatal(err)
	}
	if tasks, err = store.List(); err != nil || len(tasks) != 2 {
		t.Errorf("List() = %d tasks, %v; want 2 tasks and no error", len(tasks), err)
	}

	// A malformed task file is a hard error that names the file.
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-0003.md"), []byte("no frontmatter here"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = store.List()
	if err == nil || !strings.Contains(err.Error(), "TQ-0003.md") {
		t.Errorf("List error = %v, want it to name TQ-0003.md", err)
	}
}

func TestListRejectsIDFilenameMismatch(t *testing.T) {
	store := newTestStore(t)
	content := "---\nid: TQ-0009\ntitle: mismatched\nstatus: todo\n---\n"
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-0008.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("List error = %v, want a filename/id mismatch error", err)
	}
}

func TestUpdateRewritesFileAtomically(t *testing.T) {
	store := newTestStore(t)
	created := mustCreate(t, store, CreateTaskInput{Title: "Original", Body: "Body."})

	created.Title = "Renamed"
	created.Status = StatusInProgress
	updated, err := store.Update(created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Updated.Before(updated.Created) {
		t.Error("Update should refresh the updated timestamp")
	}

	reloaded, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.Title != "Renamed" || reloaded.Status != StatusInProgress || reloaded.Body != "Body." {
		t.Errorf("reloaded = %+v", reloaded)
	}

	// The rewrite leaves exactly one valid file behind (no temp files, and no
	// leftover from the old title).
	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "TQ-0001-renamed.md" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only TQ-0001-renamed.md", names)
	}
}

func TestTaskFileName(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Implement REST API", "TQ-0001-implement-rest-api.md"},
		{"...", "TQ-0001.md"}, // nothing slugifiable: the ID alone
	}
	for _, tc := range tests {
		task := Task{ID: "TQ-0001", Title: tc.title, Status: StatusTodo}
		if got := TaskFileName(task); got != tc.want {
			t.Errorf("TaskFileName(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestGetFindsTasksWhateverTheSuffix(t *testing.T) {
	store := newTestStore(t)
	// A file written before titles were part of the filename.
	legacy := "---\nid: TQ-0001\ntitle: Written by an older version\nstatus: todo\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-0001.md"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	task, err := store.Get("TQ-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.Title != "Written by an older version" {
		t.Errorf("task = %+v", task)
	}

	tasks, err := store.List()
	if err != nil || len(tasks) != 1 {
		t.Errorf("List() = %d tasks, %v; want 1", len(tasks), err)
	}

	// Touching it adopts the new naming.
	if _, err := store.Update(task); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, "TQ-0001-written-by-an-older-version.md")); err != nil {
		t.Errorf("update should have renamed the file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.Dir, "TQ-0001.md")); err == nil {
		t.Error("the old filename should be gone")
	}
}

func TestUpdateKeepsTheFilenameWhenTheTitleIsUnchanged(t *testing.T) {
	store := newTestStore(t)
	task := mustCreate(t, store, CreateTaskInput{Title: "Stable title"})

	task.Status = StatusInProgress
	if _, err := store.Update(task); err != nil {
		t.Fatalf("Update: %v", err)
	}

	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "TQ-0001-stable-title.md" {
		t.Errorf("directory contains %v, want only TQ-0001-stable-title.md", entries)
	}
}

func TestDuplicateFilesForOneIDAreRejected(t *testing.T) {
	store := newTestStore(t)
	task := mustCreate(t, store, CreateTaskInput{Title: "Original"})

	// A half-finished manual rename leaves two files claiming one ID.
	rendered, err := RenderTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-0001-copy.md"), rendered, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = store.Get("TQ-0001")
	if !errors.Is(err, ErrInvalidTaskFile) {
		t.Errorf("Get = %v, want ErrInvalidTaskFile", err)
	}
	for _, name := range []string{"TQ-0001-original.md", "TQ-0001-copy.md"} {
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("error %v should name %s", err, name)
		}
	}
}

func TestUpdateRejectsInvalidTask(t *testing.T) {
	store := newTestStore(t)
	created := mustCreate(t, store, CreateTaskInput{Title: "Valid"})
	created.Status = "shipped"
	if _, err := store.Update(created); err == nil {
		t.Fatal("Update should reject an invalid status")
	}

	reloaded, err := store.Get("TQ-0001")
	if err != nil || reloaded.Status != StatusTodo {
		t.Errorf("the stored task should be untouched, got %+v (%v)", reloaded, err)
	}
}

func TestUpdateUnknownTask(t *testing.T) {
	store := newTestStore(t)
	ghost := Task{ID: "TQ-0404", Title: "ghost", Status: StatusTodo}
	if _, err := store.Update(ghost); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Update(missing) = %v, want ErrTaskNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	store := newTestStore(t)
	mustCreate(t, store, CreateTaskInput{Title: "Temporary"})
	if err := store.Delete("TQ-0001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get("TQ-0001"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Get after Delete = %v, want ErrTaskNotFound", err)
	}
	if err := store.Delete("TQ-0001"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrTaskNotFound", err)
	}
}

func TestDiscoverTaskDirWalksUp(t *testing.T) {
	root := testRoot(t)
	if _, err := InitStore(root); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(root, TaskDirName); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

func TestDiscoverTaskDirNotFound(t *testing.T) {
	_, err := DiscoverTaskDir(testRoot(t))
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestDiscoverTaskDirEnvOverride(t *testing.T) {
	root := testRoot(t)
	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := testRoot(t)

	t.Setenv(EnvTaskDir, store.Dir)
	dir, err := DiscoverTaskDir(elsewhere)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != store.Dir {
		t.Errorf("dir = %q, want the %s override %q", dir, EnvTaskDir, store.Dir)
	}

	// A missing override is "not there yet", which is what lets OpenStore
	// create it.
	t.Setenv(EnvTaskDir, filepath.Join(elsewhere, "missing"))
	if _, err := DiscoverTaskDir(elsewhere); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("DiscoverTaskDir with a missing %s = %v, want ErrProjectNotFound", EnvTaskDir, err)
	}
}

// aliasOnDisk renames a task's file to a name that differs only in spelling.
// It reports whether the filesystem folded the two names into one entry, which
// is what makes the alias dangerous: on a case-sensitive, byte-exact
// filesystem they are simply two files and there is nothing to test.
func aliasOnDisk(t *testing.T, store *Store, id, alias string) bool {
	t.Helper()
	current, err := store.locate(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(store.Dir, current), filepath.Join(store.Dir, alias)); err != nil {
		t.Fatal(err)
	}
	aliased, err := store.locate(id)
	if err != nil {
		t.Fatal(err)
	}
	return aliased == alias && alias != current
}

func TestUpdateKeepsTheTaskWhenTheFilenameDiffersOnlySpelling(t *testing.T) {
	// A hand-rename, or a checkout of a directory committed from a machine
	// that folds case or decomposes accents, leaves a name the store accepts
	// but that the filesystem treats as the same entry as the canonical one.
	tests := []struct {
		name      string
		title     string
		canonical string
		alias     string
	}{
		{"case", "Fix bug", "TQ-0001-fix-bug.md", "TQ-0001-Fix-Bug.md"},
		{"normalization", "Caf\u00e9 fix", "TQ-0001-caf\u00e9-fix.md", "TQ-0001-cafe\u0301-fix.md"}, // NFC vs NFD
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			task := mustCreate(t, store, CreateTaskInput{Title: tt.title})
			if got := TaskFileName(task); got != tt.canonical {
				t.Fatalf("TaskFileName = %q, want %q", got, tt.canonical)
			}
			if !aliasOnDisk(t, store, task.ID, tt.alias) {
				t.Skipf("this filesystem keeps %q and %q apart, so there is no alias to lose", tt.canonical, tt.alias)
			}

			task.Status = StatusInProgress
			if _, err := store.Update(task); err != nil {
				t.Fatalf("Update: %v", err)
			}

			reloaded, err := store.Get(task.ID)
			if err != nil {
				t.Fatalf("the task should have survived the update: %v", err)
			}
			if reloaded.Status != StatusInProgress {
				t.Errorf("Status = %q, want %q", reloaded.Status, StatusInProgress)
			}
			// The alias is a stale title suffix like any other: it converges.
			entries, err := os.ReadDir(store.Dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != tt.canonical {
				t.Errorf("directory contains %v, want only %s", names(entries), tt.canonical)
			}
		})
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	return out
}

// The aliasing tests above only bite on a folding filesystem, so the decision
// retireOldFile makes is pinned down here on every platform: hard links give
// two names for one entry anywhere.
func TestRetireOldFile(t *testing.T) {
	write := func(t *testing.T, path string) {
		t.Helper()
		if err := os.WriteFile(path, []byte("task"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The one entry both names resolve to on a folding filesystem, expressed
	// so that every filesystem can run it: one entry, reached twice.
	t.Run("a single entry is never removed", func(t *testing.T) {
		dir := testRoot(t)
		path := filepath.Join(dir, "task.md")
		write(t, path)
		if err := retireOldFile(path, path); err != nil {
			t.Fatalf("retireOldFile: %v", err)
		}
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("the written task must survive: %v", err)
		}
	})

	// Hard links are two entries sharing one file, so the rename POSIX makes
	// a no-op leaves both. That is the store's loud "claimed by 2 files"
	// state, which is recoverable; deleting the task would not be.
	t.Run("hard links keep the written task", func(t *testing.T) {
		dir := testRoot(t)
		oldPath, newPath := filepath.Join(dir, "old.md"), filepath.Join(dir, "new.md")
		write(t, newPath)
		if err := os.Link(newPath, oldPath); err != nil {
			t.Fatal(err)
		}
		if err := retireOldFile(oldPath, newPath); err != nil {
			t.Fatalf("retireOldFile: %v", err)
		}
		if _, err := os.Lstat(newPath); err != nil {
			t.Errorf("the written task must survive: %v", err)
		}
	})

	t.Run("a genuinely different file is removed", func(t *testing.T) {
		dir := testRoot(t)
		oldPath, newPath := filepath.Join(dir, "old.md"), filepath.Join(dir, "new.md")
		write(t, oldPath)
		write(t, newPath)
		if err := retireOldFile(oldPath, newPath); err != nil {
			t.Fatalf("retireOldFile: %v", err)
		}
		if _, err := os.Lstat(oldPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the old file should be gone, got %v", err)
		}
		if _, err := os.Lstat(newPath); err != nil {
			t.Errorf("the written task must survive: %v", err)
		}
	})

	t.Run("a dangling symlink is unlinked", func(t *testing.T) {
		dir := testRoot(t)
		oldPath, newPath := filepath.Join(dir, "old.md"), filepath.Join(dir, "new.md")
		write(t, newPath)
		if err := os.Symlink(filepath.Join(dir, "gone.md"), oldPath); err != nil {
			t.Fatal(err)
		}
		if err := retireOldFile(oldPath, newPath); err != nil {
			t.Fatalf("retireOldFile: %v", err)
		}
		// Left behind, it would be a second file claiming the same task ID.
		if _, err := os.Lstat(oldPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the dangling link should be gone, got %v", err)
		}
	})

	t.Run("an already removed old file is not an error", func(t *testing.T) {
		dir := testRoot(t)
		newPath := filepath.Join(dir, "new.md")
		write(t, newPath)
		if err := retireOldFile(filepath.Join(dir, "old.md"), newPath); err != nil {
			t.Errorf("retireOldFile: %v", err)
		}
	})
}

// A queue above a project must not capture it: a developer who once ran tq in
// their home directory would otherwise have every new repository file into it.
func TestDiscoverTaskDirStopsAtTheRepositoryRoot(t *testing.T) {
	outer := testRoot(t)
	if _, err := InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverTaskDir(repo)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound rather than the queue above the repository", err)
	}
	// The message has to explain itself: the queue is plainly there, one level
	// up, so "not found" alone reads as a bug.
	for _, want := range []string{"repository root", EnvWalkForever} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

func TestDiscoverTaskDirWalksPastTheRepositoryRootWhenAsked(t *testing.T) {
	outer := testRoot(t)
	if _, err := InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvWalkForever, "true")

	dir, err := DiscoverTaskDir(repo)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(outer, TaskDirName); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// Only "true" lifts the bound; anything else leaves the default in place.
func TestDiscoverTaskDirIgnoresAnUnsetWalkForever(t *testing.T) {
	outer := testRoot(t)
	if _, err := InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvWalkForever, "1")

	if _, err := DiscoverTaskDir(repo); !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want the bound to hold for a value other than \"true\"", err)
	}
}

// The bound is the repository root, not the starting directory: a queue at the
// root is still found from a subdirectory of the same repository.
func TestDiscoverTaskDirFindsTheQueueInsideItsOwnRepository(t *testing.T) {
	repo := testRoot(t)
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := InitStore(repo); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repo, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(repo, TaskDirName); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// A developer with TQ_DIR exported — which the README and the guide both tell
// them to use — must still get an isolated suite. Without this the whole suite
// operates on their real queue, and one test deletes it.
func TestFixturesIgnoreAnAmbientTaskDirOverride(t *testing.T) {
	outside := filepath.Join(testRoot(t), "real", TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvTaskDir, outside)

	store := newTestStore(t)
	if store.Dir == outside {
		t.Fatalf("newTestStore used the ambient %s: %s", EnvTaskDir, store.Dir)
	}
	mustCreate(t, store, CreateTaskInput{Title: "fixture task"})

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the fixture wrote into the directory %s names: %d entries", EnvTaskDir, len(entries))
	}
}

// The same for the walk-forever escape hatch: an exported value must not let a
// fixture climb out of its own temp directory. Asserted on the environment
// rather than on a walk, because there is nowhere above testRoot(t) a test can
// safely plant a queue to walk into.
func TestFixturesNeutraliseAmbientConfiguration(t *testing.T) {
	t.Setenv(EnvTaskDir, "/somewhere/real/.tasks")
	t.Setenv(EnvWalkForever, "true")

	newTestStore(t)

	for _, name := range []string{EnvTaskDir, EnvWalkForever} {
		if got := os.Getenv(name); got != "" {
			t.Errorf("%s = %q after newTestStore, want it cleared", name, got)
		}
	}
}

// testRoot(t) is not an isolation barrier: it honours TMPDIR, and discovery
// walks up out of it. With TMPDIR inside a git repository — normal in Nix,
// Bazel and containerised CI — the fixtures would otherwise bind to that
// repository's committed queue and write into it.
func TestFixturesStayInsideTempDirWhenTMPDIRIsInARepository(t *testing.T) {
	// Not testRoot(t): it fixes its base directory on first use, so calling
	// it here would pin the base before TMPDIR changes and the fixture
	// would never see the repository.
	repo, mkErr := os.MkdirTemp("", "tq-tmpdir-repo-*")
	if mkErr != nil {
		t.Fatal(mkErr)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(repo, TaskDirName)
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(repo, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)

	store := newTestStore(t)
	if store.Dir == real {
		t.Fatalf("the fixture bound to the enclosing repository's queue: %s", store.Dir)
	}
	mustCreate(t, store, CreateTaskInput{Title: "fixture task"})

	entries, err := os.ReadDir(real)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the fixture wrote into the repository's queue: %d entries", len(entries))
	}
}

// The HTTP server hands one *Store to net/http, so concurrent handlers share
// it. Allocating an ID by scanning the directory and writing later gives every
// racer the same number, and a task that shares its ID with another is
// unreachable: locate refuses to guess between them.
func TestCreateUnderConcurrencyGivesEveryTaskItsOwnID(t *testing.T) {
	store := newTestStore(t)

	const racers = 20
	var wg sync.WaitGroup
	ids := make([]string, racers)
	errs := make([]error, racers)

	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := store.Create(CreateTaskInput{Title: fmt.Sprintf("task %d", i)})
			ids[i], errs[i] = task.ID, err
		}()
	}
	wg.Wait()

	seen := make(map[string]int, racers)
	for i, id := range ids {
		if errs[i] != nil {
			t.Fatalf("Create: %v", errs[i])
		}
		seen[id]++
	}
	if len(seen) != racers {
		t.Errorf("got %d distinct ids from %d creates: %v", len(seen), racers, seen)
	}

	// Every task must still be reachable by its ID.
	for _, id := range ids {
		if _, err := store.Get(id); err != nil {
			t.Errorf("Get(%s): %v", id, err)
		}
	}

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != racers {
		t.Errorf("List() = %d tasks, want %d", len(tasks), racers)
	}
}

// Racers sharing a title used to produce the same filename, so the second
// rename replaced the first and a task vanished behind a successful return.
func TestCreateUnderConcurrencyKeepsTasksWithTheSameTitle(t *testing.T) {
	store := newTestStore(t)

	const racers = 10
	var wg sync.WaitGroup
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Create(CreateTaskInput{Title: "same title"}); err != nil {
				t.Errorf("Create: %v", err)
			}
		}()
	}
	wg.Wait()

	tasks, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != racers {
		t.Errorf("List() = %d tasks, want %d: a create was lost", len(tasks), racers)
	}
}

// A note is an append, so a lost one is information nobody can reconstruct.
// Two agents working the same task is what the queue exists for.
func TestNoteUnderConcurrencyKeepsEveryNote(t *testing.T) {
	store := newTestStore(t)
	task := mustCreate(t, store, CreateTaskInput{Title: "probe"})

	const notes = 10
	var wg sync.WaitGroup
	for i := range notes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Note(task.ID, fmt.Sprintf("note %d", i)); err != nil {
				t.Errorf("Note: %v", err)
			}
		}()
	}
	wg.Wait()

	after, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i := range notes {
		if want := fmt.Sprintf("note %d", i); !strings.Contains(after.Body, want) {
			t.Errorf("%q was lost:\n%s", want, after.Body)
		}
	}
}

// --add-label is an append too, so concurrent patches must not lose one.
func TestPatchUnderConcurrencyKeepsEveryLabel(t *testing.T) {
	store := newTestStore(t)
	task := mustCreate(t, store, CreateTaskInput{Title: "probe"})

	const labels = 10
	var wg sync.WaitGroup
	for i := range labels {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Patch(task.ID, TaskPatch{AddLabels: []string{fmt.Sprintf("label-%d", i)}}); err != nil {
				t.Errorf("Patch: %v", err)
			}
		}()
	}
	wg.Wait()

	after, err := store.Get(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Labels) != labels {
		t.Errorf("Labels = %v, want %d of them", after.Labels, labels)
	}
}

// The config is the marker discovery looks for: its path decides where the
// tasks live, from anywhere in the project.
func TestDiscoverTaskDirFollowsTheConfigPath(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: docs/queue\n")
	want := filepath.Join(root, "docs", "queue")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// A config naming a directory that does not exist yet is not an error: the
// queue is created where it says.
func TestInitStoreCreatesWhereTheConfigSays(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: docs/queue\n")

	store, err := InitStore(filepath.Join(root, "src"))
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(root, "docs", "queue"); store.Dir != want {
		t.Errorf("Dir = %q, want %q", store.Dir, want)
	}
}

// TQ_DIR is the task directory, full stop — the config's path is ignored.
func TestTaskDirOverrideBeatsTheConfigPath(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 1\npath: from-config\n")
	override := filepath.Join(root, "from-env")
	if err := os.MkdirAll(override, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvTaskDir, override)

	dir, err := DiscoverTaskDir(root)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != override {
		t.Errorf("dir = %q, want the override %q", dir, override)
	}
}

// The marker is the only thing tq looks for. A directory that happens to be
// called .tasks, with no marker above it, is somebody else's business — the
// guessing it would take to claim it is what the marker replaces.
func TestDiscoverTaskDirIgnoresABareTaskDirWithNoMarker(t *testing.T) {
	root := testRoot(t)
	if err := os.MkdirAll(filepath.Join(root, TaskDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverTaskDir(root)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
	if err != nil && !strings.Contains(err.Error(), ConfigFileName) {
		t.Errorf("err = %q, want it to name the file tq looks for", err)
	}
}

// A broken config must not read as "no task directory": the caller has to know
// their file is wrong, not have a queue created somewhere else.
func TestDiscoverTaskDirReportsABrokenConfig(t *testing.T) {
	root := testRoot(t)
	writeConfig(t, root, "version: 99\n")

	_, err := DiscoverTaskDir(root)
	if !errors.Is(err, ErrConfig) {
		t.Errorf("err = %v, want it to wrap ErrConfig", err)
	}
	if errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, must not be reported as a missing task directory", err)
	}
}

// Creating a queue leaves the marker behind, so the next command finds it by
// the file rather than by guessing at directory names.
func TestInitStoreWritesTheConfigMarker(t *testing.T) {
	root := testRoot(t)

	store, err := InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	cfg, err := FindConfig(root)
	if err != nil {
		t.Fatalf("FindConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("no config written")
	}
	if cfg.TaskDir() != store.Dir {
		t.Errorf("config points at %q, want the created %q", cfg.TaskDir(), store.Dir)
	}
	if cfg.Version != ConfigVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, ConfigVersion)
	}
}

// The config is the user's file, not a generated one.
func TestInitStoreLeavesAnExistingConfigAlone(t *testing.T) {
	root := testRoot(t)
	body := "version: 1\npath: mine\n# hand written\n"
	path := writeConfig(t, root, body)

	if _, err := InitStore(root); err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != body {
		t.Errorf("config was rewritten:\ngot:\n%s\nwant:\n%s", after, body)
	}
}
