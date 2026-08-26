//go:build integration

package integration

import (
	"fmt"
	"os"
	"strings"
	"testing"
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
