package store_test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fmartingr/taskqueue/internal/config"
	"github.com/fmartingr/taskqueue/internal/store"
	"github.com/fmartingr/taskqueue/internal/task"
	"github.com/fmartingr/taskqueue/internal/tqtest"
)

func TestInitStore(t *testing.T) {
	root := tqtest.Root(t)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(root, config.TaskDirName); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
	if info, err := os.Stat(st.Dir); err != nil || !info.IsDir() {
		t.Fatalf("task directory not created: %v", err)
	}
	if !st.Created {
		t.Error("Created should be true the first time")
	}

	// Initialising again is harmless and reports that nothing was created.
	again, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("second InitStore: %v", err)
	}
	if again.Dir != st.Dir || again.Created {
		t.Errorf("second InitStore = %+v, want the same directory with Created=false", again)
	}
}

// `tq init` is the only thing that creates a queue (TQ-0085). A marker whose
// task directory is not on disk is a project a reading command cannot serve,
// and inventing the directory is what that rule forbids — so the failure names
// the marker that made the claim, the directory it named, and the command that
// would put it there.
func TestOpenStoreCreatesNothing(t *testing.T) {
	root := tqtest.Root(t)
	declared := filepath.Join(root, config.TaskDirName)

	_, err := store.OpenStore(root)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
	for _, want := range []string{filepath.Join(root, config.ConfigFileName), declared, "tq init"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
	// And where to run it. Init creates the queue where it stands, so a bare
	// "run tq init" followed from a subdirectory forks a second project
	// instead of repairing this one.
	if want := `run "tq init" in ` + root; !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want it to say %q", err, want)
	}
	if _, err := os.Stat(declared); !os.IsNotExist(err) {
		t.Errorf("OpenStore created the task directory: %v", err)
	}

	// Once init has made it, the same call finds it and reports no creation of
	// its own.
	if _, err := store.InitStore(root); err != nil {
		t.Fatal(err)
	}
	st, err := store.OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if want := filepath.Join(root, config.TaskDirName); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
	if st.Created {
		t.Error("Created = true, want false: OpenStore never creates anything")
	}
}

// Init creates the project where it is run and nowhere else: not at a
// repository root, and not at the marker of a project above.
func TestInitStoreCreatesWhereItIsRun(t *testing.T) {
	outer := tqtest.Root(t)
	nested := filepath.Join(outer, "src", "deep")
	if err := os.MkdirAll(filepath.Join(nested, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := store.InitStore(nested)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(nested, config.TaskDirName); st.Dir != want {
		t.Errorf("Dir = %q, want the queue in the directory init was run in, %q", st.Dir, want)
	}
	if !st.Created {
		t.Error("Created = false, want true")
	}
	if want := filepath.Join(nested, config.ConfigFileName); st.ConfigWritten != want {
		t.Errorf("ConfigWritten = %q, want %q", st.ConfigWritten, want)
	}
	if _, err := os.Stat(filepath.Join(outer, config.TaskDirName)); !os.IsNotExist(err) {
		t.Errorf("init made a queue at the project above it: %v", err)
	}
}

// Nothing can be created below a regular file, and init says so rather than
// leaving a caller with a filesystem error nobody can act on.
func TestInitStoreReportsAnUncreatableDir(t *testing.T) {
	root := tqtest.RootWithoutMarker(t)
	// A regular file where the task directory belongs. A permission bit would
	// not do — uid 0 ignores it, and CI runs as root in a container — but no
	// privilege makes a directory out of a file.
	if err := os.WriteFile(filepath.Join(root, config.TaskDirName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.InitStore(root)
	if err == nil {
		t.Fatal("InitStore() = nil error, want one for a task directory that cannot exist")
	}
	// Not a missing project: the project is right here and its queue cannot be
	// made, which is a plain error rather than "run tq init".
	if errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("err = %v, want it not to read as a missing project", err)
	}
}

func TestCreateWritesMarkdownFile(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{
		Title:    "Implement REST API",
		Priority: task.PriorityHigh,
		Labels:   []string{"backend"},
		Body:     "Some description.",
	})

	if tk.ID != "TQ-0001" {
		t.Errorf("ID = %q, want TQ-0001", tk.ID)
	}
	if tk.Status != task.StatusInbox {
		t.Errorf("Status = %q, want the default %q", tk.Status, task.StatusInbox)
	}
	if tk.Created.IsZero() || tk.Updated.IsZero() {
		t.Error("timestamps should be set on create")
	}

	// The filename carries a slug of the title so the directory is browsable.
	data, err := os.ReadFile(filepath.Join(st.Dir, "TQ-0001-implement-rest-api.md"))
	if err != nil {
		t.Fatalf("read task file: %v", err)
	}
	if !strings.HasPrefix(string(data), "---\nid: TQ-0001\ntitle: Implement REST API\nstatus: inbox\n") {
		t.Errorf("unexpected file contents:\n%s", data)
	}
	if !strings.HasSuffix(string(data), "\nSome description.\n") {
		t.Errorf("body missing from file:\n%s", data)
	}
}

func TestCreateDefaultsPriorityToNormal(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "No priority given"})
	if tk.Priority != task.PriorityNormal {
		t.Errorf("Priority = %q, want %q", tk.Priority, task.PriorityNormal)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	st := tqtest.NewStore(t)
	for _, in := range []store.CreateTaskInput{
		{Title: ""},
		{Title: "x", Priority: "whenever"},
		{Title: "x", Status: "shipped"},
		{Title: "x", DependsOn: []string{"nope"}},
	} {
		if _, err := st.Create(in); err == nil {
			t.Errorf("Create(%+v) should have failed", in)
		}
	}
	entries, _ := os.ReadDir(st.Dir)
	if len(entries) != 0 {
		t.Errorf("no files should be written for invalid input, found %d", len(entries))
	}
}

func TestNextIDIsSequential(t *testing.T) {
	st := tqtest.NewStore(t)
	for _, want := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		got := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "task"})
		if got.ID != want {
			t.Fatalf("ID = %q, want %q", got.ID, want)
		}
	}

	// IDs continue past four digits without renumbering existing tasks, and the
	// title suffix does not confuse the scan.
	if err := os.WriteFile(filepath.Join(st.Dir, "TQ-9999-nearly-out-of-digits.md"), []byte("---\nid: TQ-9999\ntitle: x\nstatus: todo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	next, err := st.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if next != "TQ-10000" {
		t.Errorf("NextID = %q, want TQ-10000", next)
	}
}

// remove takes a task's file out of the directory the way every removal in this
// project happens: by hand. There is no tq delete and no DELETE route, so an
// `rm`, a revert or a merge is the only path, and it is exactly the path that
// leaves the highest number free with dependencies still pointing at it.
func remove(t *testing.T, st *store.Store, of task.Task) {
	t.Helper()
	if err := os.Remove(filepath.Join(st.Dir, store.TaskFileName(of))); err != nil {
		t.Fatal(err)
	}
}

// The bug itself. Removing the newest task frees its number, and handing that
// number to the next create re-points the dependency at a task that has nothing
// to do with the one that went — so a prerequisite nobody met reads as met
// (TQ-0016).
func TestANumberATaskStillDependsOnIsNotHandedOut(t *testing.T) {
	st := tqtest.NewStore(t)
	dependent := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Real task one", Status: "todo"})
	prerequisite := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Real task two"})
	if _, err := st.Patch(dependent.ID, task.TaskPatch{DependsOn: &[]string{prerequisite.ID}}); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	remove(t, st, prerequisite)

	filed := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Buy milk", Status: "done"})
	if filed.ID == prerequisite.ID {
		t.Fatalf("Create() = %s, the number %s still depends on: an unrelated task must not inherit a dependency", filed.ID, dependent.ID)
	}
	if filed.ID != "TQ-0003" {
		t.Errorf("Create() = %q, want TQ-0003: the referenced number is stepped over, not counted from", filed.ID)
	}

	// The point of stepping over it: the dependency stays unmet, so the
	// dependent is still blocked rather than being offered as available work.
	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	columns, err := st.Columns()
	if err != nil {
		t.Fatalf("Columns: %v", err)
	}
	index := task.IndexTasks(listing.Tasks)
	blocked, ok := index[dependent.ID]
	if !ok {
		t.Fatalf("List() = %+v, want it to hold %s", listing.Tasks, dependent.ID)
	}
	if task.IsReady(blocked, index, columns) {
		t.Errorf("%s is ready, want it blocked: %s was never done, it was removed", dependent.ID, prerequisite.ID)
	}
}

// A whole run of removed tasks is still referenced, so the search does not stop
// at the first free-looking number.
func TestEveryReferencedNumberInARowIsSteppedOver(t *testing.T) {
	st := tqtest.NewStore(t)
	dependent := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Depends on both"})
	first := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "First prerequisite"})
	second := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Second prerequisite"})
	if _, err := st.Patch(dependent.ID, task.TaskPatch{DependsOn: &[]string{first.ID, second.ID}}); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	remove(t, st, first)
	remove(t, st, second)

	filed := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Unrelated"})
	if filed.ID != "TQ-0004" {
		t.Errorf("Create() = %q, want TQ-0004: both %s and %s are still spoken for", filed.ID, first.ID, second.ID)
	}
}

// The other side of the rule, and the reason it is a skip rather than a
// high-water mark: a number nothing points at has no stale reference to
// re-bind, so recycling it is invisible and is left alone.
func TestANumberNothingDependsOnIsStillRecycled(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
	newest := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Remove me"})
	remove(t, st, newest)

	filed := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Take the number"})
	if filed.ID != newest.ID {
		t.Errorf("Create() = %q, want %q reused: nothing pointed at it, so there was nothing to protect", filed.ID, newest.ID)
	}
}

// A dependency is matched as a whole string everywhere else — IndexTasks keys
// by it — so a reference tq would never write matches nothing and reserves
// nothing. Skipping on the number instead would burn TQ-0002 for a `TQ-2` that
// could never have bound to it.
func TestAReferenceSpelledUnlikeAnIDReservesNothing(t *testing.T) {
	st := tqtest.NewStore(t)
	only := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Odd reference"})
	if _, err := st.Patch(only.ID, task.TaskPatch{DependsOn: &[]string{"TQ-2"}}); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	next, err := st.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if next != "TQ-0002" {
		t.Errorf("NextID() = %q, want TQ-0002: TQ-2 is not the ID any task would be given, so nothing could bind to it", next)
	}
}

// A task withheld from the listing as one of two files claiming an ID is still
// read here: it parses, and the dependencies in it are real. Withholding it
// from the listing is about which of two files a reader means — it says nothing
// about whether the pointer inside them could re-bind.
func TestAReferenceInsideADuplicatedTaskStillReservesItsNumber(t *testing.T) {
	st := tqtest.NewStore(t)
	dependent := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Doubled"})
	prerequisite := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Removed"})
	if _, err := st.Patch(dependent.ID, task.TaskPatch{DependsOn: &[]string{prerequisite.ID}}); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	remove(t, st, prerequisite)
	reloaded, err := st.Get(dependent.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	duplicate(t, st, reloaded, dependent.ID+"-a-second-file.md")

	next, err := st.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if next != "TQ-0003" {
		t.Errorf("NextID() = %q, want TQ-0003: %s is spoken for by a file the listing withholds but the store can read", next, prerequisite.ID)
	}
}

// The accepted residue, asserted so it is a decision and not a surprise: a file
// that will not parse keeps its depends_on to itself, so a reference inside one
// reserves nothing. Nothing is offered on the strength of such a file — it is
// withheld from every listing and reported instead, tq ready never proposes it
// and tq show refuses it — but the damage is still reachable in one order: the
// merge that leaves the conflict markers also removes the task the dependency
// named, a create takes the freed number while the file is unreadable, and the
// dependency binds to it when the conflict is resolved. Closing that would mean
// guessing which TQ-#### in an unparseable file was a dependency and which was
// prose, so the answer is the loud report and not a guess.
func TestAReferenceInsideAnUnreadableFileReservesNothing(t *testing.T) {
	st := tqtest.NewStore(t)
	dependent := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Half merged", Status: "todo"})
	prerequisite := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Removed"})
	if _, err := st.Patch(dependent.ID, task.TaskPatch{DependsOn: &[]string{prerequisite.ID}}); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	remove(t, st, prerequisite)

	// The only file that names the removed number, and it will not parse.
	broken := store.TaskFileName(dependent)
	if err := os.WriteFile(filepath.Join(st.Dir, broken), []byte("<<<<<<< HEAD\ndepends_on:\n  - "+prerequisite.ID+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	filed := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Buy milk"})
	if filed.ID != prerequisite.ID {
		t.Errorf("Create() = %q, want %q: a reference tq cannot read cannot reserve anything, and this is the accepted hole", filed.ID, prerequisite.ID)
	}

	// What tq does instead is refuse to pass the file off as a task: it is
	// named, it is not in the listing, and so it is never offered as work.
	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Unreadable) != 1 || listing.Unreadable[0].File != broken {
		t.Fatalf("Unreadable = %+v, want it to name %s: the dependency is only hidden while nobody is told", listing.Unreadable, broken)
	}
	if _, held := task.IndexTasks(listing.Tasks)[dependent.ID]; held {
		t.Errorf("List() holds %s, want it withheld: a task nobody can read is a task nobody can be handed", dependent.ID)
	}
}

// A directory answering to a task file's name is not a task and nothing else in
// the store looks at it, but it does occupy the name — and a create handed its
// number could never link that name, spending every retry deriving the same one
// and blaming task IDs for an entry it never mentioned (TQ-0039). So the number
// counts, even though the entry is never read.
func TestAnEntryOccupyingATaskFileNameStillTakesItsNumber(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
	if err := os.Mkdir(filepath.Join(st.Dir, "TQ-0002-not-a-task.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	next, err := st.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if next != "TQ-0003" {
		t.Fatalf("NextID() = %q, want TQ-0003: TQ-0002 is a name a create could never link", next)
	}

	// And it is never opened: a directory is not a task file the queue is short.
	filed := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Not a task"})
	if filed.ID != "TQ-0003" {
		t.Errorf("Create() = %q, want TQ-0003", filed.ID)
	}
	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Unreadable) != 0 {
		t.Errorf("Unreadable = %+v, want nothing: a directory is not a broken task file", listing.Unreadable)
	}
}

func TestGet(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Findable", Body: "Body text."})

	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "Findable" || got.Body != "Body text." {
		t.Errorf("Get returned %+v", got)
	}

	if _, err := st.Get("TQ-4242"); !errors.Is(err, store.ErrTaskNotFound) {
		t.Errorf("Get(missing) = %v, want ErrTaskNotFound", err)
	}
	if _, err := st.Get("not-an-id"); err == nil {
		t.Error("Get should reject malformed IDs")
	}
}

// listTasks is List for a test whose premise is that every file reads. A
// skipped file is now reported rather than returned as an error, so without
// this a test could lose a task and still pass.
func listTasks(t *testing.T, st *store.Store) []task.Task {
	t.Helper()
	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Unreadable) != 0 {
		t.Fatalf("List skipped %+v, want every file readable", listing.Unreadable)
	}
	if listing.Incomplete {
		t.Fatalf("List could not square its scan with a directory nothing is writing to")
	}
	return listing.Tasks
}

func TestListSortsAndReportsBadFiles(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	second := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "second", Priority: task.PriorityUrgent})

	tasks := listTasks(t, st)
	if len(tasks) != 2 || tasks[0].ID != second.ID {
		t.Errorf("List should sort urgent first, got %+v", tasks)
	}

	// Files that are not tasks are ignored.
	if err := os.WriteFile(filepath.Join(st.Dir, "README.md"), []byte("not a task"), 0o644); err != nil {
		t.Fatal(err)
	}
	if tasks = listTasks(t, st); len(tasks) != 2 {
		t.Errorf("List() = %d tasks; want 2", len(tasks))
	}

	// A malformed task file is skipped and named, and the healthy tasks still
	// come back: one broken file must not hide the queue (TQ-0011).
	if err := os.WriteFile(filepath.Join(st.Dir, "TQ-0003.md"), []byte("no frontmatter here"), 0o644); err != nil {
		t.Fatal(err)
	}
	listing, err := st.List()
	if err != nil {
		t.Fatalf("List with a broken file: %v", err)
	}
	if len(listing.Tasks) != 2 {
		t.Errorf("List() = %d tasks, want the 2 healthy ones", len(listing.Tasks))
	}
	if len(listing.Unreadable) != 1 || listing.Unreadable[0].File != "TQ-0003.md" {
		t.Fatalf("Unreadable = %+v, want it to name TQ-0003.md", listing.Unreadable)
	}
	if !strings.Contains(listing.Unreadable[0].Reason, "frontmatter") {
		t.Errorf("reason = %q, want it to say what is wrong", listing.Unreadable[0].Reason)
	}
	// The reason is rendered beside the name, so it does not repeat it.
	if strings.Contains(listing.Unreadable[0].Reason, "TQ-0003.md") {
		t.Errorf("reason = %q, want the file name left to the File field", listing.Unreadable[0].Reason)
	}
}

// The shapes a hand-edited or merge-conflicted file arrives in. Each is one
// unreadable file among healthy ones, and each must cost only itself.
func TestListSkipsEveryShapeOfBrokenFile(t *testing.T) {
	broken := map[string]string{
		"TQ-0002-extra-key.md":         "---\nid: TQ-0002\ntitle: extra key\nstatus: todo\npriority: normal\nepic: platform\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n",
		"TQ-0003-conflicted.md":        "<<<<<<< HEAD\n---\nid: TQ-0003\ntitle: mine\nstatus: todo\n=======\n---\nid: TQ-0003\ntitle: theirs\nstatus: done\n>>>>>>> branch\n---\n",
		"TQ-0004-id-mismatch.md":       "---\nid: TQ-0009\ntitle: mismatched\nstatus: todo\npriority: normal\ncreated: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n",
		"TQ-0005-unterminated.md":      "---\nid: TQ-0005\ntitle: unterminated\nstatus: todo\n",
		"TQ-0006-invalid-yaml.md":      "---\nid: TQ-0006\ntitle: [unclosed\nstatus: todo\n---\n",
		"TQ-0007-missing-fields.md":    "---\nnothing: here\n---\n",
		"TQ-0008-not-a-task-at-all.md": "just some prose that ended up in .tasks\n",
	}

	for name, content := range broken {
		t.Run(name, func(t *testing.T) {
			st := tqtest.NewStore(t)
			healthy := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "healthy"})
			if err := os.WriteFile(filepath.Join(st.Dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}

			listing, err := st.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(listing.Tasks) != 1 || listing.Tasks[0].ID != healthy.ID {
				t.Errorf("List() = %+v, want the healthy task", listing.Tasks)
			}
			if len(listing.Unreadable) != 1 || listing.Unreadable[0].File != name {
				t.Fatalf("Unreadable = %+v, want it to name %s", listing.Unreadable, name)
			}
			if listing.Unreadable[0].Reason == "" {
				t.Error("Unreadable carries no reason, so nothing can say what to fix")
			}
		})
	}
}

// A name whose file is not there is not a broken file: `tq update --title`
// moves the task's file to the name its title asks for, and `tq delete`
// unlinks, so a scan that caught the old name has nothing to report. A dangling
// symlink is that state, held still — the directory is not moving, so there is
// nothing for the consistency check to retry either.
func TestListDoesNotReportAFileThatIsNoLongerThere(t *testing.T) {
	st := tqtest.NewStore(t)
	healthy := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "healthy"})

	link := filepath.Join(st.Dir, "TQ-0009-vanished.md")
	if err := os.Symlink(filepath.Join(st.Dir, "TQ-0009-gone.md"), link); err != nil {
		t.Fatal(err)
	}

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Tasks) != 1 || listing.Tasks[0].ID != healthy.ID {
		t.Errorf("List() = %+v, want the healthy task", listing.Tasks)
	}
	if len(listing.Unreadable) != 0 {
		t.Errorf("Unreadable = %+v, want nothing: a file that is gone is not a file that is broken", listing.Unreadable)
	}
}

// ── A listing against a directory that is being written to (TQ-0012) ──────
//
// These drive the race rather than wait for it: DuringScan runs inside the
// window a listing is blind in — the directory has been read, the files have
// not — so the interleaving is exact and the test cannot be flaky.

// A retitle moves a task to a new file. Caught mid-scan, the name the listing
// read is gone and the name the task now lives under was never read, so the
// pass is a task short and cannot tell. The check against the directory is
// what catches it, and the retry is what fixes it.
func TestAListingIsNotShortWhenATaskIsRenamedMidScan(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	retitled := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "second"})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "third"})

	scans := 0
	st.DuringScan(func() {
		scans++
		if scans > 1 {
			return // the writer is done; the directory holds still now
		}
		rename(t, st, store.TaskFileName(retitled), retitled.ID+"-second-retitled.md")
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listing.Incomplete {
		t.Error("Incomplete = true, want the retry to have settled on a directory that stopped moving")
	}
	if len(listing.Tasks) != 3 {
		t.Fatalf("List() = %d tasks, want all 3: a rename must not drop the task it renamed\n%+v", len(listing.Tasks), listing.Tasks)
	}
	if scans != 2 {
		t.Errorf("scanned %d times, want 2: one pass the rename spoiled and one that stood", scans)
	}
	if len(listing.Unreadable) != 0 {
		t.Errorf("Unreadable = %+v, want nothing: a retitle is not a broken file", listing.Unreadable)
	}
}

// The other half of the same blindness: a task filed after the directory was
// read is in no name the pass looked at, so it is missing with nothing to
// report at all. Only the second reading of the directory can see it.
func TestAListingIsNotShortWhenATaskIsCreatedMidScan(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})

	scans := 0
	st.DuringScan(func() {
		scans++
		if scans > 1 {
			return
		}
		tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "filed mid-scan"})
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listing.Incomplete {
		t.Error("Incomplete = true, want the retry to have settled")
	}
	if len(listing.Tasks) != 2 {
		t.Fatalf("List() = %d tasks, want both: the second was created mid-scan\n%+v", len(listing.Tasks), listing.Tasks)
	}
}

// A directory can hold two files for one ID for an instant, and a pass that
// reads them both holds the task twice. The instant can also fall between both
// readings of the directory, where the entry set says nothing and only the
// doubled ID does — that is the case
// TestARetitleHoldingAWholePassStillGetsItsRetry drives.
func TestAListingDoesNotHoldATaskTwiceWhileItIsBeingRetitled(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	retitled := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "second"})

	old := store.TaskFileName(retitled)
	fresh := retitled.ID + "-second-retitled.md"
	scans := 0
	st.DuringScan(func() {
		scans++
		switch scans {
		case 1:
			// Mid-retitle: the new file is written and the old one is still
			// there, and it stays that way across both readings.
			content, err := os.ReadFile(filepath.Join(st.Dir, old))
			if err != nil {
				t.Error(err)
				return
			}
			if err := os.WriteFile(filepath.Join(st.Dir, fresh), content, 0o644); err != nil {
				t.Error(err)
			}
		case 2:
			// The retitle finishes, the way `tq update` finishes it.
			if err := os.Remove(filepath.Join(st.Dir, old)); err != nil {
				t.Error(err)
			}
		}
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listing.Incomplete {
		t.Error("Incomplete = true, want the retry to have settled once the retitle finished")
	}
	if len(listing.Tasks) != 2 {
		t.Fatalf("List() = %d tasks, want 2: a task caught under two names must not be listed twice\n%+v", len(listing.Tasks), listing.Tasks)
	}
	if listing.Tasks[0].ID == listing.Tasks[1].ID {
		t.Errorf("List() holds %s twice", listing.Tasks[0].ID)
	}
}

// ── An ID two files claim (TQ-0040) ──────────────────────────────────────
//
// A pair that never resolves is not a directory in motion but a queue to fix.
// Both copies are withheld, the ID is reported, and the retries stop as soon as
// the pair has outlived a pass the directory held completely still for.

// duplicate copies a task's file under a second name claiming the same ID: an
// interrupted retitle, a hand-copied file, two branches merged.
func duplicate(t *testing.T, st *store.Store, of task.Task, as string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(st.Dir, store.TaskFileName(of)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir, as), content, 0o644); err != nil {
		t.Fatal(err)
	}
	return as
}

func TestAnIDTwoFilesClaimIsWithheldFromTheListingAndReported(t *testing.T) {
	st := tqtest.NewStore(t)
	doubled := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	healthy := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "second"})

	original := store.TaskFileName(doubled)
	copied := duplicate(t, st, doubled, doubled.ID+"-a-second-file.md")

	scans := 0
	st.DuringScan(func() { scans++ })

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Tasks) != 1 || listing.Tasks[0].ID != healthy.ID {
		t.Fatalf("List() = %+v, want only %s: neither copy of an ambiguous ID belongs in a listing", listing.Tasks, healthy.ID)
	}
	if len(listing.Duplicated) != 1 || listing.Duplicated[0].ID != doubled.ID {
		t.Fatalf("Duplicated = %+v, want it to name %s", listing.Duplicated, doubled.ID)
	}
	if got, want := listing.Duplicated[0].Files, []string{copied, original}; !slices.Equal(got, want) {
		t.Errorf("Files = %q, want both files in name order %q", got, want)
	}
	for _, name := range listing.Duplicated[0].Files {
		if !strings.Contains(listing.Duplicated[0].Reason, name) {
			t.Errorf("reason = %q, want it to name %s: fixing this means choosing between the files", listing.Duplicated[0].Reason, name)
		}
	}
	if listing.Incomplete {
		t.Error("Incomplete = true, want false: nothing is writing to this queue, and the listing knows exactly what it withheld")
	}
	if len(listing.Unreadable) != 0 {
		t.Errorf("Unreadable = %+v, want nothing: both files parse perfectly well", listing.Unreadable)
	}
	if scans >= store.ListAttempts {
		t.Errorf("scanned %d times, want fewer than %d: a pair that is simply there is not a race to retry", scans, store.ListAttempts)
	}
}

// The complaint the ticket opens with is that the two surfaces disagree: a
// listing showed the pair and a lookup refused it. They are one finding and
// must read as one sentence.
func TestAListingAndALookupSayTheSameThingAboutADuplicatedID(t *testing.T) {
	st := tqtest.NewStore(t)
	doubled := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	duplicate(t, st, doubled, doubled.ID+"-a-second-file.md")

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Duplicated) != 1 {
		t.Fatalf("Duplicated = %+v, want the one ID", listing.Duplicated)
	}

	_, err = st.Locate(doubled.ID)
	if !errors.Is(err, task.ErrInvalidTaskFile) {
		t.Fatalf("Locate = %v, want ErrInvalidTaskFile", err)
	}
	if !strings.Contains(err.Error(), listing.Duplicated[0].Reason) {
		t.Errorf("locate says %q, the listing says %q; they are the same finding", err, listing.Duplicated[0].Reason)
	}
}

// The other side of the same rule, and the TQ-0012 behaviour that must
// survive: a retitle whose two files are there for a whole pass — both
// readings of the directory and every file read between them — gets its retry
// rather than being reported. The pair is planted before the listing starts,
// so the first pass sees no other change at all and the doubled ID is the only
// reason to go round again; the retitle then finishes the way `tq update`
// finishes it.
func TestARetitleHoldingAWholePassStillGetsItsRetry(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	retitled := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "second"})

	old := store.TaskFileName(retitled)
	duplicate(t, st, retitled, retitled.ID+"-second-retitled.md")

	scans := 0
	st.DuringScan(func() {
		scans++
		if scans < 2 {
			return // the first pass sees the retitle mid-flight
		}
		if err := os.Remove(filepath.Join(st.Dir, old)); err != nil && !os.IsNotExist(err) {
			t.Error(err)
		}
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if scans < 2 {
		t.Fatalf("scanned %d times, want the pair looked at again: a retitle in flight is not a queue to fix", scans)
	}
	if len(listing.Duplicated) != 0 {
		t.Errorf("Duplicated = %+v, want nothing: the retitle finished, and a listing must not report a pair that is gone", listing.Duplicated)
	}
	if listing.Incomplete {
		t.Error("Incomplete = true, want the retry to have settled once the retitle finished")
	}
	if len(listing.Tasks) != 2 {
		t.Fatalf("List() = %d tasks, want both: a retitle must not cost the task it renamed\n%+v", len(listing.Tasks), listing.Tasks)
	}
}

// A pair the passes only ever saw while the directory was moving is not a pair
// they can condemn: a retitle nobody finishes in time looks exactly like that,
// and "keep the one you want" would be telling someone to delete a file when
// nothing was wrong. The ID is still withheld — a listing holds one once or not
// at all — and what the listing says is that it may be a task short.
func TestAPairOnlySeenWhileTheDirectoryMovedIsNotCondemned(t *testing.T) {
	st := tqtest.NewStore(t)
	retitled := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})

	scans := 0
	st.DuringScan(func() {
		scans++
		if scans == 1 {
			// The retitle's new file, written and never followed by the
			// removal of the old one.
			duplicate(t, st, retitled, retitled.ID+"-first-retitled.md")
		}
		if scans >= store.ListAttempts {
			return // the last pass sees the pair against a directory at rest
		}
		// Until then the directory never settles around the pair, so no two
		// passes at rest ever agree it is there.
		tqtest.MustCreate(t, st, store.CreateTaskInput{Title: fmt.Sprintf("churn %d", scans)})
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Duplicated) != 0 {
		t.Errorf("Duplicated = %+v, want nothing: no pass ever saw that pair against a directory that held still", listing.Duplicated)
	}
	if !listing.Incomplete {
		t.Error("Incomplete = false: a listing this short has to say so somehow")
	}
	for i, t1 := range listing.Tasks {
		for _, t2 := range listing.Tasks[i+1:] {
			if t1.ID == t2.ID {
				t.Fatalf("List() holds %s twice; an ID is in a listing once or not at all", t1.ID)
			}
		}
	}
}

// A directory nobody stops writing to cannot be read consistently, and the
// listing says so rather than passing off a short list as the whole queue. It
// is still a listing: the tasks it did read come back, and it is not an error.
func TestAListingThatCannotBeSquaredWithTheDirectorySaysSo(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})
	churned := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "second"})

	scans := 0
	current := store.TaskFileName(churned)
	st.DuringScan(func() {
		scans++
		next := fmt.Sprintf("%s-retitled-%d.md", churned.ID, scans)
		rename(t, st, current, next)
		current = next
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !listing.Incomplete {
		t.Error("Incomplete = false: a listing that could never be squared with the directory must not pass as the whole queue")
	}
	if scans != store.ListAttempts {
		t.Errorf("scanned %d times, want %d: the retry is bounded", scans, store.ListAttempts)
	}
	if len(listing.Tasks) == 0 {
		t.Error("List() = no tasks, want the ones it could read: this is a warning, not a failure")
	}
	if len(listing.Unreadable) != 0 {
		t.Errorf("Unreadable = %+v, want nothing: a file being renamed is not a file that is broken", listing.Unreadable)
	}
}

// A file that cannot be parsed does not move the directory, so it is reported
// once and the scan is not run again: the retry is for a directory that
// changed, and a broken file would otherwise cost every listing three passes
// over the whole queue (TQ-0011, kept by TQ-0012).
func TestABrokenFileIsReportedWithoutRetryingTheScan(t *testing.T) {
	st := tqtest.NewStore(t)
	healthy := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "healthy"})
	if err := os.WriteFile(filepath.Join(st.Dir, "TQ-0002-broken.md"), []byte("no frontmatter here"), 0o644); err != nil {
		t.Fatal(err)
	}

	scans := 0
	st.DuringScan(func() { scans++ })

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if scans != 1 {
		t.Errorf("scanned %d times, want 1: a broken file must not send the listing round again", scans)
	}
	if listing.Incomplete {
		t.Error("Incomplete = true, want false: the directory never moved")
	}
	if len(listing.Tasks) != 1 || listing.Tasks[0].ID != healthy.ID {
		t.Errorf("List() = %+v, want the healthy task", listing.Tasks)
	}
	if len(listing.Unreadable) != 1 || listing.Unreadable[0].File != "TQ-0002-broken.md" {
		t.Errorf("Unreadable = %+v, want it to name TQ-0002-broken.md", listing.Unreadable)
	}
}

// A write leaves a temporary file beside the tasks for an instant. It is not a
// task appearing, and a listing that treated it as one would go round again on
// every write in the project.
func TestATemporaryFileDoesNotSendTheListingRoundAgain(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})

	scans := 0
	st.DuringScan(func() {
		scans++
		if scans > 1 {
			return
		}
		if err := os.WriteFile(filepath.Join(st.Dir, ".tq-1234.tmp"), []byte("half a task"), 0o644); err != nil {
			t.Error(err)
		}
	})

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if scans != 1 {
		t.Errorf("scanned %d times, want 1: a temporary file is not a task appearing", scans)
	}
	if listing.Incomplete || len(listing.Tasks) != 1 {
		t.Errorf("List() = %+v (incomplete=%v), want the one task", listing.Tasks, listing.Incomplete)
	}
}

// rename moves one task file to another name, the way a retitle does.
func rename(t *testing.T, st *store.Store, from, to string) {
	t.Helper()
	if err := os.Rename(filepath.Join(st.Dir, from), filepath.Join(st.Dir, to)); err != nil {
		t.Fatalf("renaming %s to %s: %v", from, to, err)
	}
}

// A byte-order mark is invisible in an editor and some of them write one. The
// file is otherwise perfect, so it parses (TQ-0011).
func TestListReadsAFileWithAByteOrderMark(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "marked"})

	name := store.TaskFileName(created)
	path := filepath.Join(st.Dir, name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append([]byte("\ufeff"), content...), 0o644); err != nil {
		t.Fatal(err)
	}

	tasks := listTasks(t, st)
	if len(tasks) != 1 || tasks[0].ID != created.ID {
		t.Errorf("List() = %+v, want the task read straight through the mark", tasks)
	}
}

// A retitle ends with the task under its new name and nothing else in the
// directory: no staging file, no leftover under the old name. What the content
// itself arrives by is TestUpdateLandsTheNewContentInOneStep, and the timestamp
// the save stamps is TestUpdateRefreshesTheUpdatedTimestamp.
func TestUpdateRewritesFileAtomically(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Original", Body: "Body."})

	created.Title = "Renamed"
	created.Status = task.StatusInProgress
	if _, err := st.Update(created); err != nil {
		t.Fatalf("Update: %v", err)
	}

	reloaded, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.Title != "Renamed" || reloaded.Status != task.StatusInProgress || reloaded.Body != "Body." {
		t.Errorf("reloaded = %+v", reloaded)
	}

	// The rewrite leaves exactly one valid file behind (no temp files, and no
	// leftover from the old title).
	entries, err := os.ReadDir(st.Dir)
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

// wantReadableByAll checks the mode a save chmods its staging file to, which is
// what keeps a task readable by more than the account that wrote it whatever
// umask that account runs under. Windows has no POSIX mode bits and Go reports
// 0666 for anything writable there, so there is nothing to check.
func wantReadableByAll(t *testing.T, info os.FileInfo) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("mode = %v, want -rw-r--r--", perm)
	}
}

// The guarantee README states about a save: the new content arrives in one
// step, so nothing ever reads half of it. Three things say so here, and each
// fails on a different way of losing it — the task's name holds a different
// file afterwards (a write into the old one would keep it), that file is the
// one the content was written into elsewhere (a copy would not be), and a
// reader holding the old file across the save still reads the whole of the old
// content.
//
// What is not covered is the fsync in Store.stage. Whether the bytes reached
// the platter before the rename made them the task only shows up in a crash,
// and no assertion inside the test process can see it, so it is left untested
// on purpose rather than faked (TQ-0022).
func TestUpdateLandsTheNewContentInOneStep(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Stable title", Body: "The body as it was."})
	path := filepath.Join(st.Dir, store.TaskFileName(created))

	was, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	var staged os.FileInfo
	st.DuringStage(func(tmpName string) {
		info, err := os.Stat(tmpName)
		if err != nil {
			t.Errorf("staged file: %v", err)
			return
		}
		staged = info
		if got, err := os.ReadFile(path); err != nil || string(got) != string(was) {
			t.Errorf("the task's file holds %q (err %v) while the new content is being written, want the old content whole", got, err)
		}
	})

	created.Body = "The body as it is now, which is longer than it was."
	if _, err := st.Update(created); err != nil {
		t.Fatalf("Update: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Error("the task's name still holds the file it held: the content was written into it rather than moved onto it")
	}
	wantReadableByAll(t, after)
	if staged == nil {
		t.Fatal("nothing was staged: the new content went into the task's own file, where a reader can catch it half-written")
	}
	if !os.SameFile(staged, after) {
		t.Error("the file under the task's name is not the one the content was written into")
	}

	held, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(held) != string(was) {
		t.Errorf("a reader holding the file across the save read %q, want the old content %q", held, was)
	}
}

// The same guarantee for a create, which places its file by linking rather than
// renaming: the name a task is about to take never holds a partial file, and
// what appears under it is the file the content was written into.
func TestCreateLandsTheWholeFileAtOnce(t *testing.T) {
	st := tqtest.NewStore(t)
	const name = "TQ-0001-first-task.md"
	path := filepath.Join(st.Dir, name)

	var staged os.FileInfo
	st.DuringStage(func(tmpName string) {
		info, err := os.Stat(tmpName)
		if err != nil {
			t.Errorf("staged file: %v", err)
			return
		}
		staged = info
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("os.Stat(%s) = %v while the content is still being written, want it not to exist yet", name, err)
		}
	})

	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "First task", Body: "Body."})
	if got := store.TaskFileName(created); got != name {
		t.Fatalf("TaskFileName = %q, want %q", got, name)
	}
	landed, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	wantReadableByAll(t, landed)
	if staged == nil {
		t.Fatal("nothing was staged: the task was written into its own name, where a reader can catch it half-written")
	}
	if !os.SameFile(staged, landed) {
		t.Error("the file at the task's name is not the one the content was written into: it was copied there rather than placed")
	}

	entries, err := os.ReadDir(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != name {
		t.Errorf("directory contains %v, want only %s", names(entries), name)
	}
}

// A caller reads a task, edits it and hands it back, so the Updated a save
// receives is whatever was on disk — here a value from long ago, which is what
// makes the refresh visible without waiting for a second to pass. The save is
// what replaces it with now; Created is not its to touch.
func TestUpdateRefreshesTheUpdatedTimestamp(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Original", Body: "Body."})

	stale := time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC)
	edit := created
	edit.Updated = stale
	edit.Body = "Rewritten."

	floor := time.Now().Truncate(time.Second)
	if _, err := st.Update(edit); err != nil {
		t.Fatalf("Update: %v", err)
	}

	reloaded, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if reloaded.Updated.Before(floor) {
		t.Errorf("Updated = %s, want the time of the save (%s or later): the save carried the caller's value through instead of stamping now",
			reloaded.Updated.Format(time.RFC3339), floor.Format(time.RFC3339))
	}
	if !reloaded.Created.Equal(created.Created) {
		t.Errorf("Created = %s, want it left at %s",
			reloaded.Created.Format(time.RFC3339), created.Created.Format(time.RFC3339))
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
		tk := task.Task{ID: "TQ-0001", Title: tc.title, Status: task.StatusTodo}
		if got := store.TaskFileName(tk); got != tc.want {
			t.Errorf("TaskFileName(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestGetFindsTasksWhateverTheSuffix(t *testing.T) {
	st := tqtest.NewStore(t)
	// A file written before titles were part of the filename.
	legacy := "---\nid: TQ-0001\ntitle: Written by an older version\nstatus: todo\n---\n\nBody.\n"
	if err := os.WriteFile(filepath.Join(st.Dir, "TQ-0001.md"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	tk, err := st.Get("TQ-0001")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if tk.Title != "Written by an older version" {
		t.Errorf("task = %+v", tk)
	}

	if tasks := listTasks(t, st); len(tasks) != 1 {
		t.Errorf("List() = %d tasks; want 1", len(tasks))
	}

	// Touching it adopts the new naming.
	if _, err := st.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "TQ-0001-written-by-an-older-version.md")); err != nil {
		t.Errorf("update should have renamed the file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(st.Dir, "TQ-0001.md")); err == nil {
		t.Error("the old filename should be gone")
	}
}

func TestUpdateKeepsTheFilenameWhenTheTitleIsUnchanged(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Stable title"})

	tk.Status = task.StatusInProgress
	if _, err := st.Update(tk); err != nil {
		t.Fatalf("Update: %v", err)
	}

	entries, err := os.ReadDir(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "TQ-0001-stable-title.md" {
		t.Errorf("directory contains %v, want only TQ-0001-stable-title.md", entries)
	}
}

func TestDuplicateFilesForOneIDAreRejected(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Original"})

	// A half-finished manual rename leaves two files claiming one ID.
	rendered, err := task.RenderTask(tk)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir, "TQ-0001-copy.md"), rendered, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = st.Get("TQ-0001")
	if !errors.Is(err, task.ErrInvalidTaskFile) {
		t.Errorf("Get = %v, want task.ErrInvalidTaskFile", err)
	}
	for _, name := range []string{"TQ-0001-original.md", "TQ-0001-copy.md"} {
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("error %v should name %s", err, name)
		}
	}
}

// Update normalizes an unknown status to the first column on write.
func TestUpdateCorrectsAStatusTheBoardDoesNotHave(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Valid"})
	created.Status = "shipped"

	written, err := st.Update(created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if written.Status != task.StatusInbox {
		t.Errorf("Status = %q, want it corrected to the first column", written.Status)
	}

	reloaded, err := st.Get("TQ-0001")
	if err != nil || reloaded.Status != task.StatusInbox {
		t.Errorf("the correction should have reached the file, got %+v (%v)", reloaded, err)
	}
}

func TestUpdateUnknownTask(t *testing.T) {
	st := tqtest.NewStore(t)
	ghost := task.Task{ID: "TQ-0404", Title: "ghost", Status: task.StatusTodo}
	if _, err := st.Update(ghost); !errors.Is(err, store.ErrTaskNotFound) {
		t.Errorf("Update(missing) = %v, want ErrTaskNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	st := tqtest.NewStore(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Temporary"})
	if err := st.Delete("TQ-0001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get("TQ-0001"); !errors.Is(err, store.ErrTaskNotFound) {
		t.Errorf("Get after Delete = %v, want ErrTaskNotFound", err)
	}
	if err := st.Delete("TQ-0001"); !errors.Is(err, store.ErrTaskNotFound) {
		t.Errorf("Delete(missing) = %v, want ErrTaskNotFound", err)
	}
}

func TestDiscoverTaskDirWalksUp(t *testing.T) {
	root := tqtest.Root(t)
	if _, err := store.InitStore(root); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := store.DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(root, config.TaskDirName); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// With no marker to find, the message has to say what was looked for, where,
// and what to run about it: "no task queue found" alone leaves a caller with
// nothing to do next.
func TestDiscoverTaskDirNotFound(t *testing.T) {
	root := tqtest.RootWithoutMarker(t)

	_, err := store.DiscoverTaskDir(root)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
	for _, want := range []string{config.ConfigFileName, root, "tq init"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

// A marker whose task directory is missing is the one case that knows where
// init belongs, and a subdirectory is where following a bare hint would fork a
// second project. The message has to name the marker's own directory there.
func TestDiscoverTaskDirSaysWhereToInitialiseAMissingQueue(t *testing.T) {
	root := tqtest.Root(t)
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := store.DiscoverTaskDir(nested)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
	if want := `run "tq init" in ` + root; !strings.Contains(err.Error(), want) {
		t.Errorf("err = %q, want it to say %q rather than a bare hint that would fork a queue here", err, want)
	}
}

// A directory outside the home directory cannot reach the bound, so the walk
// runs out at the filesystem root instead and says so without naming a stop it
// never made.
func TestDiscoverTaskDirOutsideTheHomeDirectoryRunsOutOfTree(t *testing.T) {
	root := tqtest.RootWithoutMarker(t)
	// A home directory the fixture is demonstrably not under, so the branch is
	// the one under test rather than an accident of where TMPDIR points.
	t.Setenv("HOME", filepath.Join(tqtest.RootWithoutMarker(t), "home"))

	_, err := store.DiscoverTaskDir(root)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
	abs, absErr := filepath.Abs(root)
	if absErr != nil {
		t.Fatal(absErr)
	}
	want := fmt.Sprintf("%s: no %s in %s or any parent directory; run \"tq init\" to create one",
		store.ErrProjectNotFound, config.ConfigFileName, abs)
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err, want)
	}
}

// TQ_CONFIG_PATH hands a command its marker instead of the walk finding one, so
// the queue is whatever that marker declares, from wherever the command runs.
func TestDiscoverTaskDirConfigPathOverride(t *testing.T) {
	root := tqtest.Root(t)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := tqtest.Root(t)

	t.Setenv(config.EnvConfigPath, filepath.Join(root, config.ConfigFileName))
	dir, err := store.DiscoverTaskDir(elsewhere)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != st.Dir {
		t.Errorf("dir = %q, want the queue the %s marker declares, %q", dir, config.EnvConfigPath, st.Dir)
	}
}

// A marker the variable names but tq cannot use is an error, never an absence:
// someone who pointed tq at a file meant that file, and walking somewhere else
// instead would put the command on a queue they did not ask for.
func TestConfigPathOverrideRefusesWhatItCannotRead(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(t *testing.T, root string) string
	}{
		{"missing", func(t *testing.T, root string) string {
			return filepath.Join(root, "nowhere", config.ConfigFileName)
		}},
		{"a directory", func(t *testing.T, root string) string {
			dir := filepath.Join(root, "adir")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			return dir
		}},
		{"unparsable", func(t *testing.T, root string) string {
			return tqtest.WriteConfig(t, root, "version: [1,\n")
		}},
		{"a version this tq cannot read", func(t *testing.T, root string) string {
			return tqtest.WriteConfig(t, root, "version: 99\n")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tqtest.RootWithoutMarker(t)
			from := tqtest.RootWithoutMarker(t)
			t.Setenv(config.EnvConfigPath, tc.set(t, root))

			_, err := store.OpenStore(from)
			if !errors.Is(err, config.ErrConfig) {
				t.Fatalf("OpenStore = %v, want it to wrap config.ErrConfig", err)
			}
			if errors.Is(err, store.ErrProjectNotFound) {
				t.Errorf("err = %v, must not read as a queue that is simply not there", err)
			}

			// And init must not have made anything out of it either.
			if _, err := store.InitStore(from); !errors.Is(err, config.ErrConfig) {
				t.Errorf("InitStore = %v, want it to wrap config.ErrConfig", err)
			}
			if entries, err := os.ReadDir(from); err != nil || len(entries) != 0 {
				t.Errorf("%s holds %d entries, %v; want nothing created", from, len(entries), err)
			}
		})
	}
}

// aliasOnDisk renames a task's file to a name that differs only in spelling.
// It reports whether the filesystem folded the two names into one entry, which
// is what makes the alias dangerous: on a case-sensitive, byte-exact
// filesystem they are simply two files and there is nothing to test.
func aliasOnDisk(t *testing.T, st *store.Store, id, alias string) bool {
	t.Helper()
	current, err := st.Locate(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(st.Dir, current), filepath.Join(st.Dir, alias)); err != nil {
		t.Fatal(err)
	}
	aliased, err := st.Locate(id)
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
			st := tqtest.NewStore(t)
			tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: tt.title})
			if got := store.TaskFileName(tk); got != tt.canonical {
				t.Fatalf("TaskFileName = %q, want %q", got, tt.canonical)
			}
			if !aliasOnDisk(t, st, tk.ID, tt.alias) {
				t.Skipf("this filesystem keeps %q and %q apart, so there is no alias to lose", tt.canonical, tt.alias)
			}

			tk.Status = task.StatusInProgress
			if _, err := st.Update(tk); err != nil {
				t.Fatalf("Update: %v", err)
			}

			reloaded, err := st.Get(tk.ID)
			if err != nil {
				t.Fatalf("the task should have survived the update: %v", err)
			}
			if reloaded.Status != task.StatusInProgress {
				t.Errorf("Status = %q, want %q", reloaded.Status, task.StatusInProgress)
			}
			// The alias is a stale title suffix like any other: it converges.
			entries, err := os.ReadDir(st.Dir)
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

// ── A save moves the task's file, it does not replace it (TQ-0015) ──────────

// moveTaskFile is another writer moving a task's file, from inside the window
// where this store has already located it. It is the interleaving that used to
// leave two files: the save held a name that was no longer there.
func moveTaskFile(t *testing.T, st *store.Store, id, to string) {
	t.Helper()
	current, err := st.Locate(id)
	if err != nil {
		t.Fatalf("Locate(%s): %v", id, err)
	}
	if err := os.Rename(filepath.Join(st.Dir, current), filepath.Join(st.Dir, to)); err != nil {
		t.Fatal(err)
	}
}

// taskFiles is the task directory as a listing test can assert on, temporary
// files included: a save that left one behind is a save that did not clean up.
func taskFiles(t *testing.T, st *store.Store) []string {
	t.Helper()
	entries, err := os.ReadDir(st.Dir)
	if err != nil {
		t.Fatal(err)
	}
	return names(entries)
}

// The losing half of the race the ticket measured, made deterministic. Two
// processes share no mutex, so nothing tells the loser it lost except the shape
// of the write itself: its move finds nothing where it left the task. That is
// an answer, not a failure — and it is not licence to put the old name back,
// which is what used to leave two files claiming one ID.
func TestUpdateRelocatesWhenAnotherWriterMovesTheFile(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Before"})

	stolen := tk.ID + "-stolen.md"
	moved := false
	st.DuringUpdate(func() {
		if moved {
			return
		}
		moved = true
		moveTaskFile(t, st, tk.ID, stolen)
	})

	tk.Status = task.StatusInProgress
	if _, err := st.Update(tk); err != nil {
		t.Fatalf("Update should have followed the task to its new file: %v", err)
	}
	if !moved {
		t.Fatal("the hook never ran, so this test proved nothing")
	}

	// One file, and it is the one this save asked for. Two would be the brick:
	// locate refuses an ID two files claim, and every command for that task
	// fails from then on.
	if files := taskFiles(t, st); len(files) != 1 || files[0] != store.TaskFileName(tk) {
		t.Errorf("directory contains %v, want only %s", files, store.TaskFileName(tk))
	}
	reloaded, err := st.Get(tk.ID)
	if err != nil {
		t.Fatalf("the task must still be addressable by its ID: %v", err)
	}
	if reloaded.Status != task.StatusInProgress {
		t.Errorf("Status = %q, want %q: the save reported success, so it landed", reloaded.Status, task.StatusInProgress)
	}
}

// The bound on that retry, and the promise that goes with running out of it:
// a save that never claimed a name wrote nothing at all, content and filename
// alike. The old failure was the opposite — the write landed and the call
// reported failure anyway, which the HTTP layer turned into a 500 over a
// change that was on disk.
func TestUpdateGivesUpWithoutWritingWhenTheFileKeepsMoving(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Before"})

	calls := 0
	st.DuringUpdate(func() {
		calls++
		moveTaskFile(t, st, tk.ID, fmt.Sprintf("%s-moved-%d.md", tk.ID, calls))
	})

	changed := tk
	changed.Status = task.StatusInProgress
	_, err := st.Update(changed)
	if err == nil {
		t.Fatal("Update should have given up: its file was moved out from under every attempt")
	}
	if !strings.Contains(err.Error(), tk.ID) {
		t.Errorf("Update says %q, want it to name %s", err, tk.ID)
	}
	if calls != store.UpdateAttempts {
		t.Errorf("the move was attempted %d times, want %d", calls, store.UpdateAttempts)
	}

	// One file, holding what it held before: no half-written copy, no staged
	// temporary left behind, and no second claim on the ID.
	last := fmt.Sprintf("%s-moved-%d.md", tk.ID, calls)
	if files := taskFiles(t, st); len(files) != 1 || files[0] != last {
		t.Errorf("directory contains %v, want only %s", files, last)
	}
	reloaded, err := st.Get(tk.ID)
	if err != nil {
		t.Fatalf("the task must still be addressable by its ID: %v", err)
	}
	if reloaded.Status != tk.Status {
		t.Errorf("Status = %q, want %q: a save that reported failure must not have landed", reloaded.Status, tk.Status)
	}
}

// What a kill between the move and the write leaves: the file under its new
// name, still holding the old content. That is a stale title suffix like any
// other — the ID in the frontmatter is what identifies a task — so the task is
// readable, addressable, and converges on the next save. The state that used
// to be left here was two files, which converges on nothing.
func TestUpdateConvergesAfterAnInterruptedMove(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Before"})

	retitled := tk
	retitled.Title = "After"
	interrupted := store.TaskFileName(retitled)
	moveTaskFile(t, st, tk.ID, interrupted)

	if files := taskFiles(t, st); len(files) != 1 || files[0] != interrupted {
		t.Fatalf("directory contains %v, want only %s", files, interrupted)
	}
	halfway, err := st.Get(tk.ID)
	if err != nil {
		t.Fatalf("the interrupted task must still be readable: %v", err)
	}
	if halfway.Title != "Before" {
		t.Errorf("Title = %q, want %q: the content had not been written yet", halfway.Title, "Before")
	}

	if _, err := st.Update(retitled); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if files := taskFiles(t, st); len(files) != 1 || files[0] != interrupted {
		t.Errorf("directory contains %v, want only %s", files, interrupted)
	}
	if reloaded, err := st.Get(tk.ID); err != nil || reloaded.Title != "After" {
		t.Errorf("Get(%s) = %+v (%v), want the retitle finished", tk.ID, reloaded, err)
	}
}

// A repository root is not a boundary any more, and a .git in the way must not
// hide the project the marker declares — which is what made a queue fork inside
// a submodule (TQ-0059).
func TestDiscoverTaskDirWalksPastAGitDirectory(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	submodule := filepath.Join(outer, "vendor", "dep")
	if err := os.MkdirAll(filepath.Join(submodule, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := store.DiscoverTaskDir(submodule)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(outer, config.TaskDirName); dir != want {
		t.Errorf("dir = %q, want the enclosing project's queue %q", dir, want)
	}
}

// The nearest marker wins, whatever sits above it. Two projects one inside the
// other is the ordinary shape once init creates a queue wherever it is run.
func TestDiscoverTaskDirTakesTheNearestMarker(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(outer, "service")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InitStore(inner); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(inner, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := store.DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(inner, config.TaskDirName); dir != want {
		t.Errorf("dir = %q, want the nearest project's queue %q", dir, want)
	}
}

// The walk stops at the home directory, having checked it: a marker at
// ~/.taskqueue.yaml is usable, and one above it is somebody else's.
func TestDiscoverTaskDirStopsAtTheHomeDirectory(t *testing.T) {
	above := tqtest.Root(t)
	if _, err := store.InitStore(above); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(above, "home")
	project := filepath.Join(home, "work", "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	// Above the home directory, so out of reach even though it is plainly a
	// project.
	_, err := store.DiscoverTaskDir(project)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want the walk to stop before the queue above the home directory", err)
	}
	if !strings.Contains(err.Error(), home) {
		t.Errorf("err = %q, want it to name where the search stopped", err)
	}

	// The home directory itself is checked, so a marker there is reachable.
	homeStore, err := store.InitStore(home)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := store.DiscoverTaskDir(project)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != homeStore.Dir {
		t.Errorf("dir = %q, want the queue at the home directory %q", dir, homeStore.Dir)
	}
}

// A developer with TQ_CONFIG_PATH exported — which the README and the guide
// both tell them to use — must still get an isolated suite. Without this the
// whole suite operates on their real queue, and one test deletes it.
func TestFixturesIgnoreAnAmbientConfigPathOverride(t *testing.T) {
	real := tqtest.Root(t)
	outside := filepath.Join(real, config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvConfigPath, filepath.Join(real, config.ConfigFileName))

	st := tqtest.NewStore(t)
	if st.Dir == outside {
		t.Fatalf("the fixture used the ambient %s: %s", config.EnvConfigPath, st.Dir)
	}
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "fixture task"})

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the fixture wrote into the queue %s names: %d entries", config.EnvConfigPath, len(entries))
	}
}

// A queue directly above the fixture must not capture it, whatever the
// environment says. The assertion is on where the store landed, not on the
// variables — clearing them is one way to get there, and the tasks are what
// actually has to stay inside the fixture.
func TestFixturesNeutraliseAmbientConfiguration(t *testing.T) {
	// A real project, above the fixture, for the ambient configuration to
	// point at. It lives in the test's own temp space: naming an absolute
	// path like /somewhere/real/.tasks would have a suite running as uid 0
	// create it.
	above := tqtest.AboveFixtures(t)
	outside := filepath.Join(above, config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	tqtest.WriteConfig(t, above, "version: 1\npath: "+config.TaskDirName+"\n")

	// TQ_CONFIG_PATH names its marker outright, which is the strongest thing
	// the environment can say, and the fixture has to land inside itself anyway.
	t.Setenv(config.EnvConfigPath, filepath.Join(above, config.ConfigFileName))

	st := tqtest.NewStore(t)
	if st.Dir == outside {
		t.Fatalf("the fixture bound to the queue above it: %s", st.Dir)
	}
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "fixture task"})

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the fixture wrote into the queue above it: %d entries", len(entries))
	}
}

// t.TempDir() is not an isolation barrier: it honours TMPDIR, and discovery
// walks up out of it. With TMPDIR inside a git repository — normal in Nix,
// Bazel and containerised CI — the fixtures would otherwise bind to that
// repository's committed queue and write into it.
func TestFixturesStayInsideTempDirWhenTMPDIRIsInARepository(t *testing.T) {
	// Not t.TempDir(): it fixes its base directory on first use, so calling
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
	real := filepath.Join(repo, config.TaskDirName)
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(repo, "tmp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)

	st := tqtest.NewStore(t)
	if st.Dir == real {
		t.Fatalf("the fixture bound to the enclosing repository's queue: %s", st.Dir)
	}
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "fixture task"})

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
	st := tqtest.NewStore(t)

	const racers = 20
	var wg sync.WaitGroup
	ids := make([]string, racers)
	errs := make([]error, racers)

	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tk, err := st.Create(store.CreateTaskInput{Title: fmt.Sprintf("task %d", i)})
			ids[i], errs[i] = tk.ID, err
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
		if _, err := st.Get(id); err != nil {
			t.Errorf("Get(%s): %v", id, err)
		}
	}

	if tasks := listTasks(t, st); len(tasks) != racers {
		t.Errorf("List() = %d tasks, want %d", len(tasks), racers)
	}
}

// Racers sharing a title used to produce the same filename, so the second
// rename replaced the first and a task vanished behind a successful return.
func TestCreateUnderConcurrencyKeepsTasksWithTheSameTitle(t *testing.T) {
	st := tqtest.NewStore(t)

	const racers = 10
	var wg sync.WaitGroup
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.Create(store.CreateTaskInput{Title: "same title"}); err != nil {
				t.Errorf("Create: %v", err)
			}
		}()
	}
	wg.Wait()

	if tasks := listTasks(t, st); len(tasks) != racers {
		t.Errorf("List() = %d tasks, want %d: a create was lost", len(tasks), racers)
	}
}

// A note is an append, so a lost one is information nobody can reconstruct.
// Two agents working the same task is what the queue exists for.
func TestNoteUnderConcurrencyKeepsEveryNote(t *testing.T) {
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "probe"})

	const notes = 10
	var wg sync.WaitGroup
	for i := range notes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.Note(tk.ID, fmt.Sprintf("note %d", i)); err != nil {
				t.Errorf("Note: %v", err)
			}
		}()
	}
	wg.Wait()

	after, err := st.Get(tk.ID)
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
	st := tqtest.NewStore(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "probe"})

	const labels = 10
	var wg sync.WaitGroup
	for i := range labels {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := st.Patch(tk.ID, task.TaskPatch{AddLabels: []string{fmt.Sprintf("label-%d", i)}}); err != nil {
				t.Errorf("Patch: %v", err)
			}
		}()
	}
	wg.Wait()

	after, err := st.Get(tk.ID)
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
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: docs/queue\n")
	want := filepath.Join(root, "docs", "queue")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := store.DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// A marker already in the directory decides where the queue goes, even to a
// path that does not exist yet. That is what makes a second init a no-op on a
// project whose queue has been moved.
func TestInitStoreCreatesWhereTheConfigSays(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: docs/queue\n")

	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(root, "docs", "queue"); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
	if _, err := os.Stat(filepath.Join(root, config.TaskDirName)); !os.IsNotExist(err) {
		t.Errorf("the default directory should not have been created: %v", err)
	}
}

// The near-miss spelling stops init: it would otherwise put .taskqueue.yaml
// beside the .taskqueue.yml its author wrote, and nothing would ever read
// theirs again. The guard sits on the write as well as on the read, because
// WriteConfigIfMissing is exported and a caller can reach it without having
// asked ConfigIn anything.
func TestInitStoreRefusesToWriteBesideANearMiss(t *testing.T) {
	root := tqtest.RootWithoutMarker(t)
	nearMiss := filepath.Join(root, ".taskqueue.yml")
	if err := os.WriteFile(nearMiss, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.InitStore(root)
	if err == nil {
		t.Fatal("InitStore() = nil error, want one naming the file tq reads")
	}
	if !errors.Is(err, config.ErrConfig) {
		t.Errorf("err = %v, want it to wrap config.ErrConfig", err)
	}
	if _, err := os.Stat(filepath.Join(root, config.ConfigFileName)); !os.IsNotExist(err) {
		t.Errorf("init wrote a marker beside %s: %v", nearMiss, err)
	}
}

// A marker in a directory above is not this directory's marker: init creates
// the project where it is run, so it makes its own rather than following one it
// never went looking for.
func TestInitStoreIgnoresTheConfigAbove(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: docs/queue\n")
	nested := filepath.Join(root, "src")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := store.InitStore(nested)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(nested, config.TaskDirName); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
	if want := filepath.Join(nested, config.ConfigFileName); st.ConfigWritten != want {
		t.Errorf("ConfigWritten = %q, want %q", st.ConfigWritten, want)
	}
}

// TQ_CONFIG_PATH stands in for the walk entirely: a marker in the working
// directory itself does not get a look in.
func TestConfigPathOverrideBeatsTheWalk(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: from-the-walk\n")
	if err := os.MkdirAll(filepath.Join(root, "from-the-walk"), 0o755); err != nil {
		t.Fatal(err)
	}

	named := tqtest.Root(t)
	marker := tqtest.WriteConfig(t, named, "version: 1\npath: from-the-variable\n")
	queue := filepath.Join(named, "from-the-variable")
	if err := os.MkdirAll(queue, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvConfigPath, marker)

	st, err := store.OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if st.Dir != queue {
		t.Errorf("Dir = %q, want the queue the named marker declares, %q", st.Dir, queue)
	}
	if st.Marker != marker {
		t.Errorf("Marker = %q, want the one %s names, %q", st.Marker, config.EnvConfigPath, marker)
	}
}

// The marker is the only thing tq looks for. A directory that happens to be
// called .tasks, with no marker above it, is somebody else's business — the
// guessing it would take to claim it is what the marker replaces.
func TestDiscoverTaskDirIgnoresABareTaskDirWithNoMarker(t *testing.T) {
	// A marker of its own is exactly what this root must not have.
	root := tqtest.RootWithoutMarker(t)
	if err := os.MkdirAll(filepath.Join(root, config.TaskDirName), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := store.DiscoverTaskDir(root)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
	if err != nil && !strings.Contains(err.Error(), config.ConfigFileName) {
		t.Errorf("err = %q, want it to name the file tq looks for", err)
	}
}

// A broken config must not read as "no task directory": the caller has to know
// their file is wrong, not have a queue created somewhere else.
func TestDiscoverTaskDirReportsABrokenConfig(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 99\n")

	_, err := store.DiscoverTaskDir(root)
	if !errors.Is(err, config.ErrConfig) {
		t.Errorf("err = %v, want it to wrap config.ErrConfig", err)
	}
	if errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("err = %v, must not be reported as a missing task directory", err)
	}
}

// Creating a queue leaves the marker behind, so the next command finds it by
// the file rather than by guessing at directory names.
func TestInitStoreWritesTheConfigMarker(t *testing.T) {
	// The marker InitStore writes is the subject, so the fixture must not
	// already have one.
	root := tqtest.RootWithoutMarker(t)

	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("no config written")
	}
	if cfg.TaskDir() != st.Dir {
		t.Errorf("config points at %q, want the created %q", cfg.TaskDir(), st.Dir)
	}
	if cfg.Version != config.ConfigVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, config.ConfigVersion)
	}
}

// The config is the user's file, not a generated one.
func TestInitStoreLeavesAnExistingConfigAlone(t *testing.T) {
	root := tqtest.Root(t)
	body := "version: 1\npath: mine\n# hand written\n"
	path := tqtest.WriteConfig(t, root, body)

	if _, err := store.InitStore(root); err != nil {
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

// ── The project's priority vocabulary ────────────────────────────

const customPriorities = `version: 1
path: .tasks
priorities:
  - name: p0
    color: "#b60205"
  - name: p1
    color: "#c2410c"
  - name: p2
    color: "#4b5563"
    default: true
`

// storeWithPriorities returns a store whose project declares p0..p2.
func storeWithPriorities(t *testing.T) *store.Store {
	t.Helper()
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, customPriorities)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return st
}

func TestCreateUsesTheConfiguredDefault(t *testing.T) {
	st := storeWithPriorities(t)

	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Defaulted"})
	if tk.Priority != "p2" {
		t.Errorf("Priority = %q, want p2 (the entry marked default)", tk.Priority)
	}
}

func TestCreateRejectsAPriorityOutsideTheVocabulary(t *testing.T) {
	st := storeWithPriorities(t)

	_, err := st.Create(store.CreateTaskInput{Title: "Nope", Priority: task.PriorityUrgent})
	if err == nil {
		t.Fatal("Create() = nil, want an error: urgent is not in this project's set")
	}
	if !strings.Contains(err.Error(), "p0, p1, p2") {
		t.Errorf("error = %q, want it to list the valid values", err)
	}
}

func TestListSortsByTheConfiguredRank(t *testing.T) {
	st := storeWithPriorities(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "middle", Priority: "p1"})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "least", Priority: "p2"})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "most", Priority: "p0"})

	tasks := listTasks(t, st)
	got := make([]string, 0, len(tasks))
	for _, tk := range tasks {
		got = append(got, tk.Priority)
	}
	if want := "p0,p1,p2"; strings.Join(got, ",") != want {
		t.Errorf("order = %q, want %q (the order the config lists)", strings.Join(got, ","), want)
	}
}

// A project can edit its vocabulary under tasks already filed. Those keep the
// value they carry, still list, and sort last — and, crucially, can still be
// moved and closed: refusing the write would freeze every one of them.
func TestATaskKeepsAPriorityTheProjectHasDropped(t *testing.T) {
	root := tqtest.Root(t)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	stale := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Filed earlier", Priority: task.PriorityUrgent})
	fresh := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Filed after", Priority: task.PriorityLow})

	// The vocabulary changes out from under both of them.
	tqtest.WriteConfig(t, root, customPriorities)

	tasks := listTasks(t, st)
	if len(tasks) != 2 {
		t.Fatalf("List() returned %d tasks, want both", len(tasks))
	}
	if tasks[0].ID != stale.ID || tasks[0].Priority != task.PriorityUrgent {
		t.Errorf("first task = %s/%s, want %s still carrying urgent", tasks[0].ID, tasks[0].Priority, stale.ID)
	}

	// Moving it does not touch the priority, so it must not be refused.
	stale.Status = task.StatusDone
	if _, err := st.Update(stale); err != nil {
		t.Errorf("Update() on a task under a dropped priority = %v, want it to save", err)
	}
	// Nor does a patch that leaves the priority alone.
	if _, err := st.Patch(fresh.ID, task.TaskPatch{Title: ptr("Retitled")}); err != nil {
		t.Errorf("Patch() on a task under a dropped priority = %v, want it to save", err)
	}
	// Restating the value a task already carries changes nothing, so it is not
	// refused. The board's dialog sends every field at once, so this is exactly
	// what saving such a task looks like on the wire.
	if _, err := st.Patch(fresh.ID, task.TaskPatch{Priority: ptr(task.PriorityLow), Title: ptr("Same priority")}); err != nil {
		t.Errorf("Patch() restating a dropped priority = %v, want it accepted", err)
	}
	// Filing one under it afresh is the mistake worth naming.
	if _, err := st.Patch(fresh.ID, task.TaskPatch{Priority: ptr(task.PriorityUrgent)}); err == nil {
		t.Error("Patch(priority: urgent) = nil, want it refused")
	}
	if _, err := st.Patch(fresh.ID, task.TaskPatch{Priority: ptr("p0")}); err != nil {
		t.Errorf("Patch(priority: p0) = %v, want it accepted", err)
	}
}

func ptr(s string) *string { return &s }

// ── The project's board ──────────────────────────────────────────

const customBoard = `version: 1
path: .tasks
columns:
  - name: spotted
    display_name: Spotted
    consider_ready: true
  - name: doing
    display_name: Doing
    default: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

func storeWithBoard(t *testing.T) *store.Store {
	t.Helper()
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, customBoard)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	return st
}

func TestCreateUsesTheBoardsDefaultColumn(t *testing.T) {
	st := storeWithBoard(t)
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Filed with no column"})
	if tk.Status != "doing" {
		t.Errorf("Status = %q, want the column marked default", tk.Status)
	}
}

func TestCreateRejectsAStatusTheBoardHasNoColumnFor(t *testing.T) {
	st := storeWithBoard(t)
	_, err := st.Create(store.CreateTaskInput{Title: "Nope", Status: task.StatusTodo})
	if err == nil {
		t.Fatal("Create() = nil, want todo refused on a board that has no todo")
	}
	if !strings.Contains(err.Error(), "spotted, doing, shipped") {
		t.Errorf("error = %q, want it to list the columns", err)
	}
}

func TestListSortsByTheBoardOrder(t *testing.T) {
	st := storeWithBoard(t)
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "last", Status: "shipped"})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first", Status: "spotted"})
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "middle", Status: "doing"})

	tasks := listTasks(t, st)
	got := make([]string, 0, len(tasks))
	for _, tk := range tasks {
		got = append(got, tk.Status)
	}
	if want := "spotted,doing,shipped"; strings.Join(got, ",") != want {
		t.Errorf("order = %q, want %q (the order the config lists)", strings.Join(got, ","), want)
	}
}

// Removed columns normalize on read; the file updates on the next write.
func TestAColumnThatDisappearsShowsInTheFirstAndIsFixedOnTheNextWrite(t *testing.T) {
	root := tqtest.Root(t)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	stranded := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Filed before the board changed", Status: task.StatusDone})

	tqtest.WriteConfig(t, root, customBoard)

	listed := listTasks(t, st)
	if len(listed) != 1 || listed[0].Status != "spotted" {
		t.Fatalf("List() = %+v, want the task shown in the first column", listed)
	}

	onDisk, err := os.ReadFile(filepath.Join(st.Dir, store.TaskFileName(stranded)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "status: "+task.StatusDone) {
		t.Errorf("a listing rewrote the file; it should still say %q", task.StatusDone)
	}

	if _, err := st.Note(stranded.ID, "something unrelated"); err != nil {
		t.Fatalf("Note: %v", err)
	}
	onDisk, err = os.ReadFile(filepath.Join(st.Dir, store.TaskFileName(stranded)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "status: spotted") {
		t.Errorf("the correction did not ride along with the next write:\n%s", onDisk)
	}
}

// backlog resolves to inbox via the built-in alias.
func TestBacklogStillReadsAsInbox(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Filed under the old name", Status: task.StatusBacklog})
	if created.Status != task.StatusInbox {
		t.Errorf("Create(backlog) stored %q, want it resolved to inbox", created.Status)
	}

	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != task.StatusInbox {
		t.Errorf("Get() = %q, want inbox", got.Status)
	}
}

// ── A lowercase .md, and only that (TQ-0039) ─────────────────────────────
//
// `.md` is the one extension a task file may have. Matching it case-folded was
// declined: the store's view of a directory would then depend on whether the
// filesystem folds case, which APFS, ext4 and NTFS do not agree on. So a file
// arriving from outside spelled `.MD` — a Windows checkout, an editor with
// opinions, a hand-copied file — is foreign: never read, never adopted, never
// renamed. What it must not be is silent.

// plantForeign moves a real task's file to a name whose extension is not the
// lowercase .md tq writes. The content is a task file byte for byte, so what
// keeps it out of the queue is the name and nothing else.
func plantForeign(t *testing.T, st *store.Store, of task.Task, as string) string {
	t.Helper()
	current := filepath.Join(st.Dir, store.TaskFileName(of))
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(current); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir, as), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return as
}

// foldsCase reports whether dir's filesystem answers to a name it did not
// store. It is detected rather than assumed from the platform: macOS is usually
// case-insensitive and Linux usually not, but either can be mounted the other
// way, and where the two spellings are simply two files there is no collision
// to see.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, ".tq-case-probe.md")
	if err := os.WriteFile(probe, []byte("probe"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(probe) }()
	_, err := os.Lstat(filepath.Join(dir, ".TQ-CASE-PROBE.MD"))
	return err == nil
}

// The write half of the rule. Every path tq writes ends in a lowercase .md, and
// the title cannot smuggle a different one in.
func TestTaskFileNameIsAlwaysALowercaseMD(t *testing.T) {
	tests := []struct{ title, want string }{
		{"Fix BUG.MD", "TQ-0001-fix-bug-md.md"},
		{"SHOUTING", "TQ-0001-shouting.md"},
		{"README.MD", "TQ-0001-readme-md.md"},
		{"", "TQ-0001.md"},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := store.TaskFileName(task.Task{ID: "TQ-0001", Title: tt.title})
			if got != tt.want {
				t.Errorf("TaskFileName(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

// The read half. A planted .MD is not a task, however well it parses.
func TestAFileSpelledMDIsNotAdopted(t *testing.T) {
	st := tqtest.NewStore(t)
	only := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
	planted := plantForeign(t, st, only, only.ID+"-keep-me.MD")

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listing.Tasks) != 0 {
		t.Errorf("List() = %+v, want no tasks: %s is not a task file", listing.Tasks, planted)
	}

	if _, err := st.Locate(only.ID); !errors.Is(err, store.ErrTaskNotFound) {
		t.Errorf("Locate(%s) = %v, want ErrTaskNotFound", only.ID, err)
	} else if !strings.Contains(err.Error(), planted) {
		// "not found" alone sends the reader looking for a file that is
		// plainly there.
		t.Errorf("Locate(%s) says %q, want it to name %s", only.ID, err, planted)
	}

	// Not adopted means the number it claims is still free. That is what makes
	// the duplicate below possible, and why the duplicate has to be reported.
	next, err := st.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if next != only.ID {
		t.Errorf("NextID() = %q, want %q: a foreign file is not a task, so its number was never taken", next, only.ID)
	}
}

// The silent half of the gap TQ-0040 could not close: duplicatedIDs reads the
// task files, and this is not one, so an ID claimed by a .MD and a .md at once
// went entirely unmentioned.
func TestAFileSpelledMDIsReported(t *testing.T) {
	t.Run("alone", func(t *testing.T) {
		st := tqtest.NewStore(t)
		only := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
		planted := plantForeign(t, st, only, only.ID+"-keep-me.MD")

		listing, err := st.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(listing.Unreadable) != 1 || listing.Unreadable[0].File != planted {
			t.Fatalf("Unreadable = %+v, want it to name %s: an empty-looking queue with nothing said is the whole complaint", listing.Unreadable, planted)
		}
		if reason := listing.Unreadable[0].Reason; !strings.Contains(reason, ".md") {
			t.Errorf("reason = %q, want it to state the extension rule", reason)
		}
	})

	t.Run("beside the task file that holds its ID", func(t *testing.T) {
		st := tqtest.NewStore(t)
		held := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
		planted := duplicate(t, st, held, held.ID+"-second-claim.MD")

		listing, err := st.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		// The task file is a task file: it is read, and it stays on the board.
		if len(listing.Tasks) != 1 || listing.Tasks[0].ID != held.ID {
			t.Fatalf("List() = %+v, want %s: the real file is not in doubt, and withholding it would hide a healthy task", listing.Tasks, held.ID)
		}
		if len(listing.Unreadable) != 1 || listing.Unreadable[0].File != planted {
			t.Fatalf("Unreadable = %+v, want it to name %s", listing.Unreadable, planted)
		}
		reason := listing.Unreadable[0].Reason
		for _, want := range []string{held.ID, store.TaskFileName(held)} {
			if !strings.Contains(reason, want) {
				t.Errorf("reason = %q, want it to name %s: two files claiming one ID is what has to be fixed here", reason, want)
			}
		}
		if len(listing.Duplicated) != 0 {
			t.Errorf("Duplicated = %+v, want nothing: one of the two is not a task file, so there is no choosing between them", listing.Duplicated)
		}
	})
}

// The message the ticket is named for. A file the store cannot see occupies the
// name the new task wants, NextID cannot see it either, so every retry derived
// the same number and the loop ran out blaming task IDs.
func TestCreateNamesTheEntryInTheWay(t *testing.T) {
	st := tqtest.NewStore(t)
	if !foldsCase(t, st.Dir) {
		t.Skip("this filesystem keeps .md and .MD apart, so nothing is in the way of the write")
	}
	blocked := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
	planted := plantForeign(t, st, blocked, blocked.ID+"-keep-me.MD")

	_, err := st.Create(store.CreateTaskInput{Title: "Keep me"})
	if err == nil {
		t.Fatalf("Create should have been refused by %s", planted)
	}
	if !strings.Contains(err.Error(), planted) {
		t.Errorf("Create says %q, want it to name %s", err, planted)
	}
	if strings.Contains(err.Error(), "attempts") {
		t.Errorf("Create says %q; the ID was never the problem, and counting attempts told the reader nothing", err)
	}
	if _, err := os.Lstat(filepath.Join(st.Dir, planted)); err != nil {
		t.Errorf("%s must be left exactly as it is: %v", planted, err)
	}
}

// The same collision from the other side: a retitle picks its destination from
// the new title, and write replaces whatever is at it. Replacing a path tq does
// not own is the one thing it must never do.
func TestUpdateRefusesToRenameOverAnEntryItDoesNotOwn(t *testing.T) {
	st := tqtest.NewStore(t)
	if !foldsCase(t, st.Dir) {
		t.Skip("this filesystem keeps .md and .MD apart, so the retitle lands beside the file rather than on it")
	}
	tk := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Before"})
	planted := filepath.Join(st.Dir, tk.ID+"-after.MD")
	if err := os.WriteFile(planted, []byte("not a task file"), 0o644); err != nil {
		t.Fatal(err)
	}

	tk.Title = "After"
	if _, err := st.Update(tk); err == nil {
		t.Fatalf("Update should have been refused by %s", filepath.Base(planted))
	} else if !strings.Contains(err.Error(), filepath.Base(planted)) {
		t.Errorf("Update says %q, want it to name %s", err, filepath.Base(planted))
	}

	if content, err := os.ReadFile(planted); err != nil || string(content) != "not a task file" {
		t.Errorf("%s = %q (%v), want it untouched", filepath.Base(planted), content, err)
	}
	if reloaded, err := st.Get(tk.ID); err != nil || reloaded.Title != "Before" {
		t.Errorf("Get(%s) = %+v (%v), want the task still under its old title", tk.ID, reloaded, err)
	}
}

// The entry in the way is found by identity, and the directory is read a moment
// after the name was resolved: a task file written into that moment — a
// concurrent save moving a task's file to the name its title asks for — can be
// what the match lands on. That one is in nobody's way, and the retry is what handles it; only an
// entry the store does not read is worth stopping for.
//
// Hard links pin that decision without a race, by giving one file two names.
func TestCreateStopsOnlyForAnEntryTheStoreDoesNotRead(t *testing.T) {
	st := tqtest.NewStore(t)
	if !foldsCase(t, st.Dir) {
		t.Skip("this filesystem keeps .md and .MD apart, so nothing is in the way of the write")
	}
	blocked := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Keep me"})
	planted := plantForeign(t, st, blocked, blocked.ID+"-keep-me.MD")

	// A task file sharing the foreign file's identity and sorting ahead of it:
	// what a match by identity finds first if it does not know a task file when
	// it sees one.
	const decoy = "TQ-0000-a-task-file.md"
	if err := os.Link(filepath.Join(st.Dir, planted), filepath.Join(st.Dir, decoy)); err != nil {
		t.Fatal(err)
	}

	_, err := st.Create(store.CreateTaskInput{Title: "Keep me"})
	if err == nil {
		t.Fatalf("Create should have been refused by %s", planted)
	}
	if !strings.Contains(err.Error(), planted) {
		t.Errorf("Create says %q, want it to name %s", err, planted)
	}
	if strings.Contains(err.Error(), decoy) {
		t.Errorf("Create says %q; %s is a task file, and saying it is not one of a file the store reads perfectly well is worse than the retry it replaced", err, decoy)
	}
}

// ── The marker is the source of truth (TQ-0087) ─────────────────

// projectConfig is a board and two vocabularies no default and no decoy shares
// a name with, so the values a store reads say exactly which file it read.
const projectConfig = `columns:
  - name: backlog
    display_name: Backlog
    default: true
  - name: doing
    display_name: Doing
    consider_ready: true
  - name: shipped
    display_name: Shipped
    consider_done: true
priorities:
  - name: blocker
    color: "#b42318"
  - name: routine
    color: "#4b5563"
    default: true
labels:
  billing:
    color: "#00ff00"
    display_name: Billing
`

// A `path:` that leaves the marker's own directory is documented and ordinary.
// The project's board, vocabulary and labels have to survive it: walking up
// from the task directory reaches another project's marker, or none, and either
// answer silently replaced all three (TQ-0087).
func TestStoreReadsTheConfigOfTheMarkerItWasResolvedThrough(t *testing.T) {
	root, queue := tqtest.EscapedQueue(t, projectConfig)

	for _, tc := range []struct {
		name string
		open func() (*store.Store, error)
	}{
		{"init", func() (*store.Store, error) { return store.InitStore(root) }},
		{"open", func() (*store.Store, error) { return store.OpenStore(root) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st, err := tc.open()
			if err != nil {
				t.Fatalf("opening the store: %v", err)
			}
			if st.Dir != queue {
				t.Fatalf("Dir = %q, want the queue the marker declares, %q", st.Dir, queue)
			}
			if want := filepath.Join(root, config.ConfigFileName); st.Marker != want {
				t.Fatalf("Marker = %q, want %q", st.Marker, want)
			}

			columns, err := st.Columns()
			if err != nil {
				t.Fatalf("Columns: %v", err)
			}
			if got, want := strings.Join(columns.Names(), ","), "backlog,doing,shipped"; got != want {
				t.Errorf("Columns() = %q, want the project's board %q", got, want)
			}
			priorities, err := st.Priorities()
			if err != nil {
				t.Fatalf("Priorities: %v", err)
			}
			if got, want := strings.Join(priorities.Names(), ","), "blocker,routine"; got != want {
				t.Errorf("Priorities() = %q, want the project's vocabulary %q", got, want)
			}
			cfg, err := st.Config()
			if err != nil {
				t.Fatalf("Config: %v", err)
			}
			if _, ok := cfg.LabelSet()["billing"]; !ok {
				t.Errorf("LabelSet() = %v, want the project's labels", cfg.LabelSet())
			}
		})
	}
}

// The write path is where losing the board costs data: a status outside the
// columns a store validates against is normalised away, so an edit that has
// nothing to do with status rewrites it (TQ-0087).
func TestStoreKeepsTheStatusOfATaskAnUnrelatedEditTouches(t *testing.T) {
	root, _ := tqtest.EscapedQueue(t, projectConfig)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Real work"})
	if created.Status != "backlog" {
		t.Fatalf("Status = %q, want the project's default column", created.Status)
	}
	if created.Priority != "routine" {
		t.Fatalf("Priority = %q, want the project's default priority", created.Priority)
	}

	// A column the project declares, which the built-in board does not.
	moved, err := st.Patch(created.ID, task.TaskPatch{Status: ptr("doing")})
	if err != nil {
		t.Fatalf("moving to a column the project declares: %v", err)
	}
	if moved.Status != "doing" {
		t.Fatalf("Status = %q, want doing", moved.Status)
	}

	for _, tc := range []struct {
		name string
		edit func() (task.Task, error)
	}{
		{"assignee", func() (task.Task, error) {
			return st.Patch(created.ID, task.TaskPatch{Assignee: ptr("alice")})
		}},
		{"title", func() (task.Task, error) {
			return st.Patch(created.ID, task.TaskPatch{Title: ptr("Renamed")})
		}},
		{"note", func() (task.Task, error) { return st.Note(created.ID, "something happened") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			edited, err := tc.edit()
			if err != nil {
				t.Fatalf("editing the %s: %v", tc.name, err)
			}
			if edited.Status != "doing" {
				t.Errorf("Status = %q after editing the %s, want doing left alone", edited.Status, tc.name)
			}
			// And on disk, which is what a next command reads.
			reread, err := st.Get(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if reread.Status != "doing" {
				t.Errorf("status on disk = %q after editing the %s, want doing", reread.Status, tc.name)
			}
		})
	}
}

// The marker is carried as a path, not as a parsed file: an edit to it has to
// reach a running server on its next read, exactly as an edit to a task does.
func TestStoreConfigIsReadFromDiskOnEveryCall(t *testing.T) {
	root, _ := tqtest.EscapedQueue(t, "")
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	columns, err := st.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(columns.Names(), ","), strings.Join(task.Columns{}.Names(), ","); got != want {
		t.Fatalf("Columns() = %q, want the built-in board %q before the project declares one", got, want)
	}

	tqtest.WriteConfig(t, root, "version: 1\npath: ../queue\n"+projectConfig)

	columns, err = st.Columns()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(columns.Names(), ","), "backlog,doing,shipped"; got != want {
		t.Errorf("Columns() = %q after the marker was edited, want %q", got, want)
	}
}

// TQ_CONFIG_PATH hands over a marker, so the project comes with it: the board,
// both vocabularies and the queue, from any working directory at all —
// including one that belongs to no project, which is the case that used to have
// a command fall back to the built-in board and rewrite the status of every
// task it touched (TQ-0087).
func TestStoreUnderConfigPathOverrideUsesThatProject(t *testing.T) {
	root, queue := tqtest.EscapedQueue(t, projectConfig)
	marker := filepath.Join(root, config.ConfigFileName)
	outer, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	created := tqtest.MustCreate(t, outer, store.CreateTaskInput{Title: "Real work"})
	if _, err := outer.Patch(created.ID, task.TaskPatch{Status: ptr("doing")}); err != nil {
		t.Fatal(err)
	}

	// A subtest for its own temporary parent: the decoy EscapedQueue plants
	// sits above everything this test's t.TempDir hands out, and the premise
	// here is a working directory with no marker above it at all.
	t.Run("from a directory that belongs to no project", func(t *testing.T) {
		from := tqtest.RootWithoutMarker(t)
		t.Setenv(config.EnvConfigPath, marker)

		st, err := store.OpenStore(from)
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		if st.Dir != queue || st.Marker != marker {
			t.Fatalf("store = %s through %s, want %s through %s", st.Dir, st.Marker, queue, marker)
		}

		columns, err := st.Columns()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.Join(columns.Names(), ","), "backlog,doing,shipped"; got != want {
			t.Errorf("Columns() = %q, want the project's board %q", got, want)
		}
		priorities, err := st.Priorities()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.Join(priorities.Names(), ","), "blocker,routine"; got != want {
			t.Errorf("Priorities() = %q, want the project's vocabulary %q", got, want)
		}
		cfg, err := st.Config()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := cfg.LabelSet()["billing"]; !ok {
			t.Errorf("LabelSet() = %v, want the project's labels", cfg.LabelSet())
		}

		// The data-loss case, pinned: an edit that says nothing about status
		// must leave the task exactly where it was.
		for _, tc := range []struct {
			name string
			edit func() (task.Task, error)
		}{
			{"assignee", func() (task.Task, error) {
				return st.Patch(created.ID, task.TaskPatch{Assignee: ptr("alice")})
			}},
			{"title", func() (task.Task, error) {
				return st.Patch(created.ID, task.TaskPatch{Title: ptr("Renamed")})
			}},
			{"note", func() (task.Task, error) { return st.Note(created.ID, "something happened") }},
		} {
			edited, err := tc.edit()
			if err != nil {
				t.Fatalf("editing the %s: %v", tc.name, err)
			}
			if edited.Status != "doing" {
				t.Errorf("Status = %q after editing the %s, want doing left alone", edited.Status, tc.name)
			}
			reread, err := st.Get(created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if reread.Status != "doing" {
				t.Errorf("status on disk = %q after editing the %s, want doing", reread.Status, tc.name)
			}
		}
	})
}

// A Store with no marker is the one thing left that has no configuration, and
// only a test can build one: InitStore and OpenStore both resolve through a
// marker or fail. Nothing supplies a board for it. Answering with the built-in
// one would be the silence TQ-0087 removed, reachable again through whatever
// future code assembled the store — and a grep over the source cannot see that
// coming, so the store refuses instead.
func TestStoreAssembledWithoutAMarkerReportsIt(t *testing.T) {
	bare := &store.Store{Dir: tqtest.NewStore(t).Dir}

	if _, err := bare.Config(); !errors.Is(err, config.ErrNoConfig) {
		t.Errorf("Config() = %v, want config.ErrNoConfig", err)
	}
	if _, err := bare.Columns(); !errors.Is(err, config.ErrNoConfig) {
		t.Errorf("Columns() = %v, want config.ErrNoConfig rather than the built-in board", err)
	}
	if _, err := bare.Priorities(); !errors.Is(err, config.ErrNoConfig) {
		t.Errorf("Priorities() = %v, want config.ErrNoConfig rather than the built-in set", err)
	}
}

// A marker that has gone missing since the queue was resolved through it is a
// failure, not an absence. Reading it as "no config" is what put the built-in
// board under a project that declared its own.
func TestStoreConfigFailsWhenTheMarkerIsGone(t *testing.T) {
	root, _ := tqtest.EscapedQueue(t, projectConfig)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(st.Marker); err != nil {
		t.Fatal(err)
	}

	if _, err := st.Config(); !errors.Is(err, config.ErrConfig) {
		t.Errorf("Config() = %v, want it to wrap config.ErrConfig", err)
	}
	// And the store refuses the write rather than validating it against a board
	// the project never declared — the decoy above the queue included.
	if _, err := st.Columns(); err == nil {
		t.Error("Columns() = nil error with the marker gone, want the failure reported")
	}
}
