package cli

import (
	"strings"
	"testing"

	"os"
	"path/filepath"

	"github.com/fmartingr/taskqueue/internal/config"
	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// writeProjectConfig plants a hand-written marker, for the tests whose premise
// is a project that declares something other than the defaults.
func writeProjectConfig(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, config.ConfigFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// labelRow mirrors what `tq label list --json` prints, which is part of the
// agent-facing contract.
type labelRow struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	DisplayName string `json:"display_name"`
	Configured  bool   `json:"configured"`
	Count       int    `json:"count"`
}

func TestCLILabelListPrintsTheConfiguredSet(t *testing.T) {
	tc := newTestCLI(t)

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	if len(rows) != len(config.DefaultLabels()) {
		t.Fatalf("got %d labels, want the %d seeded by init", len(rows), len(config.DefaultLabels()))
	}

	byName := map[string]labelRow{}
	for _, row := range rows {
		byName[row.Name] = row
	}
	backend, ok := byName["component/backend"]
	if !ok {
		t.Fatalf("component/backend missing from %+v", rows)
	}
	if backend.DisplayName != "Backend" || backend.Color == "" || !backend.Configured {
		t.Errorf("component/backend = %+v, want a display name, a colour and configured=true", backend)
	}
	if backend.Count != 0 {
		t.Errorf("count = %d for a label no task carries, want 0", backend.Count)
	}
}

func TestCLILabelListCountsUse(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "One", "--label", "bug", "--label", "component/cli")
	tc.mustRun("add", "Two", "--label", "bug")

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Name] = row.Count
	}
	if counts["bug"] != 2 {
		t.Errorf("bug count = %d, want 2", counts["bug"])
	}
	if counts["component/cli"] != 1 {
		t.Errorf("component/cli count = %d, want 1", counts["component/cli"])
	}
}

// A label in use but not configured is accepted everywhere and worth
// surfacing, so it either joins the set or gets cleaned up.
func TestCLILabelListFlagsUnconfiguredLabels(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Improvised", "--label", "whatever")

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")

	var found *labelRow
	for i := range rows {
		if rows[i].Name == "whatever" {
			found = &rows[i]
		}
	}
	if found == nil {
		t.Fatalf("an unconfigured label in use is missing from %+v", rows)
	}
	if found.Configured {
		t.Error("configured = true for a label the config does not declare")
	}
	if found.Color != "" {
		t.Errorf("color = %q, want it empty so the board renders it neutral", found.Color)
	}
	if found.DisplayName != "whatever" {
		t.Errorf("display_name = %q, want the label itself", found.DisplayName)
	}
	if found.Count != 1 {
		t.Errorf("count = %d, want 1", found.Count)
	}
}

// Configured labels come first, each group sorted, so two runs read the same.
func TestCLILabelListOrdersConfiguredFirst(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Improvised", "--label", "aaa-unconfigured")

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	if rows[len(rows)-1].Name != "aaa-unconfigured" {
		t.Errorf("last row = %q, want the unconfigured label even though it sorts first", rows[len(rows)-1].Name)
	}

	var previous string
	for _, row := range rows {
		if !row.Configured {
			break
		}
		if row.Name < previous {
			t.Errorf("%q comes after %q, want the configured labels sorted", row.Name, previous)
		}
		previous = row.Name
	}
}

func TestCLILabelListHumanOutput(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Improvised", "--label", "whatever")

	out := tc.mustRun("label", "list")
	for _, want := range []string{"LABEL", "component/backend", "Backend", "#1d76db", "whatever", "unconfigured"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
}

// A project that declares its own vocabulary gets exactly that, and labels
// outside it stay usable.
func TestCLILabelListReadsTheProjectsOwnSet(t *testing.T) {
	tc := newBareCLI(t)
	writeProjectConfig(t, tc.root, "version: 1\npath: "+config.TaskDirName+
		"\nlabels:\n  spicy:\n    color: \"#ff0000\"\n    display_name: Spicy\n")
	tc.mustRun("add", "Hot", "--label", "spicy", "--label", "bug")

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	if len(rows) != 2 {
		t.Fatalf("got %+v, want the one configured label and the one in use", rows)
	}
	if rows[0].Name != "spicy" || !rows[0].Configured || rows[0].DisplayName != "Spicy" {
		t.Errorf("rows[0] = %+v, want the configured label first", rows[0])
	}
	// "bug" is only a default: this project replaced the set, so it is not one.
	if rows[1].Name != "bug" || rows[1].Configured {
		t.Errorf("rows[1] = %+v, want bug flagged as unconfigured", rows[1])
	}
}

// The vocabulary is a reference, not a restriction.
func TestLabelsOutsideTheSetAreAccepted(t *testing.T) {
	tc := newTestCLI(t)
	var created struct {
		Labels []string `json:"labels"`
	}
	tc.mustRunJSON(&created, "add", "Freeform", "--label", "not-in-the-set", "--json")
	if len(created.Labels) != 1 || created.Labels[0] != "not-in-the-set" {
		t.Errorf("labels = %v, want the unconfigured label to be kept", created.Labels)
	}

	var patched struct {
		Labels []string `json:"labels"`
	}
	tc.mustRunJSON(&patched, "update", "TQ-0001", "--add-label", "another-new-one", "--json")
	if len(patched.Labels) != 2 {
		t.Errorf("labels = %v, want the second unconfigured label added too", patched.Labels)
	}
}

func TestCLILabelRejectsAnUnknownSubcommand(t *testing.T) {
	tc := newTestCLI(t)
	if code := tc.run("label", "rename"); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(tc.stderr.String(), "list") {
		t.Errorf("stderr = %q, want it to name the subcommand that exists", tc.stderr)
	}
	if tc.stdout.Len() != 0 {
		t.Errorf("stdout = %q, want errors on stderr only", tc.stdout)
	}
}

func TestCLILabelNeedsASubcommand(t *testing.T) {
	tc := newTestCLI(t)
	if code := tc.run("label"); code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
}

// Nothing but data on stdout, so an agent can pipe it.
func TestCLILabelListJSONKeepsStdoutClean(t *testing.T) {
	tc := newBareCLI(t)
	out := tc.mustRun("label", "list", "--json")
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Errorf("stdout = %q, want JSON alone", out)
	}
	if !strings.Contains(tc.stderr.String(), "created") {
		t.Errorf("stderr = %q, want the queue-created note to go there", tc.stderr)
	}
}

// A store the command creates on demand still lists the vocabulary: no command
// may fail merely because a project has not been initialised.
func TestCLILabelListWorksWithoutAProject(t *testing.T) {
	tc := newBareCLI(t)
	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	if len(rows) != len(config.DefaultLabels()) {
		t.Errorf("got %d labels, want the defaults", len(rows))
	}
}

// The vocabulary has to come from the queue being listed. TQ_DIR can point at
// another project's queue, and resolving the config from the working directory
// instead would have `tq label list` call a label unconfigured while the board
// — which resolves from the store — draws it in its configured colour.
func TestCLILabelListReadsTheConfigOfTheQueueItLists(t *testing.T) {
	tc := newBareCLI(t)
	writeProjectConfig(t, tc.root, "version: 1\npath: "+config.TaskDirName+
		"\nlabels:\n  here:\n    color: \"#ff0000\"\n    display_name: Here\n")

	elsewhere := tqtest.Root(t)
	writeProjectConfig(t, elsewhere, "version: 1\npath: "+config.TaskDirName+
		"\nlabels:\n  there:\n    color: \"#00ff00\"\n    display_name: There\n")
	t.Setenv(config.EnvTaskDir, filepath.Join(elsewhere, config.TaskDirName))
	tc.mustRun("add", "Over there", "--label", "there")

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	if len(rows) != 1 {
		t.Fatalf("got %+v, want only the other project's vocabulary", rows)
	}
	if rows[0].Name != "there" || !rows[0].Configured || rows[0].DisplayName != "There" {
		t.Errorf("rows[0] = %+v, want the label configured where the queue lives", rows[0])
	}
}

// A task may carry the same label twice; the column counts tasks.
func TestCLILabelListCountsTasksNotOccurrences(t *testing.T) {
	tc := newTestCLI(t)
	tc.mustRun("add", "Twice over", "--label", "bug", "--label", "bug")

	var rows []labelRow
	tc.mustRunJSON(&rows, "label", "list", "--json")
	for _, row := range rows {
		if row.Name == "bug" && row.Count != 1 {
			t.Errorf("bug count = %d, want 1 — one task carries it, twice", row.Count)
		}
	}
}
