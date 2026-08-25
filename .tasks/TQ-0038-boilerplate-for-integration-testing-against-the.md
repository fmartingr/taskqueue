---
id: TQ-0038
title: Boilerplate for integration testing against the real binary
status: todo
priority: normal
labels:
  - tests
  - component/cli
  - component/ci
created: 2026-08-25T12:25:12+02:00
updated: 2026-08-25T18:42:55+02:00
---

## Why

Every test today runs *inside* the process: `cli_test.go` calls `runCLI` with
injected writers, `server_test.go` mounts `newAPIRouter` under `httptest`. That
covers the logic well and the binary not at all. Nothing ever executes the
compiled `tq`, so nothing checks the parts users and agents actually touch:

- real exit codes through `os.Exit`, and `main()` itself, which has 0% coverage;
- the stdout/stderr split that the `--json` contract depends on;
- `runServe` (14.7% covered): the listener, the banner, DEV versus embedded
  asset serving, request logging, graceful shutdown;
- the core product claim — a CLI write showing up in a *running* server, and a
  board write showing up in the next CLI read. `TestHTTPAndCLIProduceTheSameFile`
  compares two files written by two stores; it never has both alive at once.

Several already-filed bugs are exactly what this layer catches: the `--`
terminator (TQ-0018), stdout purity, and the exit-code contract.

## Scope

The boilerplate, plus enough tests to prove it works. Not exhaustive coverage —
that arrives ticket by ticket afterwards.

## Shape

`//go:build integration`, so `go test ./...` stays fast and unchanged, with
`make test-integration` and a CI job running them.

```go
//go:build integration

func TestMain(m *testing.M)  // build ./tq once, reuse it for every test

type project struct{ dir string }  // temp git repo containing .tasks
func newProject(t *testing.T) *project

// Runs the real binary in the project. Exit code, stdout and stderr stay
// separate, which is the point.
func (p *project) run(t *testing.T, args ...string) result
type result struct {
    Code           int
    Stdout, Stderr string
}
func (r result) JSON(t *testing.T, target any)

// Starts `tq serve`, waits until /api/status answers, returns its base URL,
// and kills it on cleanup.
func (p *project) serve(t *testing.T, env ...string) *server
```

Requirements for the harness itself:

- Build the binary once per run, not per test.
- **Neutralise the environment explicitly**: clear `TQ_DIR`, `TQ_HOST`,
  `TQ_PORT` and `DEV` per test, and give each project its own `.git` marker so
  directory discovery cannot climb out of the temp directory. TQ-0021 and
  TQ-0023 are the same mistake made in the unit tests; do not repeat it here.
- No leaked processes: `t.Cleanup` kills the server and waits for it, and a
  readiness failure prints the server's stderr rather than a bare timeout.
- Parallel-safe: no fixed ports, no shared state, no `t.Setenv` on a process
  that other tests share.

## One small change to the server this needs

`tq serve --port 0` should let the OS pick a free port, and the banner should
print the address the listener actually got (`listener.Addr()`), not the
requested one — today `server.go:306` prints the requested `addr`, so port 0
would print `127.0.0.1:0`. That removes port races from the harness, and it is
more truthful for humans too.

## First tests, to prove the harness

- The documented session end to end: `init`, `add`, `ready`, `show`, `move`,
  `note`, `done`, asserting exit codes and output at each step.
- The exit-code contract from the real binary: 0, 1 (validation), 2 (unknown
  task), 3 (uncreatable task directory).
- `--json` purity: stdout parses as JSON on its own, while notes and warnings
  come out on stderr.
- Cross-surface: with a server running, `tq add` from the shell appears in
  `GET /api/tasks` on the next request, and a `PATCH` appears in the next
  `tq show`.
- The real binary serves the embedded frontend with `DEV` unset, and serves
  `./public` from disk with `DEV=1`.
- SIGTERM shuts the server down without leaving a temp file behind in `.tasks`.

## Frontend: decide separately

The plan deliberately has no browser stack, and the repository has no
JavaScript dependencies at all. A browser layer would mean Playwright and a
`node_modules`, which is a real architecture change and belongs in its own
ticket if it is wanted. Bun's built-in test runner could cover the pure
functions in `app.ts` (`splitBody`, `joinBody`, `Slugify`-adjacent helpers)
with no new dependency — worth doing, also separately. This ticket stays Go.

## Acceptance criteria

- `make test` is unchanged in scope and speed; `make test-integration` builds
  the binary and runs the tagged tests.
- The harness offers `run`, `serve` and a project fixture, with environment and
  discovery neutralised, and no leaked processes or ports.
- `tq serve --port 0` works and the banner prints the real address.
- The first tests listed above pass, and fail informatively when the binary
  misbehaves (check by breaking something on purpose).
- A CI job runs them. If TQ-0036 has not landed yet, add the job in whichever
  workflow format is current and move it with the rest.

---

## Notes

- 2026-08-25T18:42:55+02:00 — The harness plants a .git marker to bound discovery. After TQ-0029 it should plant .taskqueue.yaml instead — closer to what a real project has, and independent of Git.
