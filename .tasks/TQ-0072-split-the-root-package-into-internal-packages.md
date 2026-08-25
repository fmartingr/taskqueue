---
id: TQ-0072
title: Split the root package into internal packages
status: done
priority: normal
labels:
  - refactor
  - component/build
  - component/cli
depends_on:
  - TQ-0029
created: 2026-08-25T18:55:16+02:00
updated: 2026-08-25T19:52:32+02:00
---

## Why now

The flat layout was a deliberate PoC choice (plan section 5) and it earned its
keep. It no longer does: the root package is 16 files, ~2,600 lines of code and
~3,400 lines of tests, with `store.go` at 627 lines, `cli.go` at 637 and
`store_test.go` at 986. Everything is mutually visible, so nothing enforces the
one boundary that matters — pure domain versus filesystem versus HTTP versus
CLI — and the test files share unexported helpers across responsibilities.

The architecture already moved once, when `cmd/tq` appeared for the module path
(TQ-0037). This finishes that thought rather than starting a new one.

## Proposed layout

```text
cmd/tq/                 main, already there
internal/task/          task.go, frontmatter.go   — model, validation, filters,
                        dependencies, notes, slugs, parse/render. No I/O.
internal/config/        config.go                 — .taskqueue.yaml loader
internal/store/         store.go                  — discovery, atomic writes,
                        ID allocation. Depends on task and config.
internal/guide/         agents.go                 — the generated .tasks/AGENTS.md
internal/web/           server.go + public/       — REST API and the frontend
internal/cli/           cli.go                    — commands, flags, exit codes
tq.go                   Main() and the version var, or move both (see below)
```

Dependency direction, leaf first: `task` <- `config` <- `store` <- `guide`,
`web`, `cli`. Nothing points back up.

## The parts that will actually hurt

- **`go:embed` cannot reach outside its own package directory.** If `server.go`
  moves to `internal/web`, `public/` has to move with it, and that ripples into
  `frontend/build.ts` (`OUTPUT_DIR`), the Makefile, the `DEV=1` disk-serving
  path, and CI's "public/ must match the committed output" gate. Alternative:
  leave an embed shim at the root, which keeps a file at the root purely to
  satisfy a compiler rule. Moving `public/` is cleaner; the ripple is the price.
- **The tests are white-box and share helpers.** They reach `runCLI`,
  `newAPIRouter`, `s.write`, `taskFileID`, and they share `newTestStore`,
  `newTestCLI`, `testRoot` and `isolate()` across files that would land in
  different packages. Each test file moves with its code, and the shared
  fixtures need a home — an `internal/tqtest` package, or duplication, which is
  worse. This is the single biggest hidden cost in the refactor.
- **Cross-cutting tests stop compiling.** `TestHTTPAndCLIProduceTheSameFile`
  spans the CLI and the HTTP API in one package today. After the split it is an
  integration test, which is why TQ-0038 should land first: it gives the whole
  refactor an end-to-end safety net that does not care how the packages are
  arranged.
- **The version variable is wired by path.** ldflags set
  `-X github.com/fmartingr/taskqueue.version`. If `version` moves, the Makefile,
  `.goreleaser.yml` and CI all change together. Decide: keep it in the root
  package, or move it to `cmd/tq` and use goreleaser's default `main.version`,
  which is one less bespoke path.

## Decisions

- **`internal/`, decided.** Everything except `cmd/tq` lives under `internal/`,
  so the module commits to no Go API at all: the stable interface is the CLI and
  its JSON contract, which is what agents and scripts actually depend on. A
  package can be promoted out of `internal/` the day a real consumer appears —
  the reverse, taking a published type back, cannot be done without breaking
  someone.
- **Still open: is `frontmatter.go` part of `task`?** It is the Task's own encoding, so
  probably yes, rather than a separate `markdown` package that exists to hold
  two functions.

## How to do it without breaking anything

- Pure move: `git mv`, package clause, imports. **No logic changes in the same
  commit**, so review is mechanical and a bisect lands somewhere useful.
- One package per commit, suite green at every step.
- Behaviour is unchanged by definition: same binary, same CLI, same JSON, same
  files on disk. If something has to change to compile, that is a separate
  commit with its own reasoning.

## Sequencing

TQ-0029 is in progress and is actively writing `config.go`, so this must not
start before it lands. TQ-0038 (integration boilerplate) is strongly
recommended first, for the reason above. Everything else in the backlog is
easier to rebase onto a refactor than the other way around, so the sooner after
those two, the better.

## Acceptance criteria

- The layout above (or the agreed variant), with each package's tests beside it
  and shared fixtures in one place.
- `go vet ./...` and golangci-lint clean, no import cycles, `task` importing
  nothing of ours.
- `make build`, `make dev`, `make test`, a goreleaser snapshot, embedded serving
  and `DEV=1` disk serving all behave exactly as before.
- The frontend build writes wherever `public/` now lives, and CI's staleness
  gate points at the same place.
- The ldflags decision applied consistently across Makefile, goreleaser and CI.
- AGENTS.md's layout rule and plan section 5 rewritten to describe the new
  shape, including why the flat layout was right until it was not.

---

## Notes

- 2026-08-25T18:55:30+02:00 — TQ-0038 (integration boilerplate) is not a hard dependency but is the safety net this refactor wants: it is the only test layer that survives the packages being rearranged. Doing it first turns a large mechanical change from 'trust the unit tests that are themselves moving' into 'the binary still behaves'.
- 2026-08-25T18:58:32+02:00 — Decided: internal/. No package outside cmd/tq is importable, so the module makes no Go API promise; promoting a package out of internal/ later stays possible, taking a published one back does not.
- 2026-08-25T19:52:32+02:00 — Done in six commits, one package each, suite green at every step: task, config with the shared fixtures, store, guide, web, cli. fsx was extracted first because two packages heading to different homes both needed the atomic write.
- 2026-08-25T19:52:32+02:00 — Import direction verified with go list, not by eye: internal/task depends on nothing of ours, and each package depends only on those below it — config on fsx, store on config and task, guide and web on store, cli on all of them.
- 2026-08-25T19:52:32+02:00 — The predicted costs all landed. public/ moved with the package that embeds it, which took build.ts, the Makefile and CI's staleness gate with it, and split the embed path from the DEV disk path since they can no longer be one string. Shared fixtures went to internal/tqtest. The cross-cutting test moved to the root and now drives Main and the router through exported API only.
- 2026-08-25T19:52:32+02:00 — Two decisions the ticket left open. The version variable stays in the root package and is passed into web and cli as a value, so ldflags, the Makefile, goreleaser and CI were untouched. frontmatter.go went into task, as its own encoding rather than a package holding two functions.
- 2026-08-25T19:52:32+02:00 — Two things had to change to compile rather than merely move, both called out in their commits: the serve command was a cli method living in server.go and moved to cli.go, and the store and cli packages needed exported entry points, Run and NewRouter, for callers across the boundary.
- 2026-08-25T19:52:32+02:00 — store keeps its own test fixtures instead of using tqtest. Its tests reach its internals, so importing a helper package that builds one of its stores would be an import cycle. config's tests went external for the same reason.
- 2026-08-25T19:52:32+02:00 — TQ-0038 was not done first, as this ticket recommended. Compensated by exercising the built binary after every package: CLI commands, embedded serving, DEV=1 disk serving, a goreleaser snapshot and the frontend build.
