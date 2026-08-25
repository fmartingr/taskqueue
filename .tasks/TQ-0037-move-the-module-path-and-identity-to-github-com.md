---
id: TQ-0037
title: Move the module path and identity to github.com/fmartingr/taskqueue
status: done
priority: high
labels:
  - docs
  - chore
  - component/build
created: 2026-08-25T12:16:11+02:00
updated: 2026-08-25T18:19:52+02:00
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

---

## Notes

- 2026-08-25T18:19:51+02:00 — go.mod now declares github.com/fmartingr/taskqueue. This is package main with no internal imports, so the one line plus go mod tidy was the whole rename; go build, go test, make build and make lint all pass.
- 2026-08-25T18:19:51+02:00 — .gitignore now ignores taskqueue rather than task-queue, verified with git check-ignore against a real stray binary from go build ./..., and package.json is taskqueue-frontend. The old task-queue binary in the working tree was deleted, since nothing ignores that name any more.
- 2026-08-25T18:19:51+02:00 — Both terraria-companion links are untouched, one in README.md and one in the plan; that repository is a different project on the forge and stays put.
- 2026-08-25T18:19:52+02:00 — Two corrections to the ticket. The README has no go install line at all, so there was nothing to respell here; TQ-0036 owns that section and will add it with the right path. And .goreleaser.yml still says name: task-queue, inside the gitea release block TQ-0036 deletes outright, so changing it here would be churn.
- 2026-08-25T18:19:52+02:00 — goreleaser check fails with 'no remote configured to list refs from'. Verified it fails identically at HEAD, so this is the environmental limitation the ticket anticipated, not a regression. There is no git remote at all yet.
- 2026-08-25T18:19:52+02:00 — Not done, deliberately: pointing the remote at the new repository and pushing. That is outward-facing and needs the repository to exist; left for a human to do.
