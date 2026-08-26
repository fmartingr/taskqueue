package task

import (
	"fmt"
	"slices"
	"strings"
)

// Priorities is a priority vocabulary: the accepted values in rank order, most
// severe first, and the one a task gets when it names none. The project
// declares it, so the ranking is a value passed in rather than a constant here
// — this package still imports nothing of ours and knows nothing about where a
// config lives.
//
// The zero value is the built-in set, so a caller with no config to consult
// (a test, a parse) can leave it out and get today's behaviour.
type Priorities struct {
	names    []string
	fallback string
}

// NewPriorities builds a vocabulary from an ordered list, most severe first.
// An empty list is the built-in set: a project cannot rank tasks by nothing,
// and refusing here would leave the config loader as the only place that could
// report it.
func NewPriorities(names []string, fallback string) Priorities {
	if len(names) == 0 {
		return Priorities{}
	}
	return Priorities{names: slices.Clone(names), fallback: fallback}
}

// Names is the vocabulary in rank order, most severe first.
func (p Priorities) Names() []string { return slices.Clone(p.effective()) }

// effective is Names without the copy, for the lookups that only read it.
// SortTasks ranks inside a comparator, so a clone per call would be one
// allocation per comparison.
func (p Priorities) effective() []string {
	if len(p.names) == 0 {
		return builtinPriorities
	}
	return p.names
}

// Default is what a task with no priority of its own is filed under.
//
// A vocabulary with no default named falls back to the most severe, matching
// defaultPriority in frontend/board.ts. The config loader refuses such a set,
// so neither branch is reachable from a real config — but these are two
// implementations of one rule, and two of them disagreeing is how they drift.
func (p Priorities) Default() string {
	if len(p.names) == 0 {
		return PriorityNormal
	}
	if p.fallback == "" {
		return p.names[0]
	}
	return p.fallback
}

// Valid reports whether this vocabulary accepts a value.
func (p Priorities) Valid(priority string) bool {
	return slices.Contains(p.effective(), priority)
}

// Rank orders one value against another, most severe first. A value the
// project no longer declares ranks last rather than being rejected: editing the
// vocabulary must not break the tasks already filed under the old one.
func (p Priorities) Rank(priority string) int {
	names := p.effective()
	if i := slices.Index(names, priority); i >= 0 {
		return i
	}
	return len(names)
}

// Check reports whether a value may be written, naming the vocabulary when it
// may not. Empty passes: the store fills it in with Default.
//
// This is deliberately not part of Validate. Reading stays tolerant of any
// value, so a task filed under a priority the project has since dropped still
// loads and still lists; only a write has to agree with the vocabulary as it
// stands now.
func (p Priorities) Check(priority string) error {
	if priority == "" || p.Valid(priority) {
		return nil
	}
	return fmt.Errorf("invalid priority %q (want one of %s)", priority, strings.Join(p.effective(), ", "))
}
