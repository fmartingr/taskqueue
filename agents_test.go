package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncAgentsDocsWritesTheGuide(t *testing.T) {
	store := newTestStore(t)

	written, err := SyncAgentsDocs(store, filepath.Dir(store.Dir))
	if err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}

	guide, err := os.ReadFile(filepath.Join(store.Dir, AgentsFileName))
	if err != nil {
		t.Fatalf("guide not written: %v", err)
	}
	for _, want := range []string{
		"tq ready --json", "tq show <id> --json", "tq move <id> in-progress",
		"tq note <id>", "tq done <id>", "tq add \"Title\"", "tq list --json",
		"backlog, todo, in-progress, done", "urgent, high, normal, low",
		store.Dir, EnvTaskDir, generatedNotice,
	} {
		if !strings.Contains(string(guide), want) {
			t.Errorf("guide is missing %q", want)
		}
	}
	if len(written) == 0 || written[0] != filepath.Join(store.Dir, AgentsFileName) {
		t.Errorf("written = %v, want it to start with the guide", written)
	}

	// The guide is not a task and must not disturb the store.
	tasks, err := store.List()
	if err != nil || len(tasks) != 0 {
		t.Errorf("List() = %d tasks, %v; want 0 and no error", len(tasks), err)
	}

	// Running again rewrites nothing.
	written, err = SyncAgentsDocs(store, filepath.Dir(store.Dir))
	if err != nil {
		t.Fatalf("second SyncAgentsDocs: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("written = %v, want nothing on a second run", written)
	}
}

func TestSyncAgentsDocsRefreshesAStaleGuide(t *testing.T) {
	store := newTestStore(t)
	path := filepath.Join(store.Dir, AgentsFileName)
	if err := os.WriteFile(path, []byte("out of date\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(store, filepath.Dir(store.Dir)); err != nil {
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

func TestSyncAgentsDocsCreatesRootDocWhenMissing(t *testing.T) {
	root := t.TempDir()
	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}

	doc, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("root AGENTS.md not created: %v", err)
	}
	want := "# Task management\n\nSee [AGENTS.md](.tasks/AGENTS.md)\n"
	if string(doc) != want {
		t.Errorf("root doc =\n%q\nwant\n%q", doc, want)
	}
}

func TestSyncAgentsDocsUpdatesExistingDocs(t *testing.T) {
	root := t.TempDir()
	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	existing := "# Project\n\nSome instructions.\n"
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}

	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		doc, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(doc), existing) {
			t.Errorf("%s lost its original content:\n%s", name, doc)
		}
		// The file already has a level-one heading, so the section is nested.
		if !strings.Contains(string(doc), "## Task management\n\nSee [AGENTS.md](.tasks/AGENTS.md)") {
			t.Errorf("%s is missing the task section:\n%s", name, doc)
		}
	}

	// A second run leaves the documents untouched.
	before, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(before) != string(after) {
		t.Errorf("the section was added twice:\n%s", after)
	}
}

func TestSyncAgentsDocsRewritesAStaleSection(t *testing.T) {
	root := t.TempDir()
	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	stale := "# Project\n\n## Task management\n\nSee [AGENTS.md](old/place/AGENTS.md)\n\n## Other\n\nKeep me.\n"
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	doc, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), "old/place") {
		t.Errorf("the stale link should be gone:\n%s", doc)
	}
	if !strings.Contains(string(doc), "See [AGENTS.md](.tasks/AGENTS.md)") {
		t.Errorf("the section should point at the task directory:\n%s", doc)
	}
	if !strings.Contains(string(doc), "## Other\n\nKeep me.") {
		t.Errorf("the following section should survive:\n%s", doc)
	}
	if strings.Count(string(doc), "Task management") != 1 {
		t.Errorf("the section should not be duplicated:\n%s", doc)
	}
}

func TestSyncAgentsDocsLinksToTheConfiguredTaskDir(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(root, "docs", "queue")
	t.Setenv(EnvTaskDir, elsewhere)

	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	doc, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	// The link follows TQ_DIR, not the default .tasks path.
	if !strings.Contains(string(doc), "See [AGENTS.md](docs/queue/AGENTS.md)") {
		t.Errorf("link should point at the configured directory:\n%s", doc)
	}
}

func TestSyncAgentsDocsUsesTheRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "backend")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvTaskDir, filepath.Join(nested, ".tasks"))

	store, err := InitStore(nested)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SyncAgentsDocs(store, nested); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}

	doc, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("the pointer belongs at the repository root: %v", err)
	}
	if !strings.Contains(string(doc), "See [AGENTS.md](backend/.tasks/AGENTS.md)") {
		t.Errorf("link should be relative to the repository root:\n%s", doc)
	}
}

func TestCLIInitWritesAgentDocs(t *testing.T) {
	tc := newBareCLI(t)

	out := tc.mustRun("init")
	guide := filepath.Join(tc.root, TaskDirName, AgentsFileName)
	if !strings.Contains(out, guide) {
		t.Errorf("init should report the guide it wrote, got %q", out)
	}
	if _, err := os.Stat(guide); err != nil {
		t.Fatalf("guide not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tc.root, "AGENTS.md")); err != nil {
		t.Fatalf("root AGENTS.md not written: %v", err)
	}

	// Re-running refreshes without reporting spurious writes.
	out = tc.mustRun("init")
	if strings.Contains(out, "Wrote ") {
		t.Errorf("nothing should be rewritten on a second init, got %q", out)
	}
}

// The guide has to read as a workflow, not a menu: an agent that follows it
// top to bottom claims a task before editing and closes it before reporting
// the work done. Numbering the commands is what carries that.
func TestTaskGuideStatesTheLifecycleAsOrderedSteps(t *testing.T) {
	guide := string(taskGuide(filepath.Join("project", ".tasks")))

	// The framing carries as much as the numbering: without these the steps
	// read as a menu again.
	for _, want := range []string{
		"## Working a task",
		"Claim a task before the first edit and close it before you report the work",
		"`tq note <id> \"what happened\"`",
	} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide is missing %q", want)
		}
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
