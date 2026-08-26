//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"
)

// The guide tells agents to put an argument starting with "-" after "--".
// Both orders are documented, and both have to work, or a note about a failing
// test cannot be filed at all.
func TestEndOfFlagsTerminator(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")

	t.Run("terminator before the positional", func(t *testing.T) {
		r := p.run(t, "note", "--", "TQ-0001", "-1 test still failing")
		if r.Code != 0 {
			t.Fatalf("exit = %d\nstderr: %s", r.Code, r.Stderr)
		}
	})

	t.Run("terminator after the positional", func(t *testing.T) {
		r := p.run(t, "note", "TQ-0001", "--", "-2 another failing")
		if r.Code != 0 {
			t.Fatalf("exit = %d\nstderr: %s", r.Code, r.Stderr)
		}
	})

	var task struct {
		Body string `json:"body"`
	}
	p.mustRun(t, "show", "TQ-0001", "--json").JSON(t, &task)
	for _, want := range []string{"-1 test still failing", "-2 another failing"} {
		if !strings.Contains(task.Body, want) {
			t.Errorf("body is missing %q:\n%s", want, task.Body)
		}
	}

	// The inverse: after "--", a flag-looking argument is an argument. Taking
	// it as a flag would silently change what the command did.
	t.Run("a flag after the terminator is an argument", func(t *testing.T) {
		r := p.run(t, "add", "--", "-weird title", "--json")
		if r.Code == 0 {
			t.Errorf("exit = 0, want a failure: --json after -- is a second positional, not a flag\nstdout: %s", r.Stdout)
		}
	})
}

// Every command that speaks --json makes the same promise: stdout parses on its
// own. Anything else it wants to say goes to stderr.
func TestJSONPurityAcrossCommands(t *testing.T) {
	t.Parallel()
	p := newProject(t)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"init", []string{"init", "--json"}},
		{"add", []string{"add", "a task", "--json"}},
		{"list", []string{"list", "--json"}},
		{"ready", []string{"ready", "--json"}},
		{"show", []string{"show", "TQ-0001", "--json"}},
		{"update", []string{"update", "TQ-0001", "--assignee", "agent-api", "--json"}},
		{"note", []string{"note", "TQ-0001", "something happened", "--json"}},
		{"move", []string{"move", "TQ-0001", "in-progress", "--json"}},
		{"done", []string{"done", "TQ-0001", "--json"}},
		{"label list", []string{"label", "list", "--json"}},
		{"version", []string{"version", "--json"}},
	} {
		// Sequential on purpose: each builds on the last, which is also how an
		// agent uses them.
		r := p.mustRun(t, tc.args...)
		var any any
		if err := json.Unmarshal([]byte(r.Stdout), &any); err != nil {
			t.Errorf("%s: stdout is not JSON on its own: %v\nstdout: %q\nstderr: %q", tc.name, err, r.Stdout, r.Stderr)
		}
	}
}

// Exit 1 is the validation code, and it covers more than a bad status.
func TestValidationFailuresExitOne(t *testing.T) {
	t.Parallel()
	p := newProject(t)
	p.mustRun(t, "add", "a task")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"empty title", []string{"add", "   "}},
		{"invalid priority", []string{"add", "x", "--priority", "nope"}},
		{"invalid status", []string{"move", "TQ-0001", "nope"}},
		{"malformed id", []string{"show", "not-an-id"}},
		{"self dependency", []string{"update", "TQ-0001", "--add-dependency", "TQ-0001"}},
		{"update with no fields", []string{"update", "TQ-0001"}},
		{"note with no text", []string{"note", "TQ-0001", "   "}},
		{"label with no subcommand", []string{"label"}},
		{"unknown label subcommand", []string{"label", "rename", "a", "b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := p.run(t, tc.args...)
			if r.Code != 1 {
				t.Errorf("tq %s = %d, want 1\nstderr: %s", strings.Join(tc.args, " "), r.Code, r.Stderr)
			}
			if strings.TrimSpace(r.Stderr) == "" {
				t.Error("a validation failure must say why on stderr")
			}
		})
	}
}
