---
id: TQ-0036
title: Migrate CI and releases from Forgejo to GitHub Actions
status: done
priority: high
labels:
  - chore
  - component/ci
depends_on:
  - TQ-0037
created: 2026-08-25T12:13:06+02:00
updated: 2026-08-25T18:28:32+02:00
---

## Why

The workflows were written for a Forgejo server and its runner. The repository
is moving to github.com/fmartingr/taskqueue, where **none of them will run**:
every job requests a runner label and a container image that only exist on that
forge, and the release publishes through the Gitea API.

## What is forge-specific today

`.github/workflows/ci.yml` (six jobs) and `.github/workflows/release.yml`:

- `runs-on: docker` — GitHub has no `docker` runner label. Needs
  `runs-on: ubuntu-latest`.
- `container: git.nakama.town/fmartingr/ci-images/ci-base:1.1.0` — a private
  image on the forge registry that a GitHub runner cannot pull, and does not
  need: the hosted image already has Go, git and the rest. Drop the container
  entirely rather than porting it.
- `actions/checkout@v7` and `actions/setup-go@v7` — those tags exist in the
  Forgejo action mirror, not on GitHub, where checkout is v4/v5 and setup-go is
  v5. Pin to versions that exist.
- `actions/goreleaser-action@v6.4.0` — on GitHub the action lives at
  `goreleaser/goreleaser-action`.
- `oven-sh/setup-bun@v2` in the `frontend` job is fine as is.
- release.yml passes `GORELEASER_FORCE_TOKEN: gitea` and
  `GITEA_TOKEN: ${{ secrets.FORGEJO_TOKEN }}`; on GitHub that becomes
  `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` plus a
  `permissions: contents: write` block on the job, since the default token is
  read-only for releases.

`.goreleaser.yml`:

- The `gitea_urls:` block (lines 5-7) and `release: gitea: {owner, name}` (line
  57) both point at the forge. GoReleaser infers the GitHub repository from the
  remote, so these come out and `prerelease: auto` stays.
- `goreleaser check` must pass afterwards; it needs a git remote to resolve the
  release target, which is worth remembering when running it locally.

## README: make `go install` the default installation method

The Install section still leads with "Download a release binary, or build from
source". With releases published from GitHub and the module path fixed by
TQ-0037, the headline becomes:

```bash
go install github.com/fmartingr/taskqueue@latest
```

with the release archives and `make build` kept below it as the alternatives.
The module path itself, and the wrong `task-queue` spelling currently in that
line, belong to TQ-0037; this ticket owns the shape of the section.

## Worth doing during the move, not after

- Add a least-privilege `permissions:` block per workflow; the repository-wide
  default is broader than these jobs need.
- Let `setup-go` cache modules (it caches by default from v4).
- TQ-0025 (the release publishes without running tests or checking that
  `public/` is fresh) and TQ-0026 (the staleness gate misses untracked files)
  both touch these same files. Fix them here or immediately after, but do not
  let the rewrite quietly re-introduce them.
- Optional hardening: pin third-party actions by commit SHA, and add a
  Dependabot config for `github-actions` so the pins get updated.

## Acceptance criteria

- Pushes to main and pull requests run format, goreleaser-check, lint, test,
  build and frontend on `ubuntu-latest` with no custom container, and pass.
- A `v*` tag publishes a GitHub Release with the four archives and
  `checksums.txt`, authenticated with `GITHUB_TOKEN`.
- `goreleaser check` passes against the GitHub configuration.
- No `git.nakama.town`, `gitea` or `FORGEJO_TOKEN` reference remains in the
  workflows or in `.goreleaser.yml`.
- The README's Install section leads with
  `go install github.com/fmartingr/taskqueue@latest`, with release binaries and
  `make build` as the alternatives below it.

---

## Notes

- 2026-08-25T12:13:36+02:00 — The README in the working tree now advertises 'go install github.com/fmartingr/task-queue@latest', which settles the module-path question: go.mod still says git.nakama.town/fmartingr/task-queue, and that go install command fails until it matches the repository. Treat the module path rename as required work in this ticket, not an option.
- 2026-08-25T12:16:50+02:00 — Correction to the note above: the repository is github.com/fmartingr/taskqueue, no hyphen. The path quoted there is what the README currently says, and it is wrong on both counts — TQ-0037 owns fixing it.
- 2026-08-25T18:28:32+02:00 — Workflows moved to ubuntu-latest with no container, checkout at v4 and setup-go at v5, goreleaser/goreleaser-action@v6, and a contents: read permissions block per workflow with contents: write only on the release job.
- 2026-08-25T18:28:32+02:00 — goreleaser config lost gitea_urls and the release: gitea block; the GitHub repository is inferred from the remote. goreleaser check passes, verified by adding the github remote temporarily and removing it again, since there is still no remote configured here.
- 2026-08-25T18:28:32+02:00 — Fixed rather than re-introduced, as this ticket asked: TQ-0025 now has a verify job the release needs, and TQ-0026's gate is git status --porcelain -- public so untracked build output is caught. Both closed.
- 2026-08-25T18:28:32+02:00 — Install section leads with go install, but the path is github.com/fmartingr/taskqueue/cmd/tq@latest rather than the module root. go install names the binary after the last element of the package path, so the module root produced a binary called taskqueue while every command in the docs is tq. Verified both ways with GOBIN.
- 2026-08-25T18:28:32+02:00 — That required an architecture change the user approved: a thin package main in cmd/tq, with the root becoming package taskqueue. public/ and its go:embed stay at the root, because an embed cannot reach outside its own package directory. AGENTS.md's no-cmd rule is rewritten to allow exactly this one subpackage and say why.
- 2026-08-25T18:28:32+02:00 — Knock-on fixes: ldflags now set github.com/fmartingr/taskqueue.version, make build and goreleaser build ./cmd/tq, and go run . became go run ./cmd/tq in the Makefile and the plan. goreleaser snapshot produces a binary named tq, and GOBIN=... go install ./cmd/tq installs tq.
- 2026-08-25T18:28:32+02:00 — Not done, deliberately: pinning actions by commit SHA and adding a Dependabot config, both listed as optional hardening. SHAs cannot be resolved without network access here, and the two belong together.
