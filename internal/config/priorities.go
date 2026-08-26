package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fmartingr/taskqueue/internal/task"
)

// Priority is one level of the project's priority vocabulary. Name is what
// task frontmatter stores and what `--priority` takes, so nothing about the
// file format changes when a project declares its own set — only which values
// are accepted, how they rank, and how the board draws them.
type Priority struct {
	Name        string `yaml:"name" json:"name"`
	Color       string `yaml:"color" json:"color"`
	DisplayName string `yaml:"display_name" json:"display_name"`

	// Default marks the level a task with no priority of its own is filed
	// under. Exactly one entry carries it.
	Default bool `yaml:"default" json:"default"`
}

// Unlike labels, priorities are ordered, and a YAML mapping has no order to
// offer once decoded. So the key is a sequence, most severe first, and the file
// itself is the ranking — there is no rank field to keep in step with it.
var basePriorities = []Priority{
	{Name: task.PriorityUrgent, Color: "#b42318", DisplayName: "Urgent"},
	{Name: task.PriorityHigh, Color: "#c2410c", DisplayName: "High"},
	{Name: task.PriorityNormal, Color: "#4b5563", DisplayName: "Normal", Default: true},
	{Name: task.PriorityLow, Color: "#6b7280", DisplayName: "Low"},
}

// DefaultPriorities is the built-in set, in rank order.
func DefaultPriorities() []Priority { return slices.Clone(basePriorities) }

// PrioritySet is the vocabulary this config declares, in rank order.
//
// An absent `priorities` key means "use the built-in set", the way an absent
// `path` means ".tasks". A nil *Config — no config file at all — is the same
// case, so callers never have to check before asking. An explicitly empty one
// is the built-in set too: a project cannot rank tasks by nothing, and the
// loader rejects it rather than letting it get this far.
func (c *Config) PrioritySet() []Priority {
	if c == nil || len(c.Priorities) == 0 {
		return DefaultPriorities()
	}
	return slices.Clone(c.Priorities)
}

// Vocabulary is the priority set as the task package takes it: the accepted
// values in rank order, and the default. It is what the store validates writes
// against and what it sorts by.
func (c *Config) Vocabulary() task.Priorities {
	set := c.PrioritySet()
	names := make([]string, 0, len(set))
	fallback := ""
	for _, priority := range set {
		names = append(names, priority.Name)
		if priority.Default {
			fallback = priority.Name
		}
	}
	return task.NewPriorities(names, fallback)
}

// FindPriority returns how the project draws one value, and whether it declares
// it at all. A value it does not is not an error on read: the vocabulary can be
// edited under tasks already filed, and those keep the value they carry.
func FindPriority(set []Priority, name string) (Priority, bool) {
	for _, priority := range set {
		if priority.Name == name {
			return priority, true
		}
	}
	return Priority{}, false
}

// validatePriorities rejects a set the rest of tq could not work from, and
// fills in the display name an entry did not give.
func validatePriorities(set []Priority) error {
	if set == nil {
		return nil // absent, which means the built-in set
	}
	if len(set) == 0 {
		return fmt.Errorf("priorities is empty (remove the key to use the built-in set: %s)",
			strings.Join(task.Priorities{}.Names(), ", "))
	}

	seen := make(map[string]bool, len(set))
	defaults := make([]string, 0, 1)
	for i := range set {
		priority := &set[i]
		switch {
		case strings.TrimSpace(priority.Name) == "":
			return fmt.Errorf("priority %d has no name", i+1)
		case seen[priority.Name]:
			return fmt.Errorf("priority %q is listed twice", priority.Name)
		case priority.Color == "":
			// `color: #b42318` is a YAML comment, so the value parses as null
			// and lands here. Naming the quoting is the whole point.
			return fmt.Errorf("priority %q has no colour (quote it, as in: color: \"#b42318\")", priority.Name)
		case !ValidLabelColor(priority.Color):
			return fmt.Errorf("priority %q has colour %q, want a hex colour like \"#b42318\"", priority.Name, priority.Color)
		}
		seen[priority.Name] = true
		if priority.Default {
			defaults = append(defaults, priority.Name)
		}
		if priority.DisplayName == "" {
			priority.DisplayName = priority.Name
		}
	}

	// The default is what a task with no priority of its own gets, so exactly
	// one entry must claim it. Both failures name the fix rather than the rule.
	switch len(defaults) {
	case 1:
		return nil
	case 0:
		return fmt.Errorf("no priority is marked `default: true` (one must be, so that a task filed without a priority has one; %s is the least severe)",
			set[len(set)-1].Name)
	default:
		return fmt.Errorf("%d priorities are marked `default: true` (%s); exactly one may be",
			len(defaults), strings.Join(defaults, ", "))
	}
}

// prioritiesYAML renders a priority set as the `priorities:` block of a config
// file, comment and all. Written by hand rather than marshalled so the comment
// survives and `default` appears only where it is true.
func prioritiesYAML(set []Priority) string {
	var b strings.Builder
	b.WriteString(`
# The project's priority vocabulary. Unlike labels, these are a closed set —
# a task can only be filed under one of them — and they are ordered: the list is
# the ranking, most severe first, which is how the board and ` + "`tq list`" + ` sort.
# Exactly one is the default, which a task filed without a priority gets. The
# name is what task frontmatter stores and what --priority takes. Hex colours
# must be quoted, or YAML reads them as a comment.
priorities:
`)
	for _, priority := range set {
		fmt.Fprintf(&b, "  - name: %s\n    color: %q\n    display_name: %s\n", priority.Name, priority.Color, priority.DisplayName)
		if priority.Default {
			b.WriteString("    default: true\n")
		}
	}
	return b.String()
}
