package config_test

import (
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/tqtest"

	"github.com/fmartingr/taskqueue/internal/config"
)

const customBoard = `version: 1
path: .tasks
columns:
  - name: spotted
    display_name: Spotted
    consider_ready: true
  - name: doing
    display_name: Doing
    default: true
  - name: shipped
    display_name: Shipped
    consider_done: true
`

func TestBoardDefaultsWithoutAConfig(t *testing.T) {
	cfg, err := config.FindConfig(tqtest.RootWithoutMarker(t))
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if got, want := strings.Join(cfg.Board().Names(), ","), "inbox,todo,in-progress,done,rejected"; got != want {
		t.Errorf("Board() = %q, want %q", got, want)
	}
	if got := cfg.Board().Default(); got != task.StatusInbox {
		t.Errorf("Default() = %q, want inbox — intake is where a task filed without a status lands", got)
	}
	if len(cfg.ColumnSet()) != len(config.DefaultColumns()) {
		t.Errorf("ColumnSet() = %d columns, want the built-in %d", len(cfg.ColumnSet()), len(config.DefaultColumns()))
	}
}

func TestBoardReadsTheProjectsColumns(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, customBoard)

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	board := cfg.Board()

	if got, want := strings.Join(board.Names(), ","), "spotted,doing,shipped"; got != want {
		t.Errorf("Board() = %q, want %q (the order the file lists)", got, want)
	}
	if board.Default() != "doing" {
		t.Errorf("Default() = %q, want the column marked default", board.Default())
	}
	if !board.Offers("spotted") || board.Offers("doing") || board.Offers("shipped") {
		t.Error("Offers() should follow each column's ready flag")
	}
	if !board.Satisfies("shipped") || board.Satisfies("doing") {
		t.Error("Satisfies() should follow consider_done")
	}
	if got, err := board.SatisfyingColumn(); err != nil || got != "shipped" {
		t.Errorf("SatisfyingColumn() = %q, %v, want shipped", got, err)
	}
	// The built-in alias belongs to the built-in column; a project that has no
	// inbox has not asked to inherit it.
	if board.Valid(task.StatusBacklog) {
		t.Error("backlog should not resolve on a board with no inbox column")
	}
}

// A project that keeps the built-in names keeps the rename's alias with them.
func TestABoardThatKeepsInboxKeepsTheBacklogAlias(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\ncolumns:\n  - {name: inbox}\n  - {name: done, consider_done: true}\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if got := cfg.Board().Normalize(task.StatusBacklog); got != task.StatusInbox {
		t.Errorf("Normalize(backlog) = %q, want inbox", got)
	}
}

func TestColumnConfigErrors(t *testing.T) {
	for _, tc := range []struct{ name, body, wantErr string }{
		{"empty list", "columns: []\n", "columns is empty"},
		{"no name", "columns:\n  - {display_name: Nameless}\n", "has no name"},
		{"duplicate", "columns:\n  - {name: a}\n  - {name: a}\n", "listed twice"},
		{"two defaults", "columns:\n  - {name: a, default: true}\n  - {name: b, default: true}\n", "2 columns are marked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tqtest.Root(t)
			path := tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n"+tc.body)

			_, err := config.FindConfig(root)
			if err == nil {
				t.Fatal("config.FindConfig() = nil, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), path) {
				t.Errorf("error = %q, want %q and the file named", err, tc.wantErr)
			}
		})
	}
}

// A board with no column claiming to satisfy dependencies loads: it is a
// legitimate half-finished state, and `tq done` is where it becomes a problem
// that can be described precisely.
func TestABoardWithoutADoneColumnLoads(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\ncolumns:\n  - {name: a}\n  - {name: b}\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if _, err := cfg.Board().SatisfyingColumn(); err == nil {
		t.Error("SatisfyingColumn() = nil error, want it to report that no column claims it")
	}
}

func TestWriteConfigSeedsTheBoard(t *testing.T) {
	// WriteConfigIfMissing only writes where there is no marker.
	root := tqtest.RootWithoutMarker(t)
	if _, err := config.WriteConfigIfMissing(root, root+"/.tasks"); err != nil {
		t.Fatalf("WriteConfigIfMissing: %v", err)
	}
	cfg, err := config.FindConfig(root)
	if err != nil || cfg == nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if got, want := strings.Join(cfg.Board().Names(), ","), "inbox,todo,in-progress,done,rejected"; got != want {
		t.Errorf("seeded board = %q, want %q", got, want)
	}
	if cfg.Board().Default() != task.StatusInbox {
		t.Errorf("seeded default = %q, want inbox", cfg.Board().Default())
	}
}
