---
id: TQ-0029
title: Project config file at .taskqueue.yaml
status: todo
priority: normal
labels:
  - component/config
  - feature
created: 2026-08-25T11:53:19+02:00
updated: 2026-08-25T18:37:59+02:00
---

## Proposal

A project configuration file at **`.taskqueue.yaml`, in the root of the
repository** — not inside the task directory. This ticket is the file, its
loader and its contract; the label set, severities and columns land on top of it
in their own tickets.

```yaml
# .taskqueue.yaml
version: 1
path: .tasks
```

Putting the marker at the root, rather than inside `.tasks/`, is what makes
discovery unambiguous: there is one file to find, it says where the tasks live,
and the search has a definite stopping point instead of guessing at directory
names on the way up.

## Contract

- `version` is an integer, starts at `1`, and only changes on a **breaking**
  change. Everything else is additive: a new key must be readable by an older
  binary, which ignores what it does not know.
- A config declaring a version newer than the binary supports is a clear error
  ("config version 2 needs a newer tq"), not a silent partial read.
- `path` is where the task files live, **resolved relative to the directory
  holding the config file**, never relative to the working directory. Default
  `.tasks`. An absolute path works but makes the committed file non-portable,
  which is worth saying in the docs.
- The file is **optional**. Without it the built-in defaults apply, so `tq add`
  in a fresh repository keeps working with no setup step.
- `tq init` writes it with only the attributes a project actually needs —
  `version` and `path` — not a dump of every default. Labels, severities and
  columns are added by the user when they want to diverge from the defaults, and
  a key that is absent means "use the default", not "empty".
- `tq init` never overwrites an existing config: unlike `.tasks/AGENTS.md`, this
  file is user-owned.
- One loader in `config.go`, shared by the CLI and the HTTP server, read from
  disk per command and per request the way tasks are. No cache, no watcher.
- `GET /api/config` exposes it to the board.

## Discovery, in order

1. `TQ_DIR`, when set, is the task directory, full stop. The config is still
   located by the walk below, so labels and columns keep working, but `path` is
   ignored.
2. The nearest `.taskqueue.yaml` at or above the working directory. Its `path`
   resolves the task directory. **The walk stops at the first one found**, which
   is what fixes the traversal ambiguity in TQ-0017.
3. No config, but an existing `.tasks/` directory at or above the working
   directory: keep working exactly as today, since repositories already exist in
   that shape — this one included. Suggest `tq init` once, do not force it.
4. Nothing at all: create the queue, as today.

Decide: when a command other than `tq init` has to create the queue, does it
write `.taskqueue.yaml` too, or only the directory? Recommendation: write both.
A queue without its marker is precisely the ambiguity this ticket removes, and
the file is two lines.

## Notes on the file itself

- YAML gotcha for the labels ticket: `color: #ff0000` is a **comment**, so that
  value parses as null. Hex colours must be quoted, and the loader should reject
  an empty colour rather than render it.
- Accept exactly `.taskqueue.yaml`. If `.taskqueue.yml` exists and the canonical
  name does not, fail with a message naming the expected file rather than
  silently ignoring it.
- The file sits at the repository root, so it is never mistaken for a task, and
  `Store.List` needs no new exclusion.
- Decide: tolerate unknown keys silently (best for the additive forward
  compatibility this contract promises) or warn once on stderr. Task frontmatter
  is strict to avoid silent data loss on rewrite, but tq never rewrites this
  file, so tolerating is defensible.

## Acceptance criteria

- No config: every command behaves exactly as it does today, including creating
  the task directory on demand.
- `tq init` writes `.taskqueue.yaml` at the repository root containing `version`
  and `path`, and leaves an existing file untouched.
- `path` moves the task directory, resolved against the config's own directory,
  and `tq` finds it from any subdirectory.
- `TQ_DIR` still wins over `path`.
- A malformed config names the file and the problem; a future version says to
  upgrade; neither is reported as a task error.
- CLI and server share one loader, and `GET /api/config` returns it as JSON.
- README and the generated `.tasks/AGENTS.md` describe the file and its
  location.
