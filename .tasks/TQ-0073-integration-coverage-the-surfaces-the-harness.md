---
id: TQ-0073
title: 'Integration coverage: the surfaces the harness does not reach yet'
status: todo
priority: normal
labels:
  - tests
  - component/cli
  - component/api
depends_on:
  - TQ-0038
created: 2026-08-25T20:12:25+02:00
updated: 2026-08-25T20:12:43+02:00
---

## Why

TQ-0038 built the harness and eight tests to prove it works, deliberately not
exhaustive coverage. This ticket is the audit of what is left, taken from the
command table in `internal/cli/cli.go`, the route table in
`internal/web/server.go`, and what the existing tests actually touch.

Nothing here is a bug. It is a list of claims the compiled binary makes that no
test currently checks.

## What is covered today

Commands: `init`, `add`, `list`, `show`, `move`, `note`, `done`, `ready`.
Flags: `--json`, `--priority`, `--label` on `add`.
API: `GET`/`POST /api/tasks`, `PATCH /api/tasks/{id}`.
Plus exit codes 0/1/2/3, `--json` stdout purity on two commands, embedded and
`DEV=1` asset serving, and shutdown leaving no temp file.

## CLI gaps

**Commands never run as a binary**

- `update` — every flag: `--title`, `--status`, `--priority`, `--assignee`,
  `--add-label`, `--remove-label`, `--add-dependency`, `--remove-dependency`.
  The incremental list edits are the interesting ones: they are read-modify-write
  and the only place `--add-label` is exercised at all is a unit test.
- `version` — and specifically that the string is the one `-ldflags` stamped.
  The harness builds without ldflags today, so it would report `dev`; pinning
  this means building the test binary the way `make build` does.
- `help`, `-h`, `--help`, and the usage text a bare `tq` prints. Exit codes for
  each, which differ between "no command" and "unknown command".

**Flags never exercised through the binary**

- `add`: `--assignee`, `--body`, `--status`, `--depends-on`.
- `list` and `ready` filters: `--status`, `--priority`, `--label`, `--assignee`,
  singly and combined, including a filter that matches nothing.
- The `--` terminator, which is TQ-0018: `tq note <id> -- "-1 test failing"`.
  That ticket is open and this is the layer that proves it.

**Contracts asserted on only two commands**

- `--json` stdout purity for `list`, `ready`, `update`, `note`, `init` and
  `show`. Every one of them prints something human-readable in its normal mode,
  and an agent parses stdout for all of them.
- Exit code 1 for each distinct validation failure, not just an invalid status:
  an empty title, a bad priority, a dependency on itself, a malformed ID.

**Discovery and configuration, which only a process can show**

- Running from a subdirectory of the project, which is the guide's own promise.
- `TQ_DIR` pointing somewhere else entirely, and that it beats the marker.
- `TQ_WALK_FOREVER=true` reaching a marker above the repository root.
- A marker with a non-default `path`, and one naming an absolute path.
- The three broken-config messages — future `version`, malformed YAML, the
  `.taskqueue.yml` typo — each exiting 1 and naming the file.
- `tq init` writing both `.taskqueue.yaml` and `.tasks/AGENTS.md`, and leaving a
  hand-written config alone on a second run.

**Serving**

- `--host` and `--port`, and the `TQ_HOST`/`TQ_PORT` defaults behind them.
- Request logging on stderr, which is the only place it is visible.
- Graceful shutdown on SIGTERM specifically. The current test kills the process;
  the signal path with in-flight requests is untested.

## WebUI gaps

**The REST API, from a running server**

Five of the eight routes are never called through the binary:
`GET /api/tasks/{id}`, `POST /api/tasks/{id}/notes`, `GET /api/config`,
`GET /api/status`, `GET /api/version`, and the `/api/` catch-all that 404s an
unknown endpoint.

- Query filters on `GET /api/tasks`: `status`, `priority`, `label`, `assignee`,
  `ready`, and the deliberate 400 for an unparsable `ready` value.
- The error envelope itself. `server.go` has ten `writeError` calls; none is
  checked end to end. Status code, `code` string and message shape are what a
  board or a script actually branches on.
- 404 for an unknown task against every route that takes an `{id}`.
- Malformed JSON, a wrong content type, and an empty body on each writing route.

**The served frontend**

- The assets are checked for a 200 and nothing else. Worth asserting the
  embedded `index.html` actually references `app.js`, `style.css` and the
  favicon, so a broken build cannot pass as a working one.
- `DEV=1` is proven to serve from disk; that it stops serving the embedded copy
  is not.

**The browser layer: a separate decision, deliberately not scoped here**

`frontend/app.ts` is 695 lines of DOM behaviour with no test at any level:
drag and drop between columns, the inline composer, the task dialog, note
editing, and the poll that refreshes the board while skipping a drag or an open
dialog. Covering that means Playwright and a `node_modules`, which the plan
rules out and which is a real architecture change. It needs its own ticket and
an explicit decision, exactly as TQ-0038 said.

What can be done with no new dependency, and probably should be its own smaller
ticket: `bun test` for the pure helpers in `app.ts` — `indexTasks`,
`pendingDependencies`, `isReady`, `visibleTasks` — the way `notes.ts` already
has them.

## Suggested approach

Take these in slices rather than as one commit, and let the priority be how
much a wrong answer would cost:

1. The `--` terminator (TQ-0018) and `--json` purity everywhere, because agents
   depend on both and both are cheap.
2. `update` and the list filters, the largest untested command surface.
3. The API error envelope and the five unreached routes.
4. Discovery and configuration through the binary.
5. `tq version` under real ldflags, which needs the harness to build the way
   `make build` does.

## Acceptance criteria

- Each area above is either covered or has a note here saying why not.
- `make test-integration` stays parallel-safe and does not slow down enough to
  stop people running it.
- The browser question is answered in its own ticket, not silently absorbed.

---

## Notes

- 2026-08-25T20:12:43+02:00 — Written from the command table in internal/cli/cli.go, the route table in internal/web/server.go and what the eight existing tests actually call, not from memory. Covered today: eight commands, three flags, two of eight routes.
- 2026-08-25T20:12:43+02:00 — TQ-0018, the -- terminator, is listed here because this is the layer that proves it. That ticket stays the place it gets fixed.
- 2026-08-25T20:12:43+02:00 — The browser layer is named but deliberately out of scope: 695 lines of DOM behaviour in app.ts with no test at any level, and covering it means Playwright and a node_modules, which the plan rules out. It needs its own ticket and a decision.
