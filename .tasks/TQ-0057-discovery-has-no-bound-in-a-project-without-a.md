---
id: TQ-0057
title: Discovery has no bound in a project without a repository root
status: done
priority: high
labels:
  - bug
  - component/store
  - wontfix
created: 2026-08-25T17:44:25+02:00
updated: 2026-08-25T17:47:31+02:00
---

## Finding

TQ-0017 bounded discovery at the repository root. A project with no repository
root therefore has no bound at all: `DiscoverTaskDir` leaves `stopAt` empty and
walks to the top of the disk. Every command is affected, not just `tq init`.

## How it fails

Reproduced: a task directory in the home folder, and a project below it with no
`.git`. From that project, `tq init` reports the home task directory and makes
none of its own. `tq add` files into it as well.

TQ-0056 removed the damaging half of this — `tq init` no longer writes a guide
into a directory outside its own tree — but the adoption itself remains.

## Suggested fix

Option B of the three considered when TQ-0056 was chosen: recognise other
project markers as roots, `go.mod`, `package.json`, `.hg` and similar, so a
project without Git still gets a boundary.

The alternative, refusing to walk at all without a repository root, was
rejected: it would undo TQ-0047, whose only remaining effect is in projects
that are not Git repositories.

Found by `/code-review` over 20b06d2; reproduced by hand.

---

## Notes

- 2026-08-25T17:47:31+02:00 — Rejected as a scope decision, not as unreproducible: the unbounded walk is still there, verified on the current build.
- 2026-08-25T17:47:31+02:00 — The damaging half is already gone. TQ-0056 stopped tq init writing a guide outside the tree it was invoked in, which was the only case that touched another project's committed files.
- 2026-08-25T17:47:31+02:00 — What remains is consistent behaviour, not a defect: tq add, tq list and tq init all walk up the same way and agree on the same task directory. A queue above your project capturing it is the documented walk-up working as described.
- 2026-08-25T17:47:31+02:00 — Option B was the fix on offer and is the reason for rejecting: treating go.mod, package.json and similar as roots is guesswork about what a project is. It would make discovery depend on which marker files happen to exist, and would surprise people whose layouts do not match the list. .git is a boundary tq can be certain about.
- 2026-08-25T17:47:31+02:00 — Reopen if the walk is shown to reach a task directory a user did not intend in a layout that has no .git and no reasonable marker.
