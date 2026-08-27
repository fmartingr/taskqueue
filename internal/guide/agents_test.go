package guide

import (
	"github.com/fmartingr/taskqueue/internal/task"

	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/store"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

func TestSyncAgentsDocsWritesTheGuide(t *testing.T) {
	st := tqtest.NewStore(t)

	written, err := SyncAgentsDocs(st)
	if err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}

	guide, err := os.ReadFile(filepath.Join(st.Dir, AgentsFileName))
	if err != nil {
		t.Fatalf("guide not written: %v", err)
	}
	for _, want := range []string{
		"tq ready --json", "tq show <id> --json", "tq move <id> in-progress",
		"tq note <id>", "tq done <id>", "tq add \"Title\"", "tq list --json",
		"inbox, todo, in-progress, done, rejected", "urgent, high, normal, low",
		st.Dir, config.EnvConfigPath, generatedNotice,
	} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("guide is missing %q", want)
		}
	}
	if len(written) == 0 || written[0] != filepath.Join(st.Dir, AgentsFileName) {
		t.Errorf("written = %v, want it to start with the guide", written)
	}

	// The guide is not a task and must not disturb the store.
	listing, err := st.List()
	if err != nil || len(listing.Tasks) != 0 || len(listing.Unreadable) != 0 {
		t.Errorf("List() = %+v, %v; want no tasks, nothing skipped and no error", listing, err)
	}

	// Running again rewrites nothing.
	written, err = SyncAgentsDocs(st)
	if err != nil {
		t.Fatalf("second SyncAgentsDocs: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("written = %v, want nothing on a second run", written)
	}
}

func TestSyncAgentsDocsRefreshesAStaleGuide(t *testing.T) {
	st := tqtest.NewStore(t)
	path := filepath.Join(st.Dir, AgentsFileName)
	if err := os.WriteFile(path, []byte("out of date\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(st); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	guide, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(guide), "out of date") {
		t.Error("a stale guide should be replaced")
	}
}

func TestSyncAgentsDocsWritesTheGuideAtTheConfiguredTaskDir(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: docs/queue\n")
	elsewhere := filepath.Join(root, "docs", "queue")

	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	instructions := "instructions\n"
	doc := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(doc, []byte(instructions), 0o644); err != nil {
		t.Fatal(err)
	}

	written, err := SyncAgentsDocs(st)
	if err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	if want := filepath.Join(elsewhere, AgentsFileName); len(written) != 1 || written[0] != want {
		t.Errorf("written = %v, want just %s", written, want)
	}

	// The repository's own instructions are none of tq's business.
	got, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != instructions {
		t.Errorf("AGENTS.md was touched:\ngot:\n%s\nwant:\n%s", got, instructions)
	}
	if got, want := GuidePath(st), filepath.Join(elsewhere, AgentsFileName); got != want {
		t.Errorf("GuidePath() = %q, want %q", got, want)
	}
}

// The path `tq init` prints is the guide's own, absolute, whatever shape the
// project has. It used to be relative to a base this package guessed — the
// repository root, or the parent of the task directory — which resolved from
// one directory and misled from every other (TQ-0061). The assertions here are
// exact for that reason: the loose strings.HasSuffix they replaced accepted the
// wrong answer for as long as it ended in the right file name.
func TestGuidePathNamesTheGuideAbsolutely(t *testing.T) {
	t.Run("a project with no repository root", func(t *testing.T) {
		root := tqtest.Root(t)
		tqtest.WriteConfig(t, root, "version: 1\npath: elsewhere/queue\n")
		elsewhere := filepath.Join(root, "elsewhere", "queue")

		st, err := store.InitStore(root)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := GuidePath(st), filepath.Join(elsewhere, AgentsFileName); got != want {
			t.Errorf("GuidePath() = %q, want %q", got, want)
		}
	})

	t.Run("run from a subdirectory of the project", func(t *testing.T) {
		root := tqtest.Root(t)
		if _, err := store.InitStore(root); err != nil {
			t.Fatal(err)
		}
		deep := filepath.Join(root, "backend", "deep")
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}

		st, err := store.OpenStore(deep)
		if err != nil {
			t.Fatal(err)
		}
		// The queue is two levels up, and the answer is the same path it
		// would be from the root: one that reaches the guide from anywhere.
		if got, want := GuidePath(st), filepath.Join(root, ".tasks", AgentsFileName); got != want {
			t.Errorf("GuidePath() = %q, want %q", got, want)
		}
	})

	t.Run("a directory that is not a project yet", func(t *testing.T) {
		root := tqtest.RootWithoutMarker(t)
		st, err := store.InitStore(root)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := GuidePath(st), filepath.Join(root, ".tasks", AgentsFileName); got != want {
			t.Errorf("GuidePath() = %q, want %q", got, want)
		}
	})

	t.Run("a store opened on a relative path", func(t *testing.T) {
		root := tqtest.Root(t)
		t.Chdir(root)

		// Nothing hands the CLI a relative task directory today, but a marker's
		// `path` is a user-supplied string and filepath.Join would carry it
		// through.
		st := &store.Store{Dir: filepath.Join(".", ".tasks")}
		if got, want := GuidePath(st), filepath.Join(root, ".tasks", AgentsFileName); got != want {
			t.Errorf("GuidePath() = %q, want %q", got, want)
		}
	})
}

func TestTaskGuideStatesTheLifecycleAsOrderedSteps(t *testing.T) {
	guide := string(taskGuide(filepath.Join("project", ".tasks"), task.Priorities{}, task.Columns{}))

	// The framing carries as much as the numbering: without these the steps
	// read as a menu again.
	for _, want := range []string{
		"## Working a task",
		"Claim a task before the first edit and close it before you report the work",
		"`tq note <id> \"what happened\"`",
		// A note keeps its line breaks, and the guide has to say so: agents
		// were told the opposite while the text was being flattened.
		"A note keeps the\n   line breaks you give it",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide is missing %q", want)
		}
	}
	if strings.Contains(guide, "single line") {
		t.Error("the guide still describes notes as being flattened to a single line")
	}

	at := -1
	for _, step := range []string{
		"1. `tq ready --json`",
		"2. `tq show <id> --json`",
		"3. `tq move <id> in-progress`",
		"4. `tq note <id> \"what happened\"`",
		"5. `tq done <id>`",
	} {
		i := strings.Index(guide, step)
		if i < 0 {
			t.Fatalf("guide is missing the step %q", step)
		}
		if i < at {
			t.Errorf("step %q is out of lifecycle order", step)
		}
		at = i
	}
}

// The guide is the vocabulary an agent reads before filing anything, so it has
// to print the project's own. Printing the built-in set beside a store that
// refuses it is the drift generating this file exists to prevent.
func TestSyncAgentsDocsPrintsTheConfiguredPriorities(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: "+config.TaskDirName+"\n"+
		"priorities:\n"+
		"  - {name: p0, color: \"#b60205\"}\n"+
		"  - {name: p1, color: \"#c2410c\"}\n"+
		"  - {name: p2, color: \"#4b5563\", default: true}\n")

	st, err := store.InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncAgentsDocs(st); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(st.Dir, AgentsFileName))
	if err != nil {
		t.Fatal(err)
	}
	guide := string(got)
	if !strings.Contains(guide, "- Priorities: p0, p1, p2") {
		t.Errorf("guide does not print the project's priorities:\n%s", guide)
	}
	if !strings.Contains(guide, "default: p2") {
		t.Errorf("guide does not print the project's default:\n%s", guide)
	}
	// The examples name a priority too, so they have to come from the same set:
	// `tq add --priority high` is as much a lie as the list would be.
	if !strings.Contains(guide, "--priority p0") {
		t.Errorf("guide examples do not use a configured priority:\n%s", guide)
	}
	// Nowhere does a built-in value survive as an actual priority. Checked per
	// line and against "--priority x"/the list line, since "low" and "high" are
	// also substrings of ordinary words in the prose around them.
	for _, line := range strings.Split(guide, "\n") {
		for _, gone := range []string{"urgent", "high", "normal", "low"} {
			if strings.Contains(line, "--priority "+gone) {
				t.Errorf("guide example still uses the built-in priority %q: %s", gone, line)
			}
			if strings.HasPrefix(line, "- Priorities: ") && strings.Contains(line, gone) {
				t.Errorf("guide still lists the built-in priority %q: %s", gone, line)
			}
		}
	}
}
