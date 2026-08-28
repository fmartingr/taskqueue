package task

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Task is the single domain object of the task queue. It maps one-to-one to a
// Markdown file with YAML frontmatter inside the task directory.
type Task struct {
	ID        string    `yaml:"id" json:"id"`
	Title     string    `yaml:"title" json:"title"`
	Status    string    `yaml:"status" json:"status"`
	Priority  string    `yaml:"priority,omitempty" json:"priority,omitempty"`
	Assignee  string    `yaml:"assignee,omitempty" json:"assignee,omitempty"`
	Labels    []string  `yaml:"labels,omitempty" json:"labels,omitempty"`
	DependsOn []string  `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Created   time.Time `yaml:"created" json:"created"`
	Updated   time.Time `yaml:"updated" json:"updated"`
	Body      string    `yaml:"-" json:"body"`
}

// The built-in priorities. A project can declare its own vocabulary, which is
// what Priorities carries; these are what it starts from and what a project
// without a config keeps.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

var builtinPriorities = []string{PriorityUrgent, PriorityHigh, PriorityNormal, PriorityLow}

// idPattern is the only accepted shape for task IDs (and therefore filenames).
var idPattern = regexp.MustCompile(`^TQ-[0-9]+$`)

func ValidID(id string) bool { return idPattern.MatchString(id) }

// Validate reports the first problem that would make a task file unusable.
func (t Task) Validate() error {
	switch {
	case t.ID == "":
		return fmt.Errorf("id is required")
	case !ValidID(t.ID):
		return fmt.Errorf("id %q must match TQ-<number>", t.ID)
	case strings.TrimSpace(t.Title) == "":
		return fmt.Errorf("title is required")
	case t.Status == "":
		return fmt.Errorf("status is required")
	}
	// Neither the status nor the priority is checked against a vocabulary here:
	// both are the project's, this package does not know them, and a task filed
	// under one that has since been dropped must still load. Columns.Check and
	// Priorities.Check guard the writes; Columns.Reconcile is what says which
	// column a task whose own is gone belongs in.
	for _, dep := range t.DependsOn {
		if dep == t.ID {
			return fmt.Errorf("task %s cannot depend on itself", t.ID)
		}
		if !ValidID(dep) {
			return fmt.Errorf("invalid dependency %q (must match TQ-<number>)", dep)
		}
	}
	return nil
}

// ValidateForWrite reports the first problem that would make a task file
// unusable, or that tq must not commit to disk. It is stricter than Validate
// on purpose: reading stays forgiving, so a file that already carries a line
// break still loads and `tq list` keeps working while it is corrected.
func (t Task) ValidateForWrite() error {
	if err := t.Validate(); err != nil {
		return err
	}
	switch {
	case containsLineBreak(t.Title):
		return fmt.Errorf("title must be a single line")
	case containsLineBreak(t.Assignee):
		return fmt.Errorf("assignee must be a single line")
	}
	for _, label := range t.Labels {
		if containsLineBreak(label) {
			return fmt.Errorf("label %q must be a single line", label)
		}
	}
	return nil
}

// containsLineBreak reports whether a value would render as a multi-line YAML
// block scalar. Every line of one is indented, so any line of it can look
// like the frontmatter delimiter — and the filename is a slug of the title,
// which a line break has no place in either.
func containsLineBreak(s string) bool {
	return strings.ContainsAny(s, "\n\r")
}

// IndexTasks keys tasks by ID so dependency lookups stay cheap.
func IndexTasks(tasks []Task) map[string]Task {
	index := make(map[string]Task, len(tasks))
	for _, t := range tasks {
		index[t.ID] = t
	}
	return index
}

// IsBlocked reports whether any dependency is missing or unfinished. A missing
// dependency blocks rather than being ignored, so typos surface as blocked work.
func IsBlocked(t Task, index map[string]Task, columns Columns) bool {
	for _, dep := range t.DependsOn {
		other, ok := index[dep]
		if !ok || !columns.Satisfies(other.Status) {
			return true
		}
	}
	return false
}

// IsReady reports whether an agent could pick this task up right now.
func IsReady(t Task, index map[string]Task, columns Columns) bool {
	if !columns.Offers(t.Status) {
		return false
	}
	return !IsBlocked(t, index, columns)
}

// SortTasks orders tasks by status, then priority, then creation time, then ID.
// The priority ranking is the project's, since the vocabulary is ordered and
// the config file is the ranking; the zero value is the built-in order.
func SortTasks(tasks []Task, priorities Priorities, columns Columns) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.Status != b.Status {
			return columns.Rank(a.Status) < columns.Rank(b.Status)
		}
		if a.Priority != b.Priority {
			return priorities.Rank(a.Priority) < priorities.Rank(b.Priority)
		}
		if !a.Created.Equal(b.Created) {
			return a.Created.Before(b.Created)
		}
		return a.ID < b.ID
	})
}

// Filter is the shared filter used by `tq list`, `tq ready` and GET /api/tasks.
type Filter struct {
	Status   string
	Priority string
	Assignee string
	Label    string
	Ready    bool
}

// Validate rejects a filter that could never match, naming what would.
// Filtering on a priority outside the vocabulary is such a filter: the task it
// would find is one the project can no longer file, and silently returning
// nothing reads as an empty queue.
func (f Filter) Validate(priorities Priorities, columns Columns) error {
	if err := columns.Check(f.Status); err != nil {
		return err
	}
	return priorities.Check(f.Priority)
}

func (f Filter) matchFields(t Task) bool {
	if f.Status != "" && t.Status != f.Status {
		return false
	}
	if f.Priority != "" && t.Priority != f.Priority {
		return false
	}
	if f.Assignee != "" && t.Assignee != f.Assignee {
		return false
	}
	if f.Label != "" && !slices.Contains(t.Labels, f.Label) {
		return false
	}
	return true
}

// FilterTasks keeps the input order and needs the full task set because the
// "ready" filter depends on the state of other tasks.
func FilterTasks(tasks []Task, f Filter, columns Columns) []Task {
	// Spell the filter's column the way the board spells it, so `--status
	// backlog` finds the tasks filed in `inbox`. Every caller runs
	// Filter.Validate first, so a status with no column at all was refused
	// before it reached here.
	f.Status = columns.Normalize(f.Status)
	index := IndexTasks(tasks)
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if f.Ready && !IsReady(t, index, columns) {
			continue
		}
		if !f.matchFields(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Notes are the last section of a task body, introduced by a horizontal rule:
//
//	Task content, which may itself contain a Notes section.
//
//	---
//
//	## Notes
//
//	- 2026-08-25T09:42:00+02:00 — the actual note
//
// The blank line above the rule is required — text directly above "---" is a
// setext heading rather than a rule — and everything after the heading is
// notes, so a "## Notes" heading anywhere else is ordinary content.
const (
	notesHeading = "## Notes"
	notesRule    = "---"
)

// notesSection splits a body into the content ahead of the notes section and
// the notes themselves. Both are empty when the body has neither.
//
// The rule is optional when reading: files written before it existed end in a
// bare "## Notes", and AppendNote writes the rule in the next time it touches
// one. A "## Notes" that is followed by another section is content, and so is
// one inside a fenced code block.
func notesSection(body string) (content, notes string) {
	body = strings.Trim(body, "\n")
	lines := strings.Split(body, "\n")

	start := notesStart(lines)
	if start == -1 {
		return body, ""
	}

	end := start
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	// A "---" that does not follow a blank line underlines the text above it as
	// a setext heading, so it is part of the content and not the notes rule.
	if end > 0 && strings.TrimSpace(lines[end-1]) == notesRule && (end == 1 || strings.TrimSpace(lines[end-2]) == "") {
		end--
	}

	content = strings.Trim(strings.Join(lines[:end], "\n"), "\n")
	notes = strings.Trim(strings.Join(lines[start+1:], "\n"), "\n")
	return content, notes
}

// notesStart returns the index of the "## Notes" heading that opens the notes
// section, or -1 when there is none. Only the heading of the body's last
// section qualifies; any heading after it makes it content.
func notesStart(lines []string) int {
	if start, balanced := scanNotesStart(lines, true); balanced {
		return start
	}
	// The body has an unclosed (or mismatched) fence, which would hide
	// everything after it — including a real notes section, so that appending a
	// note would start a second one. Read the fences as ordinary lines instead.
	start, _ := scanNotesStart(lines, false)
	return start
}

// scanNotesStart is notesStart over one pass, also reporting whether the code
// fences it honoured were balanced.
func scanNotesStart(lines []string, honourFences bool) (start int, balanced bool) {
	start, fenced, inItem := -1, false, false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if honourFences && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			fenced = !fenced
			continue
		}
		// An indented heading is ambiguous: a note's continuation lines are
		// indented under their bullet and may carry one, but CommonMark also
		// allows a real heading up to three spaces in. What separates them is
		// the list item — inside one the heading is the note's own text, and
		// outside one it is a heading like any other.
		// A blank line does not end a list item — a multi-line note has one
		// between its paragraphs — so only a line with content resets this.
		if !indented(line) && trimmed != "" {
			inItem = listMarker(trimmed)
		}
		if fenced || (indented(line) && inItem) || !isATXHeading(trimmed) {
			continue
		}
		if trimmed == notesHeading {
			start = i
		} else {
			start = -1
		}
	}
	return start, !fenced
}

// isATXHeading reports whether a trimmed line is a "#"-style Markdown heading.
func isATXHeading(trimmed string) bool {
	hashes := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
	return hashes > 0 && hashes <= 6 && strings.HasPrefix(trimmed[hashes:], " ")
}

// listMarker reports whether a trimmed line opens a Markdown list item. Notes
// are written as "- " bullets, but a body may use any of the markers, and an
// indented line below one is that item's content rather than a heading.
func listMarker(trimmed string) bool {
	if len(trimmed) > 1 && strings.ContainsRune("-*+", rune(trimmed[0])) && trimmed[1] == ' ' {
		return true
	}
	digits := len(trimmed) - len(strings.TrimLeft(trimmed, "0123456789"))
	return digits > 0 && digits < 10 && len(trimmed) > digits+1 &&
		(trimmed[digits] == '.' || trimmed[digits] == ')') && trimmed[digits+1] == ' '
}

// indented reports whether a line starts with whitespace.
func indented(line string) bool {
	return strings.TrimLeft(line, " \t") != line
}

// noteIndent is what a note's second and further lines owe the bullet they
// belong to, so that Markdown keeps reading them as part of it.
const noteIndent = "  "

// AppendNote appends a timestamped bullet to the task body's notes section,
// creating it — rule included — at the very end of the body when it is missing.
// A note that is blank once normalised leaves the body alone.
func AppendNote(body, text string, ts time.Time) string {
	note := noteBullet(text, ts)
	if note == "" {
		return body
	}

	content, notes := notesSection(body)
	section := notesHeading
	if content != "" {
		section = content + "\n\n" + notesRule + "\n\n" + notesHeading
	}
	if notes == "" {
		return section + "\n\n" + note
	}
	return section + "\n\n" + notes + "\n" + note
}

// ReplaceContent returns body with everything ahead of the notes section
// replaced by content, and the notes themselves carried over untouched. It is
// what `tq update --body` writes.
//
// Replacing the whole body would be the simpler rule and the wrong one: the
// notes are the record of how a task got where it is, appended one at a time
// and reconstructible from nothing, while the content is a document its author
// has in front of them. So a body edit rewrites the document and leaves the
// record alone.
//
// The replacement is read as a body rather than as raw content, and only its
// content half is kept. `tq show` hands out the whole body, notes included, so
// the natural way to revise one — read it, edit it, hand it back — passes the
// record straight back in, and appending the file's notes to a text that
// already carries them is how one round trip becomes two copies of every note,
// and the next three. Notes are never set this way: `tq note` appends them, and
// the file is what holds them.
//
// The section is assembled exactly as AppendNote assembles it, rule included —
// and dropped, heading and all, when the task has no notes, so revising a body
// never grows one.
func ReplaceContent(body, content string) string {
	_, notes := notesSection(body)
	content, _ = notesSection(content)
	switch {
	case notes == "":
		return content
	case content == "":
		return notesHeading + "\n\n" + notes
	}
	return content + "\n\n" + notesRule + "\n\n" + notesHeading + "\n\n" + notes
}

// noteBullet renders one note as a single Markdown list item: the first line
// follows the timestamp and every further line is indented under it, so a note
// keeps the structure its author gave it — a wrapped sentence, a list, a
// pasted command — without breaking out of the bullet. It returns "" when the
// text holds nothing but whitespace.
func noteBullet(text string, ts time.Time) string {
	lines := noteLines(text)
	if len(lines) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("- " + ts.Format(time.RFC3339) + " — " + lines[0])
	for _, line := range lines[1:] {
		b.WriteString("\n")
		if line != "" {
			b.WriteString(noteIndent + line)
		}
	}
	return b.String()
}

// noteLines normalises a note's text into the lines of its bullet: line
// endings are unified, trailing whitespace goes, runs of blank lines collapse
// to one, and the blank lines around the note are dropped. The first line is
// also stripped of its indentation, since it sits after the timestamp.
func noteLines(text string) []string {
	var lines []string
	blank := false
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimRight(strings.TrimSuffix(line, "\r"), " \t")
		if line == "" {
			// A blank line only counts once, and never before the first line.
			blank = len(lines) > 0
			continue
		}
		if blank {
			lines = append(lines, "")
			blank = false
		}
		if len(lines) == 0 {
			line = strings.TrimLeft(line, " \t")
		}
		lines = append(lines, line)
	}
	return lines
}

// TaskPatch is a partial update. Nil pointers mean "leave unchanged", which is
// what both `tq update` and PATCH /api/tasks/{id} need.
type TaskPatch struct {
	Title     *string   `json:"title"`
	Status    *string   `json:"status"`
	Priority  *string   `json:"priority"`
	Assignee  *string   `json:"assignee"`
	Body      *string   `json:"body"`
	Labels    *[]string `json:"labels"`
	DependsOn *[]string `json:"depends_on"`

	// Partial body edits, used by the CLI. Content is `tq update --body`: it
	// replaces the body's content and keeps its notes (see ReplaceContent),
	// where Body above replaces the whole thing, which is what the board's
	// dialog sends. Both are here rather than one, because the two callers want
	// different things: the dialog has the notes on screen and submits them
	// back, and an agent revising a ticket has only the new content.
	Content *string `json:"-"`

	// Incremental list edits, used by the CLI (--add-label, --remove-dependency).
	AddLabels    []string `json:"-"`
	RemoveLabels []string `json:"-"`
	AddDeps      []string `json:"-"`
	RemoveDeps   []string `json:"-"`
}

func (p TaskPatch) IsEmpty() bool {
	return p.Title == nil && p.Status == nil && p.Priority == nil && p.Assignee == nil &&
		p.Body == nil && p.Content == nil && p.Labels == nil && p.DependsOn == nil &&
		len(p.AddLabels) == 0 && len(p.RemoveLabels) == 0 &&
		len(p.AddDeps) == 0 && len(p.RemoveDeps) == 0
}

// ApplyPatch returns a copy of t with the patch applied. Validation is left to
// the store, which is the only place that writes.
func ApplyPatch(t Task, p TaskPatch) Task {
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.Status != nil {
		t.Status = *p.Status
	}
	if p.Priority != nil {
		t.Priority = *p.Priority
	}
	if p.Assignee != nil {
		t.Assignee = *p.Assignee
	}
	if p.Body != nil {
		t.Body = *p.Body
	}
	// After Body, so that a patch carrying both replaces the body and then
	// rewrites the content of what it replaced it with, rather than the other
	// way round. Nothing sends both today; the order is the one that composes.
	if p.Content != nil {
		t.Body = ReplaceContent(t.Body, *p.Content)
	}
	if p.Labels != nil {
		t.Labels = slices.Clone(*p.Labels)
	}
	if p.DependsOn != nil {
		t.DependsOn = slices.Clone(*p.DependsOn)
	}
	t.Labels = addAll(removeAll(t.Labels, p.RemoveLabels), p.AddLabels)
	t.DependsOn = addAll(removeAll(t.DependsOn, p.RemoveDeps), p.AddDeps)
	return t
}

func addAll(values, add []string) []string {
	for _, v := range add {
		if v = strings.TrimSpace(v); v != "" && !slices.Contains(values, v) {
			values = append(values, v)
		}
	}
	return values
}

func removeAll(values, remove []string) []string {
	if len(remove) == 0 {
		return values
	}
	out := values[:0:0]
	for _, v := range values {
		if !slices.Contains(remove, v) {
			out = append(out, v)
		}
	}
	return out
}

// maxSlugRunes caps the title suffix of a task filename. Long enough to stay
// recognisable, short enough to keep paths and `ls` output readable.
const maxSlugRunes = 48

// Slugify turns a task title into the filename suffix: lowercase, with runs of
// anything that is not a letter or a digit collapsed into a single dash. It
// returns "" when a title has nothing usable in it, in which case the file is
// named after the ID alone.
func Slugify(title string) string {
	slug := make([]rune, 0, len(title))
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			slug = append(slug, r)
		case len(slug) > 0 && slug[len(slug)-1] != '-':
			slug = append(slug, '-')
		}
	}

	if len(slug) > maxSlugRunes {
		slug = slug[:maxSlugRunes]
		// Prefer cutting at a word boundary over cutting mid-word.
		if i := lastIndexRune(slug, '-'); i > 0 {
			slug = slug[:i]
		}
	}
	return strings.Trim(string(slug), "-")
}

func lastIndexRune(runes []rune, target rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == target {
			return i
		}
	}
	return -1
}
