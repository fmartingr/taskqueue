//go:build integration

package integration

import (
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
