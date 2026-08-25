---
id: TQ-0036
title: Migrate CI and releases from Forgejo to GitHub Actions
status: todo
priority: high
labels:
  - ci
  - build
created: 2026-08-25T12:13:06+02:00
updated: 2026-08-25T12:13:36+02:00
---

## Why

The workflows were written for a Forgejo server and its runner. The repository
is moving to github.com/fmartingr/task-queue, where **none of them will run**:
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

## Module path — decide

`go.mod` declares `module git.nakama.town/fmartingr/task-queue`. If GitHub
becomes canonical, `go install github.com/fmartingr/task-queue@latest` only
works when the module path matches the repository. The package is `main` with no
internal imports, so the change is a one-line edit with no import rewrites, but
it does touch `go.sum` regeneration, the README, and anything quoting the path.

Recommendation: switch the module path when GitHub becomes the canonical home,
and keep the forge as a mirror. If both hosts stay canonical, leave it and say
so in the README, because `go install` from the GitHub path will not work.

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
  workflows or in `.goreleaser.yml`, and the module-path decision is recorded in
  the README either way.

## Notes

- 2026-08-25T12:13:36+02:00 — The README in the working tree now advertises 'go install github.com/fmartingr/task-queue@latest', which settles the module-path question: go.mod still says git.nakama.town/fmartingr/task-queue, and that go install command fails until it matches the repository. Treat the module path rename as required work in this ticket, not an option.
