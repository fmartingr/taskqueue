package config

import (
	"fmt"
	"strings"

	"github.com/fmartingr/taskqueue/internal/task"
)

// BoardColumn is one column of the project's board. Name is what task
// frontmatter stores and what `tq move` takes, so nothing about the file format
// changes when a project declares its own board — only which statuses are
// accepted, what order they sit in, and what the two flags below mean for them.
type BoardColumn struct {
	Name        string `yaml:"name" json:"name"`
	DisplayName string `yaml:"display_name" json:"display_name"`

	// ConsiderReady is whether `tq ready` offers work from this column. Off
	// unless the column says so: a board's columns mostly hold work that is
	// unsorted, claimed or finished, and only some hold work waiting to be
	// picked up.
	ConsiderReady bool `yaml:"consider_ready" json:"consider_ready"`

	// ConsiderDone is whether a task sitting here counts as finished — so a
	// dependency on it stops blocking, and `tq done` aims at this column.
	// Exactly one column should claim it; `tq done` says so when none or
	// several do.
	ConsiderDone bool `yaml:"consider_done" json:"consider_done"`

	// Default marks where a task filed without a status goes. Optional: without
	// it that is the first column.
	Default bool `yaml:"default" json:"default"`
}

// Unlike labels, columns are ordered and closed: the list is the board, left to
// right, and a status outside it is refused on write.
var baseColumns = []BoardColumn{
	// Inbox is intake: work lands here before anyone has decided it is worth
	// doing, so it is where a task filed without a status goes and it is not
	// offered to `tq ready` until somebody triages it into To do.
	{Name: task.StatusInbox, DisplayName: "Inbox", Default: true},
	{Name: task.StatusTodo, DisplayName: "To do", ConsiderReady: true},
	{Name: task.StatusInProgress, DisplayName: "In Progress"},
	{Name: task.StatusDone, DisplayName: "Done", ConsiderDone: true},
	{Name: task.StatusRejected, DisplayName: "Rejected"},
}

// DefaultColumns is the built-in board, in order.
func DefaultColumns() []BoardColumn {
	out := make([]BoardColumn, len(baseColumns))
	copy(out, baseColumns)
	return out
}

// ColumnSet is the board this config declares, in order. An absent `columns`
// key means the built-in board, and a nil *Config is the same case.
func (c *Config) ColumnSet() []BoardColumn {
	if c == nil || len(c.Columns) == 0 {
		return DefaultColumns()
	}
	out := make([]BoardColumn, len(c.Columns))
	copy(out, c.Columns)
	return out
}

// Board is the column set as the task package takes it: the order, the two
// flags, the default, and any aliases.
func (c *Config) Board() task.Columns {
	if c == nil || len(c.Columns) == 0 {
		return task.Columns{}
	}

	columns := make([]task.Column, 0, len(c.Columns))
	fallback := ""
	for _, column := range c.Columns {
		columns = append(columns, task.Column{
			Name:      column.Name,
			Ready:     column.ConsiderReady,
			Satisfies: column.ConsiderDone,
		})
		if column.Default {
			fallback = column.Name
		}
	}
	aliases := map[string]string{}
	declared := make(map[string]bool, len(columns))
	for _, column := range columns {
		declared[column.Name] = true
	}
	// Only when the project has an inbox and has not claimed the old name for a
	// column of its own: a board with a real `backlog` column owns that name,
	// and aliasing it away would leave that column unable to hold anything.
	if declared[task.StatusInbox] && !declared[task.StatusBacklog] {
		aliases[task.StatusBacklog] = task.StatusInbox
	}
	return task.NewColumns(columns, fallback, aliases)
}

// validateColumns rejects a board the rest of tq could not work from, and fills
// in the display name a column did not give.
func validateColumns(set []BoardColumn) error {
	if set == nil {
		return nil // absent, which means the built-in board
	}
	if len(set) == 0 {
		return fmt.Errorf("columns is empty (remove the key to use the built-in board: %s)",
			strings.Join(task.Columns{}.Names(), ", "))
	}

	seen := make(map[string]bool, len(set))
	defaults := make([]string, 0, 1)
	for i := range set {
		column := &set[i]
		switch {
		case strings.TrimSpace(column.Name) == "":
			return fmt.Errorf("column %d has no name", i+1)
		case seen[column.Name]:
			return fmt.Errorf("column %q is listed twice", column.Name)
		case strings.ContainsAny(column.Name, "\n\r"):
			// The name goes into task frontmatter as `status:`, and a line
			// break there renders as a block scalar whose every line can look
			// like the frontmatter delimiter.
			return fmt.Errorf("column %d has a line break in its name", i+1)
		case strings.TrimSpace(column.Name) != column.Name:
			return fmt.Errorf("column %q has leading or trailing whitespace", column.Name)
		}
		seen[column.Name] = true
		if column.Default {
			defaults = append(defaults, column.Name)
		}
		if column.DisplayName == "" {
			column.DisplayName = column.Name
		}
	}

	if len(defaults) > 1 {
		return fmt.Errorf("%d columns are marked `default: true` (%s); at most one may be",
			len(defaults), strings.Join(defaults, ", "))
	}
	// Zero or several columns claiming to satisfy dependencies is not refused
	// here: a board may legitimately have none while it is being worked out,
	// and `tq done` is where that becomes a problem it can describe precisely.
	return nil
}

// columnsYAML renders a board as the `columns:` block of a config file. Written
// by hand rather than marshalled so the comment survives and the optional flags
// appear only where they say something.
func columnsYAML(set []BoardColumn) string {
	var b strings.Builder
	b.WriteString(`
# The project's board. Like priorities these are a closed, ordered set — a task
# can only sit in one of them, and the list is the board from left to right.
# The name is what task frontmatter stores and what ` + "`tq move`" + ` takes.
#
# consider_ready: true   work here is waiting to be picked up, so ` + "`tq ready`" + `
#                     offers it. Left out, the column holds work that is
#                     unsorted, claimed or finished.
# consider_done: true  a task here counts as finished, so a dependency on it
#                     stops blocking and ` + "`tq done`" + ` moves tasks to this
#                     column. Exactly one column should claim it.
# default: true       where a task filed without a status goes
columns:
`)
	for _, column := range set {
		fmt.Fprintf(&b, "  - name: %s\n    display_name: %s\n", column.Name, column.DisplayName)
		if column.ConsiderReady {
			b.WriteString("    consider_ready: true\n")
		}
		if column.ConsiderDone {
			b.WriteString("    consider_done: true\n")
		}
		if column.Default {
			b.WriteString("    default: true\n")
		}
	}
	return b.String()
}
