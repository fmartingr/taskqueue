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

// A task directory that is the repository root makes the guide and the root
// document one and the same file. Pointing a document at itself would both
// bury the guide under a self-referential section and never settle: the guide
// write drops the section, the pointer write puts it back, forever.
func TestSyncAgentsDocsSkipsARootDocThatIsTheGuide(t *testing.T) {
	root := t.TempDir()
	t.Setenv(EnvTaskDir, root)

	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if store.Dir != root {
		t.Fatalf("store.Dir = %q, want the repository root %q", store.Dir, root)
	}

	guide := filepath.Join(root, AgentsFileName)
	written, err := SyncAgentsDocs(store, root)
	if err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	if len(written) != 1 || written[0] != guide {
		t.Errorf("written = %v, want the guide %q once", written, guide)
	}

	doc, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	if string(doc) != string(taskGuide(store.Dir)) {
		t.Errorf("the guide should not gain a pointer to itself:\n%s", doc)
	}

	// Run 2 has nothing left to do.
	written, err = SyncAgentsDocs(store, root)
	if err != nil {
		t.Fatalf("second SyncAgentsDocs: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("written = %v, want nothing on a second run", written)
	}
}

// The same file can be reached by two different paths, so the identity check
// cannot be a string comparison: here TQ_DIR goes through a symlink and the
// doc root does not.
func TestSyncAgentsDocsSkipsARootDocReachedThroughASymlink(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(t.TempDir(), "queue")
	if err := os.Symlink(root, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv(EnvTaskDir, link)

	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	doc, err := os.ReadFile(filepath.Join(root, AgentsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), taskSectionTitle) {
		t.Errorf("the guide should not gain a pointer to itself:\n%s", doc)
	}

	written, err := SyncAgentsDocs(store, root)
	if err != nil {
		t.Fatalf("second SyncAgentsDocs: %v", err)
	}
	if len(written) != 0 {
		t.Errorf("written = %v, want nothing on a second run", written)
	}
}

// Only the guide itself is exempt: a second root document beside it is still
// pointed at the guide, and by the path it actually sits at.
func TestSyncAgentsDocsPointsASiblingDocAtAGuideInTheRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv(EnvTaskDir, root)

	store, err := InitStore(root)
	if err != nil {
		t.Fatal(err)
	}
	existing := "# Project\n\nSome instructions.\n"
	if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := SyncAgentsDocs(store, root); err != nil {
		t.Fatalf("SyncAgentsDocs: %v", err)
	}
	doc, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(doc), existing) {
		t.Errorf("CLAUDE.md lost its original content:\n%s", doc)
	}
	if !strings.Contains(string(doc), "## Task management\n\nSee [AGENTS.md](AGENTS.md)") {
		t.Errorf("CLAUDE.md should point at the guide beside it:\n%s", doc)
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

// A shell fence whose first line is a `#` comment is the commonest construct
// in an instructions file: it must not read as the heading that ends the
// section. The section is regenerated wholesale, so a fence inside it goes
// with it — what must not happen is the fence being cut in half, leaving its
// body as prose and its closer dangling.
func TestWithTaskSectionEndsTheSectionPastAFence(t *testing.T) {
	doc := "# Project\n\n## Task management\n\nSee [AGENTS.md](old/path.md)\n\n```bash\n# how to run\ntq list\n```\n\n## Other\n\nKeep me.\n"

	updated, changed := withTaskSection(doc, ".tasks/AGENTS.md")
	if !changed {
		t.Fatal("a stale link should be rewritten")
	}
	for _, leftover := range []string{"```", "# how to run", "tq list", "old/path.md"} {
		if strings.Contains(updated, leftover) {
			t.Errorf("the replaced section left %q behind:\n%s", leftover, updated)
		}
	}
	if !strings.Contains(updated, "## Other\n\nKeep me.") {
		t.Errorf("the following section should survive:\n%s", updated)
	}
	if strings.Count(updated, "Task management") != 1 {
		t.Errorf("the section should not be duplicated:\n%s", updated)
	}
}

// The section boundary is the next real heading, so a fenced block that
// belongs to a later section is none of tq's business.
func TestWithTaskSectionLeavesALaterFenceAlone(t *testing.T) {
	doc := "# Project\n\n## Task management\n\nSee [AGENTS.md](old/path.md)\n\n## Other\n\n```bash\n# how to run\ntq list\n```\n"

	updated, changed := withTaskSection(doc, ".tasks/AGENTS.md")
	if !changed {
		t.Fatal("a stale link should be rewritten")
	}
	if !strings.Contains(updated, "## Other\n\n```bash\n# how to run\ntq list\n```") {
		t.Errorf("a fence in another section should survive intact:\n%s", updated)
	}
}

// A heading inside a fence is an example, not structure.
func TestWithTaskSectionIgnoresAFencedHeading(t *testing.T) {
	doc := "# Project\n\n~~~md\n## Task management\n\nSee [AGENTS.md](example/AGENTS.md)\n~~~\n"

	updated, changed := withTaskSection(doc, ".tasks/AGENTS.md")
	if !changed {
		t.Fatal("a document whose only mention is an example needs a real section")
	}
	if !strings.Contains(updated, "~~~md\n## Task management\n\nSee [AGENTS.md](example/AGENTS.md)\n~~~") {
		t.Errorf("the example should survive untouched:\n%s", updated)
	}
	if !strings.HasSuffix(updated, "## Task management\n\nSee [AGENTS.md](.tasks/AGENTS.md)\n") {
		t.Errorf("a real section should be appended:\n%s", updated)
	}
}

// README-style snippets showing the convention must not make a project opt
// itself out of the pointer.
func TestWithTaskSectionIgnoresAFencedPointer(t *testing.T) {
	doc := "# Project\n\nThe convention looks like this:\n\n```md\nSee [AGENTS.md](.tasks/AGENTS.md)\n```\n"

	updated, changed := withTaskSection(doc, ".tasks/AGENTS.md")
	if !changed {
		t.Fatal("a pointer inside an example does not point anywhere")
	}
	if !strings.Contains(updated, "## Task management\n\nSee [AGENTS.md](.tasks/AGENTS.md)\n") {
		t.Errorf("the document should gain a section:\n%s", updated)
	}
}

// The pointer may be an `@`-include: that is how Claude-style docs pull the
// guide in, and rewriting it as a Markdown link would break the include.
func TestWithTaskSectionKeepsAnIncludePointer(t *testing.T) {
	doc := "# Project\n\n## Task management\n\n@.tasks/AGENTS.md\n"

	updated, changed := withTaskSection(doc, ".tasks/AGENTS.md")
	if changed || updated != doc {
		t.Errorf("an include already points at the guide, got changed=%v:\n%s", changed, updated)
	}
}

// A pointer only counts where it is: inside the section that is supposed to
// carry it.
func TestWithTaskSectionIgnoresAPointerOutsideTheSection(t *testing.T) {
	doc := "# Project\n\nTasks live in [AGENTS.md](.tasks/AGENTS.md), by the way.\n"

	updated, changed := withTaskSection(doc, ".tasks/AGENTS.md")
	if !changed {
		t.Fatal("a passing mention is not a Task management section")
	}
	if !strings.HasSuffix(updated, "## Task management\n\nSee [AGENTS.md](.tasks/AGENTS.md)\n") {
		t.Errorf("the document should gain a section:\n%s", updated)
	}
}

// The level of an appended heading follows the document's real headings, not a
// `#` comment that happens to sit in a fence.
func TestWithTaskSectionIgnoresAFencedHashWhenChoosingItsLevel(t *testing.T) {
	doc := "Instructions.\n\n```sh\n# not a heading\ntq list\n```\n"

	updated, _ := withTaskSection(doc, ".tasks/AGENTS.md")
	if !strings.Contains(updated, "\n# Task management\n") {
		t.Errorf("the document has no level-one heading, so the section is one:\n%s", updated)
	}
}
