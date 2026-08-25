package taskqueue

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"
)

// Exit codes are part of the agent-facing contract and must stay stable.
const (
	exitOK              = 0
	exitError           = 1 // general or validation error
	exitTaskNotFound    = 2
	exitProjectNotFound = 3
)

// cli holds everything a command needs from its environment, which keeps the
// commands testable without spawning the binary.
type cli struct {
	stdout io.Writer
	stderr io.Writer
	dir    string // working directory used to discover the task directory
}

func runCLI(c *cli, args []string) int {
	if len(args) == 0 {
		c.usage(c.stderr)
		return exitError
	}

	switch args[0] {
	case "init":
		return c.runInit(args[1:])
	case "add":
		return c.runAdd(args[1:])
	case "list":
		return c.runList(args[1:])
	case "show":
		return c.runShow(args[1:])
	case "move":
		return c.runMove(args[1:])
	case "done":
		return c.runDone(args[1:])
	case "update":
		return c.runUpdate(args[1:])
	case "note":
		return c.runNote(args[1:])
	case "ready":
		return c.runReady(args[1:])
	case "serve":
		return c.runServe(args[1:])
	case "version", "--version":
		return c.runVersion(args[1:])
	case "help", "-h", "--help":
		c.usage(c.stdout)
		return exitOK
	default:
		fmt.Fprintf(c.stderr, "error: unknown command %q\n\n", args[0])
		c.usage(c.stderr)
		return exitError
	}
}

const usageText = `tq — a Git-native task queue: Markdown on disk, CLI for agents, Kanban for humans.

Usage:
  tq <command> [arguments]

Commands:
  init                            Create the %s directory and write the agent
                                  guide (%s); prints the line to add to your
                                  own AGENTS.md/CLAUDE.md
  add <title> [flags]             Create a task
  list [flags]                    List tasks
  show <id> [--json]              Show one task
  move <id> <status>              Change a task's status
  done <id>                       Shorthand for: move <id> done
  update <id> [flags]             Change fields of a task
  note <id> <text>                Append a timestamped note to a task
  ready [flags]                   List tasks that are unblocked and unclaimed
  serve [flags]                   Serve the Kanban board and REST API
  version                         Print the version
  help                            Print this help

task.Statuses:   %s
task.Priorities: %s (highest first; default: %s)

Common flags:
  --json                          Print JSON to stdout and nothing else
  --status, --priority, --label, --assignee   Filters for list/ready
  --                              End of flags: everything after it is an
                                  argument, even if it starts with "-"

Environment:
  %s      task.Task directory to use instead of discovering %s
  TQ_HOST, TQ_PORT   Defaults for tq serve
  DEV                Serve frontend assets from ./public instead of the embedded copy

Exit codes:
  0 success   1 general/validation error   2 task not found
  3 %s directory missing and could not be created
`

func (c *cli) usage(w io.Writer) {
	fmt.Fprintf(w, usageText,
		config.TaskDirName, filepath.Join(config.TaskDirName, AgentsFileName),
		strings.Join(task.Statuses, ", "),
		strings.Join(task.Priorities, ", "), task.PriorityNormal,
		config.EnvTaskDir, config.TaskDirName,
		config.TaskDirName)
}

// ── Commands ────────────────────────────────────────────────────

func (c *cli) runInit(args []string) int {
	fs := c.flagSet("init")
	jsonOut := fs.Bool("json", false, "print JSON output")
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}

	// Discover the queue the way every other command does, so init in a
	// subdirectory adopts the project's queue instead of forking a second one.
	// OpenStore falls back to creating at the repository root when there is
	// nothing to find, and discovery stops there too (TQ-0017), so init cannot
	// adopt a queue from outside the repository.
	store, err := OpenStore(c.dir)
	if err != nil {
		return c.fail(err)
	}

	// Only a task directory this project owns gets a marker and a guide.
	// Discovery has no bound in a project without a repository root, so the
	// store may belong to somebody else: writing a guide there would dirty
	// another repository's working tree, and writing a marker here would bind
	// this directory to their queue for good.
	var written []string
	if withinInvokedTree(store.Dir, c.dir) {
		// A queue that predates the marker gets one now: init is the command
		// that says "this project uses tq", and the file is what later
		// commands find.
		if store.ConfigWritten == "" {
			config, err := config.WriteConfigIfMissing(c.dir, store.Dir)
			if err != nil {
				return c.fail(err)
			}
			store.ConfigWritten = config
		}
		if store.ConfigWritten != "" {
			written = append(written, store.ConfigWritten)
		}

		guide, err := SyncAgentsDocs(store)
		if err != nil {
			return c.fail(err)
		}
		written = append(written, guide...)
	} else if !*jsonOut {
		fmt.Fprintf(c.stderr, "note: %s was found above this directory; leaving it and its guide alone\n", store.Dir)
	}

	if *jsonOut {
		return c.printJSON(map[string]any{
			"task_dir": store.Dir,
			"created":  store.Created,
			"written":  written,
			"pointer":  GuidePointer(store),
		})
	}
	if store.Created {
		fmt.Fprintf(c.stdout, "Initialized task queue in %s\n", store.Dir)
	} else {
		fmt.Fprintf(c.stdout, "task.Task queue already initialized in %s\n", store.Dir)
	}
	for _, path := range written {
		fmt.Fprintf(c.stdout, "Wrote %s\n", path)
	}
	// tq does not edit the repository's own agent instructions, so say what to
	// put there. One line, written once, and the guide comes with it.
	fmt.Fprintf(c.stdout, "\nAdd this line to your AGENTS.md or CLAUDE.md so agents read the guide:\n\n    %s\n", GuidePointer(store))
	return exitOK
}

// withinInvokedTree reports whether a task directory belongs to the tree the
// command was invoked in: the enclosing repository when there is one, and the
// working directory otherwise. A directory further up belongs to whatever
// project holds it, which may not be this one.
func withinInvokedTree(taskDir, workingDir string) bool {
	base, err := filepath.Abs(workingDir)
	if err != nil {
		return false
	}
	if root, ok := config.RepositoryRoot(base); ok {
		base = root
	}
	rel, err := filepath.Rel(base, taskDir)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

func (c *cli) runAdd(args []string) int {
	fs := c.flagSet("add")
	priority := fs.String("priority", "", "priority: "+strings.Join(task.Priorities, ", "))
	assignee := fs.String("assignee", "", "assignee")
	body := fs.String("body", "", "Markdown body")
	status := fs.String("status", "", "initial status (default: "+task.StatusTodo+")")
	var labels, dependsOn stringList
	fs.Var(&labels, "label", "label (repeatable)")
	fs.Var(&dependsOn, "depends-on", "dependency task ID (repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON output")

	positional, code, ok := c.parse(fs, args, 1)
	if !ok {
		return code
	}

	store, err := c.store()
	if err != nil {
		return c.fail(err)
	}
	t, err := store.Create(CreateTaskInput{
		Title:     positional[0],
		Status:    *status,
		Priority:  *priority,
		Assignee:  *assignee,
		Labels:    labels,
		DependsOn: dependsOn,
		Body:      *body,
	})
	if err != nil {
		return c.fail(err)
	}

	if *jsonOut {
		return c.printJSON(t)
	}
	fmt.Fprintf(c.stdout, "Created %s: %s\n", t.ID, t.Title)
	return exitOK
}

func (c *cli) runList(args []string) int {
	fs := c.flagSet("list")
	filter, jsonOut := c.filterFlags(fs, true)
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}
	if err := filter.Validate(); err != nil {
		return c.fail(err)
	}

	tasks, err := c.tasks()
	if err != nil {
		return c.fail(err)
	}
	tasks = task.FilterTasks(tasks, *filter)

	if *jsonOut {
		return c.printJSON(tasks)
	}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{t.ID, t.Status, t.Priority, orDash(t.Assignee), t.Title})
	}
	c.table([]string{"ID", "STATUS", "PRIORITY", "ASSIGNEE", "TITLE"}, rows)
	return exitOK
}

func (c *cli) runReady(args []string) int {
	fs := c.flagSet("ready")
	filter, jsonOut := c.filterFlags(fs, false)
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}
	filter.Ready = true
	if err := filter.Validate(); err != nil {
		return c.fail(err)
	}

	tasks, err := c.tasks()
	if err != nil {
		return c.fail(err)
	}
	tasks = task.FilterTasks(tasks, *filter)

	if *jsonOut {
		return c.printJSON(tasks)
	}
	rows := make([][]string, 0, len(tasks))
	for _, t := range tasks {
		rows = append(rows, []string{t.ID, t.Priority, orDash(strings.Join(t.Labels, ",")), t.Title})
	}
	c.table([]string{"ID", "PRIORITY", "LABELS", "TITLE"}, rows)
	return exitOK
}

func (c *cli) runShow(args []string) int {
	fs := c.flagSet("show")
	jsonOut := fs.Bool("json", false, "print JSON output")
	positional, code, ok := c.parse(fs, args, 1)
	if !ok {
		return code
	}

	store, err := c.store()
	if err != nil {
		return c.fail(err)
	}
	t, err := store.Get(positional[0])
	if err != nil {
		return c.fail(err)
	}

	if *jsonOut {
		return c.printJSON(t)
	}

	w := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID:\t%s\n", t.ID)
	fmt.Fprintf(w, "Title:\t%s\n", t.Title)
	fmt.Fprintf(w, "Status:\t%s\n", t.Status)
	fmt.Fprintf(w, "Priority:\t%s\n", t.Priority)
	if t.Assignee != "" {
		fmt.Fprintf(w, "Assignee:\t%s\n", t.Assignee)
	}
	if len(t.Labels) > 0 {
		fmt.Fprintf(w, "Labels:\t%s\n", strings.Join(t.Labels, ", "))
	}
	if len(t.DependsOn) > 0 {
		fmt.Fprintf(w, "Depends on:\t%s\n", strings.Join(c.describeDeps(t), ", "))
	}
	fmt.Fprintf(w, "Created:\t%s\n", t.Created.Format(time.RFC3339))
	fmt.Fprintf(w, "Updated:\t%s\n", t.Updated.Format(time.RFC3339))
	w.Flush()

	if t.Body != "" {
		fmt.Fprintf(c.stdout, "\n%s\n", t.Body)
	}
	return exitOK
}

// describeDeps annotates each dependency with its status. Resolving the other
// tasks is best-effort: a broken sibling file must not hide this task.
func (c *cli) describeDeps(t task.Task) []string {
	index := map[string]task.Task{}
	if tasks, err := c.tasks(); err != nil {
		fmt.Fprintf(c.stderr, "warning: could not resolve dependencies: %v\n", err)
	} else {
		index = task.IndexTasks(tasks)
	}

	out := make([]string, 0, len(t.DependsOn))
	for _, dep := range t.DependsOn {
		other, ok := index[dep]
		switch {
		case !ok:
			out = append(out, dep+" (missing)")
		case other.Status == task.StatusDone:
			out = append(out, dep+" (done)")
		default:
			out = append(out, fmt.Sprintf("%s (%s, blocking)", dep, other.Status))
		}
	}
	return out
}

func (c *cli) runMove(args []string) int {
	fs := c.flagSet("move")
	jsonOut := fs.Bool("json", false, "print JSON output")
	positional, code, ok := c.parse(fs, args, 2)
	if !ok {
		return code
	}
	return c.moveTask(positional[0], positional[1], *jsonOut)
}

func (c *cli) runDone(args []string) int {
	fs := c.flagSet("done")
	jsonOut := fs.Bool("json", false, "print JSON output")
	positional, code, ok := c.parse(fs, args, 1)
	if !ok {
		return code
	}
	return c.moveTask(positional[0], task.StatusDone, *jsonOut)
}

// moveTask is the shared status transition behind `tq move` and `tq done`.
func (c *cli) moveTask(id, status string, jsonOut bool) int {
	if !task.ValidStatus(status) {
		return c.fail(fmt.Errorf("invalid status %q (want one of %s)", status, strings.Join(task.Statuses, ", ")))
	}

	store, err := c.store()
	if err != nil {
		return c.fail(err)
	}
	t, err := store.Get(id)
	if err != nil {
		return c.fail(err)
	}

	previous := t.Status
	if previous == status {
		if jsonOut {
			return c.printJSON(t)
		}
		fmt.Fprintf(c.stdout, "%s is already %s\n", t.ID, status)
		return exitOK
	}

	t.Status = status
	t, err = store.Update(t)
	if err != nil {
		return c.fail(err)
	}

	if jsonOut {
		return c.printJSON(t)
	}
	fmt.Fprintf(c.stdout, "%s: %s -> %s\n", t.ID, previous, t.Status)
	return exitOK
}

func (c *cli) runUpdate(args []string) int {
	fs := c.flagSet("update")
	title := fs.String("title", "", "new title")
	status := fs.String("status", "", "new status")
	priority := fs.String("priority", "", "new priority")
	assignee := fs.String("assignee", "", "new assignee")
	var addLabels, removeLabels, addDeps, removeDeps stringList
	fs.Var(&addLabels, "add-label", "add a label (repeatable)")
	fs.Var(&removeLabels, "remove-label", "remove a label (repeatable)")
	fs.Var(&addDeps, "add-dependency", "add a dependency (repeatable)")
	fs.Var(&removeDeps, "remove-dependency", "remove a dependency (repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON output")

	positional, code, ok := c.parse(fs, args, 1)
	if !ok {
		return code
	}

	patch := task.TaskPatch{
		AddLabels:    addLabels,
		RemoveLabels: removeLabels,
		AddDeps:      addDeps,
		RemoveDeps:   removeDeps,
	}
	// Only flags that were actually passed become part of the patch.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "title":
			patch.Title = title
		case "status":
			patch.Status = status
		case "priority":
			patch.Priority = priority
		case "assignee":
			patch.Assignee = assignee
		}
	})
	if patch.IsEmpty() {
		return c.fail(errors.New("update needs at least one field to change (see `tq help`)"))
	}

	store, err := c.store()
	if err != nil {
		return c.fail(err)
	}
	t, err := store.Patch(positional[0], patch)
	if err != nil {
		return c.fail(err)
	}

	if *jsonOut {
		return c.printJSON(t)
	}
	fmt.Fprintf(c.stdout, "Updated %s: %s\n", t.ID, t.Title)
	return exitOK
}

func (c *cli) runNote(args []string) int {
	fs := c.flagSet("note")
	jsonOut := fs.Bool("json", false, "print JSON output")
	positional, code, ok := c.parse(fs, args, 2)
	if !ok {
		return code
	}

	text := strings.TrimSpace(positional[1])
	if text == "" {
		return c.fail(errors.New("note text cannot be empty"))
	}

	store, err := c.store()
	if err != nil {
		return c.fail(err)
	}
	t, err := store.Note(positional[0], text)
	if err != nil {
		return c.fail(err)
	}

	if *jsonOut {
		return c.printJSON(t)
	}
	fmt.Fprintf(c.stdout, "Note added to %s\n", t.ID)
	return exitOK
}

func (c *cli) runVersion(args []string) int {
	fs := c.flagSet("version")
	jsonOut := fs.Bool("json", false, "print JSON output")
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}
	if *jsonOut {
		return c.printJSON(map[string]string{"version": version})
	}
	fmt.Fprintf(c.stdout, "tq %s\n", version)
	return exitOK
}

// ── Helpers ─────────────────────────────────────────────────────

func (c *cli) store() (*Store, error) {
	store, err := OpenStore(c.dir)
	if err != nil {
		return nil, err
	}
	if store.Created {
		// stderr, so --json output stays machine-readable.
		fmt.Fprintf(c.stderr, "note: created %s\n", store.Dir)
		if shadowed, ok := ShadowedTaskDir(c.dir); ok {
			fmt.Fprintf(c.stderr, "note: %s is above this repository and was not used; set %s=true to search past the repository root\n", shadowed, config.EnvWalkForever)
		}
	}
	return store, nil
}

func (c *cli) tasks() ([]task.Task, error) {
	store, err := c.store()
	if err != nil {
		return nil, err
	}
	return store.List()
}

func (c *cli) flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("tq "+name, flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	return fs
}

// filterFlags registers the shared filters. `tq ready` omits --status, since
// readiness already implies the status.
func (c *cli) filterFlags(fs *flag.FlagSet, withStatus bool) (*task.Filter, *bool) {
	var f task.Filter
	if withStatus {
		fs.StringVar(&f.Status, "status", "", "filter by status")
	}
	fs.StringVar(&f.Priority, "priority", "", "filter by priority")
	fs.StringVar(&f.Label, "label", "", "filter by label")
	fs.StringVar(&f.Assignee, "assignee", "", "filter by assignee")
	return &f, fs.Bool("json", false, "print JSON output")
}

// parse accepts flags and positional arguments in any order (`tq add "title"
// --priority high` reads naturally) and enforces the expected argument count.
// ok is false when the command must stop — help was printed, parsing failed, or
// the argument count is wrong — and code is what tq should exit with.
func (c *cli) parse(fs *flag.FlagSet, args []string, want int) (positional []string, code int, ok bool) {
	for {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) { // -h/--help: usage was already printed
				return nil, exitOK, false
			}
			if want > 0 {
				fmt.Fprintf(c.stderr, "hint: an argument that starts with \"-\" must follow \"--\", as in: %s TQ-0001 -- \"-1 test still failing\"\n", fs.Name())
			}
			return nil, exitError, false
		}
		rest := fs.Args()
		if len(rest) == 0 {
			break
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}

	if len(positional) != want {
		fmt.Fprintf(c.stderr, "error: %s expects %d argument(s), got %d (quote arguments that contain spaces)\n",
			fs.Name(), want, len(positional))
		fs.Usage()
		return nil, exitError, false
	}
	return positional, exitOK, true
}

// fail reports an error on stderr and maps it to the documented exit code.
// stdout is never touched, so --json output stays parseable.
func (c *cli) fail(err error) int {
	fmt.Fprintf(c.stderr, "error: %v\n", err)
	switch {
	case errors.Is(err, ErrTaskNotFound):
		return exitTaskNotFound
	case errors.Is(err, ErrProjectNotFound):
		return exitProjectNotFound
	default:
		return exitError
	}
}

func (c *cli) printJSON(v any) int {
	enc := json.NewEncoder(c.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return c.fail(err)
	}
	return exitOK
}

func (c *cli) table(header []string, rows [][]string) {
	w := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// stringList collects repeatable flags such as --label.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}
