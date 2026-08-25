package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestStore returns a store backed by a fresh .tasks directory.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	root := t.TempDir()
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
	root := t.TempDir()
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
	if _, err := InitStore(root); err == nil {
		t.Error("InitStore should fail when the directory already exists")
	}
}

func TestInitStoreHonoursEnvOverride(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "tasks-elsewhere")
	t.Setenv(EnvTaskDir, target)

	store, err := InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if store.Dir != target {
		t.Errorf("Dir = %q, want the %s override %q", store.Dir, EnvTaskDir, target)
	}
	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("override directory not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, TaskDirName)); err == nil {
		t.Errorf("%s should not have been created when %s is set", TaskDirName, EnvTaskDir)
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

	data, err := os.ReadFile(filepath.Join(store.Dir, "TQ-0001.md"))
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

	// IDs continue past four digits without renumbering existing tasks.
	if err := os.WriteFile(filepath.Join(store.Dir, "TQ-9999.md"), []byte("---\nid: TQ-9999\ntitle: x\nstatus: todo\n---\n"), 0o644); err != nil {
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

	// The rewrite leaves exactly one valid file behind (no temp files).
	entries, err := os.ReadDir(store.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "TQ-0001.md" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only TQ-0001.md", names)
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
	root := t.TempDir()
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
	_, err := DiscoverTaskDir(t.TempDir())
	if !errors.Is(err, ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
}

func TestDiscoverTaskDirEnvOverride(t *testing.T) {
	root := t.TempDir()
	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := t.TempDir()

	t.Setenv(EnvTaskDir, store.Dir)
	dir, err := DiscoverTaskDir(elsewhere)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != store.Dir {
		t.Errorf("dir = %q, want the %s override %q", dir, EnvTaskDir, store.Dir)
	}

	t.Setenv(EnvTaskDir, filepath.Join(elsewhere, "missing"))
	if _, err := DiscoverTaskDir(elsewhere); err == nil {
		t.Errorf("a %s pointing at a missing directory should fail", EnvTaskDir)
	}
}
