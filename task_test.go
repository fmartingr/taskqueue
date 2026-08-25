package main

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func task(id, status string, deps ...string) Task {
	return Task{
		ID:        id,
		Title:     "Task " + id,
		Status:    status,
		Priority:  PriorityNormal,
		DependsOn: deps,
		Created:   time.Now().Truncate(time.Second),
		Updated:   time.Now().Truncate(time.Second),
	}
}

func TestReady(t *testing.T) {
	tasks := []Task{
		task("TQ-0001", StatusDone),
		task("TQ-0002", StatusTodo),
		task("TQ-0003", StatusTodo, "TQ-0001"),
		task("TQ-0004", StatusTodo, "TQ-0002"),
		task("TQ-0005", StatusTodo, "TQ-9999"),
		task("TQ-0006", StatusInProgress),
		task("TQ-0007", StatusDone),
		task("TQ-0008", StatusBacklog, "TQ-0001", "TQ-0007"),
	}
	index := IndexTasks(tasks)

	tests := []struct {
		id   string
		want bool
		why  string
	}{
		{"TQ-0002", true, "no dependencies"},
		{"TQ-0003", true, "dependency is done"},
		{"TQ-0008", true, "all dependencies done, backlog still counts"},
		{"TQ-0004", false, "dependency is not done"},
		{"TQ-0005", false, "dependency is missing"},
		{"TQ-0006", false, "already in progress"},
		{"TQ-0001", false, "already done"},
	}
	for _, tc := range tests {
		if got := IsReady(index[tc.id], index); got != tc.want {
			t.Errorf("IsReady(%s) = %v, want %v (%s)", tc.id, got, tc.want, tc.why)
		}
	}
}

func TestBlocked(t *testing.T) {
	tasks := []Task{
		task("TQ-0001", StatusDone),
		task("TQ-0002", StatusTodo),
		task("TQ-0003", StatusTodo, "TQ-0001"),
		task("TQ-0004", StatusTodo, "TQ-0002"),
		task("TQ-0005", StatusTodo, "TQ-9999"),
	}
	index := IndexTasks(tasks)
	for id, want := range map[string]bool{
		"TQ-0002": false,
		"TQ-0003": false,
		"TQ-0004": true,
		"TQ-0005": true,
	} {
		if got := IsBlocked(index[id], index); got != want {
			t.Errorf("IsBlocked(%s) = %v, want %v", id, got, want)
		}
	}
}

func TestValidateRejectsSelfDependency(t *testing.T) {
	tk := task("TQ-0001", StatusTodo, "TQ-0001")
	if err := tk.Validate(); err == nil || !strings.Contains(err.Error(), "cannot depend on itself") {
		t.Errorf("Validate() = %v, want a self-dependency error", err)
	}
}

func TestSortTasks(t *testing.T) {
	base := time.Date(2026, 8, 25, 8, 0, 0, 0, time.UTC)
	mk := func(id, status, priority string, minutes int) Task {
		tk := task(id, status)
		tk.Priority = priority
		tk.Created = base.Add(time.Duration(minutes) * time.Minute)
		return tk
	}
	tasks := []Task{
		mk("TQ-0005", StatusDone, PriorityUrgent, 0),
		mk("TQ-0004", StatusTodo, PriorityNormal, 1),
		mk("TQ-0003", StatusTodo, PriorityUrgent, 2),
		mk("TQ-0002", StatusBacklog, PriorityLow, 3),
		mk("TQ-0001", StatusInProgress, PriorityLow, 4),
		mk("TQ-0006", StatusTodo, PriorityNormal, 0),
	}
	SortTasks(tasks)

	var got []string
	for _, tk := range tasks {
		got = append(got, tk.ID)
	}
	want := []string{"TQ-0002", "TQ-0003", "TQ-0006", "TQ-0004", "TQ-0001", "TQ-0005"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("SortTasks order = %v, want %v", got, want)
	}
}

func TestFilterTasks(t *testing.T) {
	a := task("TQ-0001", StatusTodo)
	a.Labels = []string{"backend", "auth"}
	a.Assignee = "agent-api"
	a.Priority = PriorityHigh

	b := task("TQ-0002", StatusInProgress)
	b.Labels = []string{"frontend"}
	b.Assignee = "agent-ui"

	c := task("TQ-0003", StatusTodo, "TQ-0002")
	c.Labels = []string{"backend"}

	tasks := []Task{a, b, c}

	tests := []struct {
		name   string
		filter Filter
		want   string
	}{
		{"none", Filter{}, "TQ-0001,TQ-0002,TQ-0003"},
		{"status", Filter{Status: StatusTodo}, "TQ-0001,TQ-0003"},
		{"priority", Filter{Priority: PriorityHigh}, "TQ-0001"},
		{"label", Filter{Label: "backend"}, "TQ-0001,TQ-0003"},
		{"assignee", Filter{Assignee: "agent-ui"}, "TQ-0002"},
		{"combined", Filter{Status: StatusTodo, Label: "backend", Priority: PriorityHigh}, "TQ-0001"},
		{"ready", Filter{Ready: true}, "TQ-0001"},
		{"ready and label", Filter{Ready: true, Label: "auth"}, "TQ-0001"},
		{"no match", Filter{Assignee: "nobody"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ids []string
			for _, tk := range FilterTasks(tasks, tc.filter) {
				ids = append(ids, tk.ID)
			}
			if got := strings.Join(ids, ","); got != tc.want {
				t.Errorf("FilterTasks = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilterValidate(t *testing.T) {
	if err := (Filter{Status: "nope"}).Validate(); err == nil {
		t.Error("expected an invalid status error")
	}
	if err := (Filter{Priority: "nope"}).Validate(); err == nil {
		t.Error("expected an invalid priority error")
	}
	if err := (Filter{Status: StatusTodo, Priority: PriorityLow}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestAppendNote(t *testing.T) {
	ts := time.Date(2026, 8, 25, 9, 42, 0, 0, time.FixedZone("", 2*3600))
	stamp := "2026-08-25T09:42:00+02:00"

	t.Run("creates the section when missing", func(t *testing.T) {
		got := AppendNote("Some description.", "First note.", ts)
		want := "Some description.\n\n## Notes\n\n- " + stamp + " — First note."
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("creates the section for an empty body", func(t *testing.T) {
		got := AppendNote("", "First note.", ts)
		want := "## Notes\n\n- " + stamp + " — First note."
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("appends under an existing section", func(t *testing.T) {
		body := "Description.\n\n## Notes\n\n- earlier note"
		got := AppendNote(body, "Second note.", ts)
		want := body + "\n- " + stamp + " — Second note."
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("appends before a following section", func(t *testing.T) {
		body := "Description.\n\n## Notes\n\n- earlier note\n\n## Acceptance criteria\n\n- something"
		got := AppendNote(body, "Second note.", ts)
		want := "Description.\n\n## Notes\n\n- earlier note\n- " + stamp + " — Second note.\n\n## Acceptance criteria\n\n- something"
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	t.Run("collapses newlines in the note text", func(t *testing.T) {
		got := AppendNote("", "line one\nline two", ts)
		if strings.Count(got, "\n- ") != 1 || !strings.Contains(got, "line one line two") {
			t.Errorf("multiline note should collapse to one bullet, got %q", got)
		}
	})
}

func TestApplyPatch(t *testing.T) {
	base := task("TQ-0001", StatusTodo)
	base.Labels = []string{"backend"}
	base.Assignee = "agent-api"
	base.Body = "Body."

	t.Run("only the supplied fields change", func(t *testing.T) {
		title := "New title"
		got := ApplyPatch(base, TaskPatch{Title: &title})
		if got.Title != title {
			t.Errorf("Title = %q, want %q", got.Title, title)
		}
		if got.Status != base.Status || got.Assignee != base.Assignee || got.Body != base.Body {
			t.Errorf("unrelated fields changed: %+v", got)
		}
	})

	t.Run("labels and dependencies are added and removed", func(t *testing.T) {
		got := ApplyPatch(base, TaskPatch{
			AddLabels:  []string{"auth", "backend"}, // already present: not duplicated
			AddDeps:    []string{"TQ-0002"},
			RemoveDeps: []string{"TQ-0009"}, // absent: no-op
		})
		if strings.Join(got.Labels, ",") != "backend,auth" {
			t.Errorf("Labels = %v", got.Labels)
		}
		if strings.Join(got.DependsOn, ",") != "TQ-0002" {
			t.Errorf("DependsOn = %v", got.DependsOn)
		}

		got = ApplyPatch(got, TaskPatch{RemoveLabels: []string{"backend"}, RemoveDeps: []string{"TQ-0002"}})
		if strings.Join(got.Labels, ",") != "auth" {
			t.Errorf("Labels = %v", got.Labels)
		}
		if len(got.DependsOn) != 0 {
			t.Errorf("DependsOn = %v, want empty", got.DependsOn)
		}
	})

	t.Run("replacement wins over add and remove", func(t *testing.T) {
		labels := []string{"only"}
		got := ApplyPatch(base, TaskPatch{Labels: &labels, AddLabels: []string{"extra"}})
		if strings.Join(got.Labels, ",") != "only,extra" {
			t.Errorf("Labels = %v, want the replacement then the addition", got.Labels)
		}
	})

	t.Run("an empty patch is detectable", func(t *testing.T) {
		if !(TaskPatch{}).IsEmpty() {
			t.Error("empty patch should report IsEmpty")
		}
		status := StatusDone
		if (TaskPatch{Status: &status}).IsEmpty() {
			t.Error("a patch with a status should not report IsEmpty")
		}
		if (TaskPatch{AddLabels: []string{"x"}}).IsEmpty() {
			t.Error("a patch with a label addition should not report IsEmpty")
		}
	})
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Suffix tasks", "suffix-tasks"},
		{"Implement OIDC authentication", "implement-oidc-authentication"},
		{"Fix the /api/tasks endpoint", "fix-the-api-tasks-endpoint"},
		{"  Trim   the   spaces  ", "trim-the-spaces"},
		{"CamelCase AND UPPER", "camelcase-and-upper"},
		{"Release v1.2.3", "release-v1-2-3"},
		{"Implementar autenticación", "implementar-autenticación"},
		{"...", ""},
		{"", ""},
		{"-leading and trailing-", "leading-and-trailing"},
		{
			"A very long title that should be cut somewhere sensible instead of mid-word",
			"a-very-long-title-that-should-be-cut-somewhere",
		},
	}
	for _, tc := range tests {
		if got := Slugify(tc.title); got != tc.want {
			t.Errorf("Slugify(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestSlugifyStaysWithinTheLengthBudget(t *testing.T) {
	slug := Slugify(strings.Repeat("ünicöde ", 40))
	if got := len([]rune(slug)); got > maxSlugRunes {
		t.Errorf("slug is %d runes, want at most %d: %q", got, maxSlugRunes, slug)
	}
	if !utf8.ValidString(slug) {
		t.Errorf("slug is not valid UTF-8: %q", slug)
	}
}
