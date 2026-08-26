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
internal/guide/    The generated .tasks/AGENTS.md
internal/web/      REST API, the server, and public/ (embedded frontend)
internal/cli/      Commands, flags, human/JSON output, exit codes
internal/fsx/      Atomic file write, shared by the two generators
internal/tqtest/   Test fixtures shared across packages
internal/integration/ Tests that drive the compiled binary (build tag)
frontend/          main.ts, components/ (.vue), state.ts, api.ts, board.ts,
                   notes.ts, format.ts, index.html, style.css, build.ts (Bun)
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
  `board.ts` and `format.ts`). `bun build` strips types without checking them, so nothing else
  in the pipeline sees a type error.
- Run `make test-browser` after changes to the components, the board's markup or
  its styles. It needs a Chromium: `make browser-install` puts one in the cache.
- Run `make build` before completing a task.
- Do not add a database, an index file, a cache or a filesystem watcher. Markdown
  files are the source of truth and every read hits the disk — that is what makes
  CLI edits visible to a running server.
- A listing reads the task directory twice: once to learn which files to open,
  and once afterwards to check that the set of them did not change while they
  were being opened. Reading the names and then the files is a TOCTOU, and a
  retitle landing in between leaves a task under a name the pass never looked
  at, so a difference between the two readings means the pass is redone — three
  attempts, after which the listing says it may not match the directory rather
  than passing for the whole queue (TQ-0012). An ID under two names is the same
  signal from the other side, because a retitle writes the new file before
  retiring the old one; a pair two passes at rest both find is a queue to fix
  rather than a race, and is reported and withheld instead (TQ-0040). Keep both
  readings: this is a consistency check, not a cache, and nothing survives the
  call.
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
  Node runtime without an explicit architecture change. Vue arrived through
  TQ-0076 and is the one exception, bundled into `public/app.js` rather than
  fetched: what must not appear is a dependency the built output *relies on* at
  runtime — a CDN, an import map, anything under `node_modules/` served as-is.
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
- Do not have the HTTP layer shell out to the `tq` binary. Both surfaces call the
  same store functions.
- `.tasks/AGENTS.md` is generated by `tq init` (see `agents.go`): change the
  guide there, never by editing the generated file.
- Task files are named `<id>-<title-slug>.md`, but the ID in the frontmatter is
  what identifies a task: look tasks up by ID (`Store.locate`), never by
  reconstructing a filename from a title.
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
  index by it. `List` tells that from a retitle caught between writing the new
  file and retiring the old by looking again: the pair is redone, and one that
  two passes at rest both find is reported rather than retried. A pair only
  ever seen while the directory was moving is withheld too — the ID rule is
  absolute — but the listing says `Incomplete` about it rather than naming
  files to delete. The sentence it is reported with is `locate`'s, composed in
  `duplicateClaim` so that a listing and a refused write cannot say different
  things about the same two files.
- Preserve JSON CLI output compatibility; it is the stable agent API.
- Keep stdout clean when `--json` is active: data on stdout, everything else on
  stderr.
- Keep exit codes stable: 0 success, 1 general/validation, 2 task not found,
  3 `.tasks` missing and uncreatable.
- The task directory is created on demand by any command that needs it, at the
  root of the enclosing Git repository (or `TQ_DIR`). Commands must not fail
  merely because a project has not been initialised.
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
(through `httptest`). Frontend logic that is pure — the notes split/join/merge in
`frontend/notes.ts`, and the indexing, dependency and filter rules in
`frontend/board.ts` — has `bun test` unit tests next to it
(`make test-frontend`). Those two files know nothing about Vue, which
is what keeps the components down to rendering and events; `board.ts`'s
`isReady` is checked against the same cases as `task.IsReady`, since the two are
separate implementations of one rule.

A bare `t.TempDir()` is not an isolation barrier: discovery walks up out of it,
and `TQ_DIR` in a developer's shell points the whole suite at their real queue
(TQ-0021, TQ-0053, TQ-0063). So every test package that can reach the store has a
`TestMain` calling `tqtest.Isolate` and a `TestTheSuiteIsIsolated` calling
`tqtest.RequireIsolated`, which fails if that call is ever dropped. Fixtures come
from `internal/tqtest` and nowhere else: `Root` for a root anchored by the
project marker, `RootWithGit` for the tests whose premise is a directory that is
*not* a project yet — an absent marker leaves the repository bound as the only
anchor — and `NewStore`, `MustCreate`, `WriteConfig`, `AboveFixtures` above them.
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
browser can show — native drag and drop, `<dialog>`, focus and blur, and the
poll standing down while the user is working. It is also the migration's
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
