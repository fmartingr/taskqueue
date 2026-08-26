package store_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

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

func TestOpenStoreCreatesTaskDirOnDemand(t *testing.T) {
	root := tqtest.Root(t)

	st, err := store.OpenStore(root)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if want := filepath.Join(root, config.TaskDirName); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
	if !st.Created {
		t.Error("Created should report that OpenStore made the directory")
	}
	if info, err := os.Stat(st.Dir); err != nil || !info.IsDir() {
		t.Fatalf("task directory not created: %v", err)
	}

	// Opening it again finds the existing directory instead of recreating it.
	again, err := store.OpenStore(root)
	if err != nil {
		t.Fatalf("second OpenStore: %v", err)
	}
	if again.Dir != st.Dir || again.Created {
		t.Errorf("second OpenStore = %+v, want the same directory with Created=false", again)
	}
}

func TestOpenStoreCreatesAtTheRepositoryRoot(t *testing.T) {
	// The repository bound is the thing under test, so the fixture is anchored
	// by .git and carries no marker: with one, the marker would decide and the
	// fallback below it would never run.
	root := tqtest.RootWithGit(t)
	nested := filepath.Join(root, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	// A new task directory belongs next to .git, not in whichever
	// subdirectory the agent happened to be standing in.
	st, err := store.OpenStore(nested)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if want := filepath.Join(root, config.TaskDirName); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
	if _, err := os.Stat(filepath.Join(nested, config.TaskDirName)); err == nil {
		t.Errorf("no %s should have been created in the subdirectory", config.TaskDirName)
	}
}

func TestOpenStoreReportsUncreatableDir(t *testing.T) {
	root := tqtest.Root(t)
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The root is anchored, so creation falls back to the place its marker
	// names: put a regular file there too. A permission bit would not do —
	// uid 0 ignores it, and CI runs as root in a container — but no privilege
	// makes a directory out of a file.
	if err := os.WriteFile(filepath.Join(root, config.TaskDirName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Nothing can be created below a regular file, so this is still "no usable
	// task directory" rather than a filesystem error nobody can act on.
	_, err := store.OpenStore(filepath.Join(file, "sub"))
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
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
// writes the new name before retiring the old one, and `tq delete` unlinks, so
// a scan that caught the old name has nothing to report. A dangling symlink is
// that state, held still — the directory is not moving, so there is nothing for
// the consistency check to retry either.
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

// A retitle writes the new file before retiring the old one, so for an instant
// the task has two. Both readings of the directory can fall inside that
// instant and agree, which makes the entry set useless here — the pass simply
// holds the task twice, and only the repeated ID says so.
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

// Two files that keep claiming one ID are not a directory in motion — they are
// a queue to fix, which is TQ-0040 and not this. The retry must not spin on
// them, and the listing must say it could not be squared with the disk rather
// than pass the pair off as the queue.
func TestTwoFilesClaimingOneIDAreReportedRatherThanRetriedForever(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "first"})

	content, err := os.ReadFile(filepath.Join(st.Dir, store.TaskFileName(created)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir, created.ID+"-a-second-file.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	scans := 0
	st.DuringScan(func() { scans++ })

	listing, err := st.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if scans != store.ListAttempts {
		t.Errorf("scanned %d times, want %d: the retry is bounded even when the pair never resolves", scans, store.ListAttempts)
	}
	if !listing.Incomplete {
		t.Error("Incomplete = false: a listing holding one task twice must not pass as the queue")
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

func TestUpdateRewritesFileAtomically(t *testing.T) {
	st := tqtest.NewStore(t)
	created := tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "Original", Body: "Body."})

	created.Title = "Renamed"
	created.Status = task.StatusInProgress
	updated, err := st.Update(created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Updated.Before(updated.Created) {
		t.Error("Update should refresh the updated timestamp")
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

func TestDiscoverTaskDirNotFound(t *testing.T) {
	// No marker: its absence is the case under test, so .git is the anchor.
	_, err := store.DiscoverTaskDir(tqtest.RootWithGit(t))
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("err = %v, want ErrProjectNotFound", err)
	}
}

// A project without Git is the shape where the search has no bound at all: it
// runs to the filesystem root and finds nothing. What it says then is its own
// message, and it has to stay that way — the bounded one names a repository
// root that does not exist here, and offers to lift a bound that was never
// applied (TQ-0064).
func TestDiscoverTaskDirWithoutARepositoryOrAMarker(t *testing.T) {
	root := tqtest.RootWithoutAnchor(t)

	_, err := store.DiscoverTaskDir(root)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound", err)
	}
	want := fmt.Sprintf("%s (no %s in %s or any parent directory)", store.ErrProjectNotFound, config.ConfigFileName, root)
	if err.Error() != want {
		t.Errorf("err = %q, want %q", err, want)
	}
}

func TestDiscoverTaskDirEnvOverride(t *testing.T) {
	root := tqtest.Root(t)
	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	elsewhere := tqtest.Root(t)

	t.Setenv(config.EnvTaskDir, st.Dir)
	dir, err := store.DiscoverTaskDir(elsewhere)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if dir != st.Dir {
		t.Errorf("dir = %q, want the %s override %q", dir, config.EnvTaskDir, st.Dir)
	}

	// A missing override is "not there yet", which is what lets OpenStore
	// create it.
	t.Setenv(config.EnvTaskDir, filepath.Join(elsewhere, "missing"))
	if _, err := store.DiscoverTaskDir(elsewhere); !errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("DiscoverTaskDir with a missing %s = %v, want ErrProjectNotFound", config.EnvTaskDir, err)
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
		dir := tqtest.Root(t)
		path := filepath.Join(dir, "task.md")
		write(t, path)
		if err := store.RetireOldFile(path, path); err != nil {
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
		dir := tqtest.Root(t)
		oldPath, newPath := filepath.Join(dir, "old.md"), filepath.Join(dir, "new.md")
		write(t, newPath)
		if err := os.Link(newPath, oldPath); err != nil {
			t.Fatal(err)
		}
		if err := store.RetireOldFile(oldPath, newPath); err != nil {
			t.Fatalf("retireOldFile: %v", err)
		}
		if _, err := os.Lstat(newPath); err != nil {
			t.Errorf("the written task must survive: %v", err)
		}
	})

	t.Run("a genuinely different file is removed", func(t *testing.T) {
		dir := tqtest.Root(t)
		oldPath, newPath := filepath.Join(dir, "old.md"), filepath.Join(dir, "new.md")
		write(t, oldPath)
		write(t, newPath)
		if err := store.RetireOldFile(oldPath, newPath); err != nil {
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
		dir := tqtest.Root(t)
		oldPath, newPath := filepath.Join(dir, "old.md"), filepath.Join(dir, "new.md")
		write(t, newPath)
		if err := os.Symlink(filepath.Join(dir, "gone.md"), oldPath); err != nil {
			t.Fatal(err)
		}
		if err := store.RetireOldFile(oldPath, newPath); err != nil {
			t.Fatalf("retireOldFile: %v", err)
		}
		// Left behind, it would be a second file claiming the same task ID.
		if _, err := os.Lstat(oldPath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("the dangling link should be gone, got %v", err)
		}
	})

	t.Run("an already removed old file is not an error", func(t *testing.T) {
		dir := tqtest.Root(t)
		newPath := filepath.Join(dir, "new.md")
		write(t, newPath)
		if err := store.RetireOldFile(filepath.Join(dir, "old.md"), newPath); err != nil {
			t.Errorf("retireOldFile: %v", err)
		}
	})
}

// A queue above a project must not capture it: a developer who once ran tq in
// their home directory would otherwise have every new repository file into it.
func TestDiscoverTaskDirStopsAtTheRepositoryRoot(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := store.DiscoverTaskDir(repo)
	if !errors.Is(err, store.ErrProjectNotFound) {
		t.Fatalf("err = %v, want ErrProjectNotFound rather than the queue above the repository", err)
	}
	// The message has to explain itself: the queue is plainly there, one level
	// up, so "not found" alone reads as a bug.
	for _, want := range []string{"repository root", config.EnvWalkForever} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

func TestDiscoverTaskDirWalksPastTheRepositoryRootWhenAsked(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvWalkForever, "true")

	dir, err := store.DiscoverTaskDir(repo)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(outer, config.TaskDirName); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// Only "true" lifts the bound; anything else leaves the default in place.
func TestDiscoverTaskDirIgnoresAnUnsetWalkForever(t *testing.T) {
	outer := tqtest.Root(t)
	if _, err := store.InitStore(outer); err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(outer, "project")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvWalkForever, "1")

	if _, err := store.DiscoverTaskDir(repo); !errors.Is(err, store.ErrProjectNotFound) {
		t.Errorf("err = %v, want the bound to hold for a value other than \"true\"", err)
	}
}

// The bound is the repository root, not the starting directory: a queue at the
// root is still found from a subdirectory of the same repository.
func TestDiscoverTaskDirFindsTheQueueInsideItsOwnRepository(t *testing.T) {
	// The repository is the bound under test, so it is what anchors the
	// fixture; InitStore leaves the marker behind on its own.
	repo := tqtest.RootWithGit(t)
	if _, err := store.InitStore(repo); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repo, "src", "deep")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	dir, err := store.DiscoverTaskDir(nested)
	if err != nil {
		t.Fatalf("DiscoverTaskDir: %v", err)
	}
	if want := filepath.Join(repo, config.TaskDirName); dir != want {
		t.Errorf("dir = %q, want %q", dir, want)
	}
}

// A developer with TQ_DIR exported — which the README and the guide both tell
// them to use — must still get an isolated suite. Without this the whole suite
// operates on their real queue, and one test deletes it.
func TestFixturesIgnoreAnAmbientTaskDirOverride(t *testing.T) {
	outside := filepath.Join(tqtest.Root(t), "real", config.TaskDirName)
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvTaskDir, outside)

	st := tqtest.NewStore(t)
	if st.Dir == outside {
		t.Fatalf("the fixture used the ambient %s: %s", config.EnvTaskDir, st.Dir)
	}
	tqtest.MustCreate(t, st, store.CreateTaskInput{Title: "fixture task"})

	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("the fixture wrote into the directory %s names: %d entries", config.EnvTaskDir, len(entries))
	}
}

// The same for the walk-forever escape hatch: an exported value must not let a
// fixture climb out of its own temp directory. The assertion is on where the
// store landed, not on the variables — clearing them is one way to get there,
// and the tasks are what actually has to stay inside the fixture.
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

	// TQ_DIR names it outright, and walk-forever lifts the bound that would
	// otherwise stop the search short of it.
	t.Setenv(config.EnvTaskDir, outside)
	t.Setenv(config.EnvWalkForever, "true")

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

// A config naming a directory that does not exist yet is not an error: the
// queue is created where it says.
func TestInitStoreCreatesWhereTheConfigSays(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: docs/queue\n")

	st, err := store.InitStore(filepath.Join(root, "src"))
	if err != nil {
		t.Fatalf("InitStore: %v", err)
	}
	if want := filepath.Join(root, "docs", "queue"); st.Dir != want {
		t.Errorf("Dir = %q, want %q", st.Dir, want)
	}
}

// TQ_DIR is the task directory, full stop — the config's path is ignored.
func TestTaskDirOverrideBeatsTheConfigPath(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: from-config\n")
	override := filepath.Join(root, "from-env")
	if err := os.MkdirAll(override, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(config.EnvTaskDir, override)

	dir, err := store.DiscoverTaskDir(root)
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
	// A marker of its own is exactly what this root must not have.
	root := tqtest.RootWithGit(t)
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
	root := tqtest.RootWithGit(t)

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

// A project above the repository is what discovery deliberately walks past, and
// the marker is what makes it a project. Both directions matter: the marker is
// reported, and a directory that merely happens to be named .tasks is not,
// because since TQ-0029 it is not a queue and naming it would be a warning
// about nothing.
func TestShadowedProjectMarker(t *testing.T) {
	t.Run("a marker above the repository is reported", func(t *testing.T) {
		outer := tqtest.Root(t)
		repo := filepath.Join(outer, "project")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}

		marker, ok := store.ShadowedProjectMarker(repo)
		if !ok {
			t.Fatal("ShadowedProjectMarker() = false, want the project above the repository")
		}
		if want := filepath.Join(outer, config.ConfigFileName); marker != want {
			t.Errorf("marker = %q, want %q", marker, want)
		}
	})

	t.Run("a bare .tasks above the repository is not", func(t *testing.T) {
		// No marker of its own: a directory named .tasks is all there is.
		outer := tqtest.RootWithGit(t)
		stray := filepath.Join(outer, config.TaskDirName)
		if err := os.MkdirAll(stray, 0o755); err != nil {
			t.Fatal(err)
		}
		repo := filepath.Join(outer, "project")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}

		if marker, ok := store.ShadowedProjectMarker(repo); ok && marker == stray {
			t.Errorf("ShadowedProjectMarker() = %q, want a directory named %s to count for nothing", marker, config.TaskDirName)
		}
	})

	t.Run("a project without a repository excludes nothing", func(t *testing.T) {
		// No repository means no bound, so the search already went as far as
		// TQ_WALK_FOREVER would have taken it and there is nothing it failed to
		// reach. The project's own marker is not something it walked past.
		project := tqtest.RootWithoutGit(t)

		// The working directory is the project, which is how the CLI always
		// calls this: it passes the directory tq was run in. Answering from
		// the process's working directory rather than from the repository is
		// the mistake that would make a project without Git warn about its own
		// marker.
		//
		// t.Chdir moves the whole process, so this and anything running beside
		// it must stay sequential — which is why nothing in this package calls
		// t.Parallel.
		t.Chdir(project)

		if marker, ok := store.ShadowedProjectMarker(project); ok {
			t.Errorf("ShadowedProjectMarker() = %q, want nothing shadowed: without a repository nothing bounded the search", marker)
		}
	})

	t.Run("nothing is excluded when the walk is not bounded", func(t *testing.T) {
		outer := tqtest.Root(t)
		repo := filepath.Join(outer, "project")
		if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		// The variables that lift the bound: with either one set the search was
		// never stopped, so there is nothing it failed to reach.
		for _, env := range []struct{ name, value string }{
			{config.EnvWalkForever, "true"},
			{config.EnvTaskDir, filepath.Join(outer, config.TaskDirName)},
		} {
			t.Run(env.name, func(t *testing.T) {
				t.Setenv(env.name, env.value)
				if marker, ok := store.ShadowedProjectMarker(repo); ok {
					t.Errorf("ShadowedProjectMarker() = %q with %s set, want nothing shadowed", marker, env.name)
				}
			})
		}
	})
}
