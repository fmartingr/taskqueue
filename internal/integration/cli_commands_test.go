//go:build integration

package integration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type taskJSON struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Priority  string   `json:"priority"`
	Assignee  string   `json:"assignee"`
	Labels    []string `json:"labels"`
	DependsOn []string `json:"depends_on"`
	Body      string   `json:"body"`
}

// update is the largest command by flag count and had never run as a binary.
func TestUpdateEveryField(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "original title", "--label", "keep", "--label", "drop")
	p.mustRun(t, "add", "a dependency")

	p.mustRun(t, "update", "TQ-0001",
		"--title", "new title",
		"--status", "in-progress",
		"--priority", "urgent",
		"--assignee", "agent-api",
		"--add-label", "added",
		"--remove-label", "drop",
		"--add-dependency", "TQ-0002",
	)

	var got taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &got)
	if got.Title != "new title" || got.Status != "in-progress" || got.Priority != "urgent" || got.Assignee != "agent-api" {
		t.Errorf("scalars = %+v", got)
	}
	// Order is insertion order, not sorted, so compare as a set.
	labels := map[string]bool{}
	for _, l := range got.Labels {
		labels[l] = true
	}
	if !labels["keep"] || !labels["added"] || labels["drop"] || len(got.Labels) != 2 {
		t.Errorf("labels = %v, want keep and added only", got.Labels)
	}
	if strings.Join(got.DependsOn, ",") != "TQ-0002" {
		t.Errorf("depends_on = %v", got.DependsOn)
	}

	// Removing the dependency leaves nothing behind. Decoded into a fresh
	// value: an absent JSON field leaves whatever the target already held,
	// which would quietly assert the previous state.
	p.mustRun(t, "update", "TQ-0001", "--remove-dependency", "TQ-0002")
	var after taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &after)
	if len(after.DependsOn) != 0 {
		t.Errorf("depends_on = %v, want empty", after.DependsOn)
	}
}

// `tq update --body` rewrites the document and leaves the record. Driven as a
// process because the stdout/stderr split, the exit codes and a real pipe on
// stdin are things only a binary has (TQ-0044).
func TestUpdateBodyRewritesTheContentAndKeepsTheNotes(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a ticket", "--body", "## Finding\n\nThe old finding.")
	p.mustRun(t, "note", "TQ-0001", "Worth remembering.")

	r := p.mustRun(t, "update", "TQ-0001", "--body", "## Finding\n\nThe corrected finding.", "--json")
	var got taskJSON
	r.JSON(t, &got)
	if r.Stderr != "" {
		t.Errorf("stderr = %q, want it empty", r.Stderr)
	}
	if !strings.HasPrefix(got.Body, "## Finding\n\nThe corrected finding.") {
		t.Errorf("body = %q, want the new content first", got.Body)
	}
	if strings.Contains(got.Body, "The old finding.") {
		t.Errorf("the old content should be gone: %q", got.Body)
	}
	if !strings.Contains(got.Body, "Worth remembering.") {
		t.Errorf("the note should have survived: %q", got.Body)
	}

	// Reading the file back is the point of this layer: what the JSON claimed
	// has to be what landed on disk.
	onDisk, err := os.ReadFile(p.path(".tasks", "TQ-0001-a-ticket.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(onDisk), "Worth remembering.") {
		t.Errorf("the note is not in the file:\n%s", onDisk)
	}
	if strings.Count(string(onDisk), "## Notes") != 1 {
		t.Errorf("the file should hold one notes section:\n%s", onDisk)
	}

	// The body arrives on stdin when the value is "-", which is how an agent
	// hands over a document without fighting a shell over the quoting.
	piped := p.runWithStdin(t, "## Finding\n\nPiped in.\n", "update", "TQ-0001", "--body", "-", "--json")
	if piped.Code != 0 {
		t.Fatalf("update --body - = %d\nstdout: %s\nstderr: %s", piped.Code, piped.Stdout, piped.Stderr)
	}
	var fromPipe taskJSON
	piped.JSON(t, &fromPipe)
	if !strings.HasPrefix(fromPipe.Body, "## Finding\n\nPiped in.") {
		t.Errorf("body = %q, want what was piped in", fromPipe.Body)
	}
	if !strings.Contains(fromPipe.Body, "Worth remembering.") {
		t.Errorf("stdin should keep the notes too: %q", fromPipe.Body)
	}

	// The loop the guide describes: the body `tq show` hands out carries the
	// notes, so handing it straight back must not double them.
	var read taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &read)
	p.runWithStdin(t, read.Body, "update", "TQ-0001", "--body", "-")
	var round taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &round)
	if n := strings.Count(round.Body, "## Notes"); n != 1 {
		t.Errorf("a round trip left %d notes sections:\n%s", n, round.Body)
	}
	if n := strings.Count(round.Body, "Worth remembering."); n != 1 {
		t.Errorf("a round trip left %d copies of the note:\n%s", n, round.Body)
	}

	// A failure says so on stderr, leaves stdout empty, and exits 2.
	missing := p.run(t, "update", "TQ-4242", "--body", "anything", "--json")
	if missing.Code != 2 {
		t.Errorf("update on a missing task = %d, want 2", missing.Code)
	}
	if missing.Stdout != "" {
		t.Errorf("stdout = %q, want nothing", missing.Stdout)
	}
	if missing.Stderr == "" {
		t.Error("the failure should be named on stderr")
	}
}

// `--body -` blocks until standard input ends, which a pipe nobody closes never
// does. The write commands run under held signals (TQ-0015), whose whole trade
// is that they return in milliseconds — so the read happens before the hold,
// and Ctrl-C on a command still waiting for a body ends it. With the read inside
// the hold, SIGKILL was the only way out.
//
// Only a process has a signal and a pipe, so this cannot be checked anywhere
// but here.
func TestBodyFromStdinStaysInterruptible(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a ticket")

	cmd := exec.Command(binary, "update", "TQ-0001", "--body", "-")
	cmd.Dir = p.dir
	cmd.Env = os.Environ()
	// Held open for the whole test: the read must not be what ends the command.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close() }()

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	// Interrupted repeatedly rather than once, so the assertion does not rest
	// on catching the process at the instant it reaches the read.
	deadline := time.After(10 * time.Second)
	for {
		select {
		case <-done:
			return // it ended, which is the whole assertion
		case <-deadline:
			_ = cmd.Process.Kill()
			<-done
			t.Fatal("tq update --body - ignored SIGINT while waiting for a body: only SIGKILL ended it")
		case <-time.After(200 * time.Millisecond):
			_ = cmd.Process.Signal(os.Interrupt)
		}
	}
}

// Retitling moves the file, which is where a task can get lost: the ID in the
// frontmatter is what identifies it, not the name on disk.
func TestUpdateRetitlingKeepsTheTaskReachable(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "first name")
	p.mustRun(t, "update", "TQ-0001", "--title", "second name")

	var got taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &got)
	if got.Title != "second name" {
		t.Errorf("title = %q", got.Title)
	}

	var listed []taskJSON
	p.mustRun(t, "list", "--json").JSON(t, &listed)
	if len(listed) != 1 {
		t.Errorf("list = %d tasks, want 1: the old file should be gone", len(listed))
	}
}

// What a kill part-way through a retitle leaves, driven as a process because a
// signal is a thing only a process has. A save moves the task's file to the
// name the new title asks for and only then puts the new content there
// (TQ-0015), so an interrupt between the two leaves one file, under the new
// name, still holding the old content: a stale title suffix, which the ID in
// the frontmatter makes harmless, and which the next save converges. What the
// old order left instead was two files claiming TQ-0001 and every command for
// that task failing from then on.
func TestUpdateSurvivesAnInterruptedRetitle(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "first name")

	// The move without the write that follows it, which is exactly the state a
	// process killed between the two leaves on disk.
	const interrupted = "TQ-0001-second-name.md"
	if err := os.Rename(p.path(".tasks", "TQ-0001-first-name.md"), p.path(".tasks", interrupted)); err != nil {
		t.Fatal(err)
	}

	var halfway taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &halfway)
	if halfway.Title != "first name" {
		t.Errorf("title = %q, want %q: the content had not been written yet", halfway.Title, "first name")
	}

	// Success, and silence with it. The step that used to fail *after* the new
	// content was already on disk — retiring the old file — has no successor:
	// there is nothing left to do once the content is written, so there is no
	// longer a way to report failure for a save that landed.
	if r := p.mustRun(t, "update", "TQ-0001", "--title", "second name"); r.Stderr != "" {
		t.Errorf("tq update wrote to stderr: %q", r.Stderr)
	}

	var got taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &got)
	if got.Title != "second name" {
		t.Errorf("title = %q, want %q", got.Title, "second name")
	}
	entries, err := os.ReadDir(p.path(".tasks"))
	if err != nil {
		t.Fatal(err)
	}
	// One entry, so a leftover temporary is a failure here too.
	if len(entries) != 1 || entries[0].Name() != interrupted {
		found := make([]string, 0, len(entries))
		for _, entry := range entries {
			found = append(found, entry.Name())
		}
		t.Errorf(".tasks holds %v, want only %s", found, interrupted)
	}
}

// add's remaining flags, none of which had run as a binary.
func TestAddEveryField(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a dependency")
	p.mustRun(t, "add", "full task",
		"--priority", "low",
		"--assignee", "agent-web",
		"--status", "inbox",
		"--body", "the body text",
		"--label", "one",
		"--depends-on", "TQ-0001",
	)

	var got taskJSON
	p.mustRun(t, "show", "TQ-0002", "--json").JSON(t, &got)
	if got.Priority != "low" || got.Assignee != "agent-web" || got.Status != "inbox" {
		t.Errorf("scalars = %+v", got)
	}
	if got.Body != "the body text" {
		t.Errorf("body = %q", got.Body)
	}
	if strings.Join(got.DependsOn, ",") != "TQ-0001" {
		t.Errorf("depends_on = %v", got.DependsOn)
	}
}

// The filters agents use to find work, singly and combined.
func TestListAndReadyFilters(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "backend work", "--status", "todo", "--label", "backend", "--priority", "high", "--assignee", "agent-api")
	p.mustRun(t, "add", "frontend work", "--status", "todo", "--label", "frontend", "--priority", "low")
	p.mustRun(t, "move", "TQ-0002", "in-progress")

	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"by label", []string{"list", "--label", "backend", "--json"}, 1},
		{"by priority", []string{"list", "--priority", "low", "--json"}, 1},
		{"by assignee", []string{"list", "--assignee", "agent-api", "--json"}, 1},
		{"by status", []string{"list", "--status", "in-progress", "--json"}, 1},
		{"combined, matching", []string{"list", "--label", "backend", "--priority", "high", "--json"}, 1},
		{"combined, matching nothing", []string{"list", "--label", "backend", "--priority", "low", "--json"}, 0},
		{"unknown label", []string{"list", "--label", "nope", "--json"}, 0},
		// in-progress is not ready, so only the first task is
		{"ready respects status", []string{"ready", "--json"}, 1},
		{"ready with a filter", []string{"ready", "--label", "frontend", "--json"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var listed []taskJSON
			p.mustRun(t, tc.args...).JSON(t, &listed)
			if len(listed) != tc.want {
				t.Errorf("tq %s = %d tasks, want %d", strings.Join(tc.args, " "), len(listed), tc.want)
			}
		})
	}

	// A filter value that is not a valid status is a mistake worth reporting,
	// not an empty list.
	if r := p.run(t, "list", "--status", "nope"); r.Code != 1 {
		t.Errorf("an invalid status filter = %d, want 1", r.Code)
	}
}

// A file the parser cannot read is skipped, named on stderr, and the command
// still succeeds. Only a real process shows the two things this is about: the
// exit code, and which stream the warning went to (TQ-0011).
func TestListAndReadySurviveAnUnreadableFile(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "healthy", "--status", "todo")
	p.mustRun(t, "add", "healthy too", "--status", "todo")

	// A conflicted file is how this happens in practice: two agents each ran
	// `tq add` on their own branch, and .tasks is committed.
	const broken = "TQ-0003-conflicted.md"
	conflicted := "<<<<<<< HEAD\n---\nid: TQ-0003\ntitle: mine\nstatus: todo\n=======\n---\nid: TQ-0003\ntitle: theirs\nstatus: done\n>>>>>>> other\n---\n"
	if err := os.WriteFile(p.path(".tasks", broken), []byte(conflicted), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"list", "ready"} {
		t.Run(command, func(t *testing.T) {
			r := p.run(t, command)
			if r.Code != 0 {
				t.Errorf("tq %s = %d, want 0: one broken file must not fail the command\nstderr: %s", command, r.Code, r.Stderr)
			}
			for _, id := range []string{"TQ-0001", "TQ-0002"} {
				if !strings.Contains(r.Stdout, id) {
					t.Errorf("tq %s stdout is missing %s:\n%s", command, id, r.Stdout)
				}
			}
			if !strings.Contains(r.Stderr, broken) {
				t.Errorf("tq %s should name %s on stderr, got: %q", command, broken, r.Stderr)
			}
			if strings.Contains(r.Stdout, broken) {
				t.Errorf("tq %s put the warning on stdout:\n%s", command, r.Stdout)
			}
		})

		t.Run(command+" --json", func(t *testing.T) {
			r := p.run(t, command, "--json")
			if r.Code != 0 {
				t.Errorf("tq %s --json = %d, want 0\nstderr: %s", command, r.Code, r.Stderr)
			}
			// JSON decodes stdout on its own, which is the contract an agent
			// reads: the warning has to be on stderr for this to pass.
			var listed []taskJSON
			r.JSON(t, &listed)
			if len(listed) != 2 {
				t.Errorf("tq %s --json = %d tasks, want the 2 healthy ones", command, len(listed))
			}
			if !strings.Contains(r.Stderr, broken) {
				t.Errorf("tq %s --json should name %s on stderr, got: %q", command, broken, r.Stderr)
			}
		})
	}
}

// An ID two files claim is left out of the listing and named on stderr, and
// the command still succeeds. Only a real process shows the exit code and the
// stream the warning went to — and that the sentence a listing prints is the
// one `tq show` refuses that ID with (TQ-0040).
func TestListAndReadyWithholdAnIDTwoFilesClaim(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "doubled", "--status", "todo")
	p.mustRun(t, "add", "healthy", "--status", "todo")

	// What an interrupted retitle leaves behind, or two branches merging
	// cleanly because their filenames differ: a second file for TQ-0001,
	// carrying a status the real task moved on from.
	const stale = "TQ-0001-stale.md"
	content := "---\nid: TQ-0001\ntitle: doubled\nstatus: done\npriority: normal\n" +
		"created: 2026-01-01T00:00:00Z\nupdated: 2026-01-01T00:00:00Z\n---\n"
	if err := os.WriteFile(p.path(".tasks", stale), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, command := range []string{"list", "ready"} {
		t.Run(command, func(t *testing.T) {
			r := p.run(t, command)
			if r.Code != 0 {
				t.Errorf("tq %s = %d, want 0: a queue to fix must not fail the command\nstderr: %s", command, r.Code, r.Stderr)
			}
			if strings.Contains(r.Stdout, "TQ-0001") {
				t.Errorf("tq %s offers TQ-0001, which two files claim:\n%s", command, r.Stdout)
			}
			if !strings.Contains(r.Stdout, "TQ-0002") {
				t.Errorf("tq %s stdout is missing TQ-0002:\n%s", command, r.Stdout)
			}
			if !strings.Contains(r.Stderr, stale) {
				t.Errorf("tq %s should name %s on stderr, got: %q", command, stale, r.Stderr)
			}
		})

		t.Run(command+" --json", func(t *testing.T) {
			r := p.run(t, command, "--json")
			if r.Code != 0 {
				t.Errorf("tq %s --json = %d, want 0\nstderr: %s", command, r.Code, r.Stderr)
			}
			// JSON decodes stdout on its own, which is the contract an agent
			// reads: the warning has to be on stderr for this to pass.
			var listed []taskJSON
			r.JSON(t, &listed)
			if len(listed) != 1 || listed[0].ID != "TQ-0002" {
				t.Errorf("tq %s --json = %+v, want only TQ-0002", command, listed)
			}
			if !strings.Contains(r.Stderr, stale) {
				t.Errorf("tq %s --json should name %s on stderr, got: %q", command, stale, r.Stderr)
			}
		})
	}

	// The complaint the ticket opens with: a listing and a lookup disagreeing
	// about the same two files.
	listing := p.run(t, "list")
	show := p.run(t, "show", "TQ-0001")
	if show.Code != 1 {
		t.Errorf("tq show TQ-0001 = %d, want 1: the file is what is invalid", show.Code)
	}
	claim, _, _ := strings.Cut(strings.TrimPrefix(show.Stderr, "error: invalid task file: "), "\n")
	if claim == "" || !strings.Contains(listing.Stderr, claim) {
		t.Errorf("tq show says %q, tq list says %q; they are the same finding", claim, listing.Stderr)
	}
}

// A second terminal retitling tasks must not cost this one its queue. `tq
// list` used to exit 2 for a task that exists (TQ-0011) and then to print a
// short list and exit 0 (TQ-0012); now it prints the whole queue, or says on
// stderr that it may not have. Only a real process shows the exit code and
// which stream the warning went to.
func TestListStaysWholeWhileAnotherProcessRenamesTasks(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	const tasks = 12
	ids := make([]string, 0, tasks)
	for i := 1; i <= tasks; i++ {
		p.mustRun(t, "add", fmt.Sprintf("task number %d", i), "--status", "todo")
		ids = append(ids, fmt.Sprintf("TQ-%04d", i))
	}

	writer := p.renameTasks(t, ids)
	short := 0
	const runs = 20
	for i := 0; i < runs; i++ {
		r := p.run(t, "list", "--json")
		if r.Code != 0 {
			t.Fatalf("tq list --json = %d while another process was renaming tasks\nstderr: %s", r.Code, r.Stderr)
		}
		// Decodes stdout on its own, which is the contract an agent reads: a
		// warning has to be on stderr for this to pass.
		var listed []taskJSON
		r.JSON(t, &listed)

		// Every task, once each. A count alone would pass a listing that
		// dropped one task and held another twice.
		seen := map[string]int{}
		for _, task := range listed {
			seen[task.ID]++
		}
		wrong := ""
		for _, id := range ids {
			if seen[id] != 1 {
				wrong = fmt.Sprintf("%s appears %d times", id, seen[id])
				break
			}
		}
		if wrong == "" {
			continue
		}
		short++
		// A listing may still not match the directory when it never held
		// still, but it may not fail to say so: that is what an agent would
		// plan against without knowing.
		if !strings.Contains(r.Stderr, "may be missing a task") {
			t.Errorf("tq list --json: %s (%d of %d tasks) and said nothing on stderr: %q", wrong, len(listed), tasks, r.Stderr)
		}
	}
	rounds := writer.stopWhenDone(t)
	t.Logf("%d listings against %d retitles: %d did not match the queue", runs, rounds, short)
}

// A blocked task is not ready, and becomes ready when its dependency is done.
func TestReadyFollowsDependencies(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "the blocker", "--status", "todo")
	p.mustRun(t, "add", "the blocked", "--status", "todo", "--depends-on", "TQ-0001")

	var ready []taskJSON
	p.mustRun(t, "ready", "--json").JSON(t, &ready)
	if len(ready) != 1 || ready[0].ID != "TQ-0001" {
		t.Fatalf("ready = %+v, want only the blocker", ready)
	}

	p.mustRun(t, "done", "TQ-0001")
	p.mustRun(t, "ready", "--json").JSON(t, &ready)
	if len(ready) != 1 || ready[0].ID != "TQ-0002" {
		t.Errorf("ready = %+v, want the blocked task once its blocker is done", ready)
	}
}

// The reproduction TQ-0016 was filed with, run the way it happens: `rm` and
// then `tq add`. There is no tq delete and no DELETE route, so a raw file
// operation is the only way a task is ever removed — an rm, a revert, a branch
// merge — which makes this the real user path and not a contrived one.
//
// It is here rather than only in the store because every step of the damage is
// something the binary printed: the new task's ID, `tq show` calling a
// dependency done, and `tq ready` offering work whose prerequisite never
// happened.
func TestAnIDATaskDependsOnIsNotRecycledAfterTheFileIsRemoved(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "Real task one", "--status", "todo")
	p.mustRun(t, "add", "Real task two")
	p.mustRun(t, "update", "TQ-0001", "--add-dependency", "TQ-0002")

	removed := p.path(".tasks", "TQ-0002-real-task-two.md")
	if err := os.Remove(removed); err != nil {
		t.Fatalf("removing %s: %v", removed, err)
	}

	var filed taskJSON
	p.mustRun(t, "add", "Buy milk", "--status", "done", "--json").JSON(t, &filed)
	if filed.ID == "TQ-0002" {
		t.Fatalf("tq add gave the new task TQ-0002, the number TQ-0001 still depends on")
	}
	if filed.ID != "TQ-0003" {
		t.Errorf("tq add = %q, want TQ-0003", filed.ID)
	}

	// The dependency is still unmet, so it must still read as unmet.
	shown := p.mustRun(t, "show", "TQ-0001")
	if !strings.Contains(shown.Stdout, "TQ-0002 (missing)") {
		t.Errorf("tq show does not call TQ-0002 missing; it was removed, not finished:\n%s", shown.Stdout)
	}

	var ready []taskJSON
	p.mustRun(t, "ready", "--json").JSON(t, &ready)
	for _, offered := range ready {
		if offered.ID == "TQ-0001" {
			t.Errorf("tq ready offers TQ-0001; an agent picking it up believes a prerequisite that never happened")
		}
	}
}

// help and version are commands too, and the exit codes differ between no
// command and an unknown one.
func TestHelpAndUsage(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	for _, args := range [][]string{{"help"}, {"-h"}, {"--help"}} {
		r := p.run(t, args...)
		if r.Code != 0 {
			t.Errorf("tq %s = %d, want 0", strings.Join(args, " "), r.Code)
		}
		if !strings.Contains(r.Stdout, "Usage") {
			t.Errorf("tq %s printed no usage on stdout", strings.Join(args, " "))
		}
	}

	if r := p.run(t); r.Code != 1 {
		t.Errorf("bare tq = %d, want 1", r.Code)
	}
	if r := p.run(t, "nope"); r.Code != 1 || !strings.Contains(r.Stderr, "nope") {
		t.Errorf("unknown command = %d, stderr %q", r.Code, r.Stderr)
	}
}

// backlog resolves to inbox via the built-in alias.
func TestBacklogIsStillAcceptedAsInbox(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "filed the old way", "--status", "backlog")

	var got taskJSON
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &got)
	if got.Status != "inbox" {
		t.Errorf("status = %q, want inbox", got.Status)
	}

	p.mustRun(t, "move", "TQ-0001", "backlog")
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &got)
	if got.Status != "inbox" {
		t.Errorf("after move to backlog, status = %q, want inbox", got.Status)
	}
}

// ── A column the project removed (TQ-0088) ──────────────────────
//
// Editing `.taskqueue.yaml` is a documented thing to do, and deleting a column
// strands every task still filed in it. Every one of them moves to the default
// column, the command that noticed says which on stderr, and the exit code and
// the `--json` stdout contract are untouched by any of it.

const boardWithReview = `version: 1
path: .tasks
columns:
  - name: backlog
    display_name: Backlog
    default: true
  - name: review
    display_name: Review
    consider_ready: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

const boardWithoutReview = `version: 1
path: .tasks
columns:
  - name: backlog
    display_name: Backlog
    default: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

// strandedProject is the report: three tasks in `review`, and a config the user
// has since deleted `review` from.
func strandedProject(t *testing.T) *project {
	t.Helper()
	p := newProject(t)
	if err := writeFile(p.path(".taskqueue.yaml"), boardWithReview); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"One", "Two", "Three"} {
		p.mustRun(t, "add", title, "--status", "review")
	}
	if err := writeFile(p.path(".taskqueue.yaml"), boardWithoutReview); err != nil {
		t.Fatal(err)
	}
	return p
}

// statusesOnDisk reads the status line out of every task file. The files are
// what this whole report is about, so nothing here asks tq what they say.
func statusesOnDisk(t *testing.T, p *project) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(p.path(".tasks"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "TQ-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(p.path(".tasks", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		parts := strings.SplitN(strings.TrimSuffix(entry.Name(), ".md"), "-", 3)
		if len(parts) < 2 {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			if rest, found := strings.CutPrefix(line, "status: "); found {
				out[parts[0]+"-"+parts[1]] = rest
				break
			}
		}
	}
	return out
}

func TestRemovingAColumnMovesEveryTaskLeftInIt(t *testing.T) {
	t.Parallel()
	p := strandedProject(t)

	r := p.mustRun(t, "list", "--json")
	var tasks []taskJSON
	r.JSON(t, &tasks)
	if len(tasks) != 3 {
		t.Fatalf("tq list --json returned %d tasks, want 3", len(tasks))
	}
	for _, tk := range tasks {
		if tk.Status != "backlog" {
			t.Errorf("%s = %q, want the default column", tk.ID, tk.Status)
		}
	}
	// stdout was JSON alone — JSON above would have failed otherwise — so the
	// report of what moved is on stderr, where it cannot break an agent.
	if !strings.Contains(r.Stderr, "review") || !strings.Contains(r.Stderr, "backlog") {
		t.Errorf("stderr = %q, want the column that went and the one the tasks moved to", r.Stderr)
	}
	for _, id := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		if !strings.Contains(r.Stderr, id) {
			t.Errorf("stderr = %q, want it to name %s", r.Stderr, id)
		}
	}

	for id, status := range statusesOnDisk(t, p) {
		if status != "backlog" {
			t.Errorf("%s is %q on disk, want backlog: the listing must not show a status the file does not hold", id, status)
		}
	}

	// Settled: the next command has nothing to report and still exits 0.
	again := p.mustRun(t, "list", "--json")
	if again.Stderr != "" {
		t.Errorf("stderr = %q on a settled queue, want nothing", again.Stderr)
	}
}

// `tq init` is the documented answer to a config edit, and running it after one
// is what settles the queue.
func TestInitReconcilesARemovedColumn(t *testing.T) {
	t.Parallel()
	p := strandedProject(t)

	r := p.mustRun(t, "init")
	if !strings.Contains(r.Stdout, "already initialized") {
		t.Errorf("stdout = %q, want init to report the queue it found", r.Stdout)
	}
	if !strings.Contains(r.Stderr, "review") {
		t.Errorf("stderr = %q, want init to name the column that went", r.Stderr)
	}
	for id, status := range statusesOnDisk(t, p) {
		if status != "backlog" {
			t.Errorf("%s is %q on disk, want tq init to have moved it", id, status)
		}
	}

	// And `tq init --json` keeps stdout to itself, reconciliation or not.
	p2 := strandedProject(t)
	var payload map[string]any
	p2.mustRun(t, "init", "--json").JSON(t, &payload)
}

// The other half of the report: one unrelated edit used to rewrite the status
// of the single task it touched and leave the rest of the queue behind.
func TestAnUnrelatedEditNeverChangesAStatusOnItsOwn(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "Work")
	p.mustRun(t, "move", "TQ-0001", "in-progress")

	for _, args := range [][]string{
		{"update", "TQ-0001", "--assignee", "bob"},
		{"update", "TQ-0001", "--title", "Renamed"},
		{"note", "TQ-0001", "something unrelated"},
	} {
		p.mustRun(t, args...)
		if got := statusesOnDisk(t, p)["TQ-0001"]; got != "in-progress" {
			t.Fatalf("status on disk = %q after `tq %s`, want in-progress", got, strings.Join(args, " "))
		}
	}
}

// A board whose default column is not its first. The first-column answer would
// have filed every stranded task in `shipped`, which is the column that
// satisfies dependencies — marking them done and unblocking their dependents.
func TestReconciliationUsesTheDefaultColumnNotTheFirst(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	// `shipped` is both the first column and the one that satisfies
	// dependencies; `backlog` is the default and sits last.
	const doneFirstWithReview = `version: 1
path: .tasks
columns:
  - name: shipped
    display_name: Shipped
    consider_done: true
  - name: review
    display_name: Review
    consider_ready: true
  - name: backlog
    display_name: Backlog
    default: true
`
	const doneFirstWithoutReview = `version: 1
path: .tasks
columns:
  - name: shipped
    display_name: Shipped
    consider_done: true
  - name: backlog
    display_name: Backlog
    default: true
`
	if err := writeFile(p.path(".taskqueue.yaml"), doneFirstWithReview); err != nil {
		t.Fatal(err)
	}
	p.mustRun(t, "add", "Blocked work", "--status", "review")
	p.mustRun(t, "add", "Dependent", "--depends-on", "TQ-0001")

	if err := writeFile(p.path(".taskqueue.yaml"), doneFirstWithoutReview); err != nil {
		t.Fatal(err)
	}

	r := p.mustRun(t, "list", "--json")
	var tasks []taskJSON
	r.JSON(t, &tasks)
	for _, tk := range tasks {
		if tk.Status != "backlog" {
			t.Errorf("%s = %q, want backlog: the default column, not the first", tk.ID, tk.Status)
		}
	}
	if got := statusesOnDisk(t, p)["TQ-0001"]; got != "backlog" {
		t.Errorf("TQ-0001 is %q on disk, want backlog", got)
	}
	// The consequence the rule exists for: a stranded prerequisite must not
	// come back marked done.
	var ready []taskJSON
	p.mustRun(t, "ready", "--json").JSON(t, &ready)
	for _, tk := range ready {
		if tk.ID == "TQ-0002" {
			t.Error("TQ-0002 is ready, so its prerequisite was filed in the column that satisfies dependencies")
		}
	}
}

// A running board and a config edited under it: the server reconciles on the
// next request, and what it serves is what the files hold.
func TestAServingBoardReconcilesAConfigEditedUnderIt(t *testing.T) {
	t.Parallel()
	p := strandedProject(t)
	srv := p.serve(t)

	var listed []taskJSON
	srv.get(t, "/api/tasks", &listed)
	if len(listed) != 3 {
		t.Fatalf("GET /api/tasks returned %d tasks, want 3", len(listed))
	}
	for _, tk := range listed {
		if tk.Status != "backlog" {
			t.Errorf("%s = %q, want backlog", tk.ID, tk.Status)
		}
	}
	for id, status := range statusesOnDisk(t, p) {
		if status != "backlog" {
			t.Errorf("%s is %q on disk, want backlog: the board must not show a column the file is not in", id, status)
		}
	}
}

// A queue that cannot be written still lists, and still exits 0. Reconciling
// runs inside a read now, and a read-only checkout — CI, a container volume, a
// root-owned .tasks — must not become a queue nobody can list because a column
// was taken out of its config.
func TestAQueueThatCannotBeWrittenStillLists(t *testing.T) {
	t.Parallel()
	p := strandedProject(t)
	dir := p.path(".tasks")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if f, err := os.CreateTemp(dir, "probe-*"); err == nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		t.Skip("this filesystem let a write through a read-only directory")
	}

	r := p.mustRun(t, "list", "--json")
	var tasks []taskJSON
	r.JSON(t, &tasks)
	if len(tasks) != 3 {
		t.Fatalf("tq list --json returned %d tasks, want 3", len(tasks))
	}
	for _, tk := range tasks {
		// The column their files still hold: a listing shows what is on disk.
		if tk.Status != "review" {
			t.Errorf("%s = %q, want review, the status its file holds", tk.ID, tk.Status)
		}
	}
	if !strings.Contains(r.Stderr, "could not move") {
		t.Errorf("stderr = %q, want it to say the migration did not happen", r.Stderr)
	}
	for _, id := range []string{"TQ-0001", "TQ-0002", "TQ-0003"} {
		if !strings.Contains(r.Stderr, id) {
			t.Errorf("stderr = %q, want it to name %s among the tasks it could not move", r.Stderr, id)
		}
	}
}
