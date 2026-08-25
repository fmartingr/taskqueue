package guide

import (
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
		"backlog, todo, in-progress, done", "urgent, high, normal, low",
		st.Dir, config.EnvTaskDir, generatedNotice,
	} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("guide is missing %q", want)
		}
	}
	if len(written) == 0 || written[0] != filepath.Join(st.Dir, AgentsFileName) {
		t.Errorf("written = %v, want it to start with the guide", written)
	}

	// The guide is not a task and must not disturb the store.
	tasks, err := st.List()
	if err != nil || len(tasks) != 0 {
		t.Errorf("List() = %d tasks, %v; want 0 and no error", len(tasks), err)
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
	elsewhere := filepath.Join(root, "docs", "queue")
	t.Setenv(config.EnvTaskDir, elsewhere)

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
	if pointer := GuidePointer(st); !strings.HasSuffix(pointer, "queue/AGENTS.md") {
		t.Errorf("GuidePointer() = %q, want it to name the configured guide", pointer)
	}
}

func TestTaskGuideStatesTheLifecycleAsOrderedSteps(t *testing.T) {
	guide := string(taskGuide(filepath.Join("project", ".tasks")))

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
