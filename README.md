# tq — task queue

<div align="center"><img src="frontend/icon.png" width="128" alt="tq"/></div>

A local-first task queue for agentic software development.

> Markdown on disk, CLI for agents, Kanban for humans.

## Summary

Every task is a Markdown file with YAML frontmatter inside a `.tasks/` directory
in your repository. The filesystem is the database: no SQL, no service, no
account, no daemon required. Agents drive the deterministic CLI (with `--json`
everywhere it matters), humans drag cards around a Kanban board, and Git sees
plain text either way.

```text
.tasks/TQ-0001.md   ←  tq add / tq move / tq note        (agents, CLI)
                    ←  http://127.0.0.1:7331             (humans, board)
                    ←  git diff                          (review, history)
```

<div align="center"><a href="screenshots/board-1.png"><img alt="The tq board, serving this repository's own task queue" src="screenshots/board-1.png" width="560" /></a></div>

## Install

Download a release binary, or build from source:

```bash
make build     # builds the frontend with Bun, then the Go binary
./tq version
```

The binary is self-contained: the web UI is embedded with `go:embed`, and Bun is
only needed at build time.

## Quick start

```bash
cd your-project
tq add "Implement REST API" --priority high --label backend
tq add "Build Kanban board" --label frontend --depends-on TQ-0001
tq ready                                       # what can be picked up right now
tq move TQ-0001 in-progress
tq note TQ-0001 "CRUD endpoints implemented; tests remain."
tq done TQ-0001
tq serve                                       # board on http://127.0.0.1:7331
```

There is no setup step: the first command that needs `.tasks/` creates it (at
the root of the enclosing Git repository, so running `tq` from a subdirectory
does not scatter task directories around the tree) and says so on stderr.

`tq init` is still worth running once, and is harmless to repeat: besides
creating the directory it writes `.tasks/AGENTS.md` — a short CLI cheat sheet
for coding agents, generated from the statuses, priorities and exit codes the
binary actually implements — and adds a pointer to it from the repository's
`AGENTS.md`/`CLAUDE.md`:

```md
## Task management

See [AGENTS.md](.tasks/AGENTS.md)
```

Existing files are only appended to, an existing pointer is refreshed when the
task directory moves, and nothing is rewritten when it is already correct. If
the repository has neither file, `AGENTS.md` is created.

Commit `.tasks/` with your code — task history lives in the same repository as
the work it describes.

## The human workflow

```bash
tq serve                # http://127.0.0.1:7331 (localhost only, no auth)
```

- Four columns: `backlog`, `todo`, `in-progress`, `done`.
- Drag a card between columns to change its status.
- "+ Add a card" at the bottom of a column files a task straight into it:
  type a title and press Enter (the composer stays open for the next one) or
  click away. An empty card is discarded; Escape cancels.
- Click a card to edit title, status, priority, assignee, labels, dependencies
  and the Markdown body.
- Notes are not part of the body editor: they get their own list in the dialog,
  each with a pencil to correct it, and cards show a 💬 count. They are still
  stored as the trailing `## Notes` section of the Markdown file that `tq note`
  writes, so a `## Notes` heading in the body itself stays part of the body.
- "New task" creates a task; the filter bar narrows the board by status,
  priority, assignee, label, or "ready only".
- Cards show a blocked marker while a dependency is unfinished or missing.
- The board polls every 3 seconds, so tasks an agent creates or moves appear on
  their own.

## The agent workflow

Every query command speaks JSON, and in `--json` mode **stdout contains only
JSON** — diagnostics always go to stderr.

```bash
tq ready --json                       # pick work that is unblocked and unclaimed
tq show TQ-0001 --json                # read the full task, body included
tq move TQ-0001 in-progress           # claim it
tq note TQ-0001 "Login handler implemented."
tq done TQ-0001                       # finish it
```

A task is **ready** when it is neither `done` nor `in-progress`, and every task
in `depends_on` exists and is `done`. A missing dependency blocks a task rather
than silently making it ready, so a typo shows up as blocked work.

`tq` finds `.tasks/` by walking up from the current directory, so agents can run
it from any subdirectory, and creates it at the repository root when it does not
exist yet. `TQ_DIR=/path/to/.tasks` overrides both the search and where the
directory is created.

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | success |
| `1` | general or validation error |
| `2` | task not found |
| `3` | the `.tasks` directory is missing and could not be created |

## CLI reference

```text
tq init                            Create .tasks/ and refresh the agent
                                   instructions (optional: every command
                                   creates the directory on demand)
tq add <title> [flags]             --priority --assignee --label --depends-on --body --status --json
tq list [flags]                    --status --priority --label --assignee --json
tq show <id> [--json]              Frontmatter fields followed by the Markdown body
tq move <id> <status>              backlog | todo | in-progress | done
tq done <id>                       Shorthand for: tq move <id> done
tq update <id> [flags]             --title --status --priority --assignee
                                   --add-label --remove-label
                                   --add-dependency --remove-dependency
tq note <id> <text>                Append a timestamped bullet to the notes
tq ready [flags]                   --priority --label --assignee --json
tq serve [flags]                   --host --port
tq version
```

`--label` and `--depends-on` (and the `--add-*`/`--remove-*` flags) can be
repeated. Only the fields you pass to `tq update` change.

Arguments that start with `-` must follow `--`, as usual:

```bash
tq note TQ-0001 -- "-1 test still failing"
```

## Task format

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

---

## Notes

- 2026-08-25T09:42:00+02:00 — Initial investigation completed.
```

- Statuses: `backlog`, `todo`, `in-progress`, `done`.
- Priorities: `urgent`, `high`, `normal` (default), `low` — highest first, which
  is also the order `tq list` sorts by.
- Notes are the last section of the body and are introduced by a horizontal
  rule, so a `## Notes` heading in the body itself is content like any other:
  only a `## Notes` that ends the document is read as notes. The blank line
  above the `---` is required — text directly above it would make a heading
  instead of a rule. Files written before the rule existed are still read
  correctly and gain the rule the next time `tq note` touches them.
- Files are named `<id>-<title-slug>.md` (`TQ-0001-implement-oidc-authentication.md`),
  so the directory is browsable and greppable by name. The ID comes first, so
  files still sort and glob by ID.
- The `id` in the frontmatter is authoritative: tasks are looked up by ID
  whatever the suffix says, files written before this naming existed keep
  working, and retitling a task renames its file on the next write (Git records
  it as a rename).
- Writes are atomic (write to a temporary file, then rename), so a crash never
  leaves a half-written task behind.
- Frontmatter is strict: unknown fields are rejected rather than silently
  dropped on the next write.

## REST API

The server and the CLI call the same store, so there is no behavioural
difference between an agent moving a task and a human dragging a card.

| Method | Path | Notes |
| --- | --- | --- |
| `GET` | `/api/tasks` | filters: `status`, `priority`, `label`, `assignee`, `ready=true` |
| `POST` | `/api/tasks` | `{"title", "status", "priority", "assignee", "labels", "depends_on", "body"}` |
| `GET` | `/api/tasks/{id}` | one task |
| `PATCH` | `/api/tasks/{id}` | partial update; the drag-and-drop endpoint |
| `POST` | `/api/tasks/{id}/notes` | `{"text": "…"}` |
| `GET` | `/api/status` | `{"ok", "task_count", "task_dir", "version"}` |
| `GET` | `/api/version` | `{"version"}` |

Errors have a stable shape:

```json
{ "error": "task not found: TQ-9999", "code": "task_not_found" }
```

with `400` for malformed or invalid requests (including a malformed task ID or
an unparsable `ready` value), `404` for a well-formed ID that has no task, and
`500` for filesystem or unreadable-task-file problems.

One unreadable task file fails the whole listing rather than hiding a task: the
board shows the error with the offending filename so it can be fixed.

`tq serve` binds to `127.0.0.1:7331` by default; `TQ_HOST` and `TQ_PORT` change
the defaults and the flags win over both. There is no authentication, so think
before binding to `0.0.0.0`.

## Development

```bash
make help        # list targets
make dev         # Bun watch + Go server with DEV=1 (frontend served from ./public)
make test        # go test ./...
make lint        # golangci-lint
make format      # go fmt + go mod tidy
make frontend    # rebuild public/ with Bun
make build       # frontend + self-contained binary
```

Layout — a single `package main` at the repository root, as in
[terraria-companion](https://git.nakama.town/fmartingr/terraria-companion):

```text
main.go          CLI entry point
cli.go           Commands, flags, table/JSON output, exit codes
server.go        net/http server, REST API, static assets, tq serve
store.go         Filesystem store: discovery, atomic writes, ID allocation
task.go          Task model, validation, filtering, dependencies, notes
frontmatter.go   YAML frontmatter parsing and rendering
embed.go         go:embed of public/
frontend/        TypeScript, CSS and HTML sources (Bun builds them)
public/          Built frontend — committed, embedded into the binary
```

`public/` is generated by `make frontend` and is committed so that `go build`,
`go test` and releases work without Bun (it is embedded, so the package does not
compile while it is missing). `DEV=1` serves `./public` from disk instead of the embedded
copy, so Bun rebuilds show up on reload.

The frontend is vanilla TypeScript with no framework, no bundler beyond Bun, and
no runtime dependencies.

## Known limitations (deliberate, for now)

- Two processes creating a task at the same instant can race for the same ID.
- Simultaneous edits are last-writer-wins; writes are atomic but not locked.
- No authentication; the server binds to localhost.
- Every request scans the task directory. Fine at PoC scale.
- The four statuses are fixed and the board is one project per server.
- Task bodies are edited as plain Markdown; nothing is rendered.
- The board polls instead of receiving pushes.
