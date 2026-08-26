package task

import (
	"strings"
	"testing"
	"time"
)

// The zero value is the built-in vocabulary, so a caller with no config to
// consult — a parse, a test — gets today's behaviour without saying so.
func TestPrioritiesZeroValueIsTheBuiltInSet(t *testing.T) {
	var p Priorities
	if got, want := strings.Join(p.Names(), ","), "urgent,high,normal,low"; got != want {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	if p.Default() != PriorityNormal {
		t.Errorf("Default() = %q, want %q", p.Default(), PriorityNormal)
	}
	if !p.Valid(PriorityUrgent) || p.Valid("whenever") {
		t.Error("Valid() should accept the built-in set and nothing else")
	}
}

// An empty list is the built-in set too. A project cannot rank tasks by
// nothing, and the config loader refuses one before it reaches here.
func TestNewPrioritiesFallsBackOnAnEmptyList(t *testing.T) {
	p := NewPriorities(nil, "")
	if got, want := strings.Join(p.Names(), ","), strings.Join(Priorities{}.Names(), ","); got != want {
		t.Errorf("Names() = %q, want the built-in %q", got, want)
	}
	if p.Default() != PriorityNormal {
		t.Errorf("Default() = %q, want %q", p.Default(), PriorityNormal)
	}
}

func TestPrioritiesCustomSet(t *testing.T) {
	p := NewPriorities([]string{"p0", "p1", "p2"}, "p1")

	if got, want := strings.Join(p.Names(), ","), "p0,p1,p2"; got != want {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	if p.Default() != "p1" {
		t.Errorf("Default() = %q, want p1", p.Default())
	}
	if p.Valid(PriorityUrgent) {
		t.Error("Valid(urgent) = true, want false: a custom set replaces the built-in one")
	}
	if err := p.Check(PriorityUrgent); err == nil || !strings.Contains(err.Error(), "p0, p1, p2") {
		t.Errorf("Check(urgent) = %v, want an error naming the vocabulary", err)
	}
	// Empty is what a task with no priority of its own carries; the store
	// fills it in with Default rather than the check refusing it.
	if err := p.Check(""); err != nil {
		t.Errorf("Check(\"\") = %v, want nil", err)
	}
}

// The vocabulary can be edited under tasks already filed. Ranking a value it
// no longer holds last is what keeps those tasks listed instead of lost.
func TestPrioritiesRankPutsAnUnknownValueLast(t *testing.T) {
	p := NewPriorities([]string{"p0", "p1"}, "p1")
	if got := p.Rank("whenever"); got != 2 {
		t.Errorf("Rank(whenever) = %d, want 2 (past every configured value)", got)
	}
	if p.Rank("p0") >= p.Rank("p1") {
		t.Error("Rank should follow the configured order, most severe first")
	}
}

// Sorting is by position in the configured sequence: the file is the ranking.
func TestSortTasksUsesTheConfiguredOrder(t *testing.T) {
	created := time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC)
	mk := func(id, priority string) Task {
		return Task{ID: id, Title: id, Status: StatusTodo, Priority: priority, Created: created, Updated: created}
	}
	tasks := []Task{mk("TQ-0001", "p2"), mk("TQ-0002", "dropped"), mk("TQ-0003", "p0"), mk("TQ-0004", "p1")}

	SortTasks(tasks, NewPriorities([]string{"p0", "p1", "p2"}, "p1"))

	got := make([]string, 0, len(tasks))
	for _, tk := range tasks {
		got = append(got, tk.Priority)
	}
	if want := "p0,p1,p2,dropped"; strings.Join(got, ",") != want {
		t.Errorf("order = %q, want %q (a dropped value sorts last)", strings.Join(got, ","), want)
	}
}

// A filter naming a priority the project cannot file is refused rather than
// returning nothing, which would read as an empty queue.
func TestFilterValidateChecksAgainstTheVocabulary(t *testing.T) {
	p := NewPriorities([]string{"p0", "p1"}, "p1")

	if err := (Filter{Priority: "p0"}).Validate(p); err != nil {
		t.Errorf("Validate() on a configured priority = %v, want nil", err)
	}
	if err := (Filter{Priority: PriorityHigh}).Validate(p); err == nil {
		t.Error("Validate() on a priority outside the vocabulary = nil, want an error")
	}
	if err := (Filter{Status: StatusTodo}).Validate(p); err != nil {
		t.Errorf("Validate() with no priority = %v, want nil", err)
	}
}

// The config loader refuses a set with no default, so this is unreachable from
// a real config — but defaultPriority in frontend/board.ts answers the same
// question, and the two must not drift apart where one of them is guessing.
func TestPrioritiesDefaultFallsBackToTheMostSevere(t *testing.T) {
	p := NewPriorities([]string{"p0", "p1"}, "")
	if got := p.Default(); got != "p0" {
		t.Errorf("Default() with none named = %q, want p0 (the most severe)", got)
	}
}
