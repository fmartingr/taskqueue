# AGENTS.md — tq

## Project summary

`tq` is a local-first task queue for agentic development. Each task is a
Markdown file with YAML frontmatter in `.tasks/`. A CLI serves agents, a Kanban
board serves humans, and both go through the same Go store, so the Markdown
files are the only source of truth.

**The reasoning behind every rule below lives in the queue itself** — the task
that made the decision. `tq list --json` to find it, `tq show <id>` to read it.
This file is the short form: what to do, not why.

## Tech stack

- **Backend**: Go, standard library where practical.
- **Frontend**: Vue 3 single-file components + TypeScript + CSS, bundled by Bun
  into `internal/web/public/` and embedded with `go:embed`. The shipped page
  loads one script and fetches nothing at runtime.
- **Layout**: one `main` in `cmd/tq`, everything else under `internal/`, by
  responsibility and layered leaf-first.

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
make frontend       # run after frontend changes (updates public/, committed)
make build          # run before completing a task
make lint           # golangci-lint
make format         # go fmt + go mod tidy
make dev            # Bun watch + DEV=1 server
```

A clean checkout needs `bun install` once before any frontend target, and
`make browser-install` once before `make test-browser`.

## Rules

### Workflow

- `make test` after backend changes.
- `make typecheck` and `make frontend` after frontend changes, and commit the
  `public/` output. Add `make test-frontend` when the change touches the pure
  helpers (`notes.ts`, `board.ts`, `edit.ts`, `markdown.ts`, `search.ts`,
  `format.ts`) — `bun build` strips types without checking them.
- `make test-browser` after changes to components, the board's markup or styles.
- `make build` before completing a task.
- Track work in this project's own queue, following the lifecycle in
  [Task management](#task-management).

### Architecture

- No database, index file, cache or filesystem watcher. Markdown files are the
  source of truth and every read hits the disk — that is what makes CLI edits
  visible to a running server.
- The HTTP layer never shells out to the `tq` binary. Both surfaces call the
  same store functions.
- Keep the layering: `internal/task` imports nothing of ours, and nothing
  imports back up the list. Everything except `cmd/tq` stays under `internal/`,
  so the module promises no Go API — the stable interface is the CLI and its
  JSON. `public/` lives beside the package that embeds it.
- Prefer the Go standard library where practical.

### Store and task files

- Task files are named `<id>-<title-slug>.md`, but the `id` in the frontmatter
  identifies a task: look tasks up with `Store.locate`, never by rebuilding a
  filename from a title.
- `.md`, lowercase, is the only extension a task file may have. Every path tq
  writes, renames, links or removes ends in one, and every path it matches must
  too. Anything else is foreign: never read, never adopted, never renamed.
- A save **moves** the task's file to the name its title asks for and only then
  writes the content into it. Never write the new name and remove the old one.
- ID allocation reads the task files, not just their names, and skips any number
  a task still lists in `depends_on`. Do not replace it with a persisted
  counter or high-water mark — that is the index file forbidden above.
- A listing reads the task directory twice and redoes the pass if the set of
  files changed. Keep both readings; it is a consistency check, not a cache.
- A listing never fails on a bad file. `Store.List` returns a `Listing` carrying
  the tasks it read plus `Unreadable` (unparseable or wrongly-cased files),
  `Duplicated` (an ID more than one file claims, withheld) and `Incomplete` (the
  directory would not hold still). **Every caller must render all three**: the
  CLI names them on stderr and still exits 0, `GET /api/status` carries them,
  the board says them in a toast and its footer. Reading a single task by ID is
  the exception and still fails loudly.
- `task.NormalizeID` expands the `28` -> `TQ-0028` shorthand at the CLI boundary
  and nowhere deeper; `task.FormatID` is the other half, and the two are checked
  against each other.

### Configuration and discovery

- **The marker is the source of truth and the task directory is an output of
  it.** Once discovery has found `.taskqueue.yaml` it knows everything: the
  board, the priorities, the labels, where the task files live. Nothing walks
  the other way. `Store` keeps the marker it was resolved through
  (`Store.Marker`) and every consumer reads through `Store.Config`, which
  re-reads the file from disk on every call.
- Discovery is two rules: `tq init` creates the queue in the directory it is run
  in and never searches; every other command walks up from the working directory
  for the nearest marker, stopping at the home directory (or running to the
  filesystem root from outside it). `TQ_CONFIG_PATH` names a `.taskqueue.yaml`
  **file** and stands in for the walk. With no marker, exit 3 naming `tq init`.
- `tq init` is the only thing that creates a task directory or writes a marker.
  No command creates a queue implicitly.
- Nothing in `internal/config` answers with a nil `*Config` and a nil error. An
  absent marker is `config.ErrNoConfig`, folded with `config.Optional` by a
  caller for which that is fine.
- **Editing `.taskqueue.yaml` means running `tq init` again.** It is idempotent
  and it is where a board change lands: `Store.Reconcile` moves every task filed
  in a column the file no longer declares to the board's default column and
  names them through `Store.Announce`. `Store.List` and `Store.Get` reconcile
  too, so the queue is never half-migrated.
- A reconciliation never fails a read: it returns a `Reconciliation`, not an
  error, puts what it could not write in `Unfinished`, and carries on past a
  refused write rather than stopping at the first.
- Only reconciliation moves a task between columns. `Store.update` writes the
  status it was handed; `Columns.Check` guards the paths that *pick* a column;
  `Columns.Normalize` resolves an alias and nothing else.
- `.tasks/AGENTS.md` is generated by `tq init` from
  `internal/guide/AGENTS.tmpl.md`: change the template, never the generated
  file.

### CLI contract

- Preserve JSON CLI output compatibility; it is the stable agent API.
- Keep stdout clean when `--json` is active: data on stdout, everything else on
  stderr.
- Keep exit codes stable: 0 success, 1 general/validation, 2 task not found,
  3 no task queue found. Every message behind 3 names `tq init`.

### Frontend

- Do not introduce a second frontend framework, a bundler other than Bun, or a
  Node runtime the built output relies on. `vue` and `markdown-it` are the two
  exceptions, bundled into `public/app.js` rather than fetched; they are pinned
  exactly, as are `typescript` and `vue-tsc`, because the bundle is committed.
  Packages used only by tests or the toolchain are a different question.
- Bun itself is pinned in `.bun-version`, and every `oven-sh/setup-bun` step
  reads it through `bun-version-file`: the bundler is not byte-stable across
  releases, and `app.js` is committed and rebuilt by CI to check it is not
  stale, so an unpinned runner fails that check on a commit nobody touched.
  Bumping the pin is a deliberate change — raise `.bun-version`, run
  `make frontend`, and commit the new bundle with it.
- No `<style>` block in an SFC: Bun emits it as a second entry output that wants
  the same `app.js` name and the build aborts. `frontend/style.css` is the
  stylesheet, copied rather than bundled.
- `frontend/markdown.ts` is the only thing in the board that builds HTML, with
  markdown-it in GFM mode. `html: false` is load-bearing — it makes raw HTML in
  a body *text*, so there is no sanitiser after the renderer and nothing to
  sanitise. Its four extensions (an allow-list `validateLink`, task-list
  checkboxes, a scrolling table wrapper, links leaving the tab) each carry
  tests.
- The task dialog holds no draft of its task: every control is drawn from the
  task the board last read, and each field is written as a partial patch of that
  field alone when its editor closes. Do not bring back a Save that patches the
  whole task.
- A write whose field moved on disk since its editor opened is **refused**, not
  merged: nothing is written, the dialog names the field, and the user's text
  stays on screen. A body is two fields in one string, so `commitContent` and
  `commitNote` compare only their own half.
- The one thing the server keeps between requests are the two change
  fingerprints behind `/api/events` — a stat of the task directory and of
  `.taskqueue.yaml`, hashed and compared while a board is connected. They hold
  no data and subscribe to nothing.

## Testing

`go test ./...` covers frontmatter parsing/rendering, the store,
dependency/ready logic, the CLI (through `runCLI`, without spawning a binary)
and the HTTP API (through `httptest`). Pure frontend logic — `notes.ts`,
`board.ts`, `search.ts`, `edit.ts`, `markdown.ts` — has `bun test` tests next to
it (`make test-frontend`); those files know nothing about Vue, which is what
keeps the components down to rendering and events. `board.ts`'s `isReady` is
checked against the same cases as `task.IsReady`, since the two are separate
implementations of one rule.

`make typecheck` is the gate `bun build` is not: the bundler strips types
without checking them, so nothing else catches a frontend regression before it
ships. `vue-tsc` covers `frontend/` and `browser/` in one run.

`make test-integration` is a separate layer behind a build tag: it builds `tq`
once and runs it as a process — real exit codes, the stdout/stderr split, a real
listener, the CLI and a running server reading each other's writes. Anything
that only shows up in a compiled binary belongs there.

`make test-browser` drives a real Chromium through `playwright-core` against a
real `tq serve`, one temp project per test on a port the OS picks. It covers
what only a browser can show — drag and drop, `<dialog>`, focus and blur,
click-to-edit — and should never need to know how the page is built.
`playwright-core` ships no binary: `make browser-install` puts one in the cache.

**Isolation is mandatory.** A bare `t.TempDir()` is not a barrier: discovery
walks up out of it, and a `TQ_CONFIG_PATH` in a developer's shell points the
whole suite at their real queue. Every test package that can reach the store has
a `TestMain` calling `tqtest.Isolate` and a `TestTheSuiteIsIsolated` calling
`tqtest.RequireIsolated`. Fixtures come from `internal/tqtest` and nowhere else:
`Root`, `RootWithoutMarker`, `NewStore`, `MustCreate`, `WriteConfig`,
`AboveFixtures`. `internal/store`'s tests live in the external `store_test`
package because `tqtest` imports the store; `export_test.go` is the two
unexported names they still need.

## Task management

@.tasks/AGENTS.md
