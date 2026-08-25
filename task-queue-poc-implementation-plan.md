# Task Queue (`tq`) — Proof of Concept Implementation Plan

## 1. Goal

Build a small, local-first task queue designed for **agentic software-development workflows**.

The PoC should prove that the same task state can be used comfortably by:

- **Agents**, through a deterministic CLI with JSON output.
- **Humans**, through a lightweight Kanban web interface.
- **Git**, because every task is stored as a normal Markdown file with YAML frontmatter.

The filesystem is the database. There is no SQL database, remote service, account system, or required daemon.

Core product idea:

> Markdown on disk, CLI for agents, Kanban for humans.

The executable name used throughout this document is `tq`. ---

## 2. Reference Architecture

The PoC should deliberately follow the architecture and deployment style of:

<https://git.nakama.town/fmartingr/terraria-companion>

Relevant characteristics of that project:

- A **single Go module** with the main package at the repository root.
- Small, flat Go source files rather than a large `internal/` hierarchy.
- CLI dispatch from `main.go`.
- HTTP server built with Go's standard `net/http` package and `http.ServeMux`.
- Frontend assets stored under `public/`.
- `go:embed` used to package the frontend into the production executable.
- In development mode, frontend files are served from disk.
- In production, the embedded filesystem is served instead.
- A Makefile is the primary developer interface.
- `CGO_ENABLED=0` builds a self-contained binary.
- Version information is injected with Go linker flags.
- GoReleaser/container packaging can be added without changing the application architecture.
- CI architecutre
- Dev tooling

The task queue should retain this simplicity.

### Intentional difference

Unlike `terraria-companion`, the task queue frontend will use **Bun** for its build pipeline because the frontend will be written in TypeScript.

Bun is the only JavaScript toolchain required.

For the PoC:

- Bun
- Vanilla TypeScript
- Vanilla CSS
- Browser DOM APIs
- Native HTML drag-and-drop

Do **not** introduce React, Vue, Svelte, Vite, Next.js, a Node runtime, or a frontend state-management library.

---

## 3. PoC Scope

The PoC is successful when a user can:

1. Run `tq init` inside a Git repository.
2. Create Markdown-backed tasks with `tq add`.
3. Query tasks from the CLI.
4. Filter tasks from the CLI.
5. Inspect an individual task.
6. Change task status from the CLI.
7. Add notes to a task from the CLI.
8. Retrieve CLI results as JSON for agents.
9. Run `tq serve`.
10. Open a Kanban board in the browser.
11. Create and edit tasks from the web UI.
12. Drag a task between Kanban columns.
13. See CLI-created or CLI-modified tasks appear in the web UI.
14. Stop the server and still have all task state represented entirely by Markdown files.
15. Commit the task directory to Git and get understandable diffs.

The PoC should avoid features that do not prove this central workflow.

---

## 4. Non-Goals for the PoC

Do not implement these unless they become necessary to prove the core architecture:

- Authentication
- Multi-user permissions
- Remote synchronization
- Hosted service
- SQL/database persistence
- WebSockets
- Complex project management
- Time tracking
- Comments with identities
- Attachments
- Rich-text editing
- Notifications
- GitHub/GitLab issue synchronization
- MCP server
- Agent orchestration
- Agent execution
- Scheduling
- Recurring tasks
- Multiple boards
- Custom workflows
- Custom fields
- Full-text search indexing
- Cross-process distributed locking
- Mobile-specific UI
- Offline PWA support

The tool manages work. It should not attempt to execute or orchestrate agents.

---

## 5. Repository Layout

Keep the repository close to the flat structure used by `terraria-companion`.

```text
.
├── main.go
├── cli.go
├── server.go
├── task.go
├── store.go
├── frontmatter.go
├── embed.go
├── version.go                 # optional; may remain in main.go for PoC
│
├── frontend/
│   ├── index.html
│   ├── app.ts
│   └── style.css
│
├── public/                    # generated frontend output
│   ├── index.html
│   ├── app.js
│   └── style.css
│
├── go.mod
├── go.sum
├── package.json
├── bun.lock
├── Makefile
├── .gitignore
├── README.md
├── AGENTS.md
└── .goreleaser.yml           # optional during PoC
```

Avoid prematurely introducing:

```text
cmd/
internal/
pkg/
services/
repositories/
controllers/
```

The application is intentionally small. Split files by responsibility while keeping a single `package main`, as in the reference project.

If the codebase eventually grows enough to justify packages, that can be addressed after the PoC.

---

## 6. Task Storage

### 6.1 Task directory

By default, tasks live in:

```text
.tasks/
```

Example project:

```text
my-project/
├── .git/
├── .tasks/
│   ├── TQ-0001.md
│   ├── TQ-0002.md
│   └── TQ-0003.md
├── src/
└── README.md
```

The `.tasks/` directory is intended to be committed to Git.

### 6.2 Project discovery

Every CLI command should locate the task project by:

1. Starting at the current working directory.
2. Looking for `.tasks/`.
3. Walking upward through parent directories.
4. Stopping at the filesystem root.
5. Returning a useful error if `.tasks/` cannot be found.

This allows an agent to invoke `tq` from any subdirectory of a repository.

Optional override:

```bash
TQ_DIR=/path/to/project/.tasks tq list
```

The environment override is useful for automation and tests.

### 6.3 Filename

Use stable task IDs as filenames:

```text
TQ-0001.md
TQ-0002.md
TQ-0003.md
```

Do not put the title into the filename during the PoC.

Reasons:

- IDs remain stable when titles change.
- CLI references stay short.
- Agents do not need to escape filenames.
- Renaming a task does not create a Git rename.
- Parsing and lookup remain trivial.

### 6.4 Task schema

Example:

```markdown
---
id: TQ-0001
title: Implement OIDC authentication
status: in-progress
priority: high
assignee: agent-auth
labels:
  - backend
  - auth
depends_on:
  - TQ-0004
created: 2026-08-25T08:30:00+02:00
updated: 2026-08-25T09:12:00+02:00
---

Implement authentication using the existing OIDC provider.

## Acceptance criteria

- Login redirects to the identity provider.
- Callback creates a local session.
- Logout destroys the session.

## Notes

Initial investigation completed.
```

### 6.5 Go model

Conceptual model:

```go
type Task struct {
    ID        string    `yaml:"id" json:"id"`
    Title     string    `yaml:"title" json:"title"`
    Status    string    `yaml:"status" json:"status"`
    Priority  string    `yaml:"priority,omitempty" json:"priority,omitempty"`
    Assignee  string    `yaml:"assignee,omitempty" json:"assignee,omitempty"`
    Labels    []string  `yaml:"labels,omitempty" json:"labels,omitempty"`
    DependsOn []string  `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
    Created   time.Time `yaml:"created" json:"created"`
    Updated   time.Time `yaml:"updated" json:"updated"`
    Body      string    `yaml:"-" json:"body"`
}
```

### 6.6 Statuses

Hard-code the initial workflow:

```text
backlog
todo
in-progress
done
```

This gives the Kanban board four columns and avoids configuration work during the PoC.

Custom statuses can be considered later.

### 6.7 Priorities

Support:

```text
low
normal
high
urgent
```

Default:

```text
normal
```

### 6.8 Dependencies

Support simple task dependencies from the beginning:

```yaml
depends_on:
  - TQ-0002
  - TQ-0003
```

This enables the important agent-oriented command:

```bash
tq ready
```

A task is "ready" when:

- its status is not `done`;
- it is not already `in-progress`;
- every task in `depends_on` exists;
- every dependency has status `done`.

For the PoC, missing dependencies should make a task blocked rather than silently ready.

---

## 7. Markdown / Frontmatter Parsing

Use YAML frontmatter delimited by:

```text
---
...
---
```

The Markdown body after the second delimiter is kept verbatim.

A small dedicated `frontmatter.go` should own:

- splitting frontmatter from body;
- YAML decoding;
- YAML encoding;
- task validation;
- reconstruction of the Markdown file.

Prefer one focused YAML dependency rather than writing a YAML parser.

All other backend functionality should use the Go standard library where practical.

### Validation

At minimum validate:

- `id` is non-empty and matches `TQ-[0-9]+`;
- `title` is non-empty;
- `status` is one of the four known statuses;
- `priority`, when present, is valid;
- a task cannot depend on itself;
- timestamps parse correctly.

The reader should report the filename in parse/validation errors.

---

## 8. Store Layer

`store.go` owns all filesystem interaction.

Suggested API:

```go
type Store struct {
    Dir string
}

func (s *Store) List() ([]Task, error)
func (s *Store) Get(id string) (Task, error)
func (s *Store) Create(input CreateTaskInput) (Task, error)
func (s *Store) Update(task Task) error
func (s *Store) Delete(id string) error
func (s *Store) NextID() (string, error)
```

`Delete` can exist internally even if no user-facing delete command is exposed in the first PoC.

### Reads

For the PoC, simply scan `.tasks/*.md` when data is requested.

Do not add:

- a database;
- an index file;
- a long-lived in-memory cache;
- filesystem watchers.

A directory with tens or hundreds of small Markdown files is cheap enough to scan, and reading from disk on each operation has an important advantage:

> CLI modifications become visible to the running web server without synchronization infrastructure.

### Writes

Use atomic file replacement:

1. Render the full Markdown file.
2. Write to a temporary file in `.tasks/`.
3. `fsync`/close it where appropriate.
4. Rename it over the destination.

This prevents partially-written Markdown files if a process crashes during a write.

For the PoC, document that simultaneous writes from different processes can still produce a last-writer-wins conflict. Cross-process locking is explicitly out of scope.

### Ordering

Default list order:

1. status order;
2. priority;
3. creation time;
4. ID.

For the board, grouping by status happens client-side or in the HTTP handler.

---

## 9. ID Allocation

PoC algorithm:

1. Scan task filenames matching `TQ-NNNN.md`.
2. Find the highest numeric component.
3. Increment by one.
4. Format with at least four digits.

Example:

```text
TQ-0001
TQ-0002
TQ-0003
```

After `TQ-9999`, allow IDs to naturally grow:

```text
TQ-10000
```

This is intentionally simple and human-readable.

Known PoC limitation: two processes creating a task at exactly the same time may race for the same next ID.

A later version can solve this with locking or random/ULID IDs if needed.

---

## 10. CLI

`main.go` should follow the reference project's simple dispatch style.

Conceptually:

```go
func main() {
    args := os.Args[1:]

    if len(args) == 0 {
        printUsage()
        return
    }

    switch args[0] {
    case "init":
        runInit(args[1:])
    case "add":
        runAdd(args[1:])
    case "list":
        runList(args[1:])
    case "show":
        runShow(args[1:])
    case "move":
        runMove(args[1:])
    case "update":
        runUpdate(args[1:])
    case "note":
        runNote(args[1:])
    case "done":
        runDone(args[1:])
    case "ready":
        runReady(args[1:])
    case "serve":
        runServer(args[1:])
    case "version":
        printVersion()
    default:
        printUsage()
        os.Exit(1)
    }
}
```

For the PoC, prefer Go's standard `flag` package instead of Cobra unless command parsing becomes genuinely painful.

The CLI itself is a primary product surface, not an afterthought.

### 10.1 `tq init`

```bash
tq init
```

Creates:

```text
.tasks/
```

Behavior:

- fail if `.tasks/` already exists unless explicitly made idempotent;
- print the created directory;
- do not modify `.gitignore`.

### 10.2 `tq add`

```bash
tq add "Implement authentication"
tq add "Implement authentication" --priority high
tq add "Implement authentication" --label backend --label auth
tq add "Implement authentication" --assignee agent-auth
tq add "Implement authentication" --depends-on TQ-0002
```

Default fields:

```yaml
status: todo
priority: normal
labels: []
depends_on: []
```

Output:

```text
Created TQ-0012: Implement authentication
```

Agent mode:

```bash
tq add "Implement authentication" --json
```

returns the complete task object.

### 10.3 `tq list`

```bash
tq list
tq list --status todo
tq list --status in-progress
tq list --priority high
tq list --label backend
tq list --assignee agent-auth
tq list --status todo --label backend
tq list --json
```

Human output should be intentionally stable and simple:

```text
ID        STATUS        PRIORITY  ASSIGNEE    TITLE
TQ-0004   todo          high      agent-api   Add task API
TQ-0008   in-progress   normal    agent-ui    Build board
```

Do not add decorative boxes, spinners, colors required for meaning, or ambiguous abbreviations.

### 10.4 `tq show`

```bash
tq show TQ-0004
tq show TQ-0004 --json
```

Human output should include frontmatter fields followed by the Markdown body.

JSON output returns the complete model.

### 10.5 `tq move`

```bash
tq move TQ-0004 backlog
tq move TQ-0004 todo
tq move TQ-0004 in-progress
tq move TQ-0004 done
```

This is the main status-transition primitive.

No workflow-transition restrictions are needed in the PoC.

### 10.6 `tq done`

Convenience alias:

```bash
tq done TQ-0004
```

Equivalent to:

```bash
tq move TQ-0004 done
```

### 10.7 `tq update`

```bash
tq update TQ-0004 --title "New title"
tq update TQ-0004 --priority urgent
tq update TQ-0004 --assignee agent-api
tq update TQ-0004 --add-label backend
tq update TQ-0004 --remove-label backend
tq update TQ-0004 --add-dependency TQ-0002
tq update TQ-0004 --remove-dependency TQ-0002
```

Only supplied fields are modified.

### 10.8 `tq note`

Append agent/human notes without requiring full-file editing:

```bash
tq note TQ-0004 "API implemented; integration tests still failing."
```

PoC behavior:

- if `## Notes` exists, append a bullet or paragraph beneath it;
- otherwise add a `## Notes` section at the end of the body;
- update the task's `updated` timestamp.

A predictable format is preferred:

```markdown
## Notes

- 2026-08-25T09:42:00+02:00 — API implemented; integration tests still failing.
```

Do not require an identity field for PoC notes.

### 10.9 `tq ready`

```bash
tq ready
tq ready --label backend
tq ready --json
```

Returns tasks available for an agent to pick up.

This command is one of the core differentiators of the PoC.

### 10.10 Exit codes

Use predictable exit codes:

```text
0  success
1  general/validation error
2  task not found
3  project (.tasks) not found
```

Exact numbers can change, but they must be documented and stable.

### 10.11 stdout vs stderr

Important for agents:

- requested data -> `stdout`;
- errors and diagnostics -> `stderr`.

When `--json` is active, stdout must contain **only JSON**.

Never mix logs with JSON output.

---

## 11. Agent-Friendly JSON Contract

Every query-oriented command should support `--json`.

At minimum:

```bash
tq add ... --json
tq list --json
tq show TQ-0001 --json
tq ready --json
```

Example:

```json
{
  "id": "TQ-0001",
  "title": "Implement authentication",
  "status": "todo",
  "priority": "high",
  "assignee": "agent-auth",
  "labels": ["backend", "auth"],
  "depends_on": [],
  "created": "2026-08-25T08:30:00+02:00",
  "updated": "2026-08-25T08:30:00+02:00",
  "body": "Implement OIDC authentication."
}
```

Lists should be raw JSON arrays for the PoC:

```json
[
  { "...": "..." },
  { "...": "..." }
]
```

Avoid an unnecessary envelope unless pagination or metadata later requires one.

---

## 12. HTTP Server

`server.go` should use Go's standard library:

```go
mux := http.NewServeMux()
```

No HTTP framework is required.

`tq serve` starts the server.

Defaults:

```text
Host: 127.0.0.1
Port: 7331
```

CLI options:

```bash
tq serve
tq serve --port 8080
tq serve --host 0.0.0.0
```

Binding to localhost by default is safer because the PoC has no authentication.

Environment variables may optionally mirror the CLI:

```text
TQ_HOST
TQ_PORT
TQ_DIR
DEV
```

CLI flags should take precedence.

---

## 13. REST API

All API responses are JSON.

### `GET /api/tasks`

Return tasks.

Supported query parameters:

```text
status
priority
label
assignee
ready
```

Examples:

```text
GET /api/tasks
GET /api/tasks?status=todo
GET /api/tasks?label=backend
GET /api/tasks?ready=true
```

### `POST /api/tasks`

Create a task.

Request:

```json
{
  "title": "Implement authentication",
  "priority": "high",
  "labels": ["backend", "auth"],
  "assignee": "agent-auth",
  "depends_on": []
}
```

### `GET /api/tasks/{id}`

Return one task.

### `PATCH /api/tasks/{id}`

Partial update.

Example status change:

```json
{
  "status": "in-progress"
}
```

The Kanban drag-and-drop operation uses this endpoint.

### `POST /api/tasks/{id}/notes`

Request:

```json
{
  "text": "API implemented; integration tests still failing."
}
```

### `GET /api/status`

Small health endpoint:

```json
{
  "ok": true,
  "task_count": 12,
  "task_dir": "/repo/.tasks",
  "version": "dev"
}
```

For security/privacy, consider returning a relative task directory rather than the full absolute path once the PoC is validated.

### `GET /api/version`

```json
{
  "version": "v0.1.0"
}
```

This mirrors the reference project.

### Errors

Use a stable JSON error shape:

```json
{
  "error": "task not found",
  "code": "task_not_found"
}
```

Return appropriate HTTP status codes:

```text
400 malformed request / validation error
404 task not found
409 conflict, if needed
500 unexpected filesystem/server error
```

---

## 14. Server / CLI Code Reuse

The HTTP server and CLI must call the same store/domain functions.

Do **not** have the web API call the `tq` executable as a subprocess.

Correct:

```text
CLI ------\
           -> Store -> Markdown files
HTTP -----/
```

Incorrect:

```text
HTTP -> exec("tq move ...") -> CLI -> Markdown files
```

This ensures:

- one validation implementation;
- one serialization implementation;
- one ID allocator;
- one filtering implementation;
- one dependency implementation.

---

## 15. Frontend

### 15.1 Toolchain

Use Bun only.

Example `package.json`:

```json
{
  "private": true,
  "scripts": {
    "build": "bun build frontend/app.ts --target=browser --outfile=public/app.js",
    "dev": "bun build frontend/app.ts --target=browser --outfile=public/app.js --watch"
  }
}
```

The Makefile can copy:

```text
frontend/index.html -> public/index.html
frontend/style.css  -> public/style.css
```

Alternatively, a tiny Bun build script may perform both copying and bundling.

Keep the frontend build deterministic and obvious.

### 15.2 No frontend framework

`app.ts` owns:

- API calls;
- in-memory board state;
- DOM rendering;
- filtering;
- drag-and-drop;
- modal/dialog interactions.

For the PoC, that is enough.

### 15.3 Board

Render four columns:

```text
BACKLOG | TODO | IN PROGRESS | DONE
```

Each task card should display:

- ID
- title
- priority
- assignee, when present
- labels
- blocked indicator when dependencies are incomplete

### 15.4 Drag and drop

Use native browser drag/drop.

On drop:

```text
PATCH /api/tasks/TQ-0004
{
  "status": "in-progress"
}
```

Optimistic UI is optional.

Simplest robust PoC flow:

1. Send PATCH.
2. If successful, reload tasks.
3. If failed, keep/reload previous server state and display an error.

### 15.5 Task detail

Clicking a card opens a `<dialog>`.

Show:

- title;
- ID;
- status;
- priority;
- assignee;
- labels;
- dependencies;
- Markdown body in a textarea.

For the PoC, editing the body as plain Markdown is sufficient.

Rendered Markdown preview is not required.

### 15.6 Create task

A simple button opens a dialog with:

- title;
- priority;
- assignee;
- labels;
- body.

Submit through:

```text
POST /api/tasks
```

### 15.7 Filters

Small filter bar:

- status;
- priority;
- assignee;
- label;
- "Ready only".

Filtering can happen client-side after loading all tasks.

### 15.8 External CLI changes

Because an agent may modify tasks while the browser is open, the board needs a basic refresh mechanism.

For the PoC, poll:

```text
GET /api/tasks
```

every 2–5 seconds.

Do not implement WebSockets or SSE yet.

Because the server reads Markdown files from disk rather than relying on an in-memory database, CLI changes should appear on the next poll.

### 15.9 Error handling

A small non-blocking error banner/toast is enough.

Examples:

```text
Could not update TQ-0004
Task file contains invalid frontmatter
```

Do not build a notification framework.

---

## 16. Frontend Embedding

Match the reference project's deployment model.

`embed.go`:

```go
package main

import "embed"

//go:embed public/*
var publicFS embed.FS
```

### Development

When:

```text
DEV=1
```

serve files from `public/` on disk so Bun rebuilds become visible without rebuilding Go.

### Production

When `DEV` is unset:

- serve frontend assets from `publicFS`;
- ship only the Go executable;
- no Bun installation is required on the target machine.

The production binary should contain the web UI, but **not the user's task files**.

Tasks remain external project data under `.tasks/`.

---

## 17. Development Workflow

The desired developer experience should mirror `terraria-companion`.

### `make help`

Lists targets.

### `make frontend`

Build frontend with Bun.

### `make build`

Conceptually:

```make
build: frontend
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o tq .
```

The frontend is embedded automatically because `public/` exists at compile time.

### `make dev`

Run:

- Bun in watch mode;
- Go server with `DEV=1`.

Possible implementation:

```make
dev:
	@trap 'kill 0' INT TERM EXIT; \
	bun run dev & \
	DEV=1 go run . serve
```

Adjust shell handling as needed for portability.

### `make test`

```bash
go test ./...
```

### `make format`

```bash
go fmt ./...
bunx prettier --write frontend
```

However, if the requirement is literally "Bun only" and avoiding extra JS tooling is preferred, omit Prettier from the PoC and format TypeScript manually/editor-side.

### `make lint`

At minimum:

```bash
golangci-lint run ./...
```

A frontend linter is not required for the PoC.

### `make clean`

Remove:

```text
tt
dist/
public/app.js
```

Preserve source assets.

---

## 18. Versioning

Match the reference project:

```go
var version = "dev"
```

Build:

```bash
go build -ldflags "-X main.version=v0.1.0"
```

CLI:

```bash
tq version
```

API:

```text
GET /api/version
```

UI may show the version in a small footer.

---

## 19. Tests

The PoC should have useful tests despite staying small.

### 19.1 Frontmatter tests

Test:

- parse valid task;
- parse multiline Markdown body;
- serialize and parse round-trip;
- missing closing delimiter;
- invalid YAML;
- missing title;
- invalid status;
- invalid dependency;
- body preservation.

### 19.2 Store tests

Use `t.TempDir()`.

Test:

- create task;
- allocate sequential IDs;
- load task;
- list tasks;
- update task;
- filter task;
- atomic rewrite leaves valid file;
- malformed task produces useful error.

### 19.3 Dependency tests

Test:

- no dependencies -> ready;
- all dependencies done -> ready;
- incomplete dependency -> blocked;
- missing dependency -> blocked;
- self dependency rejected.

### 19.4 CLI tests

Keep CLI functions structured so logic is testable without spawning the binary where practical.

At minimum exercise:

```text
add
list
show
move
ready
--json
```

### 19.5 HTTP tests

Use `httptest`.

Test:

- list tasks;
- create task;
- get task;
- patch task status;
- add note;
- malformed JSON;
- unknown task;
- validation failures.

### 19.6 Frontend

Do not introduce a browser testing stack for the PoC.

Manual acceptance testing is enough.

---

## 20. Implementation Phases

## Phase 0 — Repository Skeleton

### Tasks

- Create Go module.
- Add `main.go`.
- Add version variable.
- Add Makefile.
- Add `frontend/`.
- Add `package.json`.
- Add Bun lockfile.
- Add `embed.go`.
- Add minimal frontend build.
- Add `/api/status`.
- Add embedded static serving.
- Add `DEV` disk serving.

### Acceptance criteria

```bash
make build
./tq serve
```

opens a page from a single Go binary.

And:

```bash
make dev
```

serves disk-built frontend assets.

This phase proves the `terraria-companion` architecture has been reproduced successfully.

---

## Phase 1 — Markdown Task Core

### Tasks

- Add `Task` model.
- Add frontmatter parser.
- Add task validator.
- Add task renderer.
- Add `Store`.
- Add `.tasks` discovery.
- Add atomic writes.
- Add sequential ID allocation.
- Add `tq init`.

### Acceptance criteria

```bash
tq init
```

creates `.tasks/`, and Go tests can create/read/update a valid Markdown task.

No HTTP behavior beyond status is required yet.

---

## Phase 2 — Agent CLI

### Tasks

Implement:

```text
tq add
tq list
tq show
tq move
tq done
tq update
tq note
tq ready
```

Add:

- filters;
- `--json`;
- stable errors;
- documented exit codes.

### Acceptance criteria

An agent can complete this entire flow without touching Markdown directly:

```bash
tq add "Implement login" --priority high --label auth --json
tq ready --json
tq show TQ-0001 --json
tq move TQ-0001 in-progress
tq note TQ-0001 "Login handler implemented."
tq done TQ-0001
```

The resulting `.tasks/TQ-0001.md` is readable and Git-friendly.

At this point the CLI alone should already be useful.

---

## Phase 3 — HTTP API

### Tasks

Implement:

```text
GET    /api/tasks
POST   /api/tasks
GET    /api/tasks/{id}
PATCH  /api/tasks/{id}
POST   /api/tasks/{id}/notes
GET    /api/status
GET    /api/version
```

Reuse the same store/domain logic as the CLI.

### Acceptance criteria

Operations made through HTTP result in the same Markdown representation as equivalent CLI operations.

For example:

```text
tq move TQ-0001 in-progress
```

and:

```http
PATCH /api/tasks/TQ-0001
{"status":"in-progress"}
```

must produce semantically equivalent task files.

---

## Phase 4 — Kanban UI

### Tasks

- Fetch tasks.
- Render four columns.
- Render cards.
- Add create-task dialog.
- Add task-detail/edit dialog.
- Implement drag/drop.
- Add filters.
- Show blocked dependencies.
- Poll for external task changes.
- Add simple error feedback.

### Acceptance criteria

Human workflow:

1. Run `tq serve`.
2. Open browser.
3. Create a task.
4. Drag it to `in-progress`.
5. Edit task details.
6. Complete it.

Agent/human synchronization workflow:

1. Keep the board open.
2. Run:

   ```bash
   tq add "Agent-created task" --label agent
   ```

3. The task appears on the board automatically after the next poll.
4. Drag it to another column.
5. Run:

   ```bash
   tq show <id>
   ```

6. CLI reflects the change.

This is the most important end-to-end PoC demonstration.

---

## Phase 5 — Packaging and PoC Polish

### Tasks

- Add README.
- Add `AGENTS.md`.
- Add usage examples.
- Add shell completion only if trivial; otherwise defer.
- Add GoReleaser configuration.
- Build Linux/macOS amd64/arm64 binaries.
- Optionally add a small Containerfile.
- Confirm frontend is embedded.
- Confirm Bun is build-time only.
- Confirm `.tasks` remains external runtime data.

### Acceptance criteria

A release artifact can be downloaded and used as:

```bash
tq init
tq add "First task"
tq serve
```

with no runtime dependency besides the executable itself.

---

## 21. Suggested `AGENTS.md`

The repository should include instructions optimized for coding agents.

At minimum document:

```text
- Run `make test` after backend changes.
- Run `make frontend` after frontend changes.
- Run `make build` before completing a task.
- Do not add a database; Markdown files are the source of truth.
- Do not introduce a frontend framework without an explicit architecture change.
- Preserve JSON CLI output compatibility.
- Keep stdout clean when --json is active.
- Prefer stdlib Go where practical.
- Keep the flat package-main architecture during the PoC.
```

This makes the project itself suitable for testing its own agent-oriented workflow.

---

## 22. Example End-to-End Session

Initialize:

```bash
$ tq init
Initialized task queue in /code/example/.tasks
```

Create work:

```bash
$ tq add "Implement REST API" --priority high --label backend
Created TQ-0001: Implement REST API

$ tq add "Build Kanban board" --label frontend --depends-on TQ-0001
Created TQ-0002: Build Kanban board
```

Agent asks for work:

```bash
$ tq ready
ID        PRIORITY  LABELS    TITLE
TQ-0001   high      backend   Implement REST API
```

Inspect:

```bash
$ tq show TQ-0001
ID:        TQ-0001
Status:    todo
Priority:  high
Labels:    backend

Implement REST API
```

Claim:

```bash
$ tq move TQ-0001 in-progress
TQ-0001: todo -> in-progress
```

Record context:

```bash
$ tq note TQ-0001 "CRUD endpoints implemented; tests remain."
Note added to TQ-0001
```

Finish:

```bash
$ tq done TQ-0001
TQ-0001: in-progress -> done
```

Now:

```bash
$ tq ready
ID        PRIORITY  LABELS     TITLE
TQ-0002   normal    frontend   Build Kanban board
```

The dependency automatically becoming ready demonstrates the intended agent workflow without implementing an orchestration engine.

---

## 23. Key Architectural Decisions

### Markdown is authoritative

Do not maintain a second representation in SQLite, JSON, or a server cache.

### CLI and HTTP share domain code

There should be no behavioral difference between an agent moving a task through the CLI and a human moving it through the board.

### Server is optional

The CLI works with no daemon.

`tq serve` exists only to expose the human web interface and REST API.

### Bun is build-time only

Production users should not need Bun.

### Single binary distribution

The built frontend is embedded in Go, following the same deployment model as `terraria-companion`.

### Read from disk rather than synchronize caches

This makes CLI/web interoperability almost free during the PoC.

### JSON is a first-class interface

Human-readable CLI output is convenient; JSON output is the stable agent API.

### Keep agentic features primitive

Dependencies and `tq ready` are valuable.

Agent spawning, model APIs, prompts, worktrees, sandboxes, and execution engines are separate concerns and should remain outside the PoC.

---

## 24. Important PoC Limitations

Document these rather than prematurely engineering around them:

1. **Concurrent creation race**  
   Two processes can theoretically allocate the same sequential ID.

2. **Concurrent update race**  
   Atomic writes prevent corruption, but simultaneous edits can still result in last-writer-wins behavior.

3. **No auth**  
   The server binds to localhost by default.

4. **Directory scan per request**  
   Fine for PoC-scale boards; optimize only after measurement.

5. **Fixed workflow**  
   Four hard-coded statuses.

6. **No rendered Markdown**  
   Task descriptions are edited as plain Markdown.

7. **Polling rather than realtime push**  
   Board freshness is near-real-time, not event-driven.

8. **No multi-project UI**  
   One `tq serve` process serves one discovered `.tasks` directory.

These limitations are acceptable because none invalidate the core product hypothesis.

---

## 25. PoC Definition of Done

The PoC is done when all of the following are true:

- [ ] Project structure resembles `terraria-companion`.
- [ ] Backend is Go.
- [ ] HTTP stack uses Go `net/http`.
- [ ] Frontend source is vanilla TypeScript/CSS.
- [ ] Bun is the only JavaScript toolchain.
- [ ] Production web assets are embedded in the Go binary.
- [ ] `DEV=1` serves frontend build output from disk.
- [ ] `tq init` creates `.tasks/`.
- [ ] Each task is one Markdown file with YAML frontmatter.
- [ ] No database exists.
- [ ] CLI can create tasks.
- [ ] CLI can list/filter tasks.
- [ ] CLI can show a task.
- [ ] CLI can update/move a task.
- [ ] CLI can append notes.
- [ ] CLI can report ready/unblocked tasks.
- [ ] Agent-facing query commands support `--json`.
- [ ] JSON mode writes only JSON to stdout.
- [ ] REST API reuses the same task store.
- [ ] Browser displays a four-column Kanban board.
- [ ] Tasks can be created from the board.
- [ ] Tasks can be edited from the board.
- [ ] Tasks can be moved with drag-and-drop.
- [ ] CLI changes appear in the open board.
- [ ] Board changes appear immediately in CLI/file reads.
- [ ] Task writes are atomic.
- [ ] Core parser/store/API behavior has Go tests.
- [ ] `make build` produces one self-contained executable.
- [ ] Bun is not required at runtime.
- [ ] The `.tasks/` directory produces understandable Git diffs.
- [ ] README documents the human workflow and agent workflow.

---

## 26. What to Evaluate After the PoC

Only after using the PoC in real agent sessions, evaluate whether the next version needs:

- cross-process locking;
- ULIDs instead of sequential IDs;
- custom statuses;
- task ordering within columns;
- project configuration;
- richer dependency graphs;
- parent/child tasks;
- Markdown rendering;
- SSE instead of polling;
- shell completions;
- `tq edit` using `$EDITOR`;
- task claiming/leases for multiple agents;
- worktree awareness;
- event/history logs;
- Git integration;
- an MCP adapter;
- a hosted/remote mode.

The important question is not which of these can be built.

The PoC should answer:

> Is a Git-native Markdown task store, manipulated by a small CLI for agents and a Kanban UI for humans, actually pleasant enough to become part of a daily agentic development workflow?
