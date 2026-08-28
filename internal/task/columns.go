package task

import (
	"fmt"
	"slices"
	"strings"
)

// The built-in board. A project can declare its own, which is what Columns
// carries; these are what it starts from and what a project without a config
// keeps.
const (
	StatusInbox      = "inbox"
	StatusTodo       = "todo"
	StatusInProgress = "in-progress"
	StatusDone       = "done"
	StatusRejected   = "rejected"

	// StatusBacklog is an alias for StatusInbox.
	StatusBacklog = "backlog"
)

// Column is one column of the board: where it sits, whether work in it can be
// picked up, and whether a dependency parked in it counts as met.
type Column struct {
	Name string

	// Ready is whether `tq ready` offers work from here. False for the columns
	// holding work that is claimed or finished.
	Ready bool

	// Satisfies is whether a dependency sitting here counts as complete. This
	// is what `tq done` aims at.
	Satisfies bool
}

var builtinColumns = []Column{
	// Inbox is intake, not a queue: a task lands here before anyone has decided
	// it is worth doing, so `tq ready` does not hand it out. Triage moves it to
	// To do, and that is what makes it work an agent can pick up.
	{Name: StatusInbox},
	{Name: StatusTodo, Ready: true},
	{Name: StatusInProgress},
	{Name: StatusDone, Satisfies: true},
	// Deliberately does not satisfy dependencies: a task waiting on work
	// somebody decided not to do is still blocked, and saying otherwise would
	// quietly treat "we will not do this" as "this is finished".
	{Name: StatusRejected},
}

var builtinAliases = map[string]string{StatusBacklog: StatusInbox}

// Columns is a board's column vocabulary: the statuses in board order, which of
// them offer work, which count a dependency as met, and the one a task with no
// status of its own is filed in.
//
// The zero value is the built-in board.
type Columns struct {
	columns  []Column
	fallback string
	aliases  map[string]string
}

// NewColumns builds a board from an ordered list. An empty list is the built-in
// board: a project cannot have a board with no columns, and refusing here would
// leave the config loader as the only place that could report it.
func NewColumns(columns []Column, fallback string, aliases map[string]string) Columns {
	if len(columns) == 0 {
		return Columns{}
	}
	return Columns{columns: slices.Clone(columns), fallback: fallback, aliases: aliases}
}

// effective is the column list without the copy, for the lookups that only read
// it. Rank runs inside a sort comparator, so a clone per call would be an
// allocation per comparison.
func (c Columns) effective() []Column {
	if len(c.columns) == 0 {
		return builtinColumns
	}
	return c.columns
}

func (c Columns) alias(status string) (string, bool) {
	if len(c.columns) == 0 {
		to, ok := builtinAliases[status]
		return to, ok
	}
	to, ok := c.aliases[status]
	return to, ok
}

// Names is the board in column order.
func (c Columns) Names() []string {
	columns := c.effective()
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		names = append(names, column.Name)
	}
	return names
}

// First is the leftmost column. It is the board's order, not its fallback:
// where a task the board has no column for goes is Default's answer, and the
// two are only the same board when no column claims `default: true`.
func (c Columns) First() string { return c.effective()[0].Name }

// Default is where a task filed without a status goes.
func (c Columns) Default() string {
	if len(c.columns) == 0 {
		return StatusInbox
	}
	if c.fallback != "" {
		return c.fallback
	}
	return c.First()
}

func (c Columns) find(status string) (Column, bool) {
	if to, ok := c.alias(status); ok {
		status = to
	}
	for _, column := range c.effective() {
		if column.Name == status {
			return column, true
		}
	}
	return Column{}, false
}

// Valid reports whether this board has somewhere to put a task with this
// status, an alias counting as the column it names.
func (c Columns) Valid(status string) bool {
	_, ok := c.find(status)
	return ok
}

// Normalize is a status spelled the way the board spells it: an alias resolved
// to the column it names, and anything else returned exactly as it came.
//
// It is for a value a caller has already had accepted — Check runs first
// everywhere it is used — so a status the board has no column for is not its
// business and it does not invent one. Moving such a task is Reconcile's job,
// which is a write, announced, and over the whole queue at once (TQ-0088).
func (c Columns) Normalize(status string) string {
	if column, ok := c.find(status); ok {
		return column.Name
	}
	return status
}

// Reconcile is where this board files a task carrying the given status, and
// whether that differs from the status itself. It is the rule a reconciliation
// applies; the store is what writes the answer to disk.
//
// A status the board has no column for goes to Default and not to First. The
// two differ exactly when a column claims `default: true`, and on a board that
// lists its done column first that difference is the whole point: First would
// mark every stranded task finished and unblock whatever waited on it.
//
// A board that names no default has no such difference to draw on — Default is
// First there, by the rule that puts a task filed with no status in the first
// column. Such a project files new work in its leading column already, and
// stranded work following it is that decision, not this one.
//
// Empty is left alone. It is no answer at all rather than a column the board
// has lost, and turning it into one would let a status explicitly cleared
// through `tq update --status ""` quietly move the task instead of being
// refused by Validate.
func (c Columns) Reconcile(status string) (string, bool) {
	if status == "" {
		return status, false
	}
	if column, ok := c.find(status); ok {
		return column.Name, column.Name != status
	}
	return c.Default(), true
}

// Rank is the column's position, for sorting. A status the board has no column
// for ranks last, as a dropped priority does — a listing showing one is a
// listing taken before the reconciliation that moves it, and it belongs at the
// end rather than displacing the first column's work.
func (c Columns) Rank(status string) int {
	columns := c.effective()
	name := c.Normalize(status)
	for i, column := range columns {
		if column.Name == name {
			return i
		}
	}
	return len(columns)
}

// Offers reports whether `tq ready` hands out work from this status's column.
func (c Columns) Offers(status string) bool {
	column, ok := c.find(status)
	return ok && column.Ready
}

// Satisfies reports whether a dependency sitting in this status's column counts
// as met.
func (c Columns) Satisfies(status string) bool {
	column, ok := c.find(status)
	return ok && column.Satisfies
}

// Check reports whether a status may be written, naming the board when it may
// not. Empty passes here because Create fills it in with Default; every other
// path reaches Task.Validate, which requires one.
//
// As with priorities this is not part of Validate. Reading stays tolerant of
// any value — a file the project's board has outgrown still parses and still
// lists — and only a write that picks a column has to agree with the board as
// it stands now. Reconcile is what moves such a task, and it moves them all.
func (c Columns) Check(status string) error {
	if status == "" || c.Valid(status) {
		return nil
	}
	return fmt.Errorf("invalid status %q (want one of %s)", status, strings.Join(c.Names(), ", "))
}

// SatisfyingColumn is the column `tq done` moves a task to: the one whose work
// counts as complete. Exactly one column may claim that, and the error says
// which way the project got it wrong rather than leaving `tq done` to guess.
func (c Columns) SatisfyingColumn() (string, error) {
	var claimed []string
	for _, column := range c.effective() {
		if column.Satisfies {
			claimed = append(claimed, column.Name)
		}
	}
	switch len(claimed) {
	case 1:
		return claimed[0], nil
	case 0:
		return "", fmt.Errorf("no column is marked `consider_done: true`, so there is nothing for done to mean (use `tq move` to pick a column)")
	default:
		return "", fmt.Errorf("%d columns are marked `consider_done: true` (%s); exactly one may be, or done cannot know which you mean",
			len(claimed), strings.Join(claimed, ", "))
	}
}
