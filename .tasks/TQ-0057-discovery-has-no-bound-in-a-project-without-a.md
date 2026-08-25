---
id: TQ-0057
title: Discovery has no bound in a project without a repository root
status: todo
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T17:44:25+02:00
updated: 2026-08-25T17:44:25+02:00
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
