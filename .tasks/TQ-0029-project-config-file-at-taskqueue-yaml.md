---
id: TQ-0029
title: Project config file at .taskqueue.yaml
status: done
priority: normal
labels:
  - component/config
  - feature
created: 2026-08-25T11:53:19+02:00
updated: 2026-08-25T19:11:32+02:00
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

---

## Notes

- 2026-08-25T18:42:55+02:00 — Impact audit for the move to .taskqueue.yaml. Superseded or re-scoped by the marker: TQ-0058 (TQ_WALK_FOREVER), TQ-0059 (.git file bound), TQ-0062 (shadowed queue), TQ-0064 (projects without Git). Needs reworking in the same change or straight after: TQ-0060 (init exit codes), TQ-0061 (guide pointer base), TQ-0063 (test isolation pins), TQ-0038 (integration harness plants .git), TQ-0066 and TQ-0054 (init docs), TQ-0044 (guide), TQ-0033 (fingerprint must include the marker). Sequencing: six of those are high while this is normal, and doing them first means hardening heuristics this ticket deletes — this should go first, or they should wait for it.
- 2026-08-25T18:59:23+02:00 — Implemented. config.go holds the marker, its loader and path resolution; discovery and creation both consult it; tq init writes it; GET /api/config exposes it; README and the generated guide describe it.
- 2026-08-25T18:59:23+02:00 — Decision on the open question about other commands creating the queue: they write the marker too, as the ticket recommended. A queue without its marker is the ambiguity the marker removes, so InitStore writes it wherever it creates a directory, not just under tq init.
- 2026-08-25T18:59:24+02:00 — Decision on unknown keys: tolerated silently, no warning. The version field is what carries breaking changes, everything else is additive, and tq never rewrites this file, so ignoring what it does not know loses nothing. A test pins that a file with columns and severities still reads.
- 2026-08-25T18:59:24+02:00 — One case the ticket did not cover: tq init in a directory whose queue was adopted from outside the invoked tree. Writing a marker there would bind that directory to another project's queue for good, so it is skipped under the same ownership rule TQ-0056 applied to the guide, with a note on stderr.
- 2026-08-25T18:59:24+02:00 — A stat failure walking up is not a broken config: a non-directory on the path just means there is none at that level. Permission errors are still reported, because walking past a config tq cannot read would silently use the wrong queue. Two existing uncreatable-directory tests caught this.
- 2026-08-25T18:59:24+02:00 — Verified against each acceptance criterion with the built binary: no config behaves as before and creates on demand; init writes version and path and leaves a hand-written file untouched; path moves the queue and is found from a deep subdirectory; TQ_DIR still wins; future version, malformed YAML and the .yml typo each name the file and exit 1, none as a task error; GET /api/config returns version, path, task_dir and file.
- 2026-08-25T18:59:24+02:00 — This repository has no marker yet, and I did not run tq init here to add one — that is a change to the project's committed files and is the user's call.
- 2026-08-25T19:08:23+02:00 — Traversal corrected after the user restated the intent. My first cut kept a fallback: no marker meant walking up for a directory named .tasks. That is gone. The marker is the only thing tq looks for, the walk stops at a .git, and TQ_WALK_FOREVER lets it run to the filesystem root.
- 2026-08-25T19:08:23+02:00 — That removes discovery step 3 from this ticket's own plan. A bare .tasks with no marker above it is not adopted, because the guessing that would take is exactly what the marker replaces.
- 2026-08-25T19:08:23+02:00 — So that existing projects do not break, InitStore writes the marker for a task directory it merely found as well as one it created. Verified against a copy of this repository, which has 72 tasks and no marker: the first command wrote the marker pointing at .tasks, every task stayed visible, and subdirectories still resolve.
- 2026-08-25T19:08:23+02:00 — Verified the corrected rule end to end: a bare .tasks above a subdirectory is ignored and the repo root gets its own queue and marker; a marker outside the repository is ignored because the walk stops at .git; and TQ_WALK_FOREVER=true reaches that outer marker instead.
- 2026-08-25T19:11:32+02:00 — Removed the migration path I had added for projects predating the marker. Nothing is published, so that situation does not exist: InitStore writes the marker only when it makes a queue, and this repository simply carries a committed .taskqueue.yaml like any other project would.
