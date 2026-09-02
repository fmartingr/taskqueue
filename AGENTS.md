# AGENTS.md — tq

## Project summary

`tq` is a local-first task queue for agentic development. Each task is a
Markdown file with YAML frontmatter in `.tasks/`. A CLI serves agents, a Kanban
board serves humans, and both go through the same Go store, so the Markdown
files are the only source of truth.

## Tech stack

- **Backend**: Go (standard library as much as possible)
- **Frontend**: Vue 3 single-file components + TypeScript + CSS, bundled by Bun
  and embedded via `go:embed`. Vue is bundled into the output, so the shipped
  page still loads one script and fetches nothing: no CDN, no Node runtime, no
  bundler but Bun
- **Layout**: one `main` in `cmd/tq`, everything else under `internal/`, by
  responsibility and layered leaf-first

```text
cmd/tq/main.go     The binary: `go install` names it after this directory
tq.go              Main() and the version variable
internal/task/     Model, validation, filters, dependencies, notes, frontmatter.
                   Imports nothing of ours.
internal/config/   .taskqueue.yaml: the project marker, its loader, the walk,
                   the board's columns and the label and priority vocabularies
internal/store/    Filesystem store: discovery, atomic writes, ID allocation
internal/guide/    AGENTS.tmpl.md; `tq init` writes .tasks/AGENTS.md
internal/web/      REST API, the server, and public/ (embedded frontend)
internal/cli/      Commands, flags, human/JSON output, exit codes
internal/fsx/      Atomic file write, shared by the two generators
internal/tqtest/   Test fixtures shared across packages
internal/integration/ Tests that drive the compiled binary (build tag)
frontend/          main.ts, components/ (.vue), state.ts, api.ts, board.ts,
                   notes.ts, edit.ts, markdown.ts, search.ts, format.ts,
                   index.html, style.css, build.ts (Bun)
browser/           Tests that drive the board in a real Chromium (bun test)
```

Dependencies run one way: `task` <- `config` <- `store` <- `guide`, `web`, `cli`.

## Commands

```bash
make test           # run after backend changes
make typecheck      # vue-tsc: .ts and the templates inside .vue, in one run
make test-integration # drives the compiled binary; slower, tagged
make test-frontend  # Bun unit tests for the pure frontend helpers
make test-browser   # drives the board in a real browser; needs Chromium
make frontend       # run after frontend changes (updates public/, which is committed)
make build          # run before completing a task
make lint           # golangci-lint
make format         # go fmt + go mod tidy
make dev            # Bun watch + DEV=1 server
```

The frontend targets need `node_modules`: Vue and the SFC loader are build-time
dependencies, so a clean checkout needs `bun install` once before `make
frontend`, `make typecheck` or `make dev` will run.

## Rules

- Run `make test` after backend changes.
- Run `make typecheck` and `make frontend` after frontend changes, and commit
  the `public/` output; add `make test-frontend` when the change touches logic
  that is unit-tested (the pure helpers in `frontend/`, currently `notes.ts`,
  `board.ts`, `edit.ts`, `markdown.ts`, `search.ts` and `format.ts`). `bun build` strips types without checking them, so nothing else
  in the pipeline sees a type error.
- Run `make test-browser` after changes to the components, the board's markup or
  its styles. It needs a Chromium: `make browser-install` puts one in the cache.
- Run `make build` before completing a task.
- Do not add a database, an index file, a cache or a filesystem watcher. Markdown
  files are the source of truth and every read hits the disk — that is what makes
  CLI edits visible to a running server.
- Allocating an ID reads the task *files*, not just their names: the number is
  one past the highest any directory entry answering to a task file's name
  claims — a directory included, since a create could never link that name
  (TQ-0039) — advanced again past any number a task still lists in
  `depends_on`. Removal is always a raw file operation — there is no `tq delete`
  — so removing the newest task frees the highest number, and
  handing it straight to the next create binds every dangling dependency to an
  unrelated new task (TQ-0016). Do not replace the skip with a high-water mark
  or any other persisted counter: that is the index file the rule above forbids,
  and two branches would each bump it and merge back to the same number. A
  create therefore costs a pass over the queue, which is the accepted price —
  creates are rare, and every listing already reads every file. A number nothing
  references is still recycled, deliberately: there is no stale pointer for it
  to re-bind to.
- A listing reads the task directory twice: once to learn which files to open,
  and once afterwards to check that the set of them did not change while they
  were being opened. Reading the names and then the files is a TOCTOU, and a
  retitle landing in between leaves a task under a name the pass never looked
  at, so a difference between the two readings means the pass is redone — three
  attempts, after which the listing says it may not match the directory rather
  than passing for the whole queue (TQ-0012). An ID under two names is the same
  signal from the other side; a pair two passes at rest both find is a queue to
  fix rather than a race, and is reported and withheld instead (TQ-0040). Keep
  both readings: this is a consistency check, not a cache, and nothing survives
  the call.
- A save *moves* the task's file to the name its title asks for, and only then
  writes the new content into it. Never the other way round: writing the new
  name and removing the old one lets a second writer put its own copy back at
  the name the first has just removed, and an ID two files claim is one
  `locate` refuses for good (TQ-0015). A move is atomic and takes the old name
  with it, so there is nothing left to remove and nothing left to fail once the
  content is on disk — a save that reports failure has written no new content,
  and at worst has moved the file to the name it was going to use. The
  loser of a race gets `ENOENT` from its move, which is it learning where the
  task went; it locates the task again and moves that instead. This is not a
  lock and is not meant to be one: two processes share none, the last one still
  wins, and the content write is itself a rename, which creates its destination
  — so a save can still land at a name a losing writer freed a microsecond
  earlier and leave two files: two 200-trial rounds of two concurrent
  `tq update` on one ID left 44 and 51, against 62 before the change. Closing
  that needs an exchange syscall
  the standard library does not expose, declined for its portability cost; the
  notes on TQ-0015 carry the reasoning, and TQ-0040 is what keeps the residue
  from being silent.
- The one thing the server keeps between requests are the two change
  fingerprints behind `/api/events` (TQ-0033): the names, sizes and modification
  times of the task directory, hashed, and the same reading of `.taskqueue.yaml`
  — compared twice a second while a board is connected. Two rather than one
  because they push different frames, `tasks` and `config`, and the board
  refetches a different endpoint for each (TQ-0034). Neither holds any data, so
  a board still reads its tasks and its configuration through the same endpoints
  and the same store as before — and one scan serves every connected board,
  where polling made a second board cost twice the disk reads. It is not a
  watcher: nothing subscribes to the filesystem, and there is no dependency. The
  config reading is a stat, never a parse: a file caught half-saved has to
  register as a change like any other, and the board is what decides what to do
  about one it cannot use.
- Do not introduce a second frontend framework, a bundler other than Bun, or a
  Node runtime without an explicit architecture change. Vue (TQ-0076) and
  markdown-it (TQ-0103) are the two exceptions, each bundled into
  `public/app.js` rather than fetched: what must not appear is a dependency the
  built output *relies on* at runtime — a CDN, an import map, anything under
  `node_modules/` served as-is.
  A package used only by tests or by the toolchain is a different question — a
  browser driver is a reasonable answer to one, and `typescript` to the other.
- No `<style>` block in an SFC. Bun emits it as a second entry output, which
  wants the same fixed `app.js` name `build.ts` pins for the bundle, and the
  build aborts with "Multiple files share the same output path".
  `frontend/style.css` is the stylesheet, and it is copied, not bundled.
- `typescript` and `vue-tsc` are pinned exactly, not floated. TypeScript 7 is
  the Go-native port and ships no programmatic API, so the Volar-based checkers
  that read Vue/Svelte/Astro templates cannot run on it
  (`ERR_PACKAGE_PATH_NOT_EXPORTED`); 6.0.3 is the newest JavaScript-implemented
  release. Upstream content mappers targeted at TypeScript 7.1 are the condition
  for lifting the pin (see TQ-0076).
- The task dialog holds no draft of its task, and writes one field at a time
  (TQ-0069). Every control is drawn from the task the board last read, so the
  whole dialog follows the file and there is nothing local for an incoming
  change to overwrite; the exception is the one editor the user has open. A
  field is written when its editor closes, as a partial patch of that field
  alone — which is what makes the first half safe, since a save of every field
  is only correct if the dialog holds a current copy of every field. Do not
  bring back a Save that patches the whole task: it is what wrote a snapshot
  taken at open time back over an agent's edit (TQ-0010, TQ-0079), and the
  adoption rules that grew to contain it (TQ-0084) went with it.
- The task dialog fills the window bar a margin, and closes on a click in that
  margin. Two things hold that up and neither is cosmetic: the element carries
  no padding of its own and everything it draws sits in one child that fills
  it, because a `<dialog>`'s backdrop belongs to the element — so "the target
  was the dialog" is only the same thing as "outside the sheet" while nothing
  else can be. And a click outside is ignored while an editor is open, because
  losing focus writes, and that write can be refused: closing on the same click
  would throw away the text a refusal exists to preserve, before the write had
  answered. The mousedown is checked too, since a `click` is dispatched at the
  common ancestor of its mousedown and mouseup and a selection released past
  the edge of the sheet is otherwise indistinguishable from a click on the
  backdrop.
- A write whose field moved on disk since its editor opened is **refused**, not
  merged: nothing is written at all, the dialog names the field, and the user's
  text stays on screen for them to copy. Settling the two versions is theirs to
  do in the VCS. The one thing this is careful about is that a body is two
  fields in one string — `tq note` appending a note is not a change to the
  paragraph somebody is rewriting, so `commitContent` and `commitNote` in
  `frontend/edit.ts` compare only their own half and rebuild the body around
  whatever the file holds for the other.
- `frontend/markdown.ts` is the only thing in the board that builds HTML, and
  markdown-it is what it builds it with (TQ-0103). The dialect is GitHub
  Flavored Markdown, because the same body is edited in a GitHub editor as
  often as here, and agreeing with GFM on tables, autolinks, escapes,
  references and loose lists is a parser's job rather than a regular
  expression's. It replaced a hand-written renderer, and cost the committed
  `app.js` 190KB unminified — that is the price of the dialect, paid once and
  visible in every rebuild of that file. It is pinned exactly, like `vue`: the
  bundle is committed, so a floating range would rewrite that file on somebody
  else's machine with no change to any source here.
  Two options are load-bearing and are not style. `html: false` is what makes
  raw HTML in a body *text*, so there is no sanitiser after the renderer and
  nothing left from the file to sanitise; GFM would pass a limited set of tags
  through, and this deliberately does not. `breaks: true` renders a soft line
  break as a `<br>` where GFM folds it into a space, because task bodies are
  hand-wrapped findings as often as they are prose.
  Four things are extended rather than configured, and each carries its own
  tests: `validateLink` is replaced outright, because markdown-it's own check
  is a deny-list and a scheme has to be on an allow-list or the link stays the
  text it was written as; a task list item draws its checkbox through a token
  of ours, since GFM has task lists and markdown-it does not; a table is
  wrapped in a box that scrolls, so a wide one cannot widen the dialog; and
  every link leaves this tab. `linkify` needs `fuzzyLink` turned on for the
  bare `www.` host GFM links and markdown-it does not.
- Do not have the HTTP layer shell out to the `tq` binary. Both surfaces call the
  same store functions.
- `.tasks/AGENTS.md` is generated by `tq init` from `internal/guide/AGENTS.tmpl.md`:
  change the template, never the generated file. The template is not named
  AGENTS.md, so agents do not read the unfilled file as instructions.
- **Editing `.taskqueue.yaml` means running `tq init` again**, and the docs say
  so — README.md, this file and the generated guide. Init is idempotent, and it
  is where a board change lands: `Store.Reconcile` moves every task still filed
  in a column the file no longer declares to the board's **default** column, all
  of them in one pass, and names them through `Store.Announce` (the CLI writes
  that to stderr, so `--json` stdout is untouched). Any command that meets a
  stranded task reconciles too — `Store.List` after its scan, `Store.Get` when
  the task it read is one — which is the safety net for an edit made without
  running init, and what stops the queue ever being half-migrated. A read that
  writes is the accepted cost, stated deliberately: the alternative was
  rewriting a status for display and letting an unrelated `tq update --assignee`
  persist that for one task (TQ-0088).
- A reconciliation never fails a read. `Store.Reconcile` returns a
  `Reconciliation`, not an error: what it could not write goes in `Unfinished`,
  every pass with anything to say goes through `Announce`, and the tasks come
  back carrying what their files hold. A queue on a read-only checkout has to
  stay listable, and a pass that got partway has to be reported rather than
  swallowed. It also carries on past a write it was refused rather than stopping
  at the first, so it leaves as little behind as it can. The cost accepted here
  is that reconciling is a `locate` and a save per stranded task, under the
  store's lock — one pass over a queue after a config edit, and rare.
- Only reconciliation moves a task between columns. `Store.update` writes the
  status it was handed and does not consult the board, the same way it leaves
  the priority alone; `Columns.Check` guards the paths that *pick* a column
  (`Create`, `Patch`, `tq move`). `Columns.Normalize` resolves an alias and
  nothing else — it no longer answers for a status the board has no column for,
  because its old answer was the *first* column, and on a board that lists its
  `consider_done` column first that marked stranded tasks finished and unblocked
  their dependents. `Columns.Reconcile` is the rule, and its answer is
  `Default()`.
- Task files are named `<id>-<title-slug>.md`, but the ID in the frontmatter is
  what identifies a task: look tasks up by ID (`Store.locate`), never by
  reconstructing a filename from a title.
- The number alone is an ID on the command line — `tq show 28` is `tq show
  TQ-0028` — and `task.NormalizeID` is the one place that expansion happens,
  called at the CLI's boundary and nowhere deeper (TQ-0071). The store looks a
  task up by the exact ID a file carries, and the API and the board deal in IDs
  tq gave them, so nothing below the CLI has to know the shorthand exists. It
  leaves anything already shaped like an ID alone: `TQ-28` is a valid ID, and
  re-padding one somebody hand-wrote would send the lookup to a file that is not
  there. `task.FormatID` is the other half — the spelling `NextID` hands out —
  and the two are checked against each other, so every number tq writes is one
  the shorthand reaches.
- `.md`, lowercase, is the only extension a task file may have, and the rule is
  one-way: every path tq writes, renames, links or removes ends in a lowercase
  `.md`, and every path it matches must too (TQ-0039). Folding case in
  `taskFilePattern` was considered and declined — the store's view of a
  directory would then depend on whether the filesystem folds case, which APFS,
  ext4 and NTFS do not agree on. So a `TQ-0001-fix-bug.MD` arriving from
  outside is foreign: never read, never adopted, never renamed. It is not
  passed over in silence either, because on a case-insensitive filesystem it
  also occupies the name a task file wants. It goes out in
  `Listing.Unreadable`, the channel a file that will not parse already uses,
  naming the task file holding that ID when there is one — which is the second
  claim `duplicatedIDs` cannot see, since that reads the task files and this is
  not one. A write refused by such an entry names it: the entry is found by
  identity, not by name, because a filesystem that answers to a spelling it did
  not store will not say what it called it.
- A task file that will not parse is skipped and reported, never fatal to a
  listing: `Store.List` returns a `Listing` carrying the tasks it read and the
  files it could not (TQ-0011). `.tasks/` is committed, so a merge conflict in
  one file is ordinary — one of them must not hide the queue from both
  surfaces. Every caller has to render the skipped files: the CLI names them on
  stderr and still exits 0, and `GET /api/status` carries them in `unreadable`
  while `GET /api/tasks` stays a plain array of the tasks that read. Reading a
  single task by ID is unchanged and still fails loudly, because there the
  broken file is the answer.
  `Listing.Incomplete` travels the same way (TQ-0012): the CLI warns on stderr
  and still exits 0, `GET /api/status` carries `incomplete`, and the board says
  it in a toast and in its footer, exactly as it says a file was skipped.
- An ID more than one file claims is withheld from a listing, both copies, and
  reported in `Listing.Duplicated` — the same channel again (TQ-0040). An ID
  appears in a listing once or not at all, which is what lets every caller
  index by it. `List` tells that from a directory that held two for an instant
  by looking again: the pair is redone, and one that two passes at rest both
  find is reported rather than retried. A pair only ever seen while the
  directory was moving is withheld too — the ID rule is
  absolute — but the listing says `Incomplete` about it rather than naming
  files to delete. The sentence it is reported with is `locate`'s, composed in
  `duplicateClaim` so that a listing and a refused write cannot say different
  things about the same two files.
- Preserve JSON CLI output compatibility; it is the stable agent API.
- Keep stdout clean when `--json` is active: data on stdout, everything else on
  stderr.
- Keep exit codes stable: 0 success, 1 general/validation, 2 task not found,
  3 no task queue found. 3 kept its number through TQ-0085 and had its meaning
  restated rather than split: it has always meant "tq could not reach a usable
  queue", and "no project at or above here" is now the ordinary way to get
  there rather than the edge case. A fourth code would have broken every script
  already treating 3 as "no queue". Every message behind it names `tq init`.
- Discovery is two rules and nothing else (TQ-0085), with `TQ_CONFIG_PATH`
  standing in for the walk when it is set (see below). `tq init` creates the
  queue in the directory it is run in — the marker, the task directory and the
  guide — and never searches, never adopts a project above, never relocates to
  a repository root. Every other command walks up from the working directory
  for `.taskqueue.yaml`, takes the nearest one, and stops at the home
  directory, which it checks before stopping. A path not under the home
  directory — `/opt/thing`, a container's `/app`, macOS's `/var/folders`
  temporary directories — cannot reach that bound, and the walk then runs to
  the filesystem root: it is bounded by the tree rather than by a directory,
  and it will use a marker it meets on the way. A process with no `HOME` lands
  in the same branch. With no marker the command fails with exit 3 and names
  `tq init`.
- `tq init` is the only thing that creates a task directory or writes a marker.
  No command creates a queue implicitly — not even one whose marker is there
  and whose task directory is missing, which is reported rather than silently
  made. A `.git` bounds nothing, so a submodule reads the superproject's queue
  (TQ-0059).
- **The marker is the source of truth, and the task directory is an output of
  it.** Once discovery has found `.taskqueue.yaml` it knows everything: the
  board, the priorities, the labels, and where the task files live. Nothing
  walks the other way. `Store` keeps the marker it was resolved through
  (`Store.Marker`) and every consumer reads the config through `Store.Config`;
  a second walk, up from `Store.Dir`, finds another project's marker or none
  whenever `path:` points outside the marker's own directory — and either
  answer silently replaced the board a write is validated against, so
  `tq update --assignee` rewrote the status of the task it touched (TQ-0087).
  The path is carried, never the parsed file: the config is read from disk on
  every call, so a CLI edit to the marker reaches a running server the way a
  task edit does. A test at the repository root refuses the shape in the
  source, and `tqtest.EscapedQueue` is the fixture that catches it by
  behaviour: a decoy marker above a task directory that sits outside its own
  project.
- Nothing in `internal/config` answers with a nil `*Config` and a nil error.
  An absent marker is `config.ErrNoConfig`, and a caller for which that is
  genuinely fine folds it with `config.Optional`; that is what stops "no marker
  found" being mistaken for "this project has no configuration".
- A command gets its marker one of two ways and no others: `TQ_CONFIG_PATH`
  hands it one, or it walks up from the working directory for one. So every
  queue has a marker, `Store.Marker` is never empty, and there is no third
  state in which a project has no configuration and something has to guess what
  its board is. `TQ_CONFIG_PATH` names a `.taskqueue.yaml` file rather than a
  task directory, and one that is missing, is a directory, or will not parse is
  an error — never an absence. It replaced the task-directory variable outright
  in TQ-0087, with no alias, the way TQ-0085 removed `TQ_WALK_FOREVER`: naming
  the queue instead of the project is what left a command holding tasks it had
  no configuration to validate, and silently rewriting their status against the
  built-in board.
- Prefer the Go standard library where practical.
- Keep the layering: `internal/task` imports nothing of ours, and nothing
  imports back up the list above. Everything except `cmd/tq` stays under
  `internal/`, so the module promises no Go API — the stable interface is the
  CLI and its JSON. `public/` lives beside the package that embeds it, because
  a `go:embed` cannot reach outside its own directory.
- Track work in this project's own queue, following the lifecycle in
  [Task management](#task-management).

## Testing

`go test ./...` covers frontmatter parsing/rendering, the store, dependency/ready
logic, the CLI (through `runCLI`, without spawning a binary) and the HTTP API
(through `httptest`). Frontend logic that is pure — the notes split/join in
`frontend/notes.ts`, the indexing, dependency and filter rules in
`frontend/board.ts`, the search bar's query language in `frontend/search.ts`,
what one edit in the dialog does about the file it is about to write to in
`frontend/edit.ts`, and the body renderer in `frontend/markdown.ts` — has `bun
test` unit tests next to it (`make test-frontend`). Those files know nothing
about Vue, which is what keeps the components down to rendering and events;
`board.ts`'s `isReady` is checked against the same cases as `task.IsReady`,
since the two are separate implementations of one rule. `markdown.test.ts` is
not a parser's test suite — markdown-it has its own — but it carries three
things: that the dialect really is GFM, which is what the dependency is for;
that each of the four extensions in `markdown.ts` does what it says; and a case
for every way a task file could try to become markup, because that renderer's
output is the one string the board puts into the page as HTML.

A bare `t.TempDir()` is not an isolation barrier: discovery walks up out of it,
and `TQ_CONFIG_PATH` in a developer's shell points the whole suite at their real queue
(TQ-0021, TQ-0053, TQ-0063). So every test package that can reach the store has a
`TestMain` calling `tqtest.Isolate` and a `TestTheSuiteIsIsolated` calling
`tqtest.RequireIsolated`, which fails if that call is ever dropped. Fixtures come
from `internal/tqtest` and nowhere else: `Root` for a root anchored by the
project marker, `RootWithoutMarker` for the tests whose premise is a directory
that is *not* a project yet — with no marker to stop the walk, the fixture
asserts instead that none sits above it, which since TQ-0085 is the only thing
that can keep such a root isolated — and `NewStore`, `MustCreate`,
`WriteConfig`, `AboveFixtures` above them.
`internal/store`'s tests are in the external `store_test` package for this
reason: `tqtest` imports the store, so an in-package test file cannot import the
fixtures. `export_test.go` is the two unexported names those tests still need.

`make typecheck` is the gate `bun build` is not: the bundler strips types
without checking them, so nothing else would catch a frontend regression before
it shipped. `vue-tsc` covers `frontend/` and `browser/` — the `.ts` files and
the templates inside the `.vue` ones — in one run.

`make test-integration` is a separate layer behind a build tag, so `go test ./...`
stays fast. It builds `tq` once and runs it as a process: real exit codes through
`os.Exit`, the stdout/stderr split the `--json` contract rests on, a real
listener, and the CLI and a running server reading each other's writes. Anything
that only shows up in a compiled binary belongs there rather than in a unit test.

`make test-browser` is the layer above that, and the only one that sees a DOM:
`bun test` drives a real Chromium through `playwright-core` against a real `tq
serve`, one temp project per test on a port the OS picks. It covers what only a
browser can show — native drag and drop, `<dialog>`, focus and blur, a refresh
standing down for a drag, a click-to-edit field turning into an input and back,
and an open dialog taking in a change to its own task without disturbing what is
under the caret. It is also the migration's
acceptance signal: it drives the real binary, so it should never need to know
how the page is built, and a test that has to change is a behaviour change.
`playwright-core` stays test-only — the only things under `node_modules/` that
reach `internal/web/public/` are Vue, bundled into `app.js`, and the SFC loader
that put it there.

The browser is not part of the dependency: `playwright-core` ships no binary, so
a clean checkout needs `make browser-install` once. The suite says so, and names
that command, when it cannot find one.

## Task management

@.tasks/AGENTS.md
