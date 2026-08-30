package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
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
		task("TQ-0008", StatusInbox, "TQ-0001", "TQ-0007"),
	}
	index := IndexTasks(tasks)

	tests := []struct {
		id   string
		want bool
		why  string
	}{
		{"TQ-0002", true, "no dependencies"},
		{"TQ-0003", true, "dependency is done"},
		{"TQ-0008", false, "dependencies all done, but intake is not offered until it is triaged"},
		{"TQ-0004", false, "dependency is not done"},
		{"TQ-0005", false, "dependency is missing"},
		{"TQ-0006", false, "already in progress"},
		{"TQ-0001", false, "already done"},
	}
	for _, tc := range tests {
		if got := IsReady(index[tc.id], index, Columns{}); got != tc.want {
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
		if got := IsBlocked(index[id], index, Columns{}); got != want {
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
	SortTasks(tasks, Priorities{}, Columns{})

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
			for _, tk := range FilterTasks(tasks, tc.filter, Columns{}) {
				ids = append(ids, tk.ID)
			}
			if got := strings.Join(ids, ","); got != tc.want {
				t.Errorf("FilterTasks = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFilterValidate(t *testing.T) {
	if err := (Filter{Status: "nope"}).Validate(Priorities{}, Columns{}); err == nil {
		t.Error("expected an invalid status error")
	}
	if err := (Filter{Priority: "nope"}).Validate(Priorities{}, Columns{}); err == nil {
		t.Error("expected an invalid priority error")
	}
	if err := (Filter{Status: StatusTodo, Priority: PriorityLow}).Validate(Priorities{}, Columns{}); err != nil {
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
			name: "a uniformly indented paste loses the margin it was copied with",
			body: "",
			text: "    make test\n    make build",
			want: "## Notes\n\n" + note + "make test\n  make build",
		},
		{
			name: "the indentation under a shared margin survives it",
			body: "",
			text: "    steps:\n      - one\n      - two",
			want: "## Notes\n\n" + note + "steps:\n    - one\n    - two",
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

// TestNoteBulletMatchesTheSharedFixture pins the note format against the file
// frontend/notes.test.ts reads too.
//
// Both surfaces render notes, and until TQ-0054 they rendered them differently:
// a blank run and a line's trailing whitespace survived a board save and did
// not survive `tq note`, so the canonical form of a committed file depended on
// which surface had touched it last. Neither suite could catch that on its own,
// because they shared no case. This is that case list, and adding to it is how
// a new rule reaches both.
func TestNoteBulletMatchesTheSharedFixture(t *testing.T) {
	var fixture struct {
		Timestamp string `json:"timestamp"`
		Cases     []struct {
			Name   string `json:"name"`
			Text   string `json:"text"`
			Bullet string `json:"bullet"`
		} `json:"cases"`
	}

	raw, err := os.ReadFile(filepath.Join("testdata", "notes.json"))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("parsing the fixture: %v", err)
	}
	ts, err := time.Parse(time.RFC3339, fixture.Timestamp)
	if err != nil {
		t.Fatalf("parsing the fixture timestamp: %v", err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture holds no cases")
	}

	for _, tt := range fixture.Cases {
		t.Run(tt.Name, func(t *testing.T) {
			if got := noteBullet(tt.Text, ts); got != tt.Bullet {
				t.Errorf("noteBullet(%q):\ngot:  %q\nwant: %q", tt.Text, got, tt.Bullet)
			}
			// The body the bullet lands in, both ways round, since that is what
			// the board writes back and what has to come out byte-identical.
			if got, want := AppendNote("", tt.Text, ts), notesHeading+"\n\n"+tt.Bullet; got != want {
				t.Errorf("AppendNote into an empty body:\ngot:  %q\nwant: %q", got, want)
			}
			want := "Description.\n\n" + notesRule + "\n\n" + notesHeading + "\n\n" + tt.Bullet
			if got := AppendNote("Description.", tt.Text, ts); got != want {
				t.Errorf("AppendNote under content:\ngot:  %q\nwant: %q", got, want)
			}
		})
	}
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

// ReplaceContent is what `tq update --body` writes, and the notes surviving it
// is the whole point: they are appended one at a time and reconstructible from
// nothing, while the content is a document its author has in front of them.
func TestReplaceContent(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		content string
		want    string
	}{
		{
			name:    "the notes survive a rewritten content",
			body:    "Old finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
			content: "New finding.",
			want:    "New finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
		},
		{
			name:    "a body with no notes is replaced whole",
			body:    "Old finding.",
			content: "New finding.",
			want:    "New finding.",
		},
		{
			// No section is grown out of nothing: a task with no notes still
			// has none after its body is rewritten.
			name:    "no notes are invented",
			body:    "Old finding.",
			content: "",
			want:    "",
		},
		{
			// Clearing the content is a body edit like any other, and it must
			// not take the record with it. The rule goes with the content it
			// separated, which is how AppendNote writes a section that opens a
			// body.
			name:    "clearing the content keeps the notes",
			body:    "Old finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
			content: "",
			want:    "## Notes\n\n- 2026-01-01T00:00:00Z — a note",
		},
		{
			// Written before the rule existed. The notes are found anyway and
			// the rewrite is where the file gains it.
			name:    "a legacy section without a rule gains one",
			body:    "Old finding.\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
			content: "New finding.",
			want:    "New finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
		},
		{
			// Prose about notes is content: a "## Notes" heading followed by
			// another section is not the notes section, and a "---" without a
			// blank line above it underlines a setext heading rather than
			// opening one. Neither may be mistaken for the record, on the way
			// in or on the way out.
			name: "a rule and a Notes heading in ordinary prose stay content",
			body: "Old finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
			content: "## Notes\nHow this project writes them.\n\n" +
				"## Format\n\nA note is a bullet.",
			want: "## Notes\nHow this project writes them.\n\n## Format\n\nA note is a bullet." +
				"\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note",
		},
		{
			name:    "surrounding blank lines are trimmed",
			body:    "Old.\n\n---\n\n## Notes\n\n- a note",
			content: "\n\nNew.\n\n",
			want:    "New.\n\n---\n\n## Notes\n\n- a note",
		},
		{
			// `tq show` hands out the whole body, notes included, so the
			// natural revision — read it, edit it, hand it back — passes the
			// record straight back in. Appending the file's notes to a text
			// that already carries them is how one round trip becomes two
			// copies of every note, and the next three.
			name:    "a replacement carrying the notes back does not double them",
			body:    "Old.\n\n---\n\n## Notes\n\n- a note",
			content: "New.\n\n---\n\n## Notes\n\n- a note",
			want:    "New.\n\n---\n\n## Notes\n\n- a note",
		},
		{
			// Notes are not settable this way, whatever the task holds now:
			// `tq note` appends them and the file is what keeps them.
			name:    "a notes section in the replacement is not adopted",
			body:    "Old.",
			content: "New.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — invented",
			want:    "New.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceContent(tt.body, tt.content)
			if got != tt.want {
				t.Errorf("ReplaceContent:\ngot:  %q\nwant: %q", got, tt.want)
			}
			// Whatever came out has to read back as the same two halves, or the
			// next note would land in the wrong one.
			_, notes := notesSection(got)
			if _, want := notesSection(tt.body); notes != want {
				t.Errorf("notes read back as %q, want %q", notes, want)
			}
		})
	}
}

// The loop the guide describes — `tq show --json`, edit, hand the body back —
// has to converge. Handing back what was read is the common case, and a rule
// that appended the file's notes to it grew the record by one copy per pass.
func TestReplaceContentIsIdempotentOverAWholeBody(t *testing.T) {
	body := "## Finding\n\nThe old finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — a note"

	for pass := range 3 {
		body = ReplaceContent(body, body)
		if got := strings.Count(body, "## Notes"); got != 1 {
			t.Fatalf("pass %d: %d notes sections, want 1:\n%s", pass+1, got, body)
		}
		if got := strings.Count(body, "— a note"); got != 1 {
			t.Fatalf("pass %d: the note appears %d times:\n%s", pass+1, got, body)
		}
		if !strings.HasPrefix(body, "## Finding\n\nThe old finding.") {
			t.Fatalf("pass %d: the content did not survive:\n%s", pass+1, body)
		}
	}
}

// The record has to survive the round trip, not just the one call: a rewritten
// body whose content ends in prose about notes is where a naive split would
// start appending into the document.
func TestReplaceContentKeepsAppendNoteOnTheRecord(t *testing.T) {
	body := "Old finding.\n\n---\n\n## Notes\n\n- 2026-01-01T00:00:00Z — first"
	body = ReplaceContent(body, "## Notes\n\nProse about notes.\n\n## Format\n\nA bullet.")
	body = AppendNote(body, "second", time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC))

	content, notes := notesSection(body)
	if !strings.Contains(content, "Prose about notes.") {
		t.Errorf("the prose left the content:\n%s", content)
	}
	if !strings.Contains(notes, "— first") || !strings.Contains(notes, "— second") {
		t.Errorf("both notes should be in the record, got:\n%s", notes)
	}
	if strings.Contains(notes, "Prose about notes.") {
		t.Errorf("the prose was read as a note:\n%s", notes)
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

	t.Run("Content rewrites the document and Body replaces the whole thing", func(t *testing.T) {
		withNotes := base
		withNotes.Body = "Old.\n\n---\n\n## Notes\n\n- a note"

		content := "New."
		if got := ApplyPatch(withNotes, TaskPatch{Content: &content}); got.Body != "New.\n\n---\n\n## Notes\n\n- a note" {
			t.Errorf("Content patch:\ngot: %q", got.Body)
		}
		// What the board's dialog sends: the notes are on screen and come back
		// with the rest, so this one is a straight replacement.
		body := "Everything."
		if got := ApplyPatch(withNotes, TaskPatch{Body: &body}); got.Body != body {
			t.Errorf("Body patch: got %q, want %q", got.Body, body)
		}
	})

	t.Run("an empty patch is detectable", func(t *testing.T) {
		if !(TaskPatch{}).IsEmpty() {
			t.Error("empty patch should report IsEmpty")
		}
		empty := ""
		if (TaskPatch{Content: &empty}).IsEmpty() {
			t.Error("clearing a body is a change, so it should not report IsEmpty")
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

func TestNormalizeID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"28", "TQ-0028"},
		{"0028", "TQ-0028"},
		{"00000028", "TQ-0028"},
		{"1", "TQ-0001"},
		{"12345", "TQ-12345"},
		{"0", "TQ-0000"},
		{"tq-28", "TQ-0028"},
		{"Tq-0028", "TQ-0028"},
		{"TQ-0028", "TQ-0028"},
		// Already an ID, so it is taken literally: a task hand-filed under
		// TQ-28 is what TQ-28 means, and padding it would look for a file that
		// is not there.
		{"TQ-28", "TQ-28"},
		// Not a spelling of a number, so it comes back for the caller to refuse.
		{"", ""},
		{"TQ-", "TQ-"},
		{"28a", "28a"},
		{"-28", "-28"},
		{"tq 28", "tq 28"},
		{"TQ-0028-fix", "TQ-0028-fix"},
	}
	for _, tc := range cases {
		if got := NormalizeID(tc.in); got != tc.want {
			t.Errorf("NormalizeID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A number far past what an int holds is still a number: it is padded like any
// other rather than parsed, so it names a task that does not exist instead of
// wrapping into one that does.
func TestNormalizeIDDoesNotParseTheNumber(t *testing.T) {
	huge := "99999999999999999999999999"
	if got := NormalizeID(huge); got != "TQ-"+huge {
		t.Errorf("NormalizeID(%q) = %q", huge, got)
	}
}

func TestFormatID(t *testing.T) {
	for n, want := range map[int]string{0: "TQ-0000", 1: "TQ-0001", 42: "TQ-0042", 12345: "TQ-12345"} {
		if got := FormatID(n); got != want {
			t.Errorf("FormatID(%d) = %q, want %q", n, got, want)
		}
	}
}

// Every ID tq hands out is one the shorthand reaches: the two spellings are one
// rule, and a drift between them would leave numbers only typable in full.
func TestNormalizeIDReachesEveryFormattedID(t *testing.T) {
	for _, n := range []int{0, 1, 9, 10, 999, 1000, 9999, 10000, 123456} {
		id := FormatID(n)
		if !ValidID(id) {
			t.Fatalf("FormatID(%d) = %q, which is not a valid ID", n, id)
		}
		if got := NormalizeID(strconv.Itoa(n)); got != id {
			t.Errorf("NormalizeID(%d) = %q, want %q", n, got, id)
		}
	}
}
