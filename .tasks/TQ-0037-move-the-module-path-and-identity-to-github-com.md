---
id: TQ-0037
title: Move the module path and identity to github.com/fmartingr/taskqueue
status: todo
priority: high
labels:
  - build
  - docs
created: 2026-08-25T12:16:11+02:00
updated: 2026-08-25T12:16:11+02:00
---

## Why

The project's home becomes github.com/fmartingr/**taskqueue** — note the name
has no hyphen, while the module path, the local directory and a few identity
strings all say `task-queue`. Everything that names the project has to move
before the workflows and the release can be rewritten, which is why this comes
before TQ-0036.

## What moves

- `go.mod:1` — `module git.nakama.town/fmartingr/task-queue` becomes
  `module github.com/fmartingr/taskqueue`. This is `package main` with no
  internal imports, so nothing to rewrite beyond the one line, plus
  `go mod tidy`.
- **Side effect worth knowing:** `go build ./...` names its output after the
  last element of the module path, so the stray binary becomes `taskqueue`.
  `.gitignore:3` currently ignores `task-queue`, and would stop catching it.
- `README.md:26` — the install line reads
  `go install github.com/fmartingr/task-queue@latest`. Wrong name, and it fails
  regardless until `go.mod` matches the repository.
- `package.json:2` — `"name": "task-queue-frontend"`; cosmetic, but it is an
  identity string.

## What must not move

`README.md:242` and `task-queue-poc-implementation-plan.md:25` both link to
`https://git.nakama.town/fmartingr/terraria-companion`, the reference project
whose architecture this one deliberately follows. That is a **different
repository** on the forge and stays exactly as it is. A blind search-and-replace
of `git.nakama.town` breaks both links, which is the main trap in this ticket.

The forge paths quoted inside `.tasks/TQ-0036-*.md` are descriptions of what
needs changing, not live references. Leave them.

## Boundary with TQ-0036

This ticket moves identity: module path, `.gitignore`, `package.json`, and the
install line's path. TQ-0036 owns the CI and release plumbing — runner labels,
the container image, action versions, `gitea_urls`, the release target — and
restructuring the README's Install section so `go install` is the headline
method.

## Also worth doing here

- Point the git remote at the new repository and push, if that has not happened
  yet.
- Decide whether the forge stays as a mirror, and if it does, say in the README
  which host is canonical so contributors and `go install` agree.

## Acceptance criteria

- `go.mod` declares `github.com/fmartingr/taskqueue`, and `go build ./...`,
  `go test ./...`, `make build` and `goreleaser check` all still pass.
- `.gitignore` catches the renamed stray binary.
- No `task-queue` identity string left, apart from the local checkout directory
  and historical text inside tickets.
- Both terraria-companion links are untouched.
- `go install github.com/fmartingr/taskqueue@latest` works once the repository
  is pushed and public.
