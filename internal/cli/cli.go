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

	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	dir    string // working directory used to discover the task directory
}

// Run executes one command and returns the process exit code. It takes its
// streams, its working directory and the binary's version rather than reaching
// for globals, so a test can drive it the same way the binary does.
func Run(stdin io.Reader, stdout, stderr io.Writer, dir, version string, args []string) int {
	return runCLI(&cli{stdin: stdin, stdout: stdout, stderr: stderr, dir: dir, version: version}, args)
}

// writeCommands are the commands that put a task file on disk. An interrupt
// arriving during one of them is held until it returns (see holdSignals). The
// others have nothing to protect, and serve installs a handler of its own,
// because it is the one command whose whole job happens after the signal.
var writeCommands = map[string]bool{
	"init": true, "add": true, "move": true, "done": true, "update": true, "note": true,
}

func runCLI(c *cli, args []string) int {
	if len(args) == 0 {
		c.usage(c.stderr)
		return exitError
	}
	if writeCommands[args[0]] {
		// Standard input is read here rather than inside the command, because
		// inside is inside the hold: a read from a pipe nobody closes never
		// returns, and holdSignals turns a command that cannot return into one
		// only SIGKILL ends. Its whole trade rests on these commands taking
		// milliseconds, which a read from a terminal does not.
		if stdinRequested(args) {
			text, err := c.readStdin()
			if err != nil {
				return c.fail(err)
			}
			c.stdin = strings.NewReader(text)
		}
		return holdSignals(c.stderr, func() int { return dispatch(c, args) })
	}
	return dispatch(c, args)
}

// stdinRequested reports whether these arguments ask for a value to be read
// from standard input. It is a scan and not a parse, and it only has to be
// right about one thing: whether the command below is going to block on a read.
// A false positive consumes a standard input nothing wanted, on a command that
// then fails on its own arguments; a false negative puts the read back inside
// the hold.
func stdinRequested(args []string) bool {
	for i, arg := range args[:len(args)-1] {
		if (arg == "-body" || arg == "--body") && args[i+1] == stdinArg {
			return true
		}
	}
	return false
}

// holdSignals runs a command with SIGINT and SIGTERM held: registering a
// channel for a signal is what takes the process off Go's default disposition,
// so a Ctrl-C part-way through a save waits for the save to return instead of
// killing the process between two of its steps.
//
// The exit code is the command's own. A save that finished before the signal
// was acted on is a save that landed, and saying otherwise would put a failure
// on stderr and a nonzero code on a file that is on disk — the shape of report
// TQ-0015 is about. The line only says the interrupt was seen, and only for a
// command that succeeded: one that failed has said why already, and telling a
// caller nothing was left half-written reads as a claim about a write.
//
// Nothing here is a teardown, and holding is all it does: a command that cannot
// return — `.tasks` on a stalled network mount is the case — holds the signal
// with it, and SIGKILL is then the only way out. That is the trade for the
// cheap version. These commands are one move each and return in milliseconds
// when the filesystem answers, and it is the move itself that makes an
// interrupt survivable (TQ-0015); this only keeps one from landing mid-move.
func holdSignals(stderr io.Writer, run func() int) int {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	code := run()
	select {
	case <-signals:
		if code == exitOK {
			fmt.Fprintln(stderr, "interrupted: the command had already finished, so nothing was left half-written")
		}
	default:
	}
	return code
}

func dispatch(c *cli, args []string) int {
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
  init                            Create the queue in the current directory:
                                  %s, the %s directory, and the
                                  agent guide (%s); prints how to
                                  start the board and where the guide is, to
                                  reference from your own AGENTS.md/CLAUDE.md
  add <title> [flags]             Create a task
  list [flags]                    List tasks
  show <id> [--json]              Show one task
  move <id> <status>              Change a task's status
  done <id>                       Shorthand for: move <id> done
  update <id> [flags]             Change fields of a task, its body included
  note <id> <text>                Append a timestamped note to a task
  ready [flags]                   List tasks that are unblocked and unclaimed
  label list [--json]             Print the project's label vocabulary and the
                                  labels in use that it does not declare
  serve [flags]                   Serve the Kanban board and REST API
  version                         Print the version
  help                            Print this help

Statuses:   %s
Priorities: %s (most severe first; default: %s)

Task IDs:
  <id> is TQ-0028, and the number on its own says the same thing: tq show 28.
  The prefix is optional and case does not matter, on an argument and on a
  dependency flag alike.

Common flags:
  --json                          Print JSON to stdout and nothing else
  --status, --priority, --label, --assignee   Filters for list/ready
  --body <text>                   The Markdown body. On add it is the whole
                                  body; on update the task's notes are kept and
                                  a notes section in the text is ignored, so a
                                  body read with "tq show" can be edited and
                                  handed straight back. "-" reads the text from
                                  stdin, which is how a document should arrive
  --                              End of flags: everything after it is an
                                  argument, even if it starts with "-"

Server address (tq serve), highest precedence first:
  --host, --port                  Flags
  TQ_HOST, TQ_PORT                Environment
  server.host, server.port        In %s, committed with the project
  %-30s  Built in

  A committed host that is not loopback exposes the board on the network of
  everyone who clones the project; tq says so on start-up when it happens.

Where the queue lives:
  tq init creates %s and the task directory in the directory it is
  run in, and nowhere else: no search, no adopting a project above, no
  relocating to a repository root. Run it where you want the queue.

  Every other command walks up from the current directory looking for
  %s, stopping at your home directory, and uses the nearest one it
  finds. From a directory outside your home directory there is no home to stop
  at, so the walk runs to the filesystem root. With no marker at all it exits 3
  and says to run tq init; it never creates a queue itself.

  Once a command has the marker, that file is the whole answer: the board, both
  vocabularies, and where the tasks are. Nothing goes the other way, so its
  path may point outside the marker's own directory.

Environment:
  %s     The marker to use instead of walking for one, for every
                     command, tq init included. It names a %s
                     file, not a directory; the tasks are wherever its path says
  DEV                Serve frontend assets from disk instead of the embedded copy

Exit codes:
  0 success   1 general/validation error   2 task not found
  3 no task queue found (run tq init)
`

func (c *cli) usage(w io.Writer) {
	priorities := c.priorities()
	fmt.Fprintf(w, usageText,
		config.ConfigFileName, config.TaskDirName, filepath.Join(config.TaskDirName, guide.AgentsFileName),
		strings.Join(c.board().Names(), ", "),
		strings.Join(priorities.Names(), ", "), priorities.Default(),
		config.ConfigFileName, defaultHost+", "+defaultPort,
		config.ConfigFileName, config.ConfigFileName,
		config.EnvConfigPath, config.ConfigFileName)
}

// priorities is the vocabulary for help text and for the filters, which are
// both needed before a command has opened a store. It comes from the project's
// marker, which is the same file the store validates a write against.
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

// config is the project's config: the nearest marker at or above the directory
// the command was run in. That is the same file a store opened here resolves
// through, so the CLI's help and filters cannot disagree with the writes that
// follow them — and it holds before a store exists, which is the case this is
// for, a project whose task directory has not been created yet.
//
// TQ_CONFIG_PATH is honoured here as everywhere, because what it hands over is
// a marker: the same file a store opened here would resolve through, so the
// variable cannot put the help and the writes on two different projects
// (TQ-0087).
//
// A config that cannot be read reads as none. Every caller here wants a default
// rather than a failure — help must print, and a command that goes on to touch
// the store reports the broken config itself, with the file named.
func (c *cli) config() *config.Config {
	cfg, err := config.Optional(config.FindConfig(c.dir))
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

	// Not through c.st(): init is the one command that does not walk. It
	// creates the project in the directory it was run in, so a marker above
	// cannot adopt this directory and a repository root cannot relocate it —
	// where the queue lives is the caller's decision, expressed by where they
	// stood when they ran this (TQ-0085). TQ_CONFIG_PATH is the one thing that
	// answers instead, because it is the caller saying it outright: init then
	// creates what that marker declares and writes no marker of its own.
	st, err := store.InitStore(c.dir)
	if err != nil {
		return c.fail(err)
	}
	st.Announce = c.announceReconciliation

	var written []string
	if st.ConfigWritten != "" {
		written = append(written, st.ConfigWritten)
	}
	guides, err := guide.SyncAgentsDocs(st)
	if err != nil {
		return c.fail(err)
	}
	written = append(written, guides...)

	// Init is where a board change lands. Editing the columns in
	// .taskqueue.yaml and running init again is the documented way to change
	// one, and this is what makes that true: every task left in a column the
	// file no longer declares is moved to the default column, and named on
	// stderr (TQ-0088).
	//
	// After the guide, because a marker that will not parse fails there first
	// and with the file named; reconciling ahead of it would report the same
	// broken config twice. What it cannot write it warns about and does not
	// fail on, the way every other command treats one.
	st.Reconcile()

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
	// The queue is empty and the next thing to do is work one task through it,
	// so init says what that is rather than leaving it to a README the reader
	// may not have (TQ-0102). Through the store's board, so a project that
	// renamed its columns is not handed commands its own config refuses.
	if board, err := st.Columns(); err == nil {
		c.printNextSteps(board)
	}
	// tq does not edit the repository's own agent instructions, so it names the
	// guide and leaves the choice of file to the person running it. The path is
	// absolute because tq does not know which file they will reference it from,
	// and a relative one would only resolve from the directory tq guessed.
	fmt.Fprintf(c.stdout, "\nStart the board with:\n\n    tq serve\n\n")
	fmt.Fprintf(c.stdout, "It listens on http://%s:%s by default (localhost only).\n", defaultHost, defaultPort)
	fmt.Fprintf(c.stdout, "\nThe agent guide is at:\n\n    %s\n", guide.GuidePath(st))
	fmt.Fprint(c.stdout, "\nInclude it in your preferred agent context file (AGENTS.md, CLAUDE.md, or\n"+
		"whatever your tool reads) so agents pick up the task workflow.\n")
	return exitOK
}

// printNextSteps is the loop a queue is worked in, written out with the
// project's own column names: what a task is filed in, what queues it for `tq
// ready`, and what closes it. Only what the board can answer is spelled as a
// command — there is no flag saying which column holds claimed work, so
// claiming is named in the sentence below rather than guessed at.
//
// Init is idempotent and is run again after every config edit, so this stays
// short: someone settling a board change reads past it every time.
func (c *cli) printNextSteps(board task.Columns) {
	// The ID a fresh queue hands out first, so the lines can be pasted in
	// order on the queue init has just created.
	exampleID := task.FormatID(1)

	entry := board.Default()
	var queue string
	for _, name := range board.Names() {
		if board.Offers(name) {
			queue = name
			break
		}
	}
	// A board with no `consider_done` column, or several, has nothing for
	// `tq done` to mean — the same error the command itself gives — so the
	// step is left out rather than printed as advice that fails.
	finish, err := board.SatisfyingColumn()
	if err != nil {
		finish = ""
	}

	filed := fmt.Sprintf("files it in %s", entry)
	if queue == entry {
		filed = fmt.Sprintf("files it in %s, where tq ready offers it", entry)
	}
	steps := [][2]string{{`tq add "Your first task"`, filed}}
	if queue != "" && queue != entry {
		steps = append(steps, [2]string{
			fmt.Sprintf("tq move %s %s", exampleID, queue),
			"queues it, so tq ready offers it",
		})
	}
	steps = append(steps, [2]string{
		fmt.Sprintf(`tq note %s "what happened"`, exampleID),
		"appends a timestamped note as you go",
	})
	if finish != "" {
		steps = append(steps, [2]string{
			fmt.Sprintf("tq done %s", exampleID),
			"closes it once the work is verified",
		})
	}

	width := 0
	for _, step := range steps {
		width = max(width, len(step[0]))
	}
	fmt.Fprint(c.stdout, "\nNext steps:\n\n")
	for _, step := range steps {
		fmt.Fprintf(c.stdout, "    %-*s  %s\n", width, step[0], step[1])
	}
	fmt.Fprint(c.stdout, "\ntq ready lists the work that is unblocked and unclaimed; claim one with\n"+
		"tq move <id> <status> before the first edit. The columns are:\n\n")
	fmt.Fprintf(c.stdout, "    %s\n", strings.Join(board.Names(), ", "))
}

func (c *cli) runAdd(args []string) int {
	fs := c.flagSet("add")
	priorities := c.priorities()
	priority := fs.String("priority", "",
		"priority: "+strings.Join(priorities.Names(), ", ")+" (default: "+priorities.Default()+")")
	assignee := fs.String("assignee", "", "assignee")
	body := fs.String("body", "", `Markdown body ("`+stdinArg+`" reads stdin)`)
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

	// `--body -` means the same thing here as it does on update: the document
	// arrives on stdin rather than as an argument.
	if *body == stdinArg {
		text, err := c.readStdin()
		if err != nil {
			return c.fail(err)
		}
		*body = text
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
		DependsOn: taskIDs(dependsOn),
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

// taskID is every ID this package takes from a person, expanded from the
// shorthand they may have typed: `tq show 28` means TQ-0028. Expanding happens
// here and nowhere deeper — the store looks a task up by the exact ID a file
// carries, and the board and the API deal in IDs they were given by tq
// (TQ-0071).
func taskID(arg string) string { return task.NormalizeID(arg) }

// taskIDs is taskID over a repeatable flag: a dependency is typed the same way
// an argument is, and it is written to a file, so it is expanded before it
// gets there rather than being refused as an invalid dependency.
func taskIDs(args []string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = taskID(arg)
	}
	return out
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
	t, err := st.Get(taskID(positional[0]))
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
	return c.moveTask(taskID(positional[0]), positional[1], *jsonOut)
}

func (c *cli) runDone(args []string) int {
	fs := c.flagSet("done")
	jsonOut := fs.Bool("json", false, "print JSON output")
	positional, code, ok := c.parse(fs, args, 1)
	if !ok {
		return code
	}
	// Whichever column claims finished work; none or several is an error. Read
	// through the store, so it is the marker the queue was resolved through
	// that answers rather than a second look at the project.
	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	board, err := st.Columns()
	if err != nil {
		return c.fail(err)
	}
	target, err := board.SatisfyingColumn()
	if err != nil {
		return c.fail(err)
	}
	return c.moveTask(taskID(positional[0]), target, *jsonOut)
}

// moveTask is the shared status transition behind `tq move` and `tq done`.
//
// The board comes from the store, which is to say from the marker this queue
// was resolved through — the same board the write is validated against a moment
// later. Checking against a separately resolved one told a project whose marker
// tq could not read that its own columns did not exist, while `tq list` in the
// same directory named the broken file (TQ-0087).
func (c *cli) moveTask(id, status string, jsonOut bool) int {
	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	board, err := st.Columns()
	if err != nil {
		return c.fail(err)
	}
	if err := board.Check(status); err != nil {
		return c.fail(err)
	}
	status = board.Normalize(status)

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
	body := fs.String("body", "", `new body; the task's notes are kept, and a notes `+
		`section in the text is ignored ("`+stdinArg+`" reads stdin)`)
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
		AddDeps:      taskIDs(addDeps),
		RemoveDeps:   taskIDs(removeDeps),
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
		case "body":
			// Content, not Body: --body replaces the document and leaves the
			// notes where they are (TQ-0044).
			patch.Content = body
		}
	})
	if patch.Content != nil && *patch.Content == stdinArg {
		text, err := c.readStdin()
		if err != nil {
			return c.fail(err)
		}
		patch.Content = &text
	}
	if patch.IsEmpty() {
		return c.fail(errors.New("update needs at least one field to change (see `tq help`)"))
	}

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Patch(taskID(positional[0]), patch)
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

	// Trimmed to answer "is there a note here at all", and not otherwise: the
	// indent a pasted block shares is what makes it one block, and trimming it
	// off the first line alone is what used to leave the rest reading as an
	// indented code block (TQ-0054). task.AppendNote normalises the text it is
	// given, surrounding blank lines included.
	if strings.TrimSpace(positional[1]) == "" {
		return c.fail(errors.New("note text cannot be empty"))
	}

	st, err := c.st()
	if err != nil {
		return c.fail(err)
	}
	t, err := st.Note(taskID(positional[0]), positional[1])
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

// st is the queue the command works on: the project at or above the working
// directory. Every command but `tq init` goes through it, and none of them
// creates anything — a directory with no project above it fails here, with a
// message naming `tq init`.
func (c *cli) st() (*store.Store, error) {
	st, err := store.OpenStore(c.dir)
	if err != nil {
		return nil, err
	}
	st.Announce = c.announceReconciliation
	return st, nil
}

// announceReconciliation says what a reconciliation did: the tasks it refiled,
// and the ones it could not. Any command can trigger one — a board column
// removed from `.taskqueue.yaml` strands every task still in it, and the first
// command to notice moves them all — so this is wired to every store the CLI
// opens rather than to the commands that expect it.
//
// On stderr, and the exit code stays 0, the way a skipped task file is reported
// (see warnListing). The tasks the command was asked for are still its answer,
// and a queue that could not be written is still one to read; what must not
// happen is a task changing column, or failing to, with nothing said (TQ-0088).
func (c *cli) announceReconciliation(done store.Reconciliation) {
	for _, group := range groupMoves(done.Moved) {
		fmt.Fprintf(c.stderr, "warning: %s, so %s moved to %q\n",
			group.reason, strings.Join(group.ids, ", "), group.to)
	}
	if done.Unfinished == nil {
		return
	}
	// One line per task that could not be moved: errors.Join separates them by
	// newline, and every warning here is a line somebody reads on its own.
	for _, line := range strings.Split(done.Unfinished.Error(), "\n") {
		fmt.Fprintf(c.stderr, "warning: %s\n", line)
	}
}

// moveGroup is every task that came out of one column, so a board edited under
// fifty tasks is one line and not fifty.
type moveGroup struct {
	reason string
	to     string
	ids    []string
}

// groupMoves gathers moves by where they came from and where they went, keeping
// the order the reconciliation made them in.
func groupMoves(moves []store.Move) []moveGroup {
	var groups []moveGroup
	seen := map[string]int{}
	for _, m := range moves {
		key := m.From + "\x00" + m.To
		at, ok := seen[key]
		if !ok {
			at = len(groups)
			seen[key] = at
			groups = append(groups, moveGroup{reason: m.Reason, to: m.To})
		}
		groups[at].ids = append(groups[at].ids, m.ID)
	}
	return groups
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

// stdinArg is the flag value that means "the text is on stdin". A body is a
// document, and a shell has better ways to hand one over — a heredoc, a pipe,
// a file — than a quoted argument that has to survive its own escaping.
//
// The cost is that a body of exactly "-" cannot be given as an argument, which
// is the convention every tool with this affordance already carries; echo it in
// instead.
const stdinArg = "-"

// readStdin reads a flag's value from standard input. An empty stdin is a value
// like any other — it is how `tq update --body ""` is spelled through a pipe —
// so only a read that fails is an error.
func (c *cli) readStdin() (string, error) {
	if c.stdin == nil {
		return "", errors.New("nothing to read: this command has no standard input")
	}
	text, err := io.ReadAll(c.stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	return string(text), nil
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
