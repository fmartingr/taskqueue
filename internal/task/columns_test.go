package task

import (
	"strings"
	"testing"
	"time"
)

func custom() Columns {
	return NewColumns([]Column{
		{Name: "spotted", Ready: true},
		{Name: "doing"},
		{Name: "shipped", Satisfies: true},
	}, "spotted", nil)
}

func TestColumnsZeroValueIsTheBuiltInBoard(t *testing.T) {
	var c Columns
	if got, want := strings.Join(c.Names(), ","), "inbox,todo,in-progress,done,rejected"; got != want {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	// Intake is where a task filed without a status lands: it has not been
	// triaged yet, so it is not work anyone has agreed to do.
	if c.Default() != StatusInbox {
		t.Errorf("Default() = %q, want %q", c.Default(), StatusInbox)
	}
	if c.First() != StatusInbox {
		t.Errorf("First() = %q, want %q", c.First(), StatusInbox)
	}
}

// Ready and Satisfies drive `tq ready` and dependency blocking.
func TestColumnsCarryTheSemanticsThatWereLiterals(t *testing.T) {
	var c Columns
	for status, want := range map[string]bool{
		// Inbox is intake and is deliberately not offered: a task nobody has
		// triaged is not work an agent should pick up.
		StatusInbox: false, StatusTodo: true,
		StatusInProgress: false, StatusDone: false, StatusRejected: false,
	} {
		if got := c.Offers(status); got != want {
			t.Errorf("Offers(%q) = %v, want %v", status, got, want)
		}
	}
	if !c.Satisfies(StatusDone) {
		t.Error("done should satisfy a dependency")
	}
	// The deliberate asymmetry: work nobody will do is not work that is done.
	if c.Satisfies(StatusRejected) {
		t.Error("rejected must not satisfy a dependency; a task waiting on rejected work is still blocked")
	}
}

func TestColumnsNormalize(t *testing.T) {
	var builtin Columns
	if got := builtin.Normalize(StatusBacklog); got != StatusInbox {
		t.Errorf("Normalize(backlog) = %q, want %q", got, StatusInbox)
	}
	if got := builtin.Normalize(StatusDone); got != StatusDone {
		t.Errorf("Normalize(done) = %q, want it unchanged", got)
	}
	// A column the project removed comes back exactly as it went in. Normalize
	// only spells a status the board's way; deciding where a task the board has
	// no column for goes is Reconcile's, and it is a write (TQ-0088).
	if got := custom().Normalize(StatusDone); got != StatusDone {
		t.Errorf("Normalize(done) under a custom board = %q, want it left alone", got)
	}
	if got := custom().Normalize(""); got != "" {
		t.Errorf("Normalize(\"\") = %q, want it left empty", got)
	}
}

// Reconcile is where a board says what to do about a status it has no column
// for, and the answer is the default column rather than the first one: a board
// that lists its done column first would otherwise mark every stranded task
// finished and unblock whatever waited on it (TQ-0088).
func TestColumnsReconcile(t *testing.T) {
	// The default is deliberately not the first column here.
	board := NewColumns([]Column{
		{Name: "shipped", Satisfies: true},
		{Name: "spotted", Ready: true},
		{Name: "doing"},
	}, "doing", nil)

	for _, tc := range []struct {
		status  string
		want    string
		changed bool
	}{
		{status: "doing", want: "doing"},
		{status: "shipped", want: "shipped"},
		{status: "review", want: "doing", changed: true},
		// Empty is no answer at all rather than a column the board has lost, so
		// it is left for Validate to refuse: turning it into one would let a
		// status cleared through `tq update --status ""` quietly move the task.
		{status: "", want: ""},
	} {
		got, changed := board.Reconcile(tc.status)
		if got != tc.want || changed != tc.changed {
			t.Errorf("Reconcile(%q) = %q, %v; want %q, %v", tc.status, got, changed, tc.want, tc.changed)
		}
	}

	// An alias is a change too: the file and the board have to agree on one
	// spelling, or a read has to correct the file for display.
	if got, changed := (Columns{}).Reconcile(StatusBacklog); got != StatusInbox || !changed {
		t.Errorf("Reconcile(backlog) = %q, %v; want %q, true", got, changed, StatusInbox)
	}

	// A board that names no default has only its first column to fall back on,
	// which is the same column a task filed with no status of its own lands in.
	// Pinned so that following the project's own choice is a decision rather
	// than an accident.
	noDefault := NewColumns([]Column{
		{Name: "shipped", Satisfies: true},
		{Name: "spotted", Ready: true},
	}, "", nil)
	if noDefault.Default() != "shipped" {
		t.Fatalf("Default() = %q, want the first column when none is declared", noDefault.Default())
	}
	if got, changed := noDefault.Reconcile("review"); got != "shipped" || !changed {
		t.Errorf("Reconcile(review) = %q, %v; want the board's own default, %q", got, changed, "shipped")
	}
}

// A status the board has no column for sorts last, as a dropped priority does.
// It is a task waiting for the reconciliation that will move it, and it does
// not belong at the head of the board in the meantime.
func TestColumnsRankPutsAStatusTheBoardHasNoColumnForLast(t *testing.T) {
	c := custom()
	if got, want := c.Rank("review"), len(c.Names()); got != want {
		t.Errorf("Rank(review) = %d, want %d", got, want)
	}
	if got := c.Rank("spotted"); got != 0 {
		t.Errorf("Rank(spotted) = %d, want 0", got)
	}
}

func TestColumnsCheckRejectsWhatTheBoardHasNoPlaceFor(t *testing.T) {
	c := custom()
	if err := c.Check("doing"); err != nil {
		t.Errorf("Check(doing) = %v, want nil", err)
	}
	err := c.Check(StatusDone)
	if err == nil {
		t.Fatal("Check(done) = nil, want it refused under a board that has no done")
	}
	if !strings.Contains(err.Error(), "spotted, doing, shipped") {
		t.Errorf("error = %q, want it to list the columns", err)
	}
	// An alias is a spelling of a real column, not an unknown one.
	if err := (Columns{}).Check(StatusBacklog); err != nil {
		t.Errorf("Check(backlog) = %v, want it accepted as the alias it is", err)
	}
}

// `tq done` needs exactly one column to aim at, and has to say so clearly when
// the project has given it none or several.
func TestSatisfyingColumn(t *testing.T) {
	got, err := (Columns{}).SatisfyingColumn()
	if err != nil || got != StatusDone {
		t.Errorf("SatisfyingColumn() = %q, %v, want done", got, err)
	}

	none := NewColumns([]Column{{Name: "a", Ready: true}, {Name: "b"}}, "a", nil)
	if _, err := none.SatisfyingColumn(); err == nil || !strings.Contains(err.Error(), "no column") {
		t.Errorf("SatisfyingColumn() with none marked = %v, want an error saying so", err)
	}

	many := NewColumns([]Column{{Name: "a", Satisfies: true}, {Name: "b", Satisfies: true}}, "a", nil)
	_, err = many.SatisfyingColumn()
	if err == nil || !strings.Contains(err.Error(), "a, b") {
		t.Errorf("SatisfyingColumn() with two marked = %v, want an error naming both", err)
	}
}

func TestReadinessFollowsTheFlagsNotTheStrings(t *testing.T) {
	c := custom()
	mk := func(id, status string, deps ...string) Task {
		return Task{ID: id, Title: id, Status: status, DependsOn: deps}
	}
	// "shipped" satisfies dependencies here, and "done" is not a column at all.
	tasks := []Task{mk("TQ-0001", "shipped"), mk("TQ-0002", "spotted", "TQ-0001"), mk("TQ-0003", "spotted", "TQ-0004")}
	index := IndexTasks(tasks)

	if IsReady(tasks[0], index, c) {
		t.Error("a task in a column that offers no work is not ready")
	}
	if !IsReady(tasks[1], index, c) {
		t.Error("a task whose dependency is shipped should be ready")
	}
	if IsReady(tasks[2], index, c) {
		t.Error("a missing dependency still blocks")
	}
}

func TestSortTasksUsesTheConfiguredColumnOrder(t *testing.T) {
	c := custom()
	at := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	mk := func(id, status string) Task {
		return Task{ID: id, Title: id, Status: status, Created: at, Updated: at}
	}
	tasks := []Task{mk("TQ-0001", "shipped"), mk("TQ-0002", "spotted"), mk("TQ-0003", "doing")}

	SortTasks(tasks, Priorities{}, c)

	got := make([]string, 0, len(tasks))
	for _, tk := range tasks {
		got = append(got, tk.Status)
	}
	if want := "spotted,doing,shipped"; strings.Join(got, ",") != want {
		t.Errorf("order = %q, want %q", strings.Join(got, ","), want)
	}
}
