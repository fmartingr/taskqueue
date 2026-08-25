package main

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
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

// The workflow is intentionally fixed during the PoC: four statuses, four
// priorities. Both slices are also the display order (board columns, sorting).
const (
	StatusBacklog    = "backlog"
	StatusTodo       = "todo"
	StatusInProgress = "in-progress"
	StatusDone       = "done"
)

const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

var (
	Statuses   = []string{StatusBacklog, StatusTodo, StatusInProgress, StatusDone}
	Priorities = []string{PriorityUrgent, PriorityHigh, PriorityNormal, PriorityLow}
)

// idPattern is the only accepted shape for task IDs (and therefore filenames).
var idPattern = regexp.MustCompile(`^TQ-[0-9]+$`)

func ValidID(id string) bool      { return idPattern.MatchString(id) }
func ValidStatus(s string) bool   { return slices.Contains(Statuses, s) }
func ValidPriority(p string) bool { return slices.Contains(Priorities, p) }
func statusRank(s string) int     { return rankOf(Statuses, s) }
func priorityRank(p string) int   { return rankOf(Priorities, p) }
func rankOf(all []string, v string) int {
	if i := slices.Index(all, v); i >= 0 {
		return i
	}
	return len(all)
}

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
	case !ValidStatus(t.Status):
		return fmt.Errorf("invalid status %q (want one of %s)", t.Status, strings.Join(Statuses, ", "))
	case t.Priority != "" && !ValidPriority(t.Priority):
		return fmt.Errorf("invalid priority %q (want one of %s)", t.Priority, strings.Join(Priorities, ", "))
	}
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
func IsBlocked(t Task, index map[string]Task) bool {
	for _, dep := range t.DependsOn {
		other, ok := index[dep]
		if !ok || other.Status != StatusDone {
			return true
		}
	}
	return false
}

// IsReady reports whether an agent could pick this task up right now.
func IsReady(t Task, index map[string]Task) bool {
	if t.Status == StatusDone || t.Status == StatusInProgress {
		return false
	}
	return !IsBlocked(t, index)
}

// SortTasks orders tasks by status, then priority, then creation time, then ID.
func SortTasks(tasks []Task) {
	sort.SliceStable(tasks, func(i, j int) bool {
		a, b := tasks[i], tasks[j]
		if a.Status != b.Status {
			return statusRank(a.Status) < statusRank(b.Status)
		}
		if a.Priority != b.Priority {
			return priorityRank(a.Priority) < priorityRank(b.Priority)
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

func (f Filter) Validate() error {
	if f.Status != "" && !ValidStatus(f.Status) {
		return fmt.Errorf("invalid status %q (want one of %s)", f.Status, strings.Join(Statuses, ", "))
	}
	if f.Priority != "" && !ValidPriority(f.Priority) {
		return fmt.Errorf("invalid priority %q (want one of %s)", f.Priority, strings.Join(Priorities, ", "))
	}
	return nil
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
func FilterTasks(tasks []Task, f Filter) []Task {
	index := IndexTasks(tasks)
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if f.Ready && !IsReady(t, index) {
			continue
		}
		if !f.matchFields(t) {
			continue
		}
		out = append(out, t)
	}
	return out
}

const notesHeading = "## Notes"

// AppendNote appends a timestamped bullet to the task body's "## Notes"
// section, creating the section at the end of the body when it is missing.
func AppendNote(body, text string, ts time.Time) string {
	note := "- " + ts.Format(time.RFC3339) + " — " + strings.Join(strings.Fields(text), " ")
	body = strings.Trim(body, "\n")

	lines := strings.Split(body, "\n")
	heading := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == notesHeading {
			heading = i
			break
		}
	}
	if heading == -1 {
		if body == "" {
			return notesHeading + "\n\n" + note
		}
		return body + "\n\n" + notesHeading + "\n\n" + note
	}

	// The notes section ends at the next heading (or at the end of the body).
	end := len(lines)
	for i := heading + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ") {
			end = i
			break
		}
	}
	insert := end
	for insert > heading+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}

	out := append([]string{}, lines[:insert]...)
	if insert == heading+1 { // empty section: keep a blank line under the heading
		out = append(out, "")
	}
	out = append(out, note)
	if end < len(lines) {
		out = append(out, "")
	}
	out = append(out, lines[end:]...)
	return strings.Join(out, "\n")
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

	// Incremental list edits, used by the CLI (--add-label, --remove-dependency).
	AddLabels    []string `json:"-"`
	RemoveLabels []string `json:"-"`
	AddDeps      []string `json:"-"`
	RemoveDeps   []string `json:"-"`
}

func (p TaskPatch) IsEmpty() bool {
	return p.Title == nil && p.Status == nil && p.Priority == nil && p.Assignee == nil &&
		p.Body == nil && p.Labels == nil && p.DependsOn == nil &&
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
