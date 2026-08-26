package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fmartingr/taskqueue/internal/task"

	"github.com/fmartingr/taskqueue/internal/config"

	"github.com/fmartingr/taskqueue/internal/store"

	"context"
	"github.com/fmartingr/taskqueue/internal/guide"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/fmartingr/taskqueue/internal/web"
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
	// version is stamped on the binary at build time and passed in, because
	// this package is not the binary.
	version string

	stdout io.Writer
	stderr io.Writer
	dir    string // working directory used to discover the task directory
}

// Run executes one command and returns the process exit code. It takes its
// streams, its working directory and the binary's version rather than reaching
// for globals, so a test can drive it the same way the binary does.
func Run(stdout, stderr io.Writer, dir, version string, args []string) int {
	return runCLI(&cli{stdout: stdout, stderr: stderr, dir: dir, version: version}, args)
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
	case "label":
		return c.runLabel(args[1:])
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
                                  guide (%s); prints where the guide is,
                                  to reference from your own AGENTS.md/CLAUDE.md
  add <title> [flags]             Create a task
  list [flags]                    List tasks
  show <id> [--json]              Show one task
  move <id> <status>              Change a task's status
  done <id>                       Shorthand for: move <id> done
  update <id> [flags]             Change fields of a task
  note <id> <text>                Append a timestamped note to a task
  ready [flags]                   List tasks that are unblocked and unclaimed
  label list [--json]             Print the project's label vocabulary and the
                                  labels in use that it does not declare
  serve [flags]                   Serve the Kanban board and REST API
  version                         Print the version
  help                            Print this help

Statuses:   %s
Priorities: %s (most severe first; default: %s)

Common flags:
  --json                          Print JSON to stdout and nothing else
  --status, --priority, --label, --assignee   Filters for list/ready
  --                              End of flags: everything after it is an
                                  argument, even if it starts with "-"

Server address (tq serve), highest precedence first:
  --host, --port                  Flags
  TQ_HOST, TQ_PORT                Environment
  server.host, server.port        In %s, committed with the project
  %-30s  Built in

  A committed host that is not loopback exposes the board on the network of
  everyone who clones the project; tq says so on start-up when it happens.

Environment:
  %s      Task directory to use instead of discovering %s
  DEV                Serve frontend assets from disk instead of the embedded copy

Exit codes:
  0 success   1 general/validation error   2 task not found
  3 %s directory missing and could not be created
`

func (c *cli) usage(w io.Writer) {
	priorities := c.priorities()
	fmt.Fprintf(w, usageText,
		config.TaskDirName, filepath.Join(config.TaskDirName, guide.AgentsFileName),
		strings.Join(c.board().Names(), ", "),
		strings.Join(priorities.Names(), ", "), priorities.Default(),
		config.ConfigFileName, defaultHost+", "+defaultPort,
		config.EnvTaskDir, config.TaskDirName,
		config.TaskDirName)
}

// priorities is the vocabulary for help text and for the filters, which are
// both needed before a command has opened a store. It resolves from the queue
// the command will work on, not from the working directory, so TQ_DIR pointing
// at another project's queue does not have the CLI offer this project's set.
//
// A config that cannot be read falls back to the built-in set rather than
// failing: help must print, and a filter against a broken config is reported by
// the read that follows it. The store is what validates a write, and it reads
// the config itself.
func (c *cli) priorities() task.Priorities {
	return c.config().Vocabulary()
}

// board is the project's columns, resolved the same way and for the same
// reasons as priorities: help and filters both need it before a store exists.
func (c *cli) board() task.Columns {
	return c.config().Board()
}

// config is the project's config, resolved from the queue the command will
// work on rather than from the working directory, so TQ_DIR pointing at another
// project's queue reads that project's config.
//
// A config that cannot be read reads as none. Every caller here wants a default
// rather than a failure — help must print, and a command that goes on to touch
// the store reports the broken config itself, with the file named.
func (c *cli) config() *config.Config {
	dir := c.dir
	if taskDir, err := store.DiscoverTaskDir(c.dir); err == nil {
		dir = taskDir
	}
	cfg, err := config.FindConfig(dir)
	if err != nil {
		return nil
	}
	return cfg
}

// warnIfExposed says so when the board has just been bound somewhere other
// machines can reach it. There is no authentication in front of it, and a host
// committed to .taskqueue.yaml exposes every clone of the project, so this is
// worth a line rather than a surprise.
//
// On stderr, so it cannot be mistaken for part of the banner a script might be
// reading the address out of.
func warnIfExposed(w io.Writer, host string) {
	if !config.ExposedHost(host) {
		return
	}
	fmt.Fprintf(w, "warning: bound to %s, which is reachable from other machines, and the board has no authentication in front of it\n", host)
}

// serveDefaults is the address `tq serve` binds to before flags are parsed:
// the environment, then what the project pins, then the built-in. The flag
// layer sits on top of this by being its default value.
func (c *cli) serveDefaults() (host, port string) {
	host, port = defaultHost, defaultPort

	cfg := c.config()
	if pinned := cfg.ServerHost(); pinned != "" {
		host = pinned
	}
	// Not `if pinned != 0`: zero is a port, and it means "let the OS pick".
	if pinned, ok := cfg.ServerPort(); ok {
		port = strconv.Itoa(pinned)
	}
	return envOr("TQ_HOST", host), envOr("TQ_PORT", port)
}

// ── Commands ────────────────────────────────────────────────────

func (c *cli) runInit(args []string) int {
	fs := c.flagSet("init")
	jsonOut := fs.Bool("json", false, "print JSON output")
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}

	// Through c.st(), like every other command: init in a subdirectory adopts
	// the project's queue instead of forking a second one, and it reports what
	// the others report. Init's whole job is telling a caller where the queue
	// is, so it is the last command that should explain less than `tq list`
	// does in the same directory (TQ-0062).
	//
	// c.st() falls back to creating at the repository root when there is nothing
	// to find, and discovery stops there too (TQ-0017), so within a repository
	// init cannot adopt a queue from outside it. Without a repository root there
	// is no bound at all, which is what withinInvokedTree below answers for.
	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}

	// Only a task directory this project owns gets a marker and a guide.
	// Discovery has no bound in a project without a repository root, so the
	// store may belong to somebody else: writing a guide there would dirty
	// another repository's working tree, and writing a marker here would bind
	// this directory to their queue for good.
	var written []string
	if withinInvokedTree(st.Dir, c.dir) {
		// A queue that predates the marker gets one now: init is the command
		// that says "this project uses tq", and the file is what later
		// commands find.
		if st.ConfigWritten == "" {
			config, err := config.WriteConfigIfMissing(c.dir, st.Dir)
			if err != nil {
				return c.fail(err)
			}
			st.ConfigWritten = config
		}
		if st.ConfigWritten != "" {
			written = append(written, st.ConfigWritten)
		}

		guide, err := guide.SyncAgentsDocs(st)
		if err != nil {
			return c.fail(err)
		}
		written = append(written, guide...)
	} else if !*jsonOut {
		fmt.Fprintf(c.stderr, "note: %s was found above this directory; leaving it and its guide alone\n", st.Dir)
	}

	if *jsonOut {
		return c.printJSON(map[string]any{
			"task_dir": st.Dir,
			"created":  st.Created,
			"written":  written,
			"pointer":  guide.GuidePath(st),
		})
	}
	if st.Created {
		fmt.Fprintf(c.stdout, "Initialized task queue in %s\n", st.Dir)
	} else {
		fmt.Fprintf(c.stdout, "Task queue already initialized in %s\n", st.Dir)
	}
	for _, path := range written {
		fmt.Fprintf(c.stdout, "Wrote %s\n", path)
	}
	// tq does not edit the repository's own agent instructions, so it names the
	// guide and leaves the choice of file to the person running it. The path is
	// absolute because tq does not know which file they will reference it from,
	// and a relative one would only resolve from the directory tq guessed.
	fmt.Fprintf(c.stdout, "\nThe agent guide is at:\n\n    %s\n", guide.GuidePath(st))
	fmt.Fprint(c.stdout, "\nInclude it in your preferred agent context file (AGENTS.md, CLAUDE.md, or\n"+
		"whatever your tool reads) so agents pick up the task workflow.\n")
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
	priorities := c.priorities()
	priority := fs.String("priority", "",
		"priority: "+strings.Join(priorities.Names(), ", ")+" (default: "+priorities.Default()+")")
	assignee := fs.String("assignee", "", "assignee")
	body := fs.String("body", "", "Markdown body")
	board := c.board()
	status := fs.String("status", "",
		"initial status: "+strings.Join(board.Names(), ", ")+" (default: "+board.Default()+")")
	var labels, dependsOn stringList
	fs.Var(&labels, "label", "label (repeatable)")
	fs.Var(&dependsOn, "depends-on", "dependency task ID (repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON output")

	positional, code, ok := c.parse(fs, args, 1)
	if !ok {
		return code
	}

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Create(store.CreateTaskInput{
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
	board := c.board()
	if err := filter.Validate(c.priorities(), board); err != nil {
		return c.fail(err)
	}

	tasks, err := c.tasks()
	if err != nil {
		return c.fail(err)
	}
	tasks = task.FilterTasks(tasks, *filter, board)

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
	board := c.board()
	if err := filter.Validate(c.priorities(), board); err != nil {
		return c.fail(err)
	}

	tasks, err := c.tasks()
	if err != nil {
		return c.fail(err)
	}
	tasks = task.FilterTasks(tasks, *filter, board)

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

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Get(positional[0])
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

	// Resolved once: c.board() walks for the task directory and parses the
	// config, which is not something to do per dependency.
	board := c.board()

	out := make([]string, 0, len(t.DependsOn))
	for _, dep := range t.DependsOn {
		other, ok := index[dep]
		switch {
		case !ok:
			out = append(out, dep+" (missing)")
		case board.Satisfies(other.Status):
			out = append(out, fmt.Sprintf("%s (%s)", dep, other.Status))
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
	// Whichever column claims finished work; none or several is an error.
	target, err := c.board().SatisfyingColumn()
	if err != nil {
		return c.fail(err)
	}
	return c.moveTask(positional[0], target, *jsonOut)
}

// moveTask is the shared status transition behind `tq move` and `tq done`.
func (c *cli) moveTask(id, status string, jsonOut bool) int {
	board := c.board()
	if err := board.Check(status); err != nil {
		return c.fail(err)
	}
	status = board.Normalize(status)

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Get(id)
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
	t, err = st.Update(t)
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
	status := fs.String("status", "", "new status: "+strings.Join(c.board().Names(), ", "))
	priority := fs.String("priority", "", "new priority: "+strings.Join(c.priorities().Names(), ", "))
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

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Patch(positional[0], patch)
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

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Note(positional[0], text)
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
		return c.printJSON(map[string]string{"version": c.version})
	}
	fmt.Fprintf(c.stdout, "tq %s\n", c.version)
	return exitOK
}

// ── Helpers ─────────────────────────────────────────────────────

func (c *cli) st() (*store.Store, error) {
	st, err := store.OpenStore(c.dir)
	if err != nil {
		return nil, err
	}
	if st.Created {
		// stderr, so --json output stays machine-readable.
		fmt.Fprintf(c.stderr, "note: created %s\n", st.Dir)
		if marker, ok := store.ShadowedProjectMarker(c.dir); ok {
			fmt.Fprintf(c.stderr, "note: the project marker %s is above this repository and was not used; set %s=true to search past the repository root\n", marker, config.EnvWalkForever)
		}
	}
	return st, nil
}

// tasks lists the queue and names on stderr every file the scan had to skip,
// so a broken file is impossible to miss without it hiding the healthy tasks.
func (c *cli) tasks() ([]task.Task, error) {
	st, err := c.st()
	if err != nil {
		return nil, err
	}
	listing, err := st.List()
	if err != nil {
		return nil, err
	}
	c.warnListing(listing)
	return listing.Tasks, nil
}

// warnListing reports what a scan could not do: the files it had to skip, the
// IDs it could not tell apart, and a directory that changed under every
// attempt to read it, which leaves the listing possibly a task short
// (TQ-0012).
//
// On stderr, always: the listing itself is the answer a caller parses, and a
// warning on stdout would break --json for the agents that read it. A warning
// and not a failure, too — the tasks it did read are still the answer, and the
// exit code stays 0.
func (c *cli) warnListing(l store.Listing) {
	for _, f := range l.Unreadable {
		fmt.Fprintf(c.stderr, "warning: skipped %s: %s\n", f.File, f.Reason)
	}
	// Neither copy is in the listing, so say the ID is missing from it before
	// the reason it is: the reason is the same sentence a lookup of that ID
	// refuses with, and by itself it would not explain a short list (TQ-0040).
	for _, d := range l.Duplicated {
		fmt.Fprintf(c.stderr, "warning: not listed: %s\n", d.Reason)
	}
	if l.Incomplete {
		fmt.Fprintln(c.stderr, "warning: the task directory kept changing while it was read; this listing may be missing a task")
	}
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
		fs.StringVar(&f.Status, "status", "", "filter by status: "+strings.Join(c.board().Names(), ", "))
	}
	fs.StringVar(&f.Priority, "priority", "", "filter by priority: "+strings.Join(c.priorities().Names(), ", "))
	fs.StringVar(&f.Label, "label", "", "filter by label")
	fs.StringVar(&f.Assignee, "assignee", "", "filter by assignee")
	return &f, fs.Bool("json", false, "print JSON output")
}

// parse accepts flags and positional arguments in any order (`tq add "title"
// --priority high` reads naturally) and enforces the expected argument count.
// ok is false when the command must stop — help was printed, parsing failed, or
// the argument count is wrong — and code is what tq should exit with.
func (c *cli) parse(fs *flag.FlagSet, args []string, want int) (positional []string, code int, ok bool) {
	// Everything after a "--" is an argument, whatever it looks like. Split it
	// off before parsing: the loop below re-feeds what flag.Parse hands back,
	// which would re-enable flag parsing and leave the guarantee true only for
	// the first argument after the terminator.
	var terminated []string
	for i, arg := range args {
		if arg == "--" {
			terminated = args[i+1:]
			args = args[:i]
			break
		}
	}

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
	positional = append(positional, terminated...)

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
	case errors.Is(err, store.ErrTaskNotFound):
		return exitTaskNotFound
	case errors.Is(err, store.ErrProjectNotFound):
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

const (
	defaultHost = "127.0.0.1"
	defaultPort = "7331"
)

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func (c *cli) runServe(args []string) int {
	fs := c.flagSet("serve")
	boundHost, boundPort := c.serveDefaults()
	host := fs.String("host", boundHost, "host to bind to")
	port := fs.String("port", boundPort, "port to listen on")
	if _, code, ok := c.parse(fs, args, 0); !ok {
		return code
	}

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}

	dev := os.Getenv("DEV") != ""
	handler, err := web.NewRouter(st, dev, c.version)
	if err != nil {
		return c.fail(err)
	}
	// Ends the event streams and the scan behind them. Deferred for the paths
	// that never reach the signal handler below, and idempotent so both can
	// call it.
	defer func() { _ = handler.Close() }()

	addr := net.JoinHostPort(*host, *port)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           web.RequestLogger(c.stderr, handler),
		ReadHeaderTimeout: 10 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return c.fail(err)
	}

	// Before the banner, not after: the banner is what tells a caller the
	// server is up, and a signal arriving between the two would meet Go's
	// default disposition and kill the process rather than shut it down.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	// The listener's address, not the requested one: --port 0 asks the OS to
	// pick, and the caller has no other way to learn what it picked.
	fmt.Fprintf(c.stdout, "Serving %s on http://%s\n", st.Dir, listener.Addr())
	warnIfExposed(c.stderr, *host)
	if dev {
		fmt.Fprintf(c.stdout, "DEV mode: frontend served from ./%s\n", web.DevDir)
	}

	// Ctrl-C should leave no half-served requests behind.
	shutdown := make(chan struct{})
	go func() {
		<-signals
		// The event streams close first. Shutdown waits for handlers to
		// return, and a stream is a request that never finishes on its own, so
		// leaving them open would hold shutdown until the timeout expired.
		_ = handler.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			fmt.Fprintf(c.stderr, "shutdown: %v\n", err)
		}
		close(shutdown)
	}()

	if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return c.fail(err)
	}
	<-shutdown
	return exitOK
}
