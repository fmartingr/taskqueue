---
id: TQ-0011
title: One unreadable task file hides every other task
status: done
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-26T21:46:31+02:00
---

## Finding

Store.List aborts the whole scan on the first unreadable file (`return nil, err` inside the per-entry loop), and ParseTask rejects any extra frontmatter key (frontmatter.go:43 `dec.KnownFields(true)`) or a UTF-8 BOM, so one hand-edited or merge-conflicted file hides every other task on both surfaces.

Source: `store.go:245`

## How it fails

Reproduced with two healthy tasks plus one file carrying an extra `epic: platform` key: `tq list` -> exit 1 with no tasks printed, `tq ready` -> exit 1, `GET /api/tasks` -> HTTP 500, `GET /api/status` -> 500 (the board loses even its task_dir line), while `tq show TQ-0001` still works. Git conflict markers and a BOM (strings.TrimSpace does not strip U+FEFF, so the error misleadingly says the file does not start with `---`) produce the same outage. Two agents each `tq add` on their own branch is the ordinary path to a conflicted .tasks/ — a directory the generated guide tells users to commit. Rejecting unknown keys is defensible; taking the whole queue down instead of skipping one file is not.

## Suggested fix

Decide the policy deliberately: either skip unreadable files and report them alongside the good ones, or keep failing fast (today's behaviour, chosen so a broken file cannot be missed). `describeDeps` already degrades gracefully, so the two paths disagree.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-26T21:35:57+02:00 — Skip and report, as decided.

  Store.List now returns a Listing{Tasks, Unreadable} rather than ([]Task, error) for a broken file: the error return is kept for what makes the whole directory unreadable (the directory itself, or the config the order depends on). Every caller has to look at Unreadable to compile, which is the point.

  CLI: c.tasks() warns on stderr and returns the healthy tasks, so list, ready, show's dependency resolution and label list all skip-and-report; exit stays 0 and --json stdout stays pure JSON.
  HTTP: /api/tasks is 200 and still a plain array; /api/status gained "unreadable": [{file, reason}], always an array.
  Parser: a UTF-8 BOM is stripped, so a marked file parses instead of being blamed for not starting with ---. dec.KnownFields(true) untouched.
  Board: /api/status is refetched whenever a tasks signal is answered (one extra request per change, not per tick); a new skipped file raises a toast naming it, and the footer carries a persistent count for as long as it lasts.

  TQ-0012's TOCTOU is incidentally survivable now: a file renamed between ReadDir and readFile comes back as ErrTaskNotFound, which is a skip rather than a failed listing. Its own work (not reporting a rename as a broken file) is untouched.
- 2026-08-26T21:46:14+02:00 — Code review at high effort, findings addressed in the diff.

  - List no longer reports a file that is simply gone: a readFile that comes back ErrTaskNotFound is a concurrent rename or delete (update writes the new name before retiring the old one), and reporting it would put a red toast on the board for an ordinary retitle. A dangling symlink pins the behaviour. Whether the task itself is missed by that scan is still TQ-0012's.
  - loadServerStatus took a ticket guard like refresh's: two change signals half a second apart put two status requests in flight, and the older one landing last would leave the footer complaining about a file already fixed.
  - The board names at most three broken files in toasts and summarises the rest, since a conflicted merge breaks several at once.
  - README: the 500 paragraph now says which endpoints an unreadable file still fails (the by-ID ones), and the exit-code section says list and ready exit 0 with a partial listing.

  Considered and left alone: /api/status parsing the directory on every change signal. Serving the skipped files from the event hub's scan would mean the hub holding data, which AGENTS.md rules out — it fingerprints and holds nothing. The cost is one extra read-through request per change, not per tick.
