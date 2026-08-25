package task

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
	note := "- " + stamp + " — "

	tests := []struct {
		name string
		body string
		text string
		want string
	}{
		{
			name: "creates the section under a rule when missing",
			body: "Some description.",
			text: "First note.",
			want: "Some description.\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "creates the section without a rule for an empty body",
			body: "",
			text: "First note.",
			want: "## Notes\n\n" + note + "First note.",
		},
		{
			name: "appends under an existing section",
			body: "Description.\n\n---\n\n## Notes\n\n- earlier note",
			text: "Second note.",
			want: "Description.\n\n---\n\n## Notes\n\n- earlier note\n" + note + "Second note.",
		},
		{
			name: "upgrades a legacy section that has no rule",
			body: "Description.\n\n## Notes\n\n- earlier note",
			text: "Second note.",
			want: "Description.\n\n---\n\n## Notes\n\n- earlier note\n" + note + "Second note.",
		},
		{
			name: "keeps continuation lines of an existing note",
			body: "Description.\n\n---\n\n## Notes\n\n- earlier note\n  wrapped onto a second line",
			text: "Second note.",
			want: "Description.\n\n---\n\n## Notes\n\n- earlier note\n  wrapped onto a second line\n" + note + "Second note.",
		},
		{
			name: "an empty section keeps its blank line",
			body: "Description.\n\n---\n\n## Notes",
			text: "First note.",
			want: "Description.\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "a Notes section followed by other content is content",
			body: "Description.\n\n## Notes\n\nProse about notes.\n\n## Acceptance criteria\n\n- something",
			text: "First note.",
			want: "Description.\n\n## Notes\n\nProse about notes.\n\n## Acceptance criteria\n\n- something" +
				"\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "a Notes section followed by a level-1 heading is content",
			body: "## Notes\n\n- old note\n\n# Appendix\n\ntext",
			text: "First note.",
			want: "## Notes\n\n- old note\n\n# Appendix\n\ntext\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "a Notes section followed by a sub-heading is content",
			body: "## Notes\n\n- old note\n\n### Sub\n\ntext",
			text: "First note.",
			want: "## Notes\n\n- old note\n\n### Sub\n\ntext\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "a Notes heading inside a fenced block is content",
			body: "Description.\n\n```markdown\n## Notes\n\n- an example\n```",
			text: "First note.",
			want: "Description.\n\n```markdown\n## Notes\n\n- an example\n```" +
				"\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "an unclosed fence does not hide the notes section",
			body: "Description.\n\n```go\nfunc x() {}\n\n---\n\n## Notes\n\n- earlier note",
			text: "Second note.",
			want: "Description.\n\n```go\nfunc x() {}\n\n---\n\n## Notes\n\n- earlier note\n" + note + "Second note.",
		},
		{
			name: "a rule with nothing above it introduces the notes",
			body: "---\n\n## Notes\n\n- earlier note",
			text: "Second note.",
			want: "## Notes\n\n- earlier note\n" + note + "Second note.",
		},
		{
			name: "a setext heading underline is not the notes rule",
			body: "Description\n---\n\n## Notes\n\n- earlier note",
			text: "Second note.",
			want: "Description\n---\n\n---\n\n## Notes\n\n- earlier note\n" + note + "Second note.",
		},
		{
			name: "content ending in a rule keeps it",
			body: "Description.\n\n---",
			text: "First note.",
			want: "Description.\n\n---\n\n---\n\n## Notes\n\n" + note + "First note.",
		},
		{
			name: "surrounding blank lines are normalised away",
			body: "\n\nDescription.\n\n\n---\n\n\n## Notes\n\n- earlier note\n\n",
			text: "Second note.",
			want: "Description.\n\n---\n\n## Notes\n\n- earlier note\n" + note + "Second note.",
		},
		{
			name: "keeps the note's own line breaks, indented under the bullet",
			body: "",
			text: "line one\nline two",
			want: "## Notes\n\n" + note + "line one\n  line two",
		},
		{
			name: "a note can hold a list of its own",
			body: "",
			text: "Line one.\n\n- bullet a\n- bullet b",
			want: "## Notes\n\n" + note + "Line one.\n\n  - bullet a\n  - bullet b",
		},
		{
			name: "existing indentation inside a note is kept relative to the bullet",
			body: "",
			text: "Ran:\n\n    tq list --json\n",
			want: "## Notes\n\n" + note + "Ran:\n\n      tq list --json",
		},
		{
			name: "carriage returns, trailing spaces and blank runs are normalised",
			body: "",
			text: "line one  \r\n\r\n\r\n\r\nline two\t\n",
			want: "## Notes\n\n" + note + "line one\n\n  line two",
		},
		{
			name: "a note that is only whitespace is dropped rather than half-written",
			body: "Description.",
			text: "  \n\n\t\n",
			want: "Description.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AppendNote(tt.body, tt.text, ts); got != tt.want {
				t.Errorf("AppendNote(%q):\ngot:  %q\nwant: %q", tt.body, got, tt.want)
			}
		})
	}

	t.Run("a second note lands under the first", func(t *testing.T) {
		body := AppendNote("Description.", "First note.", ts)
		got := AppendNote(body, "Second note.", ts.Add(time.Hour))
		want := "Description.\n\n---\n\n## Notes\n\n" + note + "First note.\n" +
			"- 2026-08-25T10:42:00+02:00 — Second note."
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
	})

	// A multi-line note has to survive being written, read back and appended
	// to: the second note must land in the same section, under the first.
	t.Run("a multi-line note survives a second append", func(t *testing.T) {
		body := AppendNote("Description.", "Line one.\n\n- bullet a\n- bullet b", ts)
		got := AppendNote(body, "Second note.", ts.Add(time.Hour))
		want := "Description.\n\n---\n\n## Notes\n\n" + note + "Line one.\n\n  - bullet a\n  - bullet b\n" +
			"- 2026-08-25T10:42:00+02:00 — Second note."
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
		content, notes := notesSection(got)
		if content != "Description." {
			t.Errorf("content = %q, want %q", content, "Description.")
		}
		if !strings.Contains(notes, "  - bullet a") {
			t.Errorf("the note lost its list:\n%q", notes)
		}
	})

	// A heading inside a note is indented under its bullet, so it must not be
	// read as the heading that ends the notes section — otherwise the next
	// note would open a second one and the board would show the notes as
	// content.
	t.Run("a heading inside a note does not end the section", func(t *testing.T) {
		body := AppendNote("Description.", "Fixed it.\n\n## Details\n\ntext", ts)
		got := AppendNote(body, "Second note.", ts.Add(time.Hour))
		want := "Description.\n\n---\n\n## Notes\n\n" + note + "Fixed it.\n\n  ## Details\n\n  text\n" +
			"- 2026-08-25T10:42:00+02:00 — Second note."
		if got != want {
			t.Errorf("got:\n%q\nwant:\n%q", got, want)
		}
		if strings.Count(got, notesHeading+"\n") != 1 {
			t.Errorf("a second notes section was opened:\n%s", got)
		}
	})
}

func TestNotesSection(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		content string
		notes   string
	}{
		{
			name: "no notes at all",
			body: "Just content.", content: "Just content.",
		},
		{
			// A heading indented 1-3 spaces is still a heading to CommonMark,
			// so it must keep closing a mid-body "## Notes" the way an
			// unindented one does — the whole body stays content.
			name:    "an indented heading below a mid-body Notes heading is content",
			body:    "# Task\n\n## Notes\n\nWe keep decisions here, in prose.\n\n ## Acceptance\n\n- ships",
			content: "# Task\n\n## Notes\n\nWe keep decisions here, in prose.\n\n ## Acceptance\n\n- ships",
		},
		{
			// The same allowance applies to the notes heading itself: indented
			// by two spaces it still opens the section, so a note appends in
			// place instead of starting a second one.
			name:    "an indented Notes heading still opens the section",
			body:    "Description.\n\n---\n\n  ## Notes\n\n- 2026-01-01T00:00:00Z — old note",
			content: "Description.",
			notes:   "- 2026-01-01T00:00:00Z — old note",
		},
		{
			name:    "canonical section",
			body:    "Content.\n\n---\n\n## Notes\n\n- a note",
			content: "Content.", notes: "- a note",
		},
		{
			name:    "legacy section without a rule",
			body:    "Content.\n\n## Notes\n\n- a note",
			content: "Content.", notes: "- a note",
		},
		{
			name:    "notes followed by another section are content",
			body:    "Content.\n\n## Notes\n\n- a note\n\n## After\n\nmore",
			content: "Content.\n\n## Notes\n\n- a note\n\n## After\n\nmore",
		},
		{
			name:    "notes followed by a level-1 heading are content",
			body:    "## Notes\n\n- a note\n\n# Appendix\n\nmore",
			content: "## Notes\n\n- a note\n\n# Appendix\n\nmore",
		},
		{
			name:    "notes followed by a sub-heading are content",
			body:    "## Notes\n\n- a note\n\n### Sub\n\nmore",
			content: "## Notes\n\n- a note\n\n### Sub\n\nmore",
		},
		{
			name:    "the last of several Notes headings wins",
			body:    "## Notes\n\ncontent\n\n## Other\n\nx\n\n---\n\n## Notes\n\n- a note",
			content: "## Notes\n\ncontent\n\n## Other\n\nx", notes: "- a note",
		},
		{
			name:    "an empty notes section",
			body:    "Content.\n\n---\n\n## Notes",
			content: "Content.", notes: "",
		},
		{
			name:    "notes only",
			body:    "## Notes\n\n- a note",
			content: "", notes: "- a note",
		},
		{
			// An indented heading belongs to the bullet above it — a note may
			// carry one — so it does not open a section of its own.
			name:    "an indented heading inside a note is part of the note",
			body:    "Content.\n\n---\n\n## Notes\n\n- a note\n\n  ## Details\n\n  text",
			content: "Content.", notes: "- a note\n\n  ## Details\n\n  text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, notes := notesSection(tt.body)
			if content != tt.content {
				t.Errorf("content:\ngot:  %q\nwant: %q", content, tt.content)
			}
			if notes != tt.notes {
				t.Errorf("notes:\ngot:  %q\nwant: %q", notes, tt.notes)
			}
		})
	}
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

// A newline in a single-line field renders as a block scalar, which is how a
// task file grows a line that looks like the frontmatter delimiter. The
// filename is a slug of the title too, so a line break there is a mistake.
func TestValidateRejectsLineBreaksInSingleLineFields(t *testing.T) {
	base := Task{ID: "TQ-0001", Title: "ok", Status: StatusTodo}

	for _, tc := range []struct {
		name string
		task Task
	}{
		{"title", func() Task { t := base; t.Title = "line1\n---\nline2"; return t }()},
		{"title with a carriage return", func() Task { t := base; t.Title = "line1\rline2"; return t }()},
		{"assignee", func() Task { t := base; t.Assignee = "agent\n---\nx"; return t }()},
		{"label", func() Task { t := base; t.Labels = []string{"ok", "bad\nlabel"}; return t }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.task.ValidateForWrite(); err == nil {
				t.Error("Validate() = nil, want an error for a line break")
			}
		})
	}

	if err := base.ValidateForWrite(); err != nil {
		t.Errorf("ValidateForWrite() on a clean task = %v, want nil", err)
	}
}
