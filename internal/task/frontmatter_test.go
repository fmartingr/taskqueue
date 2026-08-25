package task

import (
	"strings"
	"testing"
	"time"
)

const validTask = `---
id: TQ-0001
title: Implement OIDC authentication
status: in-progress
priority: high
assignee: agent-auth
labels:
  - backend
  - auth
depends_on:
  - TQ-0004
created: 2026-08-25T08:30:00+02:00
updated: 2026-08-25T09:12:00+02:00
---

Implement authentication using the existing OIDC provider.

## Acceptance criteria

- Login redirects to the identity provider.

## Notes

Initial investigation completed.
`

func TestParseTaskValid(t *testing.T) {
	task, err := ParseTask("TQ-0001.md", []byte(validTask))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if task.ID != "TQ-0001" {
		t.Errorf("ID = %q, want TQ-0001", task.ID)
	}
	if task.Title != "Implement OIDC authentication" {
		t.Errorf("Title = %q", task.Title)
	}
	if task.Status != StatusInProgress {
		t.Errorf("Status = %q", task.Status)
	}
	if task.Priority != PriorityHigh {
		t.Errorf("Priority = %q", task.Priority)
	}
	if task.Assignee != "agent-auth" {
		t.Errorf("Assignee = %q", task.Assignee)
	}
	if got, want := strings.Join(task.Labels, ","), "backend,auth"; got != want {
		t.Errorf("Labels = %q, want %q", got, want)
	}
	if got, want := strings.Join(task.DependsOn, ","), "TQ-0004"; got != want {
		t.Errorf("DependsOn = %q, want %q", got, want)
	}
	want := time.Date(2026, 8, 25, 8, 30, 0, 0, time.FixedZone("", 2*3600))
	if !task.Created.Equal(want) {
		t.Errorf("Created = %v, want %v", task.Created, want)
	}
}

func TestParseTaskKeepsMultilineBody(t *testing.T) {
	task, err := ParseTask("TQ-0001.md", []byte(validTask))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if !strings.HasPrefix(task.Body, "Implement authentication using") {
		t.Errorf("body should start with the first paragraph, got %q", task.Body)
	}
	if !strings.Contains(task.Body, "## Acceptance criteria") {
		t.Error("body lost the acceptance criteria heading")
	}
	if !strings.HasSuffix(task.Body, "Initial investigation completed.") {
		t.Errorf("body should end with the last line, got %q", task.Body)
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	original, err := ParseTask("TQ-0001.md", []byte(validTask))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	rendered, err := RenderTask(original)
	if err != nil {
		t.Fatalf("RenderTask: %v", err)
	}
	again, err := ParseTask("TQ-0001.md", rendered)
	if err != nil {
		t.Fatalf("ParseTask(rendered): %v", err)
	}
	if again.Body != original.Body {
		t.Errorf("body changed:\n got %q\nwant %q", again.Body, original.Body)
	}
	if again.ID != original.ID || again.Title != original.Title || again.Status != original.Status {
		t.Errorf("frontmatter changed: %+v vs %+v", again, original)
	}
	if !again.Created.Equal(original.Created) || !again.Updated.Equal(original.Updated) {
		t.Errorf("timestamps changed: %v/%v vs %v/%v", again.Created, again.Updated, original.Created, original.Updated)
	}

	// Rendering is stable: a second render of the parsed task is byte-identical.
	rendered2, err := RenderTask(again)
	if err != nil {
		t.Fatalf("RenderTask(again): %v", err)
	}
	if string(rendered2) != string(rendered) {
		t.Errorf("render is not stable:\n%s\n---\n%s", rendered, rendered2)
	}
}

// A body may contain both a horizontal rule and a "## Notes" heading of its
// own: the frontmatter scan stops at the real closing delimiter, so neither
// confuses the parser and the body survives a round trip byte for byte.
func TestRoundTripKeepsRulesAndNotesInTheBody(t *testing.T) {
	body := "Description.\n\n---\n\n## Notes\n\nProse that is content, not notes.\n\n" +
		"## Acceptance criteria\n\n- something\n\n---\n\n## Notes\n\n- 2026-08-25T09:42:00+02:00 — a real note"

	task := Task{
		ID:      "TQ-0001",
		Title:   "Round trip",
		Status:  StatusTodo,
		Created: time.Date(2026, 8, 25, 8, 30, 0, 0, time.UTC),
		Updated: time.Date(2026, 8, 25, 9, 12, 0, 0, time.UTC),
		Body:    body,
	}

	rendered, err := RenderTask(task)
	if err != nil {
		t.Fatalf("RenderTask: %v", err)
	}
	parsed, err := ParseTask("TQ-0001.md", rendered)
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if parsed.Body != body {
		t.Errorf("body changed:\ngot:  %q\nwant: %q", parsed.Body, body)
	}

	again, err := RenderTask(parsed)
	if err != nil {
		t.Fatalf("RenderTask(parsed): %v", err)
	}
	if string(again) != string(rendered) {
		t.Errorf("render is not stable:\n%s\n---\n%s", rendered, again)
	}
}

func TestRenderUsesTwoSpaceListIndent(t *testing.T) {
	task, err := ParseTask("TQ-0001.md", []byte(validTask))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	rendered, err := RenderTask(task)
	if err != nil {
		t.Fatalf("RenderTask: %v", err)
	}
	if !strings.Contains(string(rendered), "labels:\n  - backend\n  - auth\n") {
		t.Errorf("unexpected label formatting:\n%s", rendered)
	}
}

func TestRenderOmitsEmptyOptionalFields(t *testing.T) {
	task := Task{
		ID:      "TQ-0002",
		Title:   "Minimal",
		Status:  StatusTodo,
		Created: time.Now().Truncate(time.Second),
		Updated: time.Now().Truncate(time.Second),
	}
	rendered, err := RenderTask(task)
	if err != nil {
		t.Fatalf("RenderTask: %v", err)
	}
	for _, field := range []string{"assignee:", "labels:", "depends_on:", "priority:"} {
		if strings.Contains(string(rendered), field) {
			t.Errorf("rendered file should omit %q:\n%s", field, rendered)
		}
	}
}

func TestParseTaskErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"no frontmatter", "just a body\n", "missing YAML frontmatter"},
		{"empty frontmatter", "---\n---\n\nbody\n", "id is required"},
		{"missing closing delimiter", "---\nid: TQ-0001\ntitle: x\n", "unterminated YAML frontmatter"},
		{"invalid yaml", "---\nid: [TQ-0001\n---\n", "invalid YAML"},
		{"missing title", "---\nid: TQ-0001\nstatus: todo\n---\n", "title is required"},
		{"missing id", "---\ntitle: x\nstatus: todo\n---\n", "id is required"},
		{"bad id", "---\nid: nope\ntitle: x\nstatus: todo\n---\n", "must match TQ-<number>"},
		{"invalid status", "---\nid: TQ-0001\ntitle: x\nstatus: shipped\n---\n", "invalid status"},
		{"invalid priority", "---\nid: TQ-0001\ntitle: x\nstatus: todo\npriority: whenever\n---\n", "invalid priority"},
		{"self dependency", "---\nid: TQ-0001\ntitle: x\nstatus: todo\ndepends_on:\n  - TQ-0001\n---\n", "cannot depend on itself"},
		{"invalid dependency", "---\nid: TQ-0001\ntitle: x\nstatus: todo\ndepends_on:\n  - nope\n---\n", "invalid dependency"},
		{"bad timestamp", "---\nid: TQ-0001\ntitle: x\nstatus: todo\ncreated: yesterday\n---\n", "invalid YAML"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseTask("TQ-0001.md", []byte(tc.content))
			if err == nil {
				t.Fatalf("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), "TQ-0001.md") {
				t.Errorf("error %q should name the file", err)
			}
		})
	}
}

func TestParseTaskAcceptsCRLF(t *testing.T) {
	content := strings.ReplaceAll("---\nid: TQ-0001\ntitle: x\nstatus: todo\n---\n\nbody\n", "\n", "\r\n")
	task, err := ParseTask("TQ-0001.md", []byte(content))
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if task.Body != "body" {
		t.Errorf("Body = %q, want %q", task.Body, "body")
	}
}

// A YAML block scalar may contain a line that looks like the frontmatter
// delimiter. Only an unindented "---" closes the block, so an indented one is
// data — and tq's own renderer produces exactly that for a multi-line value.
func TestParseTaskKeepsAnIndentedRuleInsideFrontmatter(t *testing.T) {
	data := []byte("---\n" +
		"id: TQ-0001\n" +
		"title: |-\n  line1\n  ---\n  line2\n" +
		"status: todo\n" +
		"priority: normal\n" +
		"---\n\nBody.\n")

	task, err := ParseTask("TQ-0001-x.md", data)
	if err != nil {
		t.Fatalf("ParseTask: %v", err)
	}
	if want := "line1\n---\nline2"; task.Title != want {
		t.Errorf("Title = %q, want %q", task.Title, want)
	}
	if task.Status != StatusTodo {
		t.Errorf("Status = %q, want the frontmatter to have been read past the indented rule", task.Status)
	}
	if task.Body != "Body." {
		t.Errorf("Body = %q, want %q", task.Body, "Body.")
	}
}
